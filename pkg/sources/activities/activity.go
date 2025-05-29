package activities

import (
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

func NewActivity(sourceType string) (types.Activity, error) {
	var a types.Activity

	switch sourceType {
	case mastodon.TypeMastodonAccount:
		a = mastodon.NewPost()
	case mastodon.TypeMastodonTag:
		a = mastodon.NewPost()
	case hackernews.TypeHackerNewsPosts:
		a = hackernews.NewPost()
	case reddit.TypeRedditSubreddit:
		a = reddit.NewPost()
	case lobsters.TypeLobstersTag:
		a = lobsters.NewPost()
	case lobsters.TypeLobstersFeed:
		a = lobsters.NewPost()
	case rss.TypeRSSFeed:
		a = rss.NewFeedItem()
	case github.TypeGithubReleases:
		a = github.NewRelease()
	case github.TypeGithubIssues:
		a = github.NewIssue()
	case changedetection.TypeChangedetectionWebsite:
		a = changedetection.NewWebsiteChange()
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return a, nil
}
