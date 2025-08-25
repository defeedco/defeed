package lib

import (
	"math/rand"
	"time"
)

// DefaultSourceTicker returns a ticker that ticks at a random time
// between 0 and 10% of the approximate duration.
//
// This is useful to avoid overwhelming the source API with requests.
// Use this as the default ticker for sources.
//
// TODO(sources): Implement more sophisticated rate source-specific rate limiting
func DefaultSourceTicker(_ time.Duration) *time.Ticker {

	// TODO(sources): Remove this override
	// Note: Set a fixed override duration for all sources for now
	approxDuration := 30 * time.Minute

	jitter := time.Duration(rand.Int63n(int64(approxDuration/10))) * time.Millisecond

	return time.NewTicker(approxDuration + jitter)
}
