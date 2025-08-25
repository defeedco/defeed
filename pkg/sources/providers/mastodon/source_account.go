package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/mattn/go-mastodon"
	"github.com/rs/zerolog"
)

const TypeMastodonAccount = "mastodonaccount"

type SourceAccount struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	Account     string `json:"account" validate:"required"`
	AccountBio  string `json:"accountBio"`
	client      *mastodon.Client
	logger      *zerolog.Logger
}

func NewSourceAccount() *SourceAccount {
	return &SourceAccount{
		InstanceURL: "https://mastodon.social",
	}
}

func (s *SourceAccount) UID() types.TypedUID {
	return lib.NewTypedUID(TypeMastodonAccount, lib.StripURL(s.InstanceURL), s.Account)
}

func (s *SourceAccount) Name() string {
	return fmt.Sprintf("User @%s", s.Account)
}

func (s *SourceAccount) Description() string {
	description := s.AccountBio
	if description != "" {
		return description
	}

	instanceName, err := lib.StripURLHost(s.InstanceURL)
	if err != nil {
		return fmt.Sprintf("Posts from @%s account on %s", s.Account, instanceName)
	}
	return fmt.Sprintf("Posts from @%s account on %s", s.Account, instanceName)
}

func (s *SourceAccount) URL() string {
	return fmt.Sprintf("%s/tags/%s", s.InstanceURL, s.Account)
}

func (s *SourceAccount) Validate() []error { return lib.ValidateStruct(s) }

func (s *SourceAccount) Initialize(logger *zerolog.Logger) error {
	s.client = mastodon.NewClient(&mastodon.Config{
		Server:       s.InstanceURL,
		ClientID:     "pulse-feed-aggregation",
		ClientSecret: "pulse-feed-aggregation",
	})

	s.logger = logger

	return nil
}

func (s *SourceAccount) Stream(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	account, err := s.fetchAccount(ctx)
	if err != nil {
		errs <- fmt.Errorf("fetch account: %w", err)
		return
	}

	s.fetchAccountPosts(ctx, account.ID, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAccountPosts(ctx, account.ID, since, feed, errs)
		}
	}
}

func (s *SourceAccount) fetchAccount(ctx context.Context) (*mastodon.Account, error) {
	acct := s.Account

	account, err := s.client.AccountLookup(ctx, acct)
	if err != nil {
		return nil, fmt.Errorf("account lookup: %w", err)
	}
	return account, nil
}

func (s *SourceAccount) fetchAccountPosts(ctx context.Context, accountID mastodon.ID, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	var sinceID mastodon.ID
	if since != nil {
		sincePost := since.(*Post)
		sinceID = sincePost.Status.ID
	} else {
		// If this is the first time we're fetching posts,
		// only fetch the last few posts to avoid retrieving all historic posts.
		s.fetchLatestPosts(ctx, accountID, feed, errs)
		return
	}

outer:
	for {
		accLogger := s.logger.With().
			Str("account_id", string(accountID)).
			Str("since_id", string(sinceID)).
			Logger()

		accLogger.Debug().Msg("Fetching account statuses")

		statuses, err := s.client.GetAccountStatuses(ctx, accountID, &mastodon.Pagination{
			Limit:   int64(15),
			SinceID: sinceID,
		})
		if err != nil {
			errs <- fmt.Errorf("fetch account statuses: %w", err)
			return
		}

		accLogger.Debug().Int("count", len(statuses)).Msg("Fetched account statuses")

		if len(statuses) == 0 {
			break outer
		}

		for _, status := range statuses {
			post := &Post{
				Status:    status,
				SourceTyp: TypeMastodonAccount,
				SourceID:  s.UID(),
			}
			feed <- post
		}

		sinceID = statuses[len(statuses)-1].ID
	}
}

func (s *SourceAccount) fetchLatestPosts(ctx context.Context, accountID mastodon.ID, feed chan<- types.Activity, errs chan<- error) {
	accLogger := s.logger.With().
		Str("account_id", string(accountID)).
		Logger()

	accLogger.Debug().Msg("Fetching latest post from account timeline")

	statuses, err := s.client.GetAccountStatuses(ctx, accountID, &mastodon.Pagination{
		Limit: 10,
	})
	if err != nil {
		errs <- fmt.Errorf("fetch account statuses: %w", err)
		return
	}

	if len(statuses) == 0 {
		accLogger.Debug().Msg("No posts found in account timeline")
		return
	}

	for _, status := range statuses {
		post := &Post{
			Status:    status,
			SourceTyp: TypeMastodonAccount,
			SourceID:  s.UID(),
		}
		feed <- post
	}

	accLogger.Debug().
		Int("count", len(statuses)).
		Msg("Fetched latest posts from account timeline")
}

func (s *SourceAccount) MarshalJSON() ([]byte, error) {
	type Alias SourceAccount
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  TypeMastodonAccount,
	})
}

func (s *SourceAccount) UnmarshalJSON(data []byte) error {
	type Alias SourceAccount
	aux := &struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	return nil
}
