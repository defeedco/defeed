package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/glanceapp/glance/pkg/sources"
	"github.com/rs/zerolog"
)

type sourceTypeConfig struct {
	Type string `json:"type"`
}

func main() {
	logger := zerolog.New(os.Stdout)
	registry := sources.NewRegistry(&logger)

	sourceConfig, err := io.ReadAll(os.Stdin)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to read source config")
	}

	t := sourceTypeConfig{}
	err = json.Unmarshal(sourceConfig, &t)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse source type config")
	}

	s, err := sources.NewSource(t.Type)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to create source")
	}

	err = json.Unmarshal(sourceConfig, &s)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to parse source config")
	}

	err = s.Initialize()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize source")
	}

	err = registry.Add(s)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to add source to registry")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info().Msg("Shutting down...")
	registry.Shutdown()
}
