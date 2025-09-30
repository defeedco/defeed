package types

// SearchRequest represents a search query for activities
type SearchRequest struct {
	SourceUIDs        []TypedUID
	ActivityUIDs      []TypedUID
	MinSimilarity     float32
	Limit             int
	Cursor            string
	SortBy            SortBy
	Period            Period
	QueryEmbedding    []float32
	SimilarityWeight  float64
	SocialScoreWeight float64
	RecencyWeight     float64
}

// SearchResult represents paginated search results
type SearchResult struct {
	Activities []*DecoratedActivity
	// NextCursor is the cursor to use for the next page of results
	NextCursor string
	// HasMore is whether there are more results available
	HasMore bool
}

// SortBy defines how to sort search results
type SortBy string

const (
	SortBySimilarity    SortBy = "similarity"
	SortByDate          SortBy = "date"
	SortBySocialScore   SortBy = "social_score"
	SortByWeightedScore SortBy = "weighted_score"
)

// Period defines time periods for filtering activities
type Period string

const (
	PeriodAll   Period = "all"
	PeriodMonth Period = "month"
	PeriodWeek  Period = "week"
	PeriodDay   Period = "day"
)
