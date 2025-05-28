package sources

import (
	"context"
	"fmt"

	"github.com/glanceapp/glance/pkg/sources/changedetection"
	"github.com/glanceapp/glance/pkg/sources/common"
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
	case "mastodon-account":
		s = mastodon.NewSourceAccount()
	case "mastodon-tag":
		s = mastodon.NewSourceTag()
	case "hackernews-posts":
		s = hackernews.NewSourcePosts()
	case "reddit-subreddit":
		s = reddit.NewSourceSubreddit()
	case "lobsters-tag":
		s = lobsters.NewSourceTag()
	case "lobsters-feed":
		s = lobsters.NewSourceFeed()
	case "rss-feed":
		s = rss.NewSourceFeed()
	case "github-releases":
		s = github.NewReleaseSource()
	case "github-issues":
		s = github.NewIssuesSource()
	case "changedetection-website-change":
		s = changedetection.NewSourceWebsiteChange()
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return s, nil
}

type Source interface {
	UID() string
	// Name is a human-readable UID.
	Name() string
	// URL is a web resource representation of UID.
	URL() string
	Initialize() error
	Stream(ctx context.Context, feed chan<- common.Activity, errs chan<- error)
}
