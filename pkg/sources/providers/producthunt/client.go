package producthunt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type Client struct {
	httpClient *http.Client
	apiToken   string
	logger     *zerolog.Logger
}

func NewClient(apiToken string, logger *zerolog.Logger) *Client {
	return &Client{
		httpClient: &http.Client{
			// ProductHunt API can take some more time to respond
			Timeout: 60 * time.Second,
		},
		apiToken: apiToken,
		logger:   logger,
	}
}

func (c *Client) FetchPosts(ctx context.Context, order PostOrder, limit int, timePeriod TimePeriod) ([]*PostNode, error) {
	query := c.buildPostsQuery(order, limit, timePeriod)
	return c.executeGraphQLQuery(ctx, query)
}

func (c *Client) buildPostsQuery(order PostOrder, limit int, timePeriod TimePeriod) string {
	orderStr := "VOTES"
	if order == PostOrderNewest {
		orderStr = "NEWEST"
	}

	dateFilter := ""
	if timePeriod != TimePeriodAll {
		dateFilter = c.buildDateFilter(timePeriod)
	}

	// Docs: http://api-v2-docs.producthunt.com.s3-website-us-east-1.amazonaws.com/object/post
	// Note: Selecting more than the first 5 comments, will lead to query complexity error.
	return fmt.Sprintf(`
		query {
			posts(order: %s, first: %d%s) {
				edges {
					node {
						id
						name
						tagline
						slug
						votesCount
						commentsCount
						createdAt
						description
						featuredAt
						isCollected
						isVoted
						reviewsCount
						reviewsRating
						url
						userId
						website
						thumbnail {
							type
							url
							videoUrl
						}
						media {
							type
							url
							videoUrl
						}
						productLinks {
							type
							url
						}
						comments(order: VOTES_COUNT, first: 5) {
							edges {
								node {
									id
									body
									votesCount
									createdAt
								}
							}
						}
					}
				}
			}
		}
	`, orderStr, limit, dateFilter)
}

func (c *Client) executeGraphQLQuery(ctx context.Context, query string) ([]*PostNode, error) {
	reqBody := map[string]string{
		"query": query,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.producthunt.com/v2/api/graphql", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %v", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %v", result.Errors)
	}

	products := make([]*PostNode, 0, len(result.Data.Posts.Edges))
	for _, edge := range result.Data.Posts.Edges {
		if edge.Node != nil {
			products = append(products, edge.Node)
		}
	}

	return products, nil
}

type PostOrder int

const (
	PostOrderVotes PostOrder = iota
	PostOrderNewest
)

func (o PostOrder) String() string {
	switch o {
	case PostOrderVotes:
		return "VOTES"
	case PostOrderNewest:
		return "NEWEST"
	default:
		return "VOTES"
	}
}

type TimePeriod int

const (
	TimePeriodToday TimePeriod = iota
	TimePeriodWeek
	TimePeriodMonth
	TimePeriodAll
)

func (tp TimePeriod) String() string {
	switch tp {
	case TimePeriodToday:
		return "today"
	case TimePeriodWeek:
		return "week"
	case TimePeriodMonth:
		return "month"
	case TimePeriodAll:
		return "all"
	default:
		return "today"
	}
}

func (c *Client) buildDateFilter(timePeriod TimePeriod) string {
	now := time.Now()
	var startDate time.Time

	switch timePeriod {
	case TimePeriodToday:
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case TimePeriodWeek:
		startDate = now.AddDate(0, 0, -7)
	case TimePeriodMonth:
		startDate = now.AddDate(0, -1, 0)
	default:
		return ""
	}

	// Format as ISO 8601 date string
	dateStr := startDate.Format("2006-01-02")
	return fmt.Sprintf(`, postedAfter: "%s"`, dateStr)
}

type GraphQLResponse struct {
	Data   Data           `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLError struct {
	Message string `json:"message"`
}

type Data struct {
	Posts PostConnection `json:"posts"`
}

type PostConnection struct {
	Edges []PostEdge `json:"edges"`
}

type PostEdge struct {
	Node *PostNode `json:"node"`
}

// Docs: http://api-v2-docs.producthunt.com.s3-website-us-east-1.amazonaws.com/object/post
type PostNode struct {
	ID            string             `json:"id"`
	Name          string             `json:"name"`
	Tagline       string             `json:"tagline"`
	Slug          string             `json:"slug"`
	VotesCount    int                `json:"votesCount"`
	CommentsCount int                `json:"commentsCount"`
	CreatedAt     time.Time          `json:"createdAt"`
	Thumbnail     *MediaNode         `json:"thumbnail"`
	WebsiteURL    string             `json:"website"`
	Description   string             `json:"description"`
	FeaturedAt    *time.Time         `json:"featuredAt"`
	IsCollected   bool               `json:"isCollected"`
	IsVoted       bool               `json:"isVoted"`
	ReviewsCount  int                `json:"reviewsCount"`
	ReviewsRating float64            `json:"reviewsRating"`
	URL           string             `json:"url"`
	UserID        string             `json:"userId"`
	Media         []*MediaNode       `json:"media"`
	ProductLinks  []*ProductLinkNode `json:"productLinks"`
	Comments      *CommentConnection `json:"comments"`
}

type CommentConnection struct {
	// Note: The first comment is usually posted by the maker as a note.
	Edges []CommentEdge `json:"edges"`
}

type CommentEdge struct {
	Node *CommentNode `json:"node"`
}

type CommentNode struct {
	ID         string    `json:"id"`
	Body       string    `json:"body"`
	VotesCount int       `json:"votesCount"`
	CreatedAt  time.Time `json:"createdAt"`
}

type MediaNode struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	VideoURL string `json:"videoUrl"`
}

type ProductLinkNode struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}
