package activities

import (
	"fmt"
	"github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/providers/changedetection"
	"github.com/defeedco/defeed/pkg/sources/providers/github"
	"github.com/defeedco/defeed/pkg/sources/providers/hackernews"
	"github.com/defeedco/defeed/pkg/sources/providers/lobsters"
	"github.com/defeedco/defeed/pkg/sources/providers/mastodon"
	"github.com/defeedco/defeed/pkg/sources/providers/reddit"
	"github.com/defeedco/defeed/pkg/sources/providers/rss"
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
	case github.TypeGithubTopic:
		a = github.NewRepository()
	case changedetection.TypeChangedetectionWebsite:
		a = changedetection.NewWebsiteChange()
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return a, nil
}
