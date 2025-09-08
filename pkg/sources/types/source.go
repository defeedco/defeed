package types

import (
	"context"
	"encoding/json"

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
	// Icon returns the favicon URL for the source.
	Icon() string
	// Topics return the niche topic tags this source is relevant for.
	Topics() []TopicTag
	// Initialize stores the logger and initializes the internal state given config.
	// The caller should validate the config before usage.
	Initialize(logger *zerolog.Logger, config *ProviderConfig) error
	// Stream performs a one-time fetch of new activities from the source.
	// Since is the last activity emitted by the source.
	// Feed is a channel to send activities to. Already seen activities are permitted.
	// Err is a channel to send errors to.
	// The method should send data to the channels and return when done. The caller is responsible for closing the channels.
	Stream(ctx context.Context, since activitytypes.Activity, feed chan<- activitytypes.Activity, err chan<- error)
}
