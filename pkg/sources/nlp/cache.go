package nlp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/rs/zerolog"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"
)

type cacheEntry struct {
	value      any
	expiration time.Time
}

type LLMCache struct {
	logger  *zerolog.Logger
	entries map[string]cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
}

func NewLLMCache(ttl time.Duration, logger *zerolog.Logger) *LLMCache {
	return &LLMCache{
		logger:  logger,
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

func (c *LLMCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[key]
	if !exists {
		return "", false
	}

	if time.Now().After(entry.expiration) {
		return "", false
	}

	c.logger.Debug().
		Str("key", key).
		Any("value", entry.value).
		Msg("LLM cache hit")

	return entry.value, true
}

func (c *LLMCache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		value:      value,
		expiration: time.Now().Add(c.ttl),
	}
}

type CachedModel struct {
	model model
	cache *LLMCache
}

type model interface {
	completionModel
	embedderModel
}

func NewCachedModel(model model, cache *LLMCache) *CachedModel {
	return &CachedModel{
		model: model,
		cache: cache,
	}
}

func (cm *CachedModel) CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error) {
	// Cache each text element separately
	results := make([][]float32, len(texts))
	uncachedIndices := make([]int, 0)
	uncachedTexts := make([]string, 0)

	// Check cache for each text element
	for i, text := range texts {
		// TODO: We should include the model ID (and any other params) as well,
		// 	although there won't be a need to switch between different models for now
		key := cacheKey("embedding", text)
		if response, found := cm.cache.Get(key); found {
			if embedding, ok := response.([]float32); ok {
				results[i] = embedding
				continue
			}
		}
		uncachedIndices = append(uncachedIndices, i)
		uncachedTexts = append(uncachedTexts, text)
	}

	// If all texts were cached, return results
	if len(uncachedTexts) == 0 {
		return results, nil
	}

	// Generate embeddings for uncached texts
	uncachedEmbeddings, err := cm.model.CreateEmbedding(ctx, uncachedTexts)
	if err != nil {
		return nil, err
	}

	// Cache the new embeddings and update results
	for i, embedding := range uncachedEmbeddings {
		originalIndex := uncachedIndices[i]
		originalText := uncachedTexts[i]

		// TODO: We should include the model ID (and any other params) as well,
		// 	although there won't be a need to switch between different models for now
		key := cacheKey("embedding", originalText)
		cm.cache.Set(key, embedding)

		results[originalIndex] = embedding
	}

	return results, nil
}

func (cm *CachedModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	// TODO: We should include the model ID (and any other params) as well,
	// 	although there won't be a need to switch between different models for now
	key := cacheKey("completion", prompt)

	if response, found := cm.cache.Get(key); found {
		value, ok := response.(string)
		if ok {
			return value, nil
		}
	}

	response, err := cm.model.Call(ctx, prompt, options...)
	if err != nil {
		return "", err
	}

	cm.cache.Set(key, response)
	return response, nil
}

func cacheKey(params ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(params, ",")))
	return fmt.Sprintf("%x", hash)
}
