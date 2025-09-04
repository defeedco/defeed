package nlp

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/outputparser"
)

type QueryRewriter struct {
	model llms.Model
}

func NewQueryRewriter(model llms.Model) *QueryRewriter {
	return &QueryRewriter{model: model}
}

type TopicQueryGroup struct {
	Topic       string   `json:"topic" describe:"Clear and concise topic name that describes the theme"`
	Queries     []string `json:"queries" describe:"Re-written sub-queries for the topic (max 1-3)"`
	Description string   `json:"description" describe:"Brief description of what this topic covers"`
}

type queryRewriteResponse struct {
	Topics []*TopicQueryGroup `json:"topics" describe:"List of topic-based queries, maximum 5 topics"`
}

type queryRewriteInput struct {
	OriginalQuery string `json:"original_query"`
}

func (qr *QueryRewriter) RewriteToTopics(ctx context.Context, originalQuery string) ([]*TopicQueryGroup, error) {
	prompt := promptBuilder{}

	systemPrompt := `You are an AI assistant tasked with reformulating user queries to improve retrieval in a RAG system. The system searches embeddings of online activity summaries. 

Given the original query, rewrite it into multiple topic-based queries that are more specific, detailed, and likely to retrieve relevant information.

Guidelines:
1. Break down the original query into 2-5 distinct topics/themes
2. Each topic should have a clear, descriptive name
3. Each topic should have 1-3 specific queries as an array
4. Focus on different aspects or angles of the original query
5. Make queries more specific than the original to get better retrieval results
6. If the original query is already very specific, create related topics that would be of interest

Example:
Original: "AI developments"
Topics:
- topic: "Machine Learning Breakthroughs", queries: ["recent machine learning research breakthroughs", "neural networks deep learning advances", "ML model optimization techniques"]
- topic: "AI Industry News", queries: ["artificial intelligence industry news", "company announcements funding AI startups", "AI market trends"]
- Topic: "AI Ethics and Regulation", queries: ["AI ethics artificial intelligence regulation", "policy governance responsible AI", "AI safety guidelines"]
`

	prompt.WriteString("SystemPrompt", systemPrompt)

	parser, err := outputparser.NewDefined(queryRewriteResponse{})
	if err != nil {
		return nil, fmt.Errorf("creating parser: %w", err)
	}
	prompt.WriteString("FormatInstructions", parser.GetFormatInstructions())

	input := queryRewriteInput{
		OriginalQuery: originalQuery,
	}
	if err := prompt.WriteJSON("Input", input); err != nil {
		return nil, fmt.Errorf("write json: %w", err)
	}

	out, err := llms.GenerateFromSinglePrompt(
		ctx,
		qr.model,
		prompt.String(),
		// Note: Fixed temperature of 1 must be applied for gpt-5-mini
		llms.WithTemperature(1.0),
	)
	if err != nil {
		return nil, fmt.Errorf("generate completion: %w", err)
	}

	response, err := parseResponse(parser, out)
	if err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return response.Topics, nil
}
