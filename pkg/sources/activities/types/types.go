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
	// UpvotesCount is the number of upvotes/likes. -1 if not available.
	UpvotesCount() int
	// DownvotesCount is the number of downvotes/dislikes. -1 if not available.
	DownvotesCount() int
	// CommentsCount is the number of comments/discussions. -1 if not available.
	CommentsCount() int
	// AmplificationCount is the number of shares/reposts/forks/etc. -1 if not available.
	AmplificationCount() int
	// SocialScore is a source-specific score to measure the activity's social engagement.
	// Range is 0-1. -1 if not available.
	SocialScore() float64
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
