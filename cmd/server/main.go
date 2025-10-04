package main

import (
	"context"
	"fmt"
	"time"

	"github.com/defeedco/defeed/pkg/feeds"
	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources"
	"github.com/defeedco/defeed/pkg/sources/activities"
	"github.com/defeedco/defeed/pkg/sources/nlp"
	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/defeedco/defeed/pkg/api"
	"github.com/defeedco/defeed/pkg/api/auth"
	"github.com/defeedco/defeed/pkg/config"
	"github.com/defeedco/defeed/pkg/lib/log"
	"github.com/defeedco/defeed/pkg/storage/postgres"
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
		openai.WithEmbeddingModel("text-embedding-3-large"),
		openai.WithHTTPClient(limiter),
	)
	if err != nil {
		return nil, fmt.Errorf("create embedder model: %w", err)
	}

	llmCache := lib.NewCache(2*time.Hour, logger)
	cachedEmbeddingModel := nlp.NewCachedModel(embeddingModel, llmCache)
	cachedCompletionModel := nlp.NewCachedModel(completionModel, llmCache)

	// Cache will help mostly with request-time LLM computations like query-rewrites
	summarizer := nlp.NewSummarizer(cachedCompletionModel, logger)
	queryRewriter := nlp.NewQueryRewriter(cachedCompletionModel, logger)
	embedder := nlp.NewEmbedder(cachedEmbeddingModel)

	activityRepo := postgres.NewActivityRepository(db)
	sourceRepo := postgres.NewSourceRepository(db)

	activityRegistry := activities.NewRegistry(logger, activityRepo, summarizer, embedder)

	sourceScheduler := sources.NewScheduler(logger, sourceRepo, activityRegistry, &config.Sources, &config.SourceProviders)
	if config.SourceInitialization {
		// Don't block the server startup
		go func() {
			if err := sourceScheduler.Initialize(ctx); err != nil {
				logger.Error().Err(err).Msg("failed to initialize source scheduler")
			}
		}()
	}

	// Cache source results to avoid hitting the 3rd party APIs for every FindByUID call
	sourceRegistry := sources.NewCachedRegistry(sources.NewRegistry(logger, &config.SourceProviders), logger)
	if err := sourceRegistry.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize source registry: %w", err)
	}

	feedStore := postgres.NewFeedRepository(db)
	feedRegistry := feeds.NewRegistry(feedStore, sourceScheduler, sourceRegistry, activityRegistry, summarizer, queryRewriter, &config.Feeds, logger)

	authMw, err := authMiddleware(config)
	if err != nil {
		return nil, fmt.Errorf("create auth middleware: %w", err)
	}

	server, err := api.NewServer(logger, &config.API, authMw, sourceRegistry, sourceScheduler, feedRegistry)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}

	return server, nil
}

func authMiddleware(config *config.Config) (*auth.RouteAuthMiddleware, error) {
	apiKeys, err := config.API.Auth.ParseAPIKeys()
	if err != nil {
		return nil, fmt.Errorf("parse API keys: %w", err)
	}

	// Set up default auth provider (api key for backward compatibility)
	apiKeyProvider := auth.NewKeyAuthProvider(apiKeys)

	authMiddleware := auth.NewRouteAuthMiddleware(&auth.AuthConfig{
		Provider: apiKeyProvider,
		Required: true,
	})

	// Per-route configuration
	authMiddleware.
		SetRouteAuthProvider("GET /sources", apiKeyProvider, true).
		SetRouteAuthProvider("GET /sources/{uid}", apiKeyProvider, true).
		SetRouteAuthProvider("GET /feeds", apiKeyProvider, true).
		SetRouteAuthProvider("POST /feeds", apiKeyProvider, true).
		SetRouteAuthProvider("PUT /feeds/{uid}", apiKeyProvider, true).
		SetRouteAuthProvider("DELETE /feeds/{uid}", apiKeyProvider, true).
		SetRouteAuthProvider("GET /feeds/{uid}/activities", apiKeyProvider, true)

	return authMiddleware, nil
}
