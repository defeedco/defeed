package nlp

import (
	"context"
	"encoding/json"
	"fmt"

	sourcetypes "github.com/glanceapp/glance/pkg/sources/types"
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
	Name    string   `json:"name" describe:"Clear and concise topic name that describes the theme"`
	Emoji   string   `json:"emoji" describe:"Emoji that best represents the topic"`
	Queries []string `json:"queries" describe:"Re-written sub-queries for the topic (max 1-3)"`
}

type RewriteRequest struct {
	Query   string
	Sources []sourcetypes.Source
}

func (qr *QueryRewriter) RewriteToTopics(ctx context.Context, req RewriteRequest) ([]*TopicQueryGroup, error) {
	template := prompts.NewPromptTemplate(`You are an AI assistant tasked with reformulating user queries to improve retrieval in a RAG system. The system searches embeddings of online activity summaries.
## Task
Given the original query, rewrite it into multiple topic-based queries that are more specific, detailed, and likely to retrieve relevant information from the provided sources.

Guidelines:
1. Break down the original query into 2-5 distinct and diverse topics
2. Each topic should have a clear, descriptive name
3. Each topic should have 1-3 specific queries as an array
4. Each query should include representative emoji
5. Focus on different aspects or angles of the original query
6. Make queries more specific than the original to get better retrieval results
7. If the original query is already very specific, create related topics that would be of interest

## Output format

{{.output_format_instructions}}

## Input

Original (user) query: {{.original_query}}
Sources of activity data: {{.sources}}

## Output
`, []string{
		"output_format_instructions",
		"original_query",
		"sources",
	})

	type queryRewriteResponse struct {
		// Note: fields should not be pointers, or the format instructions won't include them
		Topics []TopicQueryGroup `json:"topics" describe:"List of topic-based queries, maximum 5 topics"`
	}

	parser, err := outputparser.NewDefined(queryRewriteResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}

	sourcesJSON, err := serializeSources(req.Sources)
	if err != nil {
		return nil, fmt.Errorf("serialize sources: %w", err)
	}

	prompt, err := template.Format(map[string]any{
		"output_format_instructions": parser.GetFormatInstructions(),
		"original_query":             req.Query,
		"sources":                    sourcesJSON,
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

	// TODO: validate the response and retry if invalid
	response, err := parseResponse(parser, out)
	if err != nil {
		qr.logger.Error().
			Err(err).
			Str("prompt", prompt).
			Str("output", out).
			Msg("Error parsing query rewrite response")
		return nil, fmt.Errorf("parse response: %w", err)
	}

	topics := make([]*TopicQueryGroup, len(response.Topics))
	for i, topic := range response.Topics {
		topics[i] = &topic
	}

	return topics, nil
}

type sourceInput struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func serializeSources(sources []sourcetypes.Source) (string, error) {
	sourceInfos := make([]sourceInput, len(sources))
	for i, source := range sources {
		sourceInfos[i] = sourceInput{
			Type: source.UID().Type(),
			Name: source.Name(),
		}
	}

	jsonBytes, err := json.Marshal(sourceInfos)
	if err != nil {
		return "", fmt.Errorf("marshal sources: %w", err)
	}

	return string(jsonBytes), nil
}
