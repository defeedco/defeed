package lib

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// OpenAILimiter is rate limiter for OpenAI API.
// It implements an openaiclient.Doer.
type OpenAILimiter struct {
	client *http.Client
	logger *zerolog.Logger
}

func NewOpenAILimiter(logger *zerolog.Logger) *OpenAILimiter {
	return &OpenAILimiter{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

func (r *OpenAILimiter) Do(req *http.Request) (*http.Response, error) {
	maxRetries := 5

	for attempt := range maxRetries {
		if attempt > 0 {
			clonedReq, err := r.cloneRequest(req)
			if err != nil {
				return nil, fmt.Errorf("clone request: %w", err)
			}
			req = clonedReq
		}

		resp, err := r.client.Do(req)

		errBody := ""
		errStatusCode := 0
		if resp != nil && resp.StatusCode != 200 {
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read response body: %w", err)
			}
			errBody = string(body)
			errStatusCode = resp.StatusCode
		}

		if err != nil {
			r.logger.Error().
				Err(err).
				Int("status_code", errStatusCode).
				Str("body", errBody).
				Msg("OpenAI returned error")
			return nil, err
		}

		rateLimitHeaders := parseRateLimitHeaders(resp)
		attemptEvent := r.attemptEvent(rateLimitHeaders, errBody, resp.StatusCode, attempt)

		// See: https://platform.openai.com/docs/guides/error-codes#api-errors
		if resp.StatusCode == 429 {
			resp.Body.Close()

			delay := backoffWithJitter(rateLimitHeaders)
			attemptEvent.
				Dur("delay", delay).
				Msg("OpenAI rate limit reached, retrying with backoff")

			time.Sleep(delay)
			continue
		}

		// See: https://platform.openai.com/docs/guides/error-codes#api-errors
		if resp.StatusCode == 503 {
			resp.Body.Close()

			delay := backoffWithJitter(rateLimitHeaders)
			attemptEvent.
				Dur("delay", delay).
				Msg("OpenAI service overloaded, retrying with backoff")

			time.Sleep(delay)
			continue
		}

		// See: https://platform.openai.com/docs/guides/error-codes#api-errors
		if resp.StatusCode != 200 {
			resp.Body.Close()

			// API sometimes returns 400 response, log the body for debugging.
			attemptEvent.
				Msg("OpenAI returned non-ok response")

			return resp, nil
		}

		attemptEvent.
			Msg("OpenAI request successful")

		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded for rate limited request")
}

func backoffWithJitter(headers *rateLimitHeaders) time.Duration {
	jitter := time.Duration(rand.Intn(1000)) * time.Millisecond

	if headers.RemainingRequests >= 0 && headers.RemainingRequests <= 1 && headers.ResetRequests > 0 {
		return headers.ResetRequests + jitter
	}
	if headers.RemainingTokens >= 0 && headers.RemainingTokens <= 1 && headers.ResetTokens > 0 {
		return headers.ResetTokens + jitter
	}

	return 0
}

func (r *OpenAILimiter) cloneRequest(req *http.Request) (*http.Request, error) {
	clonedReq := req.Clone(req.Context())
	if req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		clonedReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	return clonedReq, nil
}

type rateLimitHeaders struct {
	RemainingRequests int
	// The time until the request rate limit resets to its initial state.
	ResetRequests   time.Duration
	RemainingTokens int
	// The time until the token rate limit resets to its initial state.
	ResetTokens time.Duration
}

func parseRateLimitHeaders(resp *http.Response) *rateLimitHeaders {
	// See: https://platform.openai.com/docs/guides/error-codes#api-errors
	return &rateLimitHeaders{
		RemainingRequests: parseInt(resp.Header.Get("x-ratelimit-remaining-requests")),
		ResetRequests:     parseReset(resp.Header.Get("x-ratelimit-reset-requests")),
		RemainingTokens:   parseInt(resp.Header.Get("x-ratelimit-remaining-tokens")),
		ResetTokens:       parseReset(resp.Header.Get("x-ratelimit-reset-tokens")),
	}
}

func (r *OpenAILimiter) attemptEvent(headers *rateLimitHeaders, errBody string, statusCode int, attempt int) *zerolog.Event {
	return r.logger.Debug().
		Int("remaining_requests", headers.RemainingRequests).
		Dur("reset_requests", headers.ResetRequests).
		Int("remaining_tokens", headers.RemainingTokens).
		Dur("reset_tokens", headers.ResetTokens).
		Int("status_code", statusCode).
		Str("body", errBody).
		Int("attempt", attempt)
}

// parseInt converts a numeric header string to int; returns -1 on failure.
func parseInt(s string) int {
	if s == "" {
		return -1
	}

	if i, err := strconv.Atoi(s); err == nil {
		return i
	}

	return -1
}

// parseReset parses the time until the rate limit (based on requests) resets to its initial state.
// See: https://platform.openai.com/docs/guides/rate-limits#rate-limits-in-headers
func parseReset(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return time.Duration(f * float64(time.Second))
	}

	if d, err := time.ParseDuration(s); err == nil {
		return d
	}

	return 0
}
