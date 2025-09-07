package main

import (
	"context"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/feeds"
	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources"
	"github.com/glanceapp/glance/pkg/sources/activities"
	"github.com/glanceapp/glance/pkg/sources/nlp"
	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/glanceapp/glance/pkg/api"
	"github.com/glanceapp/glance/pkg/config"
	"github.com/glanceapp/glance/pkg/lib/log"
	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/joho/godotenv"
)

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	err := godotenv.Load()
	if err != nil {
		return fmt.Errorf("load .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger, err := log.NewLogger(&cfg.Log)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	ctx := context.Background()
	server, err := initServer(ctx, logger, cfg)
	if err != nil {
		return fmt.Errorf("initialize server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	return nil
}

func initServer(ctx context.Context, logger *zerolog.Logger, config *config.Config) (*api.Server, error) {
	db := postgres.NewDB(&config.DB)
	err := db.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	usageTracker := lib.NewUsageTracker(logger)
	limiter := lib.NewOpenAILimiterWithTracker(logger, usageTracker)

	completionModel, err := openai.New(
		openai.WithModel("gpt-5-nano-2025-08-07"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return nil, fmt.Errorf("create summarizer model: %w", err)
	}

	embeddingModel, err := openai.New(
		openai.WithEmbeddingModel("text-embedding-3-small"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return nil, fmt.Errorf("create embedder model: %w", err)
	}

	llmCache := nlp.NewLLMCache(2*time.Hour, logger)
	cachedEmbeddingModel := nlp.NewCachedModel(embeddingModel, llmCache)
	cachedCompletionModel := nlp.NewCachedModel(completionModel, llmCache)

	// Cache will help mostly with request-time LLM computations like query-rewrites
	summarizer := nlp.NewSummarizer(cachedCompletionModel, logger)
	queryRewriter := nlp.NewQueryRewriter(cachedCompletionModel, logger)
	embedder := nlp.NewEmbedder(cachedEmbeddingModel)

	activityRepo := postgres.NewActivityRepository(db)
	sourceRepo := postgres.NewSourceRepository(db)

	activityRegistry := activities.NewRegistry(logger, activityRepo, summarizer, embedder)

	sourceScheduler := sources.NewScheduler(logger, sourceRepo, activityRegistry, &config.Sources)
	if err := sourceScheduler.Initialize(ctx); err != nil {
		return nil, fmt.Errorf("initialize sourceScheduler: %w", err)
	}

	sourceRegistry := sources.NewRegistry(logger)
	if err := sourceRegistry.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize source registry: %w", err)
	}

	feedStore := postgres.NewFeedRepository(db)
	feedRegistry := feeds.NewRegistry(feedStore, sourceScheduler, sourceRegistry, activityRegistry, summarizer, queryRewriter, &config.Feeds)

	server, err := api.NewServer(logger, &config.API, sourceRegistry, sourceScheduler, feedRegistry)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	return server, nil
}
