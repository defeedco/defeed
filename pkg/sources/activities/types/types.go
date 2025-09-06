package types

import (
	"encoding/json"
	"time"
)

type Activity interface {
	json.Marshaler
	json.Unmarshaler
	// UID is the unique identifier for the activity.
	// It should not contain any slashes.
	UID() TypedUID
	SourceUID() TypedUID
	Title() string
	Body() string
	URL() string
	ImageURL() string
	CreatedAt() time.Time
}

// TypedUID is a semi-structured ID format for easy resource type extraction.
type TypedUID interface {
	json.Marshaler
	json.Unmarshaler
	Type() string
	String() string
}

type ActivitySummary struct {
	ShortSummary string
	FullSummary  string
}

type DecoratedActivity struct {
	Activity   Activity
	Summary    *ActivitySummary
	Embedding  []float32
	Similarity float32
}
