package common

import "time"

// Activity TODO(pulse): Compute LLM summary
type Activity interface {
	UID() string
	SourceUID() string
	Title() string
	Body() string
	URL() string
	ImageURL() string
	CreatedAt() time.Time
}
