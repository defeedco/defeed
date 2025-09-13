package reddit

import (
	"context"
	"fmt"

	types2 "github.com/defeedco/defeed/pkg/sources/activities/types"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/defeedco/defeed/pkg/sources/types"

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

func (f *SubredditFetcher) SourceType() string {
	return TypeRedditSubreddit
}

var popularSubredditSources = []types.Source{
	&SourceSubreddit{
		Subreddit:        "programming",
		SubredditSummary: "Programming discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "MachineLearning",
		SubredditSummary: "Machine learning research and discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "javascript",
		SubredditSummary: "JavaScript programming language",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "reactjs",
		SubredditSummary: "React.js library discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "Python",
		SubredditSummary: "Python programming language",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "golang",
		SubredditSummary: "Go programming language",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "rust",
		SubredditSummary: "Rust programming language",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "webdev",
		SubredditSummary: "Web development discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "startups",
		SubredditSummary: "Startup discussions and news",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "technology",
		SubredditSummary: "Technology news and discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "science",
		SubredditSummary: "Science news and discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "askscience",
		SubredditSummary: "Ask science questions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "datascience",
		SubredditSummary: "Data science discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "artificial",
		SubredditSummary: "Artificial intelligence discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "OpenAI",
		SubredditSummary: "Everything OpenAI",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "LocalLLaMA",
		SubredditSummary: "Running LLaMA models locally",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "ChatGPT",
		SubredditSummary: "ChatGPT discussions and use cases",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "gamedev",
		SubredditSummary: "Game development discussions",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "linux",
		SubredditSummary: "Linux operating system",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
	&SourceSubreddit{
		Subreddit:        "mcp",
		SubredditSummary: "Model Context Protocol (MCP)",
		SortBy:           "hot",
		TopPeriod:        "day",
	},
}

func (f *SubredditFetcher) FindByID(ctx context.Context, id types2.TypedUID, config *types.ProviderConfig) (types.Source, error) {
	for _, source := range popularSubredditSources {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *SubredditFetcher) Search(ctx context.Context, query string, config *types.ProviderConfig) ([]types.Source, error) {
	// TODO(sources): Support searching custom subreddits
	// Ignore the query, since the set of all available sources is small
	return popularSubredditSources, nil
}
