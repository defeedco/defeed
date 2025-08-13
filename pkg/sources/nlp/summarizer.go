package nlp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

//go:embed summarize-prompt.md
var systemPrompt string

type ActivitySummarizer struct {
	model llms.Model
}

func NewSummarizer(model llms.Model) *ActivitySummarizer {
	return &ActivitySummarizer{model: model}
}

type completionResponse struct {
	FullSummary  string `json:"full_summary" describe:"An extensive one-paragraph Markdown summary"`
	ShortSummary string `json:"short_summary" describe:"A concise one-line plain-text summary"`
}

type completionInput struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

func (llm *ActivitySummarizer) Summarize(
	ctx context.Context,
	activity types.Activity,
) (*types.ActivitySummary, error) {
	prompt := promptBuilder{}

	// static system prompt
	prompt.WriteString("SystemPrompt", systemPrompt)

	parser, err := outputparser.NewDefined(completionResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}
	prompt.WriteString("FormatInstructions", parser.GetFormatInstructions())

	input := completionInput{
		Title:     activity.Title(),
		Body:      activity.Body(),
		URL:       activity.URL(),
		CreatedAt: activity.CreatedAt().Format(time.RFC3339) + "Z",
	}
	prompt.WriteJSON("ActivityToProcess", input)

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		llm.model,
		prompt.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("generate completion: %w", err)
	}

	response, err := parseResponse(parser, out)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &types.ActivitySummary{
		FullSummary:  response.FullSummary,
		ShortSummary: response.ShortSummary,
	}, nil
}

type multiActivityCompletionResponse struct {
	Highlights []activityHighlight `json:"highlights" describe:"A list of key highlights from the activities"`
}

type activityHighlight struct {
	Content   string   `json:"content" describe:"A concise highlight summarizing a key point"`
	SourceIDs []string `json:"source_ids" describe:"List of activity IDs that contributed to this highlight"`
}

type multiActivityInputActivity struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

type multiActivityInput struct {
	Activities []multiActivityInputActivity `json:"activities"`
}

func (llm *ActivitySummarizer) SummarizeMany(
	ctx context.Context,
	activities []*types.DecoratedActivity,
	query string,
) (*types.ActivitiesSummary, error) {
	prompt := promptBuilder{}

	prompt.WriteString("SystemPrompt", `You are an expert at summarizing information and finding key insights that are relevant to the user's interests.
Your task is to analyze multiple activities and extract up to 5 key highlights that are most relevant to the user's query.

Rules for generating highlights:
1. Each highlight should be extremely concise - preferably one line
2. Group related activities into a single highlight when they discuss the same concept or news
3. Write in a direct, straightforward style - avoid fillers like "This article..."
4. Focus only on information relevant to the user's query (if provided)
5. Limit output to maximum 5 highlights, even if there are more activities

For each highlight, you must also list the IDs of the source activities that contributed to that insight.
`)

	parser, err := outputparser.NewDefined(multiActivityCompletionResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}

	prompt.WriteString("FormatInstructions", parser.GetFormatInstructions())

	input := multiActivityInput{
		Activities: make([]multiActivityInputActivity, len(activities)),
	}

	for i, activity := range activities {
		input.Activities[i] = multiActivityInputActivity{
			ID:        activity.UID(),
			Title:     activity.Title(),
			Body:      activity.Body(),
			URL:       activity.URL(),
			CreatedAt: activity.CreatedAt().Format(time.RFC3339) + "Z",
		}
	}

	if err := prompt.WriteJSON("ActivitiesToProcess", input); err != nil {
		return nil, fmt.Errorf("write json: %w", err)
	}

	if query != "" {
		prompt.WriteString("UserQuery", query)
	}

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		llm.model,
		prompt.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("generate completion: %w", err)
	}

	response, err := parseResponse(parser, out)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	highlights := make([]types.ActivityHighlight, len(response.Highlights))
	for i, h := range response.Highlights {
		highlights[i] = types.ActivityHighlight{
			Content:           h.Content,
			SourceActivityIDs: h.SourceIDs,
		}
	}

	return &types.ActivitiesSummary{
		Highlights: highlights,
	}, nil
}

func parseResponse[T any](parser outputparser.Defined[T], response string) (*T, error) {
	// Parser expects backsticks but the output usually doesn't contain them
	wrappedRes := response
	if !strings.HasPrefix(response, "```json") {
		wrappedRes = fmt.Sprintf("```json\n%s\n```", response)
	}
	out, err := parser.Parse(wrappedRes)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return &out, nil
}

type promptBuilder struct {
	prompt strings.Builder
}

func (p *promptBuilder) WriteString(tag string, s string) {
	p.prompt.WriteString(fmt.Sprintf("<%s>\n", tag))
	p.prompt.WriteString(s)
	p.prompt.WriteString(fmt.Sprintf("</%s>\n", tag))
}

func (p *promptBuilder) WriteJSON(tag string, v any) error {
	serialized, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	p.prompt.WriteString(fmt.Sprintf("<%s>\n", tag))
	p.prompt.WriteString("```json\n")
	p.prompt.Write(serialized)
	p.prompt.WriteString("\n```\n")
	p.prompt.WriteString(fmt.Sprintf("</%s>\n", tag))
	return nil
}

func (p *promptBuilder) String() string {
	return p.prompt.String()
}
