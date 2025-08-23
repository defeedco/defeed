package reddit

import (
	"context"
	"github.com/glanceapp/glance/pkg/sources/types"
	"strings"

	"github.com/rs/zerolog"
)

// SubredditFetcher implements preset search functionality for Reddit subreddits
type SubredditFetcher struct {
	Logger *zerolog.Logger
}

func NewSubredditFetcher(logger *zerolog.Logger) *SubredditFetcher {
	return &SubredditFetcher{
		Logger: logger,
	}
}

var popularSubreddits = []struct {
	name        string
	description string
}{
	{"programming", "Computer programming discussions"},
	{"MachineLearning", "Machine learning research and discussions"},
	{"javascript", "JavaScript programming language"},
	{"reactjs", "React.js library discussions"},
	{"Python", "Python programming language"},
	{"golang", "Go programming language"},
	{"rust", "Rust programming language"},
	{"webdev", "Web development discussions"},
	{"startups", "Startup discussions and news"},
	{"technology", "Technology news and discussions"},
	{"science", "Science news and discussions"},
	{"askscience", "Ask science questions"},
	{"datascience", "Data science discussions"},
	{"artificial", "Artificial intelligence discussions"},
	{"gamedev", "Game development discussions"},
	{"linux", "Linux operating system"},
	{"sysadmin", "System administration"},
	{"devops", "DevOps practices and tools"},
	{"cybersecurity", "Cybersecurity discussions"},
}

func (f *SubredditFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	query = strings.ToLower(query)
	var matchingSources []types.Source

	for _, sub := range popularSubreddits {
		subName := strings.ToLower(sub.name)
		description := strings.ToLower(sub.description)

		if query == "" || strings.Contains(subName, query) || strings.Contains(description, query) {
			source := &SourceSubreddit{
				Subreddit: sub.name,
				SortBy:    "hot",
				TopPeriod: "day",
			}
			matchingSources = append(matchingSources, source)
		}
	}

	f.Logger.Debug().
		Str("query", query).
		Int("matches", len(matchingSources)).
		Msg("Reddit fetcher found subreddits")

	return matchingSources, nil
}
