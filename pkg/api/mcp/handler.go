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
	FeedUID string `json:"feedUid"`
	Limit   *int   `json:"limit,omitempty"`
}

type GetFeedActivitiesOutput struct {
	Results []ActivityOutput `json:"results"`
}

type ActivityOutput struct {
	Title        string `json:"title" jsonschema:"The title of the activity"`
	Body         string `json:"body" jsonschema:"The main content or body text of the activity"`
	URL          string `json:"url" jsonschema:"The URL link to the original activity"`
	ShortSummary string `json:"shortSummary" jsonschema:"A brief summary of the activity"`
	CreatedAt    string `json:"createdAt" jsonschema:"The timestamp when the activity was created"`
}

type ListFeedsInput struct{}

type ListFeedsOutput struct {
	Feeds []FeedOutput `json:"feeds" jsonschema:"The list of available feeds for the user"`
}

type FeedOutput struct {
	UID        string   `json:"uid" jsonschema:"The unique identifier of the feed"`
	Name       string   `json:"name" jsonschema:"The display name of the feed"`
	Icon       string   `json:"icon" jsonschema:"The feed emoji character"`
	Query      string   `json:"query" jsonschema:"The search query or filter for more fine-grained feed"`
	SourceUids []string `json:"sourceUids" jsonschema:"The list of source IDs where the feed activities are pulled from"`
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
			Name:        "list_feeds",
			Description: "List all available feeds for the user",
		}, h.listFeeds)

		mcp.AddTool(mcpServer, &mcp.Tool{
			Name:        "list_feed_activities",
			Description: "Retrieve activities (posts, articles, etc.) from a specific feed with optional filtering and sorting",
		}, h.listFeedActivities)

		return mcpServer
	}

	return mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})
}

func (h *Handler) listFeeds(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListFeedsInput,
) (*mcp.CallToolResult, ListFeedsOutput, error) {
	feedList, err := h.feedRegistrt.ListByUserID(ctx, h.userID)
	if err != nil {
		return nil, ListFeedsOutput{}, fmt.Errorf("list feeds: %w", err)
	}

	feeds := make([]FeedOutput, len(feedList))
	for i, feed := range feedList {
		sourceUIDStrings := make([]string, len(feed.SourceUIDs))
		for j, uid := range feed.SourceUIDs {
			sourceUIDStrings[j] = uid.String()
		}

		feeds[i] = FeedOutput{
			UID:        feed.ID,
			Name:       feed.Name,
			Icon:       feed.Icon,
			Query:      feed.Query,
			SourceUids: sourceUIDStrings,
		}
	}

	return nil, ListFeedsOutput{
		Feeds: feeds,
	}, nil
}

func (h *Handler) listFeedActivities(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetFeedActivitiesInput,
) (*mcp.CallToolResult, GetFeedActivitiesOutput, error) {
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
		"",
		activitytypes.PeriodDay,
		false,
	)
	if err != nil {
		return nil, GetFeedActivitiesOutput{}, fmt.Errorf("list feed activities: %w", err)
	}

	activities := make([]ActivityOutput, len(out.Results))
	for i, activity := range out.Results {
		activities[i] = ActivityOutput{
			Title:        activity.Activity.Title(),
			URL:          activity.Activity.URL(),
			ShortSummary: activity.Summary.ShortSummary,
			CreatedAt:    activity.Activity.CreatedAt().Format(time.RFC3339),
		}
	}

	return nil, GetFeedActivitiesOutput{
		Results: activities,
	}, nil
}
