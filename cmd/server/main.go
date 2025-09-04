package main

import (
	"context"
	"fmt"

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

	logger, err := log.NewLogger(&cfg.LogConfig)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	db := postgres.NewDB(&cfg.DBConfig)
	err = db.Connect(context.Background())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	server, err := api.NewServer(logger, &cfg.APIConfig, &cfg.FeedsConfig, db)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	return nil
}
