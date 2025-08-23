package types

import (
	"context"
	"encoding/json"
	types2 "github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

type Source interface {
	json.Marshaler
	json.Unmarshaler
	// UID is the unique identifier for the source.
	// It should not contain any slashes.
	UID() string
	// Type is the non-parameterized ID (e.g. "reddit:subreddit" vs "reddit:subreddit:top:day")
	Type() string
	// Name is a short human-readable descriptor.
	// Example: "Programming Subreddit"
	Name() string
	// Description provides more context about the specific source parameters.
	// Example: "Top posts from r/programming"
	Description() string
	// URL is a web resource representation of UID.
	URL() string
	// Validate returns a list of configuration validation errors.
	// When non-empty, the caller should not proceed to Initialize.
	Validate() []error
	// Initialize initializes the internal state and prepares the logger.
	Initialize(logger *zerolog.Logger) error
	// Stream starts streaming new activities from the source.
	// Since is the last activity emitted by the source.
	// Feed is a channel to send activities to. Already seen activities are permitted.
	// Err is a channel to send errors to.
	// The caller should close the channels when done.
	Stream(ctx context.Context, since types2.Activity, feed chan<- types2.Activity, err chan<- error)
}
