package mastodon

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/mattn/go-mastodon"
)

const TypeMastodonAccount = "mastodon-account"

type SourceAccount struct {
	InstanceURL string `json:"instance_url"`
	Account     string `json:"account"`
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

func (s *SourceAccount) Initialize() error {
	s.client = mastodon.NewClient(&mastodon.Config{
		Server:       s.InstanceURL,
		ClientID:     "pulse-feed-aggregation",
		ClientSecret: "pulse-feed-aggregation",
	})

	return nil
}

func (s *SourceAccount) Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error) {
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

	for _, post := range posts {
		feed <- post
	}
}

func (s *SourceAccount) fetchAccount(ctx context.Context) (*mastodon.Account, error) {
	accounts, err := s.client.Search(ctx, s.Account, false)
	if err != nil {
		return nil, fmt.Errorf("search account: %w", err)
	}

	if len(accounts.Accounts) == 0 {
		return nil, fmt.Errorf("account not found: %s", s.Account)
	}

	return accounts.Accounts[0], nil
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
