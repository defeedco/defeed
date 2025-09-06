package sources

type Config struct {
	MaxActivityProcessorConcurrency int `env:"MAX_ACTIVITY_PROCESSOR_CONCURRENCY,default=10"`
}
