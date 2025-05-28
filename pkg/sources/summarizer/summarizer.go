package summarizer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/common"
	"strings"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

//go:embed prompt.md
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
	activity common.Activity,
) (*common.ActivitySummary, error) {
	prompt := strings.Builder{}

	// static system prompt
	prompt.WriteString(systemPrompt)

	// format instructions
	parser, err := outputparser.NewDefined(completionResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}
	prompt.WriteString(`
───────────────────────────────
FORMAT INSTRUCTIONS
───────────────────────────────

`)
	prompt.WriteString(parser.GetFormatInstructions())

	// input
	input := completionInput{
		Title:     activity.Title(),
		Body:      activity.Body(),
		URL:       activity.URL(),
		CreatedAt: activity.CreatedAt().Format(time.RFC3339) + "Z",
	}
	serializedInput, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializing input: %w", err)
	}
	prompt.WriteString(`
───────────────────────────────
INPUT ACTIVITY TO PROCESS
───────────────────────────────

`)
	prompt.WriteString("```json\n")
	prompt.Write(serializedInput)
	prompt.WriteString("\n```\n")

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		llm.model,
		prompt.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("generate completion: %w", err)
	}

	// Parser expects backsticks but the output usually doesn't contain them
	wrappedOut := out
	if !strings.HasPrefix(out, "```json") {
		wrappedOut = fmt.Sprintf("```json\n%s\n```", out)
	}
	response, err := parser.Parse(wrappedOut)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &common.ActivitySummary{
		FullSummary:  response.FullSummary,
		ShortSummary: response.ShortSummary,
	}, nil
}
