package embedding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Cache defines the interface for embedding cache.
type Cache interface {
	// Get retrieves cached embedding for a text.
	Get(ctx context.Context, key string) ([]float32, bool, error)

	// Set stores an embedding in cache.
	Set(ctx context.Context, key string, vector []float32) error

	// Clear clears all cached embeddings.
	Clear(ctx context.Context) error
}

// CacheKey generates a cache key for embedding.
func CacheKey(provider, model, text string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", provider, model, text)))
	return fmt.Sprintf("embed:%s:%s:%s", provider, model, hex.EncodeToString(hash[:]))
}

// CachedEmbedder wraps an embedder with caching.
type CachedEmbedder struct {
	embedder Embedder
	cache    Cache
}

// NewCachedEmbedder creates a new cached embedder.
func NewCachedEmbedder(embedder Embedder, cache Cache) *CachedEmbedder {
	return &CachedEmbedder{
		embedder: embedder,
		cache:    cache,
	}
}

// Embed generates an embedding with caching.
func (c *CachedEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Check cache first
	key := CacheKey(c.embedder.Name(), c.getModel(), text)
	if cached, found, err := c.cache.Get(ctx, key); err == nil && found {
		return cached, nil
	}

	// Generate embedding
	vector, err := c.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := c.cache.Set(ctx, key, vector); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: Failed to cache embedding: %v\n", err)
	}

	return vector, nil
}

// EmbedBatch generates embeddings with caching.
func (c *CachedEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	toEmbed := make([]int, 0, len(texts))
	toEmbedTexts := make([]string, 0, len(texts))

	// Check cache for each text
	for i, text := range texts {
		key := CacheKey(c.embedder.Name(), c.getModel(), text)
		if cached, found, err := c.cache.Get(ctx, key); err == nil && found {
			vectors[i] = cached
		} else {
			toEmbed = append(toEmbed, i)
			toEmbedTexts = append(toEmbedTexts, text)
		}
	}

	// Embed uncached texts
	if len(toEmbedTexts) > 0 {
		embedded, err := c.embedder.EmbedBatch(ctx, toEmbedTexts)
		if err != nil {
			return nil, err
		}

		// Store in cache and assign to result
		for j, idx := range toEmbed {
			vectors[idx] = embedded[j]
			key := CacheKey(c.embedder.Name(), c.getModel(), toEmbedTexts[j])
			if err := c.cache.Set(ctx, key, embedded[j]); err != nil {
				fmt.Printf("Warning: Failed to cache embedding: %v\n", err)
			}
		}
	}

	return vectors, nil
}

// Dimensions returns the embedding vector dimensions.
func (c *CachedEmbedder) Dimensions() int {
	return c.embedder.Dimensions()
}

// Name returns the provider name.
func (c *CachedEmbedder) Name() string {
	return c.embedder.Name()
}

// getModel extracts model name from embedder (helper method).
func (c *CachedEmbedder) getModel() string {
	// Try to get model from embedder if it has a Model() method
	// For now, use a default based on provider
	switch c.embedder.Name() {
	case "openai":
		return "text-embedding-3-small"
	case "ollama":
		return "nomic-embed-text"
	default:
		return "default"
	}
}

