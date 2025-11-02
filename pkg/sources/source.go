package sources

import (
	"fmt"
	"strings"

	"github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/providers/github"
	"github.com/defeedco/defeed/pkg/sources/providers/hackernews"
	"github.com/defeedco/defeed/pkg/sources/providers/lobsters"
	"github.com/defeedco/defeed/pkg/sources/providers/mastodon"
	"github.com/defeedco/defeed/pkg/sources/providers/producthunt"
	"github.com/defeedco/defeed/pkg/sources/providers/reddit"
	"github.com/defeedco/defeed/pkg/sources/providers/rss"
	sourcestypes "github.com/defeedco/defeed/pkg/sources/types"
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
	case github.TypeGithubTopic:
		s = github.NewSourceTopic()
	case producthunt.TypeProductHuntPosts:
		s = producthunt.NewSourcePosts()
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return s, nil
}
