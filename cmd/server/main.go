package main

import (
	"fmt"
	"github.com/glanceapp/glance/pkg/api"
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

func run() (err error) {
	logger := zerolog.New(os.Stdout)
	server, err := api.NewServer(&logger, api.NewDefaultConfig())
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	if err := server.Start(); err != nil {
		log.Printf("Failed to start server: %v", err)
	}

	return nil
}
