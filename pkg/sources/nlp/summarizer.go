package nlp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defeedco/defeed/pkg/sources/activities/types"
	"github.com/rs/zerolog"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

const (
	shortSummaryMaxWords = 20
	longSummaryMaxWords  = 200
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
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

func (s *Summarizer) SummarizeActivity(
	ctx context.Context,
	activity types.Activity,
) (*types.ActivitySummary, error) {
	// Preprocess input to reduce token count
	processedInput := s.activityToInput(activity)

	type result struct {
		summary string
		err     error
	}
	fullChan := make(chan result, 1)
	shortChan := make(chan result, 1)

	// Generate full and short summary in parallel
	go func() {
		fullSummary, err := s.summarizeWithRetry(ctx, processedInput, s.generateFullSummary, longSummaryMaxWords)
		fullChan <- result{summary: fullSummary, err: err}
	}()
	go func() {
		shortSummary, err := s.summarizeWithRetry(ctx, processedInput, s.generateShortSummary, shortSummaryMaxWords)
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

func (s *Summarizer) summarizeWithRetry(
	ctx context.Context,
	input summarizeActivityInput,
	summarizer func(ctx context.Context, input string) (string, error),
	maxWords int,
) (string, error) {

	out := s.formatActivityInput(input)
	for range 3 {
		curr, err := summarizer(ctx, out)
		if err != nil {
			return "", fmt.Errorf("summarizer: %w", err)
		}

		prevWords := wordCount(out)
		currWords := wordCount(curr)

		if currWords <= maxWords {
			return curr, nil
		}

		s.logger.Debug().
			Int("previous_words", prevWords).
			Int("current_words", currWords).
			Int("max_words", maxWords).
			Msg("summary too long, retrying")

		if currWords < prevWords {
			out = curr
		}
	}

	// fallback to the last summary
	return out, nil
}

func wordCount(s string) int {
	return len(strings.Split(s, " "))
}

func (s *Summarizer) generateFullSummary(ctx context.Context, input string) (string, error) {
	prompt := fmt.Sprintf(`You are a summarizer.

Rules:
- Be faithful to the input.
- Do NOT add new information.
- Use Markdown exactly as shown.
- Output ONLY the Markdown.
- Keep it under %d words.

Summarize the input in Markdown using EXACTLY these document sections:

<document>
### Context
(1-3 sentences)

### Key Points
- point 1
- point 2
- point 3

### Why it matters
(1-2 sentences)
</document>

Input:
%s

Output:
`, longSummaryMaxWords, input)

	out, err := s.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(s.logger, err, prompt, out, "Error generating full summary completion")
		return "", fmt.Errorf("generate full summary completion: %w", err)
	}

	return trimMarkdown(out), nil
}

func trimMarkdown(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "```markdown", "")
	s = strings.ReplaceAll(s, "```", "")
	return s
}

func (s *Summarizer) generateShortSummary(ctx context.Context, input string) (string, error) {
	prompt := fmt.Sprintf(`You are a summarizer.

Write ONE sentence of MAX %d WORDS about the input.

Rules:
- %d words or fewer.
- Plain text only.
- No explanations.
- If unsure, make it shorter.

Input:
%s

Output:
`, shortSummaryMaxWords, shortSummaryMaxWords, input)

	out, err := s.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(0.0),
	)
	if err != nil {
		logGenerateCompletionError(s.logger, err, prompt, out, "Error generating short summary completion")
		return "", fmt.Errorf("generate short summary completion: %w", err)
	}

	return strings.TrimSpace(out), nil
}

func (s *Summarizer) formatActivityInput(input summarizeActivityInput) string {
	inputJSON, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"title": "%s", "body": "%s", "url": "%s"}`,
			input.Title, input.Body, input.URL)
	}
	return string(inputJSON)
}

func (s *Summarizer) activityToInput(activity types.Activity) summarizeActivityInput {
	return summarizeActivityInput{
		Title: activity.Title(),
		Body:  activity.Body(),
		URL:   activity.URL(),
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

func (s *Summarizer) SummarizeTopic(ctx context.Context, topic *TopicQueryGroup, activities []*types.DecoratedActivity) (string, error) {
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
4. Output plain text, no Markdown or formatting.

Topic name: %s
Topic activities: %s

Activity summary:`, topic.Name, string(activitiesJSON))

	out, err := s.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		logGenerateCompletionError(s.logger, err, prompt, out, "Error generating topic summary completion")
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
