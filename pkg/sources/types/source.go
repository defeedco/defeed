package types

import (
	"context"
	"encoding/json"
	"strings"

	activitytypes "github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"
)

type Source interface {
	json.Marshaler
	json.Unmarshaler
	// UID is the unique identifier for the source.
	UID() activitytypes.TypedUID
	// Name is a short human-readable descriptor.
	// Example: "Programming Subreddit"
	Name() string
	// Description provides more context about the specific source parameters.
	// Example: "Top posts from r/programming"
	Description() string
	// URL is a web resource representation of UID.
	URL() string
	// Initialize initializes the internal state and prepares the logger.
	Initialize(logger *zerolog.Logger) error
	// Stream performs a one-time fetch of new activities from the source.
	// Since is the last activity emitted by the source.
	// Feed is a channel to send activities to. Already seen activities are permitted.
	// Err is a channel to send errors to.
	// The method should send data to the channels and return when done. The caller is responsible for closing the channels.
	Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, err chan<- error)
}

func IsFuzzyMatch(source Source, query string) bool {
	// Currently a very naive fuzzy match implementation.
	query = strings.ToLower(query)

	if strings.Contains(source.Name(), query) {
		return true
	}

	if strings.Contains(source.Description(), query) {
		return true
	}

	return false
}
