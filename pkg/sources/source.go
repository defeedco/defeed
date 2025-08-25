package sources

import (
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"strings"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/providers/changedetection"
	"github.com/glanceapp/glance/pkg/sources/providers/github"
	"github.com/glanceapp/glance/pkg/sources/providers/hackernews"
	"github.com/glanceapp/glance/pkg/sources/providers/lobsters"
	"github.com/glanceapp/glance/pkg/sources/providers/mastodon"
	"github.com/glanceapp/glance/pkg/sources/providers/reddit"
	"github.com/glanceapp/glance/pkg/sources/providers/rss"
	sourcestypes "github.com/glanceapp/glance/pkg/sources/types"
)

func NewTypedUID(uid string) (types.TypedUID, error) {
	parts := strings.SplitN(uid, ":", 2)
	switch parts[0] {
	case github.TypeGithubIssues, github.TypeGithubReleases:
		return github.NewTypedUIDFromString(uid)
	default:
		return lib.NewTypedUIDFromString(uid)
	}
}

func NewSource(sourceType string) (sourcestypes.Source, error) {
	var s sourcestypes.Source

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
