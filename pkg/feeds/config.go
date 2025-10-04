package feeds

type Config struct {
	// SummarizeTopics controls whether summaries are computed for each activity topic returned from GET /feed/{id}/activities
	SummarizeTopics bool `env:"SUMMARIZE_TOPICS,default=false"`
	// AllowQueryRewrite controls whether the userquery can be rewritten to sub-queries.
	// Note: query rewrites add cost and latency to the request.
	AllowQueryRewrite bool `env:"ALLOW_QUERY_REWRITE,default=true"`
	// MinSimilarity controls the minimum similarity score threeshold, when searching by query embedding.
	MinSimilarity float32 `env:"MIN_SIMILARITY,default=0.3"`
}
