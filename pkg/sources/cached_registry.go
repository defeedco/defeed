package sources

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/defeedco/defeed/pkg/lib"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/defeedco/defeed/pkg/sources/types"
	"github.com/rs/zerolog"
)

// CachedRegistry wraps a SourceRegistry and adds caching functionality
type CachedRegistry struct {
	registry    *Registry
	logger      *zerolog.Logger
	searchCache *lib.Cache
	sourceCache *lib.Cache
}

func NewCachedRegistry(registry *Registry, logger *zerolog.Logger) *CachedRegistry {
	return &CachedRegistry{
		registry:    registry,
		logger:      logger,
		searchCache: lib.NewCache(15*time.Minute, logger),
		sourceCache: lib.NewCache(30*time.Minute, logger),
	}
}

func (c *CachedRegistry) Initialize() error {
	return c.registry.Initialize()
}

// FindByUID finds a source by UID with caching
func (c *CachedRegistry) FindByUID(ctx context.Context, uid activitytypes.TypedUID) (types.Source, error) {
	cacheKey := c.generateSourceCacheKey(uid)

	if cached, found := c.sourceCache.Get(cacheKey); found {
		if source, ok := cached.(types.Source); ok {
			c.logger.Debug().
				Str("uid", uid.String()).
				Msg("source cache hit")
			return source, nil
		}
	}

	source, err := c.registry.FindByUID(ctx, uid)
	if err != nil {
		return nil, err
	}

	c.sourceCache.Set(cacheKey, source)
	c.logger.Debug().
		Str("uid", uid.String()).
		Msg("cached source")

	return source, nil
}

// Search searches for sources with caching
func (c *CachedRegistry) Search(ctx context.Context, params SearchRequest) ([]types.Source, error) {
	cacheKey := c.generateSearchCacheKey(params.Query, params.Topics)

	if cached, found := c.searchCache.Get(cacheKey); found {
		if results, ok := cached.([]types.Source); ok {
			c.logger.Debug().
				Str("query", params.Query).
				Int("topics", len(params.Topics)).
				Int("count", len(results)).
				Msg("search cache hit")
			return results, nil
		}
	}

	results, err := c.registry.Search(ctx, params)
	if err != nil {
		return nil, err
	}

	// Cache the final results
	c.searchCache.Set(cacheKey, results)
	c.logger.Debug().
		Str("query", params.Query).
		Int("topics", len(params.Topics)).
		Int("count", len(results)).
		Msg("cached search results")

	// Also populate the source cache with individual sources
	for _, source := range results {
		sourceCacheKey := c.generateSourceCacheKey(source.UID())
		c.sourceCache.Set(sourceCacheKey, source)
	}

	return results, nil
}

// generateSearchCacheKey creates a cache key for search results
func (c *CachedRegistry) generateSearchCacheKey(query string, topics []types.TopicTag) string {
	topicsStr := ""
	if len(topics) > 0 {
		topicStrs := make([]string, len(topics))
		for i, topic := range topics {
			topicStrs[i] = string(topic)
		}
		sort.Strings(topicStrs)
		topicsStr = strings.Join(topicStrs, ",")
	}
	return lib.HashParams("search", query, topicsStr)
}

// generateSourceCacheKey creates a cache key for individual source lookups
func (c *CachedRegistry) generateSourceCacheKey(uid activitytypes.TypedUID) string {
	return lib.HashParams("source", uid.String())
}
