package llms

import (
	"context"
	"fmt"

	"github.com/defeedco/defeed/pkg/lib"
	"github.com/tmc/langchaingo/llms"
)

type CachedEmbedderModel struct {
	model embedderModel
	cache *lib.Cache
}

type embedderModel interface {
	CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error)
}

func NewCachedEmbedderModel(model embedderModel, cache *lib.Cache) *CachedEmbedderModel {
	return &CachedEmbedderModel{
		model: model,
		cache: cache,
	}
}

func (cm *CachedEmbedderModel) CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error) {
	// Cache each text element separately
	results := make([][]float32, len(texts))
	uncachedIndices := make([]int, 0)
	uncachedTexts := make([]string, 0)

	// Check cache for each text element
	for i, text := range texts {
		key := embeddingCacheKey(text)
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

		key := embeddingCacheKey(originalText)
		cm.cache.Set(key, embedding)

		results[originalIndex] = embedding
	}

	return results, nil
}

type CachedCompletionModel struct {
	model completionModel
	cache *lib.Cache
}

type completionModel interface {
	Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error)
}

func NewCachedCompletionModel(model completionModel, cache *lib.Cache) *CachedCompletionModel {
	return &CachedCompletionModel{
		model: model,
		cache: cache,
	}
}

func (cm *CachedCompletionModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	key := completionCacheKey(prompt)

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

func embeddingCacheKey(text string) string {
	// TODO: We should include the model ID (and any other params) as well,
	// 	although there won't be a need to switch between different models for now
	return fmt.Sprintf("embedding:%s", lib.HashParams(text))
}

func completionCacheKey(prompt string) string {
	// TODO: We should include the model ID (and any other params) as well,
	// 	although there won't be a need to switch between different models for now
	return fmt.Sprintf("completion:%s", lib.HashParams(prompt))
}
