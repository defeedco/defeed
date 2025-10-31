package config

import (
	"fmt"

	"github.com/defeedco/defeed/pkg/feeds"
	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources"
	sourcetypes "github.com/defeedco/defeed/pkg/sources/types"

	"github.com/defeedco/defeed/pkg/api"
	"github.com/defeedco/defeed/pkg/lib/log"
	"github.com/defeedco/defeed/pkg/llms"
	"github.com/defeedco/defeed/pkg/storage/postgres"
	"github.com/joeshaw/envdecode"
)

type Config struct {
	DB              postgres.Config            `env:""`
	API             api.Config                 `env:""`
	Log             log.Config                 `env:""`
	Feeds           feeds.Config               `env:""`
	Sources         sources.Config             `env:""`
	SourceProviders sourcetypes.ProviderConfig `env:""`
	LLMs            llms.Config                `env:""`
	// Dev-only variables

	// SourceInitialization true if the scheduler should not be initialized to process existing sources.
	SourceInitialization bool `env:"SOURCE_INITIALIZATION,default=true"`
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
