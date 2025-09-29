package feeds

type Config struct {
	// SummarizeTopics controls whether summaries are computed for each activity topic returned from GET /feed/{id}/activities
	SummarizeTopics bool `env:"SUMMARIZE_TOPICS,default=false"`
}
