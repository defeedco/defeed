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
	default:
		return nil, fmt.Errorf("unknown source type: %s", sourceType)
	}

	return s, nil
}

func SourceTypeToLabel(in string) (string, error) {
	switch in {
	case mastodon.TypeMastodonAccount:
		return "Mastodon Account", nil
	case mastodon.TypeMastodonTag:
		return "Mastodon Tag", nil
	case hackernews.TypeHackerNewsPosts:
		return "Hackernews Posts", nil
	case reddit.TypeRedditSubreddit:
		return "Reddit Subreddit", nil
	case lobsters.TypeLobstersTag:
		return "Lobsters Tag", nil
	case lobsters.TypeLobstersFeed:
		return "Lobsters Feed", nil
	case rss.TypeRSSFeed:
		return "RSS Feed", nil
	case github.TypeGithubReleases:
		return "Github Releases", nil
	case github.TypeGithubIssues:
		return "Github Issues", nil
	case github.TypeGithubTopic:
		return "Github Topics", nil
		// Note: temporarily removed in commit a8c728a86cefadd20f67a424363dc6f61c41cf66
		// case changedetection.TypeChangedetectionWebsite:
		// return ChangedetectionWebsite, nil
	}

	return "", fmt.Errorf("unknown source type: %s", in)
}

func SourceTypeToEmoji(in string) (string, error) {
	switch in {
	case mastodon.TypeMastodonAccount:
		return "🐘", nil
	case mastodon.TypeMastodonTag:
		return "🐘", nil
	case hackernews.TypeHackerNewsPosts:
		return "🧑‍💻", nil
	case reddit.TypeRedditSubreddit:
		return "🔥", nil
	case lobsters.TypeLobstersTag:
		return "🐙", nil
	case lobsters.TypeLobstersFeed:
		return "🐙", nil
	case rss.TypeRSSFeed:
		return "🔗", nil
	case github.TypeGithubReleases:
		return "🏷️", nil
	case github.TypeGithubIssues:
		return "🔘", nil
	case github.TypeGithubTopic:
		return "⭐", nil
		// Note: temporarily removed in commit a8c728a86cefadd20f67a424363dc6f61c41cf66
		// case changedetection.TypeChangedetectionWebsite:
		// return ChangedetectionWebsite, nil
	}

	return "", fmt.Errorf("unknown source type: %s", in)
}
