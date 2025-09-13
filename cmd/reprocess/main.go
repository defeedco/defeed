package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/alitto/pond/v2"
	"github.com/defeedco/defeed/pkg/sources/activities"
	"os"
	"strings"
	"sync/atomic"

	appconfig "github.com/defeedco/defeed/pkg/config"
	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/lib/log"
	"github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/nlp"
	"github.com/defeedco/defeed/pkg/storage/postgres"
	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/llms/openai"
)

type Config struct {
	SourceUIDs     []string
	ActivityUIDs   []string
	DryRun         bool
	BatchSize      int
	MaxActivities  int
	MaxConcurrency int
	ForceReprocess bool
	Period         types.Period `json:"period" validate:"required,oneof=all month week day"`
	EnvFilePath    string       `validate:"required"`
}

func main() {
	var config Config

	flag.Var((*stringSlice)(&config.SourceUIDs), "source", "Source UID to reprocess (can be specified multiple times)")
	flag.Var((*stringSlice)(&config.ActivityUIDs), "activity", "Activity UID to reprocess (can be specified multiple times)")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Show what would be reprocessed without actually doing it")
	flag.IntVar(&config.BatchSize, "batch-size", 50, "Number of activities to process in each batch")
	flag.IntVar(&config.MaxActivities, "max-activities", 0, "Maximum number of activities to reprocess (0 = no limit)")
	flag.IntVar(&config.MaxConcurrency, "max-concurrency", 100, "Maximum number of activities to reprocess (0 = no limit)")
	flag.BoolVar(&config.ForceReprocess, "force", false, "Force reprocess even if activity already has summary/embedding")
	flag.StringVar((*string)(&config.Period), "period", "all", "Time period to filter activities (all, month, week, day)")
	flag.StringVar(&config.EnvFilePath, "env-file", ".env", "Path to .env file")
	flag.Parse()

	ctx := context.Background()
	if err := run(ctx, config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, config Config) error {
	if err := lib.ValidateStruct(config); err != nil {
		return fmt.Errorf("config validation: %w", err)
	}

	// Load environment
	err := godotenv.Load(config.EnvFilePath)
	if err != nil {
		fmt.Println("Warning: Could not load .env file")
	}

	cfg, err := appconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := log.NewLogger(&cfg.Log)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	// Connect to database
	db := postgres.NewDB(&cfg.DB)
	err = db.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	// Initialize OpenAI models
	usageTracker := lib.NewUsageTracker(logger)
	limiter := lib.NewOpenAILimiterWithTracker(logger, usageTracker)

	completionModel, err := openai.New(
		openai.WithModel("gpt-5-nano-2025-08-07"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return fmt.Errorf("create summarizer model: %w", err)
	}

	embeddingModel, err := openai.New(
		openai.WithEmbeddingModel("text-embedding-3-small"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return fmt.Errorf("create embedder model: %w", err)
	}

	summarizer := nlp.NewSummarizer(completionModel, logger)

	embedder := nlp.NewEmbedder(embeddingModel)

	activityRepo := postgres.NewActivityRepository(db)
	activityRegistry := activities.NewRegistry(logger, activityRepo, summarizer, embedder)

	searchReq, err := buildSearchRequest(config)
	if err != nil {
		return fmt.Errorf("build search request: %w", err)
	}

	logger.Info().
		Strs("source_uids", config.SourceUIDs).
		Strs("activity_uids", config.ActivityUIDs).
		Bool("dry_run", config.DryRun).
		Int("batch_size", config.BatchSize).
		Int("max_activities", config.MaxActivities).
		Int("max_concurrency", config.MaxConcurrency).
		Bool("force", config.ForceReprocess).
		Str("period", string(config.Period)).
		Msg("Starting reprocessing")

	// Create a pool with limited concurrency
	pool := pond.NewPool(config.MaxConcurrency)
	skipped := atomic.Int32{}
	errored := atomic.Int32{}
	fetchCount := 0

	for {
		result, err := activityRegistry.Search(ctx, searchReq)
		if err != nil {
			return fmt.Errorf("search activities: %w", err)
		}
		searchReq.Cursor = result.NextCursor
		fetchCount += len(result.Activities)

		logger.Info().
			Int("activities_count", len(result.Activities)).
			Str("next_cursor", result.NextCursor).
			Bool("has_more", result.HasMore).
			Msg("Processing batch")

		if !result.HasMore {
			break
		}

		if config.MaxActivities > 0 && fetchCount > config.MaxActivities {
			break
		}

		if config.DryRun {
			continue
		}

		for _, act := range result.Activities {
			pool.Submit(func() {
				isAdded, err := activityRegistry.Create(ctx, act.Activity, config.ForceReprocess)
				if err != nil {
					logger.Error().
						Err(err).
						Str("activity_id", act.Activity.UID().String()).
						Msg("Error reprocessing activity")
					errored.Add(1)
				}
				if !isAdded {
					skipped.Add(1)
				}
				logger.Info().
					Str("activity_id", act.Activity.UID().String()).
					Bool("is_added", isAdded).
					Msg("Processed activity")
			})
		}
	}

	pool.StopAndWait()

	logger.Info().
		Int32("skipped", skipped.Load()).
		Int32("errored", errored.Load()).
		Msg("Reprocessing completed")

	return nil
}

func buildSearchRequest(config Config) (activities.SearchRequest, error) {
	req := activities.SearchRequest{
		Limit:  config.BatchSize,
		SortBy: types.SortByDate,
		Period: config.Period,
	}

	// Convert source UIDs
	if len(config.SourceUIDs) > 0 {
		req.SourceUIDs = make([]types.TypedUID, len(config.SourceUIDs))
		for i, uid := range config.SourceUIDs {
			parsedUID, err := lib.NewTypedUIDFromString(uid)
			if err != nil {
				return req, fmt.Errorf("new typed source uid: %w", err)
			}

			req.SourceUIDs[i] = parsedUID
		}
	}

	// Convert activity UIDs
	if len(config.ActivityUIDs) > 0 {
		req.ActivityUIDs = make([]types.TypedUID, len(config.ActivityUIDs))
		for i, uid := range config.ActivityUIDs {
			parsedUID, err := lib.NewTypedUIDFromString(uid)
			if err != nil {
				return req, fmt.Errorf("invalid activity UID: %w", err)
			}

			req.ActivityUIDs[i] = parsedUID
		}
	}

	return req, nil
}

// stringSlice implements flag.Value for string slices
type stringSlice []string

func (s *stringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}
