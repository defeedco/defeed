package mastodon

import (
	"context"
	"fmt"
	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

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

func (f *AccountFetcher) SourceType() string {
	return TypeMastodonAccount
}

var defaultInstanceURL = "https://mastodon.social"
var popularTechAccountSources = []types.Source{
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "Gargron",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "leo",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "SwiftOnSecurity",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "jasonhowell",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "davidbisset",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "thurrott",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "joannastern",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "jperlow",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "h0x0d",
	},
	&SourceAccount{
		InstanceURL: defaultInstanceURL,
		Account:     "docpop",
	},
}

func (f *AccountFetcher) FindByID(ctx context.Context, id types2.TypedUID) (types.Source, error) {
	for _, source := range popularTechAccountSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *AccountFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	// TODO(sources): Support searching custom accounts
	// Ignore the query, since the set of all available sources is small
	return popularTechAccountSources, nil
}
