package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/utils"

	"github.com/mattn/go-mastodon"
	"github.com/rs/zerolog"
)

const TypeMastodonAccount = "mastodon-account"

type SourceAccount struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	Account     string `json:"account" validate:"required"`
	client      *mastodon.Client
	logger      *zerolog.Logger
}

func NewSourceAccount() *SourceAccount {
	return &SourceAccount{
		InstanceURL: "https://mastodon.social",
	}
}

func (s *SourceAccount) UID() string {
	return fmt.Sprintf("%s/%s/%s", s.Type(), s.InstanceURL, s.Account)
}

func (s *SourceAccount) Name() string {
	return fmt.Sprintf("Mastodon (%s)", s.Account)
}

func (s *SourceAccount) URL() string {
	return fmt.Sprintf("%s/tags/%s", s.InstanceURL, s.Account)
}

func (s *SourceAccount) Type() string {
	return TypeMastodonAccount
}

func (s *SourceAccount) Validate() []error { return utils.ValidateStruct(s) }

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

	s.fetchAndSendNewPosts(ctx, since, feed, errs)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndSendNewPosts(ctx, since, feed, errs)
		}
	}
}

func (s *SourceAccount) fetchAndSendNewPosts(ctx context.Context, since types.Activity, feed chan<- types.Activity, errs chan<- error) {
	account, err := s.fetchAccount(ctx)
	if err != nil {
		errs <- fmt.Errorf("fetch account: %w", err)
		return
	}

	posts, err := s.fetchAccountPosts(ctx, account.ID, since)
	if err != nil {
		errs <- fmt.Errorf("fetch posts: %w", err)
		return
	}

	for _, post := range posts {
		feed <- post
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

func (s *SourceAccount) fetchAccountPosts(ctx context.Context, accountID mastodon.ID, since types.Activity) ([]*Post, error) {
	var sinceID mastodon.ID
	if since != nil {
		sincePost := since.(*Post)
		sinceID = sincePost.Status.ID
	} else {
		// If this is the first time we're fetching posts,
		// only fetch the last few posts to avoid retrieving all historic posts.
		latestPosts, err := s.fetchLatestPosts(ctx, accountID)
		if err != nil {
			return nil, fmt.Errorf("fetch latest post: %w", err)
		}
		return latestPosts, nil
	}

	posts := make([]*Post, 0)
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
			return nil, fmt.Errorf("fetch account statuses: %w", err)
		}

		accLogger.Debug().Int("count", len(statuses)).Msg("Fetched account statuses")

		if len(statuses) == 0 {
			break outer
		}

		for _, status := range statuses {
			posts = append(posts, &Post{
				Status:    status,
				SourceTyp: s.Type(),
				SourceID:  s.UID(),
			})
		}

		sinceID = statuses[len(statuses)-1].ID
	}

	return posts, nil
}

func (s *SourceAccount) fetchLatestPosts(ctx context.Context, accountID mastodon.ID) ([]*Post, error) {
	accLogger := s.logger.With().
		Str("account_id", string(accountID)).
		Logger()

	accLogger.Debug().Msg("Fetching latest post from account timeline")

	statuses, err := s.client.GetAccountStatuses(ctx, accountID, &mastodon.Pagination{
		Limit: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch account statuses: %w", err)
	}

	if len(statuses) == 0 {
		accLogger.Debug().Msg("No posts found in account timeline")
		return nil, nil
	}

	posts := make([]*Post, 0)
	for _, status := range statuses {
		posts = append(posts, &Post{
			Status:    status,
			SourceTyp: s.Type(),
			SourceID:  s.UID(),
		})
	}

	accLogger.Debug().Int("count", len(posts)).Msg("Fetched latest posts from account timeline")

	return posts, nil
}

func (s *SourceAccount) MarshalJSON() ([]byte, error) {
	type Alias SourceAccount
	return json.Marshal(&struct {
		*Alias
		Type string `json:"type"`
	}{
		Alias: (*Alias)(s),
		Type:  s.Type(),
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
