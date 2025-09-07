package lib

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/rs/zerolog"
)

func TestUsageTracker(t *testing.T) {
	logger := zerolog.Nop()
	tracker := NewUsageTracker(&logger)

	// Test successful usage tracking
	mockResponse := &http.Response{
		StatusCode: 200,
		Body: createMockResponseBody(map[string]interface{}{
			"usage": map[string]interface{}{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
			"model": "gpt-5-nano-2025-08-07",
		}),
	}

	metrics, err := tracker.TrackUsage(mockResponse)
	if err != nil {
		t.Fatalf("Failed to track usage: %v", err)
	}

	if metrics.PromptTokens != 100 {
		t.Errorf("Expected prompt tokens 100, got %d", metrics.PromptTokens)
	}
	if metrics.CompletionTokens != 50 {
		t.Errorf("Expected completion tokens 50, got %d", metrics.CompletionTokens)
	}
	if metrics.TotalTokens != 150 {
		t.Errorf("Expected total tokens 150, got %d", metrics.TotalTokens)
	}
	if metrics.Model != "gpt-5-nano-2025-08-07" {
		t.Errorf("Expected model gpt-5-nano-2025-08-07, got %s", metrics.Model)
	}

	// Test cost calculation
	expectedPromptCost := float64(100) * 0.05 / 1000000   // $0.05 per 1M tokens
	expectedCompletionCost := float64(50) * 0.4 / 1000000 // $0.4 per 1M tokens
	expectedTotalCost := expectedPromptCost + expectedCompletionCost

	if metrics.PromptCost != expectedPromptCost {
		t.Errorf("Expected prompt cost %f, got %f", expectedPromptCost, metrics.PromptCost)
	}
	if metrics.CompletionCost != expectedCompletionCost {
		t.Errorf("Expected completion cost %f, got %f", expectedCompletionCost, metrics.CompletionCost)
	}
	if metrics.TotalCost != expectedTotalCost {
		t.Errorf("Expected total cost %f, got %f", expectedTotalCost, metrics.TotalCost)
	}
}

func TestUsageTrackerAggregation(t *testing.T) {
	logger := zerolog.Nop()
	tracker := NewUsageTracker(&logger)

	// Track multiple usages
	mockResponse1 := &http.Response{
		StatusCode: 200,
		Body: createMockResponseBody(map[string]interface{}{
			"usage": map[string]interface{}{
				"prompt_tokens":     100,
				"completion_tokens": 0,
				"total_tokens":      100,
			},
			"model": "text-embedding-3-small",
		}),
	}

	mockResponse2 := &http.Response{
		StatusCode: 200,
		Body: createMockResponseBody(map[string]interface{}{
			"usage": map[string]interface{}{
				"prompt_tokens":     200,
				"completion_tokens": 0,
				"total_tokens":      200,
			},
			"model": "text-embedding-3-small",
		}),
	}

	tracker.TrackUsage(mockResponse1)
	tracker.TrackUsage(mockResponse2)

	totalUsage := tracker.TotalUsage()
	if totalUsage.PromptTokens != 300 {
		t.Errorf("Expected total prompt tokens 300, got %d", totalUsage.PromptTokens)
	}
	if totalUsage.CompletionTokens != 0 {
		t.Errorf("Expected total completion tokens 0, got %d", totalUsage.CompletionTokens)
	}
	if totalUsage.TotalTokens != 300 {
		t.Errorf("Expected total tokens 300, got %d", totalUsage.TotalTokens)
	}

	// Test usage by model
	modelUsage := tracker.UsageByModel()
	if len(modelUsage) != 1 {
		t.Errorf("Expected 1 model, got %d", len(modelUsage))
	}
	if modelUsage["text-embedding-3-small"].TotalTokens != 300 {
		t.Errorf("Expected text-embedding-3-small total tokens 300, got %d", modelUsage["text-embedding-3-small"].TotalTokens)
	}
}

func TestUsageTrackerUnknownModel(t *testing.T) {
	logger := zerolog.Nop()
	tracker := NewUsageTracker(&logger)

	mockResponse := &http.Response{
		StatusCode: 200,
		Body: createMockResponseBody(map[string]interface{}{
			"usage": map[string]interface{}{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
			"model": "unknown-model",
		}),
	}

	metrics, err := tracker.TrackUsage(mockResponse)
	if err != nil {
		t.Fatalf("Failed to track usage: %v", err)
	}

	// Should fallback to GPT-4 pricing
	expectedPromptCost := float64(100) * 30.0 / 1000000
	expectedCompletionCost := float64(50) * 60.0 / 1000000

	if metrics.PromptCost != expectedPromptCost {
		t.Errorf("Expected prompt cost %f, got %f", expectedPromptCost, metrics.PromptCost)
	}
	if metrics.CompletionCost != expectedCompletionCost {
		t.Errorf("Expected completion cost %f, got %f", expectedCompletionCost, metrics.CompletionCost)
	}
}

func createMockResponseBody(data map[string]interface{}) io.ReadCloser {
	jsonData, _ := json.Marshal(data)
	return io.NopCloser(bytes.NewReader(jsonData))
}
