package nlp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"github.com/rs/zerolog"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

type Summarizer struct {
	model  completionModel
	logger *zerolog.Logger
}

func NewSummarizer(model completionModel, logger *zerolog.Logger) *Summarizer {
	return &Summarizer{
		model:  model,
		logger: logger,
	}
}

type completionModel interface {
	Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error)
}

type summarizeActivityInput struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
}

func (sum *Summarizer) SummarizeActivity(
	ctx context.Context,
	activity types.Activity,
) (*types.ActivitySummary, error) {
	// Preprocess input to reduce token count
	processedInput := sum.activityToInput(activity)

	type result struct {
		summary string
		err     error
	}
	fullChan := make(chan result, 1)
	shortChan := make(chan result, 1)

	// Generate full and short summary in parallel
	go func() {
		fullSummary, err := sum.generateFullSummary(ctx, processedInput)
		fullChan <- result{summary: fullSummary, err: err}
	}()
	go func() {
		shortSummary, err := sum.generateShortSummary(ctx, processedInput)
		shortChan <- result{summary: shortSummary, err: err}
	}()

	// Wait for both results
	fullResult := <-fullChan
	shortResult := <-shortChan

	// Check for errors
	if fullResult.err != nil {
		return nil, fmt.Errorf("generate full summary: %w", fullResult.err)
	}
	if shortResult.err != nil {
		return nil, fmt.Errorf("generate short summary: %w", shortResult.err)
	}

	return &types.ActivitySummary{
		FullSummary:  fullResult.summary,
		ShortSummary: shortResult.summary,
	}, nil
}

func (sum *Summarizer) generateFullSummary(ctx context.Context, input summarizeActivityInput) (string, error) {
	prompt := fmt.Sprintf(`You are a summarizer. Summarize the provided JSON faithfully and concisely.

Rules:
- Use only title, body, created_at.
- Use **bold** for key terms, `+"`code`"+` for identifiers, hyperlinks for URLs.
- Include date only if relevant.
- Max 80 words.
- Format as Markdown with Context/Key Points/Why it matters structure (titles formatted with ###).

Input:
%s

Output only the summary text, no JSON formatting.`, sum.formatActivityInput(input))

	out, err := sum.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(sum.logger, err, prompt, out, "Error generating full summary completion")
		return "", fmt.Errorf("generate full summary completion: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func (sum *Summarizer) generateShortSummary(ctx context.Context, input summarizeActivityInput) (string, error) {
	prompt := fmt.Sprintf(`You are a summarizer. Create a concise summary of the provided JSON.

Rules:
- Use only title, body, created_at.
- Max 20 words.
- Plain text, no Markdown.
- Capture the essence.

Input:
%s

Output only the summary text, no JSON formatting.`, sum.formatActivityInput(input))

	out, err := sum.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(sum.logger, err, prompt, out, "Error generating short summary completion")
		return "", fmt.Errorf("generate short summary completion: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func (sum *Summarizer) formatActivityInput(input summarizeActivityInput) string {
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"title": "%s", "body": "%s", "url": "%s", "created_at": "%s"}`,
			input.Title, input.Body, input.URL, input.CreatedAt)
	}
	return string(inputJSON)
}

func (sum *Summarizer) activityToInput(activity types.Activity) summarizeActivityInput {
	return summarizeActivityInput{
		Title:     activity.Title(),
		Body:      activity.Body(),
		URL:       activity.URL(),
		CreatedAt: activity.CreatedAt().Format(time.RFC3339) + "Z",
	}
}

type topicSummaryActivityInput struct {
	Title        string `json:"title"`
	ShortSummary string `json:"short_summary"`
	LongSummary  string `json:"long_summary"`
}

type topicSummaryActivitiesInput struct {
	Activities []topicSummaryActivityInput `json:"activities"`
}

func (sum *Summarizer) SummarizeTopic(ctx context.Context, topic *TopicQueryGroup, activities []*types.DecoratedActivity) (string, error) {
	if len(activities) == 0 {
		return "", nil
	}

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

	prompt := fmt.Sprintf(`You are an expert at analyzing and summarizing online activity information. 
Given a list of activities, generate the summary of key insights that are relevant for the given topic.

Guidelines:
1. Summaries should be 1-3 sentences that capture the main high-level themes
2. Focus on the most important insights that are shared by the activities 
3. Be direct and informative in your summaries

Topic name: %s
Topic description: %s
Topic activities: %s

Activity summary:`, topic.Topic, topic.Description, string(activitiesJSON))

	out, err := sum.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(sum.logger, err, prompt, out, "Error generating topic summary completion")
		return "", fmt.Errorf("generate topic summary completion: %w", err)
	}

	return strings.TrimSpace(out), nil
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

func logGenerateCompletionError(logger *zerolog.Logger, err error, prompt, out, msg string) {
	logger.Error().
		Err(err).
		// Log in base64 for a more compact representation
		Str("prompt_base64", base64.StdEncoding.EncodeToString([]byte(prompt))).
		Int("prompt_bytes", len(prompt)).
		Str("output_base64", base64.StdEncoding.EncodeToString([]byte(out))).
		Int("output_bytes", len(out)).
		Msg(msg)
}
