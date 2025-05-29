package types

import (
	"encoding/json"
	"time"
)

type Activity interface {
	json.Marshaler
	json.Unmarshaler
	UID() string
	SourceUID() string
	SourceType() string
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

type DecoratedActivity struct {
	Activity
	Summary *ActivitySummary
}
