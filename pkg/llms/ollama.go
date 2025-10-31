package llms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/tmc/langchaingo/llms"
)

type OllamaModel struct {
	baseURL     string
	model       string
	client      *http.Client
	contextSize int
}

type ollamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

func NewOllamaModel(baseURL, model string, client *http.Client, contextSize int) *OllamaModel {
	if client == nil {
		client = http.DefaultClient
	}
	if contextSize == 0 {
		contextSize = 32768
	}
	return &OllamaModel{
		baseURL:     baseURL,
		model:       model,
		client:      client,
		contextSize: contextSize,
	}
}

func (o *OllamaModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	opts := llms.CallOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	apiURL, err := url.JoinPath(o.baseURL, "api", "generate")
	if err != nil {
		return "", fmt.Errorf("construct API URL: %w", err)
	}

	reqBody := ollamaGenerateRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
	}

	if reqBody.Options == nil {
		reqBody.Options = make(map[string]any)
	}

	reqBody.Options["num_ctx"] = o.contextSize

	if opts.Temperature != 0 {
		reqBody.Options["temperature"] = opts.Temperature
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return ollamaResp.Response, nil
}
