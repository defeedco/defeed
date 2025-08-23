package mastodon

import (
	"context"
	"github.com/glanceapp/glance/pkg/sources/types"
	"strings"

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

var popularTechAccounts = []struct {
	account     string
	instanceURL string
	description string
}{
	{"Gargron", "https://mastodon.social", "Creator of Mastodon - Eugen Rochko"},
	{"leo", "https://twit.social", "Leo Laporte - Host of TWiT podcast"},
	{"SwiftOnSecurity", "https://infosec.exchange", "Infosec professional and satirist"},
	{"jasonhowell", "https://mastodon.social", "Tech podcaster and content creator"},
	{"davidbisset", "https://mastodon.social", "PHP, Laravel, and WordPress developer"},
	{"thurrott", "https://twit.social", "Paul Thurrott - Technology journalist"},
	{"joannastern", "https://mastodon.world", "WSJ Technology columnist"},
	{"jperlow", "https://journa.host", "Tech writer Jason Perlow"},
	{"h0x0d", "https://mstdn.social", "WalkingCat - Tech enthusiast and leaker"},
	{"docpop", "https://mastodon.social", "Musician, artist, and game designer"},
}

func (f *AccountFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []types.Source

	for _, account := range popularTechAccounts {
		accountName := strings.ToLower(account.account)
		description := strings.ToLower(account.description)

		if query == "" || strings.Contains(accountName, query) || strings.Contains(description, query) {
			source := &SourceAccount{
				InstanceURL: account.instanceURL,
				Account:     account.account,
			}
			matchingSources = append(matchingSources, source)
		}
	}

	// Custom account (that's not necessarily valid) if no existing ones are found
	// TODO: Handle this better
	if query != "" && len(matchingSources) == 0 {
		source := &SourceAccount{
			InstanceURL: "https://mastodon.social",
			Account:     query,
		}
		matchingSources = append(matchingSources, source)
	}

	f.Logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Mastodon Account fetcher found accounts")

	return matchingSources, nil
}
