package config

import (
	"fmt"
	"github.com/glanceapp/glance/pkg/feeds"
	"github.com/glanceapp/glance/pkg/lib"

	"github.com/glanceapp/glance/pkg/api"
	"github.com/glanceapp/glance/pkg/lib/log"
	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/joeshaw/envdecode"
)

type Config struct {
	DBConfig    postgres.Config `env:""`
	APIConfig   api.Config      `env:""`
	LogConfig   log.Config      `env:""`
	FeedsConfig feeds.Config    `env:""`
}

func Load() (*Config, error) {
	var cfg Config

	if err := envdecode.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := lib.ValidateStruct(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}
