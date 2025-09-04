package nlp

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/prompts"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

//go:embed summarize-prompt.md
var summarizeSinglePrompt string

type ActivitySummarizer struct {
	model  llms.Model
	logger *zerolog.Logger
}

func NewSummarizer(model llms.Model, logger *zerolog.Logger) *ActivitySummarizer {
	return &ActivitySummarizer{model: model, logger: logger}
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

func (sum *ActivitySummarizer) Summarize(
	ctx context.Context,
	activity types.Activity,
) (*types.ActivitySummary, error) {
	prompt := promptBuilder{}

	// static system prompt
	prompt.WriteString("SystemPrompt", summarizeSinglePrompt)

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
		sum.model,
		prompt.String(),
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(sum.logger, prompt.String(), err)
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
	Overview   string              `json:"overview" describe:"A concise one-paragraph overview of the overall direction and themes"`
	Highlights []activityHighlight `json:"highlights" describe:"A list of key highlights from the activities"`
}

type activityHighlight struct {
	Content   string   `json:"content" describe:"A concise highlight summarizing a key point"`
	SourceIDs []string `json:"source_ids" describe:"List of activity IDs that contributed to this highlight"`
}

type summarizeActivityInput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

type multiActivityInput struct {
	Activities []summarizeActivityInput `json:"activities"`
}

func (sum *ActivitySummarizer) SummarizeMany(
	ctx context.Context,
	activities []*types.DecoratedActivity,
	query string,
) (*types.ActivitiesSummary, error) {
	prompt := promptBuilder{}

	prompt.WriteString("SystemPrompt", `You are an expert at summarizing information and finding key insights that are relevant to the user's interests.
Your task is to analyze multiple activities and provide:
1. A concise one-paragraph overview that captures the overall direction and themes
2. Up to 5 key highlights that are most relevant to the user's query

Rules for the overview:
1. Write a single paragraph that captures the main themes and trends
2. Be direct and concise - avoid fillers like "These articles..."
3. Focus on the overall narrative, not individual details

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
		Activities: make([]summarizeActivityInput, len(activities)),
	}

	for i, activity := range activities {
		input.Activities[i] = summarizeActivityInput{
			ID:        activity.Activity.UID().String(),
			Title:     activity.Activity.Title(),
			Body:      activity.Activity.Body(),
			URL:       activity.Activity.URL(),
			CreatedAt: activity.Activity.CreatedAt().Format(time.RFC3339) + "Z",
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
		sum.model,
		prompt.String(),
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(sum.logger, prompt.String(), err)
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
		Overview:   response.Overview,
		Highlights: highlights,
		CreatedAt:  time.Now(),
	}, nil
}

// TODO: Refactor to langchain prompt templates
func (sum *ActivitySummarizer) SummarizeTopicActivities(ctx context.Context, topic *TopicQueryGroup, activities []*types.DecoratedActivity) (string, error) {
	if len(activities) == 0 {
		return "", nil
	}

	template := prompts.NewPromptTemplate(`You are an expert at analyzing and summarizing online activity information. 
Given a list of activities, generate the summary of key insights that are relevant for the given topic.

Guidelines:
1. Summaries should be 1-2 sentences that capture the main themes
2. Focus on the most important insights and trends for each topic
3. Be direct and informative in your summaries

Topic name: {{.topic_name}}
Topic description: {{.topic_description}}
Topic activities: {{.topic_activities}}

Activity summary:
`, []string{"topic_name", "topic_description", "topic_activities"})

	activitiesInput := multiActivityInput{}
	for _, activity := range activities {
		activitiesInput.Activities = append(activitiesInput.Activities, summarizeActivityInput{
			ID:        activity.Activity.UID().String(),
			Title:     activity.Activity.Title(),
			Body:      activity.Activity.Body(),
			URL:       activity.Activity.URL(),
			CreatedAt: activity.Activity.CreatedAt().Format(time.RFC3339) + "Z",

			// TODO(bart): Above is just a test, start using below code once the feed summary is removed to save costs.
			//Title:   activity.Activity.Title(),
			//Summary: activity.Summary.ShortSummary,
		})
	}

	activitiesJSON, err := json.MarshalIndent(activitiesInput, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal activities: %w", err)
	}

	prompt, err := template.Format(map[string]any{
		"topic_name":        topic.Topic,
		"topic_description": topic.Description,
		"topic_activities":  string(activitiesJSON),
	})
	if err != nil {
		return "", fmt.Errorf("format prompt: %w", err)
	}

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		sum.model,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		return "", fmt.Errorf("generate completion: %w", err)
	}

	return out, nil
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

func logGenerateCompletionError(logger *zerolog.Logger, prompt string, err error) {
	logger.Error().
		Err(err).
		// Log in base64 for a more compact representation
		Str("prompt_base64", base64.StdEncoding.EncodeToString([]byte(prompt))).
		Int("prompt_bytes", len(prompt)).
		Msg("Error generating completion")
}
