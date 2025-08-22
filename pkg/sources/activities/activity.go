package activities

import (
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/glanceapp/glance/pkg/sources/providers/changedetection"
	github2 "github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	lobsters2 "github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	mastodon2 "github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"
)

func NewActivity(sourceType string) (types.Activity, error) {
	var a types.Activity

	switch sourceType {
	case mastodon2.TypeMastodonAccount:
		a = mastodon2.NewPost()
	case mastodon2.TypeMastodonTag:
		a = mastodon2.NewPost()
	case hackernews.TypeHackerNewsPosts:
		a = hackernews.NewPost()
	case reddit.TypeRedditSubreddit:
		a = reddit.NewPost()
	case lobsters2.TypeLobstersTag:
		a = lobsters2.NewPost()
	case lobsters2.TypeLobstersFeed:
		a = lobsters2.NewPost()
	case rss.TypeRSSFeed:
		a = rss.NewFeedItem()
	case github2.TypeGithubReleases:
		a = github2.NewRelease()
	case github2.TypeGithubIssues:
		a = github2.NewIssue()
	case changedetection.TypeChangedetectionWebsite:
		a = changedetection.NewWebsiteChange()
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return a, nil
}
