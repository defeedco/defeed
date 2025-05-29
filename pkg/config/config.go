package config

import (
	"fmt"
	"github.com/glanceapp/glance/pkg/api"
	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/joeshaw/envdecode"
)

type Config struct {
	DBConfig  postgres.Config `env:""`
	APIConfig api.Config      `env:""`
}

func Load() (*Config, error) {
	var cfg Config

	if err := envdecode.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	cfg.APIConfig.Init()

	return &cfg, nil
}
