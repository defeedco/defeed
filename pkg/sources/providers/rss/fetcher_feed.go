package rss

import (
	"context"
	_ "embed"
	"fmt"

	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/types"

	"github.com/rs/zerolog"
)

// Source: https://raw.githubusercontent.com/tuan3w/awesome-tech-rss/refs/heads/main/feeds.opml
//
//go:embed awesome-tech-rss.opml
var awesomeTechRSS string

// FeedFetcher implements preset search functionality for RSS feeds
type FeedFetcher struct {
	ExampleFeeds []types.Source
	Logger       *zerolog.Logger
}

func NewFeedFetcher(logger *zerolog.Logger) (*FeedFetcher, error) {
	opmlSources, err := loadOPMLSources(logger)
	if err != nil {
		return nil, fmt.Errorf("load OPML sources: %w", err)
	}

	exampleFeeds := []types.Source{
		&SourceFeed{
			Title:     "ArXiv AI",
			FeedURL:   "https://rss.arxiv.org/rss/cs.AI",
			AboutFeed: "Covers all areas of AI except Vision, Robotics, Machine Learning, Multiagent Systems, and Computation and Language.",
		},
		&SourceFeed{
			Title:     "ArXiv Machine Learning",
			FeedURL:   "https://rss.arxiv.org/rss/cs.LG",
			AboutFeed: "All aspects of machine learning research including supervised, unsupervised, reinforcement learning, robustness, and fairness.",
		},
		&SourceFeed{
			Title:     "ArXiv Computer Vision",
			FeedURL:   "https://rss.arxiv.org/rss/cs.CV",
			AboutFeed: "Image processing, computer vision, pattern recognition, and scene understanding.",
		},
		&SourceFeed{
			Title:     "ArXiv Natural Language Processing",
			FeedURL:   "https://rss.arxiv.org/rss/cs.CL",
			AboutFeed: "Natural language processing and computational linguistics research.",
		},
		&SourceFeed{
			Title:     "ArXiv Cryptography & Security",
			FeedURL:   "https://rss.arxiv.org/rss/cs.CR",
			AboutFeed: "Cryptography, authentication, public key systems, and security research.",
		},
		&SourceFeed{
			Title:     "ArXiv Distributed Computing",
			FeedURL:   "https://rss.arxiv.org/rss/cs.DC",
			AboutFeed: "Fault-tolerance, distributed algorithms, parallel computation, and cluster computing.",
		},
		&SourceFeed{
			Title:     "ArXiv Databases",
			FeedURL:   "https://rss.arxiv.org/rss/cs.DB",
			AboutFeed: "Database management, data mining, and data processing research.",
		},
		&SourceFeed{
			Title:     "ArXiv Software Engineering",
			FeedURL:   "https://rss.arxiv.org/rss/cs.SE",
			AboutFeed: "Software engineering, development methodologies, and software quality research.",
		},
		&SourceFeed{
			Title:     "ArXiv Human-Computer Interaction",
			FeedURL:   "https://rss.arxiv.org/rss/cs.HC",
			AboutFeed: "Human factors, user interfaces, and collaborative computing research.",
		},
		&SourceFeed{
			Title:     "ArXiv Information Retrieval",
			FeedURL:   "https://rss.arxiv.org/rss/cs.IR",
			AboutFeed: "Indexing, search, content analysis, and information retrieval systems.",
		},
	}

	exampleFeeds = append(exampleFeeds, opmlSources...)

	return &FeedFetcher{
		ExampleFeeds: exampleFeeds,
		Logger:       logger,
	}, nil
}

func (f *FeedFetcher) SourceType() string {
	return TypeRSSFeed
}

func (f *FeedFetcher) FindByID(ctx context.Context, id types2.TypedUID) (types.Source, error) {
	for _, source := range f.ExampleFeeds {
		if lib.Equals(source.UID(), id) {
			return source, nil
		}
	}
	return nil, fmt.Errorf("source not found")
}

func (f *FeedFetcher) Search(ctx context.Context, query string) ([]types.Source, error) {
	// TODO(sources): Support adding custom feed URL?
	// Ignore the query, since the set of all available sources is small
	return f.ExampleFeeds, nil
}

func loadOPMLSources(logger *zerolog.Logger) ([]types.Source, error) {
	opml, err := lib.ParseOPML(awesomeTechRSS)
	if err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	opmlSources, err := opmlToRSSSources(opml)
	if err != nil {
		return nil, fmt.Errorf("convert OPML to RSS sources: %w", err)
	}

	logger.Info().
		Int("count", len(opmlSources)).
		Msg("loaded OPML RSS sources")

	return opmlSources, nil
}

func opmlToRSSSources(opml *lib.OPML) ([]types.Source, error) {
	var result []types.Source

	seen := make(map[string]bool)
	for _, category := range opml.Body.Outlines {
		for _, outline := range category.Outlines {
			if outline.Type != "rss" {
				return nil, fmt.Errorf("invalid outline type: %s", outline.Type)
			}

			if outline.XMLUrl == "" {
				return nil, fmt.Errorf("outline missing url: %s", outline.Text)
			}

			source := &SourceFeed{
				Title:   outline.Title,
				FeedURL: outline.XMLUrl,
			}

			if seen[source.UID().String()] {
				continue
			}
			seen[source.UID().String()] = true

			result = append(result, source)
		}
	}

	return result, nil
}
