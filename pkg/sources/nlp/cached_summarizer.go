package nlp

import (
	"context"
	"fmt"
	"time"

	"github.com/glanceapp/glance/pkg/lib"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

type CachedSummarizer struct {
	summarizer summarizer
	cache      *lib.Cache
	logger     *zerolog.Logger
}

type summarizer interface {
	SummarizeTopic(ctx context.Context, topic *TopicQueryGroup, activities []*types.DecoratedActivity) (string, error)
}

func NewCachedSummarizer(summarizer summarizer, ttl time.Duration, logger *zerolog.Logger) *CachedSummarizer {
	return &CachedSummarizer{
		summarizer: summarizer,
		cache:      lib.NewCache(ttl, logger),
		logger:     logger,
	}
}

func (cs *CachedSummarizer) SummarizeTopic(ctx context.Context, topic *TopicQueryGroup, activities []*types.DecoratedActivity) (string, error) {
	if len(activities) == 0 {
		return "", nil
	}

	key := fmt.Sprintf("topic_summary:%s", topic.Topic)

	if cached, found := cs.cache.Get(key); found {
		if summary, ok := cached.(string); ok {
			cs.logger.Debug().
				Str("topic", topic.Topic).
				Int("activity_count", len(activities)).
				Msg("topic summary cache hit")
			return summary, nil
		}
	}

	summary, err := cs.summarizer.SummarizeTopic(ctx, topic, activities)
	if err != nil {
		return "", err
	}

	cs.cache.Set(key, summary)
	cs.logger.Debug().
		Str("topic", topic.Topic).
		Int("activity_count", len(activities)).
		Msg("topic summary cached")

	return summary, nil
}
