package types

import (
	"encoding/json"
	"github.com/glanceapp/glance/pkg/lib"
	"time"
)

type Activity interface {
	json.Marshaler
	json.Unmarshaler
	// UID is the unique identifier for the activity.
	// It should not contain any slashes.
	UID() lib.TypedUID
	SourceUID() lib.TypedUID
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
	Activity   Activity
	Summary    *ActivitySummary
	Embedding  []float32
	Similarity float32
}

type ActivitiesSummary struct {
	Overview   string
	Highlights []ActivityHighlight
}

type ActivityHighlight struct {
	Content           string
	SourceActivityIDs []string
}

type SortBy string

const (
	SortBySimilarity SortBy = "similarity"
	SortByDate       SortBy = "created_date"
)

type SearchRequest struct {
	QueryEmbedding []float32
	// MinSimilarity filters out entries with lower vector embedding similarity
	MinSimilarity float32
	// SourceUIDs ignored if empty
	SourceUIDs []lib.TypedUID
	// Limit maximum number of results to return
	Limit int
	// SortBy specifies the field to sort results by (similarity or date)
	SortBy SortBy
}
