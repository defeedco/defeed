package nlp

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/tmc/langchaingo/prompts"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

type QueryRewriter struct {
	model  completionModel
	logger *zerolog.Logger
}

func NewQueryRewriter(model completionModel, logger *zerolog.Logger) *QueryRewriter {
	return &QueryRewriter{model: model, logger: logger}
}

type TopicQueryGroup struct {
	Topic       string   `json:"topic" describe:"Clear and concise topic name that describes the theme"`
	Queries     []string `json:"queries" describe:"Re-written sub-queries for the topic (max 1-3)"`
	Description string   `json:"description" describe:"Brief description of what this topic covers"`
}

type queryRewriteResponse struct {
	Topics []*TopicQueryGroup `json:"topics" describe:"List of topic-based queries, maximum 5 topics"`
}

func (qr *QueryRewriter) RewriteToTopics(ctx context.Context, originalQuery string) ([]*TopicQueryGroup, error) {
	template := prompts.NewPromptTemplate(`You are an AI assistant tasked with reformulating user queries to improve retrieval in a RAG system. The system searches embeddings of online activity summaries. 
## Task
Given the original query, rewrite it into multiple topic-based queries that are more specific, detailed, and likely to retrieve relevant information.

Guidelines:
1. Break down the original query into 2-5 distinct topics/themes
2. Each topic should have a clear, descriptive name
3. Each topic should have 1-3 specific queries as an array
4. Focus on different aspects or angles of the original query
5. Make queries more specific than the original to get better retrieval results
6. If the original query is already very specific, create related topics that would be of interest

## Example
Original: "AI developments"
Topics:
- topic: "Machine Learning Breakthroughs", queries: ["recent machine learning research breakthroughs", "neural networks deep learning advances", "ML model optimization techniques"]
- topic: "AI Industry News", queries: ["artificial intelligence industry news", "company announcements funding AI startups", "AI market trends"]
- Topic: "AI Ethics and Regulation", queries: ["AI ethics artificial intelligence regulation", "policy governance responsible AI", "AI safety guidelines"]

## Output format

{{.output_format_instructions}}

## Input

Original (user) query: {{.original_query}}

## Output
`, []string{
		"output_format_instructions",
		"original_query",
	})

	parser, err := outputparser.NewDefined(queryRewriteResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}

	prompt, err := template.Format(map[string]any{
		"output_format_instructions": parser.GetFormatInstructions(),
		"original_query":             originalQuery,
	})
	if err != nil {
		return nil, fmt.Errorf("format prompt: %w", err)
	}

	out, err := qr.model.Call(
		ctx,
		prompt,
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		return nil, fmt.Errorf("generate completion: %w", err)
	}

	response, err := parseResponse(parser, out)
	if err != nil {
		qr.logger.Error().
			Err(err).
			Str("prompt", prompt).
			Str("output", out).
			Msg("Error parsing query rewrite response")
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return response.Topics, nil
}
