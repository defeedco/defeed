package mastodon

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/utils"

	"github.com/mattn/go-mastodon"
)

const TypeMastodonAccount = "mastodon-account"

type SourceAccount struct {
	InstanceURL string `json:"instanceUrl" validate:"required,url"`
	Account     string `json:"account" validate:"required"`
	client      *mastodon.Client
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

func (s *SourceAccount) Initialize() error {
	s.client = mastodon.NewClient(&mastodon.Config{
		Server:       s.InstanceURL,
		ClientID:     "pulse-feed-aggregation",
		ClientSecret: "pulse-feed-aggregation",
	})

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

	limit := int64(15)
	posts, err := s.fetchAccountPosts(ctx, account.ID, limit)
	if err != nil {
		errs <- fmt.Errorf("fetch posts: %w", err)
		return
	}

	var sinceTime time.Time
	if since != nil {
		sinceTime = since.CreatedAt()
	}

	for _, post := range posts {
		if since == nil || post.CreatedAt().After(sinceTime) {
			feed <- post
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

func (s *SourceAccount) fetchAccountPosts(ctx context.Context, accountID mastodon.ID, limit int64) ([]*Post, error) {
	statuses, err := s.client.GetAccountStatuses(ctx, accountID, &mastodon.Pagination{
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch account statuses: %w", err)
	}

	posts := make([]*Post, len(statuses))
	for i, status := range statuses {
		posts[i] = &Post{Status: status, SourceTyp: s.Type(), SourceID: s.UID()}
	}

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
