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

//go:embed summarize-activity.md
var summarizeActivityPrompt string

type ActivitySummarizer struct {
	model  llms.Model
	logger *zerolog.Logger
}

func NewSummarizer(model llms.Model, logger *zerolog.Logger) *ActivitySummarizer {
	return &ActivitySummarizer{model: model, logger: logger}
}

type summarizeActivityOutput struct {
	FullSummary  string `json:"full_summary" describe:"An extensive one-paragraph Markdown summary"`
	ShortSummary string `json:"short_summary" describe:"A concise one-line plain-text summary"`
}

type summarizeActivityInput struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

func (sum *ActivitySummarizer) SummarizeActivity(
	ctx context.Context,
	activity types.Activity,
) (*types.ActivitySummary, error) {
	template := prompts.NewPromptTemplate(summarizeActivityPrompt, []string{
		"output_format_instructions",
		"activity",
	})

	parser, err := outputparser.NewDefined(summarizeActivityOutput{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}

	activityInput, err := json.MarshalIndent(summarizeActivityInput{
		Title:     activity.Title(),
		Body:      activity.Body(),
		URL:       activity.URL(),
		CreatedAt: activity.CreatedAt().Format(time.RFC3339) + "Z",
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal activity input: %w", err)
	}

	prompt, err := template.Format(map[string]any{
		"output_format_instructions": parser.GetFormatInstructions(),
		"activity":                   string(activityInput),
	})
	if err != nil {
		return nil, fmt.Errorf("format prompt: %w", err)
	}

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		sum.model,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(sum.logger, prompt, err)
		return nil, fmt.Errorf("generate completion: %w", err)
	}

	response, err := parseResponse(parser, out)
	if err != nil {
		sum.logger.Error().
			Err(err).
			Str("prompt", prompt).
			Str("output", out).
			Msg("Error parsing activity summary response")
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &types.ActivitySummary{
		FullSummary:  response.FullSummary,
		ShortSummary: response.ShortSummary,
	}, nil
}

type topicSummaryActivityInput struct {
	Title        string `json:"title"`
	ShortSummary string `json:"short_summary"`
	LongSummary  string `json:"long_summary"`
}

type topicSummaryActivitiesInput struct {
	Activities []topicSummaryActivityInput `json:"activities"`
}

func (sum *ActivitySummarizer) SummarizeTopic(ctx context.Context, topic *TopicQueryGroup, activities []*types.DecoratedActivity) (string, error) {
	if len(activities) == 0 {
		return "", nil
	}

	template := prompts.NewPromptTemplate(`You are an expert at analyzing and summarizing online activity information. 
Given a list of activities, generate the summary of key insights that are relevant for the given topic.

Guidelines:
1. Summaries should be 1-3 sentences that capture the main high-level themes
2. Focus on the most important insights that are shared by the activities 
3. Be direct and informative in your summaries

Topic name: {{.topic_name}}
Topic description: {{.topic_description}}
Topic activities: {{.topic_activities}}

Activity summary:
`, []string{"topic_name", "topic_description", "topic_activities"})

	activitiesInput := topicSummaryActivitiesInput{}
	for _, activity := range activities {
		activitiesInput.Activities = append(activitiesInput.Activities, topicSummaryActivityInput{
			Title:        activity.Activity.Title(),
			ShortSummary: activity.Summary.ShortSummary,
			LongSummary:  activity.Summary.FullSummary,
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

func logGenerateCompletionError(logger *zerolog.Logger, prompt string, err error) {
	logger.Error().
		Err(err).
		// Log in base64 for a more compact representation
		Str("prompt_base64", base64.StdEncoding.EncodeToString([]byte(prompt))).
		Int("prompt_bytes", len(prompt)).
		Msg("Error generating completion")
}
