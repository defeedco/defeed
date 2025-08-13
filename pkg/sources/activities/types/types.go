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
	Summary    *ActivitySummary
	Embedding  []float32
	Similarity float32
}

type ActivitiesSummary struct {
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
	SourceUIDs []string
	// Limit maximum number of results to return
	Limit int
	// SortBy specifies the field to sort results by (similarity or date)
	SortBy SortBy
}
