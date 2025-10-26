package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/defeedco/defeed/pkg/feeds"
	activitytypes "github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"
)

type Handler struct {
	userID       string
	feedRegistrt *feeds.Registry
	logger       *zerolog.Logger
}

type GetFeedActivitiesInput struct {
	FeedUID string  `json:"feedUid" jsonschema:"The unique identifier of the feed"`
	Period  *string `json:"period,omitempty" jsonschema:"Time period to filter activities (all/month/week/day)"`
	Query   *string `json:"query,omitempty" jsonschema:"Filter query to override the default feed query"`
	Limit   *int    `json:"limit,omitempty" jsonschema:"Maximum number of activities to return (default 20)"`
}

type GetFeedActivitiesOutput struct {
	Results []ActivityOutput `json:"results" jsonschema:"List of activities"`
}

type ActivityOutput struct {
	Title              string   `json:"title" jsonschema:"The title of the activity"`
	Body               string   `json:"body" jsonschema:"The main content or body text of the activity"`
	URL                string   `json:"url" jsonschema:"The URL link to the original activity"`
	ShortSummary       string   `json:"shortSummary" jsonschema:"A brief summary of the activity"`
	FullSummary        string   `json:"fullSummary" jsonschema:"A comprehensive summary of the activity"`
	CreatedAt          string   `json:"createdAt" jsonschema:"The timestamp when the activity was created"`
	UpvotesCount       int      `json:"upvotesCount" jsonschema:"The number of upvotes received by the activity"`
	CommentsCount      int      `json:"commentsCount" jsonschema:"The number of comments on the activity"`
	AmplificationCount int      `json:"amplificationCount" jsonschema:"The number of times the activity was amplified or shared"`
	Similarity         *float32 `json:"similarity,omitempty" jsonschema:"Similarity score of the activity (if available)"`
}

func NewHandler(
	userID string,
	feedRegistry *feeds.Registry,
	logger *zerolog.Logger,
) http.Handler {
	h := &Handler{
		userID:       userID,
		feedRegistrt: feedRegistry,
		logger:       logger,
	}

	getServer := func(r *http.Request) *mcp.Server {
		logger.Debug().
			Str("user_id", h.userID).
			Msg("Creating new MCP server instance for request")

		mcpServer := mcp.NewServer(&mcp.Implementation{
			Name:    "defeed-mcp-server",
			Version: "v0.1.0",
		}, nil)

		mcp.AddTool(mcpServer, &mcp.Tool{
			Name:        "get_feed_activities",
			Description: "Retrieve activities (posts, articles, etc.) from a specific feed with optional filtering and sorting",
		}, h.getFeedActivities)

		return mcpServer
	}

	return mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})
}

func (h *Handler) getFeedActivities(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetFeedActivitiesInput,
) (*mcp.CallToolResult, GetFeedActivitiesOutput, error) {
	var queryOverride string
	if input.Query != nil {
		queryOverride = *input.Query
	}

	limit := 20
	if input.Limit != nil {
		limit = *input.Limit
	}

	out, err := h.feedRegistrt.Activities(
		ctx,
		input.FeedUID,
		h.userID,
		activitytypes.SortByWeightedScore,
		limit,
		queryOverride,
		activitytypes.PeriodAll,
		false,
	)
	if err != nil {
		return nil, GetFeedActivitiesOutput{}, fmt.Errorf("list feed activities: %w", err)
	}

	activities := make([]ActivityOutput, len(out.Results))
	for i, activity := range out.Results {
		activities[i] = ActivityOutput{
			Title:              activity.Activity.Title(),
			Body:               activity.Activity.Body(),
			URL:                activity.Activity.URL(),
			ShortSummary:       activity.Summary.ShortSummary,
			FullSummary:        activity.Summary.FullSummary,
			CreatedAt:          activity.Activity.CreatedAt().Format(time.RFC3339),
			UpvotesCount:       activity.Activity.UpvotesCount(),
			CommentsCount:      activity.Activity.CommentsCount(),
			AmplificationCount: activity.Activity.AmplificationCount(),
			Similarity:         &activity.Similarity,
		}
	}

	return nil, GetFeedActivitiesOutput{
		Results: activities,
	}, nil
}
