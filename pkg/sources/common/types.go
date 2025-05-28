package common

import "time"

type Activity interface {
	UID() string
	SourceUID() string
	Title() string
	Body() string
	URL() string
	ImageURL() string
	CreatedAt() time.Time
}

type ActivitySummary struct {
	ShortSummary string
	FullSummary  string
}
