package sources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/sources/changedetection"
	"github.com/glanceapp/glance/pkg/sources/github"
	"github.com/glanceapp/glance/pkg/sources/hackernews"
	"github.com/glanceapp/glance/pkg/sources/lobsters"
	"github.com/glanceapp/glance/pkg/sources/mastodon"
	"github.com/glanceapp/glance/pkg/sources/reddit"
	"github.com/glanceapp/glance/pkg/sources/rss"
)

func NewSource(sourceType string) (Source, error) {
	var s Source

	switch sourceType {
	case mastodon.TypeMastodonAccount:
		s = mastodon.NewSourceAccount()
	case mastodon.TypeMastodonTag:
		s = mastodon.NewSourceTag()
	case hackernews.TypeHackerNewsPosts:
		s = hackernews.NewSourcePosts()
	case reddit.TypeRedditSubreddit:
		s = reddit.NewSourceSubreddit()
	case lobsters.TypeLobstersTag:
		s = lobsters.NewSourceTag()
	case lobsters.TypeLobstersFeed:
		s = lobsters.NewSourceFeed()
	case rss.TypeRSSFeed:
		s = rss.NewSourceFeed()
	case github.TypeGithubReleases:
		s = github.NewReleaseSource()
	case github.TypeGithubIssues:
		s = github.NewIssuesSource()
	case changedetection.TypeChangedetectionWebsite:
		s = changedetection.NewSourceWebsiteChange()
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return s, nil
}

type Source interface {
	json.Marshaler
	json.Unmarshaler
	UID() string
	Type() string
	// Name is a human-readable UID.
	Name() string
	// URL is a web resource representation of UID.
	URL() string
	// Validate returns a list of configuration validation errors.
	// When non-empty, the caller should not proceed to Initialize.
	Validate() []error
	Initialize() error
	Stream(ctx context.Context, feed chan<- types.Activity, errs chan<- error)
}
