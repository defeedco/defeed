package lib

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// UsageMetrics represents the token usage and cost information from OpenAI API
type UsageMetrics struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	// ReasoningTokens are part of completion tokens
	ReasoningTokens int       `json:"reasoningTokens"`
	TotalTokens     int       `json:"totalTokens"`
	PromptCost      float64   `json:"promptCost"`
	CompletionCost  float64   `json:"completionCost"`
	ReasoningCost   float64   `json:"reasoningCost"`
	TotalCost       float64   `json:"totalCost"`
	Model           string    `json:"model"`
	Timestamp       time.Time `json:"timestamp"`
}

// UsageTracker handles tracking OpenAI API usage and costs
type UsageTracker struct {
	logger  *zerolog.Logger
	metrics []UsageMetrics
	mu      sync.RWMutex
	pricing map[string]ModelPricing
}

// ModelPricing defines the cost per token for different models
type ModelPricing struct {
	InputCostPer1MTokens  float64
	OutputCostPer1MTokens float64
}

// NewUsageTracker creates a new usage tracker instance
func NewUsageTracker(logger *zerolog.Logger) *UsageTracker {
	return &UsageTracker{
		logger:  logger,
		metrics: make([]UsageMetrics, 0),
		pricing: getDefaultPricing(),
	}
}

// response docs: https://platform.openai.com/docs/api-reference/completions/object#completions/object-usage
type response struct {
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	}
	Model  string `json:"model"`
	Object string `json:"object"`
	ID     string `json:"id"`
}

// TrackUsage processes an OpenAI API response and extracts usage metrics
// It reads the response body for tracking and then replaces it with a new reader
// so the caller can still read the same data
func (ut *UsageTracker) TrackUsage(resp *http.Response) (*UsageMetrics, error) {
	if resp == nil || resp.Body == nil {
		return nil, fmt.Errorf("response or response body is nil")
	}

	// Read the response body for usage tracking
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Replace the response body with a new reader containing the same data
	// so the caller can still read it (otherwise we'll get EOF errors)
	resp.Body = io.NopCloser(bytes.NewReader(body))

	var apiResponse response
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w", err)
	}

	pricing, exists := ut.pricing[apiResponse.Model]
	if !exists {
		base64Body := base64.StdEncoding.EncodeToString(body)
		ut.logger.Warn().
			Str("model", apiResponse.Model).
			Str("body_base64", base64Body).
			Str("request_uri", resp.Request.RequestURI).
			Int("status_code", resp.StatusCode).
			Msg("Unknown model pricing, using default GPT-4 pricing")
		return nil, fmt.Errorf("unknown model pricing")
	}

	metrics := ut.calculateCosts(apiResponse, pricing)

	ut.mu.Lock()
	ut.metrics = append(ut.metrics, metrics)
	ut.mu.Unlock()

	ut.logUsage(metrics)

	return &metrics, nil
}

// TotalUsage returns aggregated usage statistics
func (ut *UsageTracker) TotalUsage() UsageMetrics {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	var total UsageMetrics
	for _, metric := range ut.metrics {
		total.PromptTokens += metric.PromptTokens
		total.CompletionTokens += metric.CompletionTokens
		total.ReasoningTokens += metric.ReasoningTokens
		total.TotalTokens += metric.TotalTokens
		total.PromptCost += metric.PromptCost
		total.ReasoningCost += metric.ReasoningCost
		total.CompletionCost += metric.CompletionCost
		total.TotalCost += metric.TotalCost
	}

	return total
}

// UsageByModel returns usage statistics grouped by model
func (ut *UsageTracker) UsageByModel() map[string]UsageMetrics {
	ut.mu.RLock()
	defer ut.mu.RUnlock()

	modelUsage := make(map[string]UsageMetrics)
	for _, metric := range ut.metrics {
		if existing, exists := modelUsage[metric.Model]; exists {
			existing.PromptTokens += metric.PromptTokens
			existing.CompletionTokens += metric.CompletionTokens
			existing.ReasoningTokens += metric.ReasoningTokens
			existing.TotalTokens += metric.TotalTokens
			existing.PromptCost += metric.PromptCost
			existing.ReasoningCost += metric.ReasoningCost
			existing.CompletionCost += metric.CompletionCost
			existing.TotalCost += metric.TotalCost
			modelUsage[metric.Model] = existing
		} else {
			modelUsage[metric.Model] = metric
		}
	}

	return modelUsage
}

// ClearUsage clears all stored usage metrics
func (ut *UsageTracker) ClearUsage() {
	ut.mu.Lock()
	defer ut.mu.Unlock()
	ut.metrics = make([]UsageMetrics, 0)
}

func (ut *UsageTracker) calculateCosts(res response, pricing ModelPricing) UsageMetrics {
	promptCost := float64(res.Usage.PromptTokens) * pricing.InputCostPer1MTokens / 1000000
	completionCost := float64(res.Usage.CompletionTokens) * pricing.OutputCostPer1MTokens / 1000000
	reasoningCost := float64(res.CompletionTokensDetails.ReasoningTokens) * pricing.OutputCostPer1MTokens / 1000000

	return UsageMetrics{
		PromptTokens:     res.Usage.PromptTokens,
		CompletionTokens: res.Usage.CompletionTokens,
		ReasoningTokens:  res.CompletionTokensDetails.ReasoningTokens,
		TotalTokens:      res.Usage.TotalTokens,
		PromptCost:       promptCost,
		CompletionCost:   completionCost,
		ReasoningCost:    reasoningCost,
		TotalCost:        promptCost + completionCost,
		Model:            res.Model,
		Timestamp:        time.Now(),
	}
}

func (ut *UsageTracker) logUsage(metrics UsageMetrics) {
	ut.logger.Info().
		Str("model", metrics.Model).
		Int("prompt_tokens", metrics.PromptTokens).
		Int("reasoning_tokens", metrics.ReasoningTokens).
		Int("completion_tokens", metrics.CompletionTokens).
		Int("total_tokens", metrics.TotalTokens).
		Float64("prompt_cost", metrics.PromptCost).
		Float64("completion_cost", metrics.CompletionCost).
		Float64("reasoning_cost", metrics.ReasoningCost).
		Float64("total_cost", metrics.TotalCost).
		Time("timestamp", metrics.Timestamp).
		Msg("OpenAI API usage tracked")
}

func getDefaultPricing() map[string]ModelPricing {
	// Docs: https://openai.com/api/pricing
	return map[string]ModelPricing{
		"gpt-5-nano-2025-08-07": {
			InputCostPer1MTokens:  0.05, // $0.050 per 1M tokens
			OutputCostPer1MTokens: 0.4,  // $0.40 per 1M tokens
		},
		"text-embedding-3-small": {
			InputCostPer1MTokens:  0.02, // $0.02 per 1M tokens
			OutputCostPer1MTokens: 0.0,  // No output tokens for embeddings
		},
		"text-embedding-3-large": {
			InputCostPer1MTokens:  0.13, // $0.13 per 1M tokens
			OutputCostPer1MTokens: 0.0,  // No output tokens for embeddings
		},
	}
}
