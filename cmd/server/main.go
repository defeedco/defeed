package main

import (
	"context"
	"fmt"
	"github.com/glanceapp/glance/pkg/api"
	"github.com/glanceapp/glance/pkg/config"
	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"log"
	"os"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("Failed to run: %v", err)
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

	logger := zerolog.New(os.Stdout)

	db := postgres.NewDB(&cfg.DBConfig)
	err = db.Connect(context.Background())
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	server, err := api.NewServer(&logger, &cfg.APIConfig, db)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	return nil
}
