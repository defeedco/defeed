package mastodon

import (
	"context"

	"github.com/glanceapp/glance/pkg/sources/fetcher"
	"github.com/rs/zerolog"
)

// AccountFetcher implements preset search functionality for Mastodon accounts
type AccountFetcher struct {
	Logger *zerolog.Logger
}

func NewAccountFetcher(logger *zerolog.Logger) *AccountFetcher {
	return &AccountFetcher{
		Logger: logger,
	}
}

func (f *AccountFetcher) Search(ctx context.Context, query string) ([]fetcher.Source, error) {
	// Return template source for user customization
	source := &SourceAccount{
		InstanceURL: "https://mastodon.social",
		Account:     "",
	}

	f.Logger.Debug().
		Str("query", query).
		Msg("Mastodon Account fetcher - returning template for customization")

	return []fetcher.Source{source}, nil
}
