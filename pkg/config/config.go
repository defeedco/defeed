package config

import (
	"fmt"

	"github.com/glanceapp/glance/pkg/api"
	"github.com/glanceapp/glance/pkg/lib/log"
	"github.com/glanceapp/glance/pkg/storage/postgres"
	"github.com/go-playground/validator/v10"
	"github.com/joeshaw/envdecode"
)

type Config struct {
	DBConfig  postgres.Config `env:""`
	APIConfig api.Config      `env:""`
	LogConfig log.Config      `env:""`
}

func Load() (*Config, error) {
	var cfg Config

	if err := envdecode.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	if err := validateStruct(&cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	cfg.APIConfig.Init()

	return &cfg, nil
}

func validateStruct(s any) error {
	validate := validator.New()
	if err := validate.Struct(s); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("validation errors: %w", validationErrors)
		}
		return err
	}
	return nil
}
