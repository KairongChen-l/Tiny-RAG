package embedding

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/krc/rag/pkg/circuitbreaker"
	"github.com/krc/rag/pkg/retry"
)

// OpenAIEmbedder implements Embedder using OpenAI's API.
type OpenAIEmbedder struct {
	client         *openai.Client
	model          string
	dimensions     int
	batchSize      int
	circuitBreaker *circuitbreaker.CircuitBreaker
	retryConfig    retry.RetryConfig
}

// OpenAIConfig holds OpenAI embedder configuration.
type OpenAIConfig struct {
	APIKey     string
	Model      string // e.g., "text-embedding-3-small"
	Dimensions int    // e.g., 1536
	BatchSize  int    // Max texts per batch (default: 100)
	BaseURL    string // Optional custom base URL
}

// NewOpenAIEmbedder creates a new OpenAI embedder.
func NewOpenAIEmbedder(cfg OpenAIConfig) (*OpenAIEmbedder, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	// Set defaults
	if cfg.Model == "" {
		cfg.Model = "text-embedding-3-small"
	}
	if cfg.Dimensions == 0 {
		cfg.Dimensions = 1536
	}
	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}

	// Create client config
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	return &OpenAIEmbedder{
		client:     openai.NewClientWithConfig(clientConfig),
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		batchSize:  cfg.BatchSize,
		circuitBreaker: circuitbreaker.NewCircuitBreaker(circuitbreaker.DefaultCircuitBreakerConfig()),
		retryConfig: retry.RetryConfig{
			MaxAttempts: 3,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     2 * time.Second,
			Multiplier:   2.0,
		},
	}, nil
}

// Embed generates an embedding vector for a single text.
func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vectors, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vectors[0], nil
}

// EmbedBatch generates embedding vectors for multiple texts.
func (e *OpenAIEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Process in batches
	var allVectors [][]float32
	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]

		vectors, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d-%d failed: %w", i, end, err)
		}
		allVectors = append(allVectors, vectors...)
	}

	return allVectors, nil
}

// embedBatch processes a single batch of texts with retry and circuit breaker.
func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var result [][]float32
	var lastErr error

	// Use circuit breaker to protect against cascading failures
	err := e.circuitBreaker.Execute(func() error {
		// Use retry with exponential backoff
		err := retry.RetryWithExponentialBackoff(ctx, func() error {
			req := openai.EmbeddingRequest{
				Model: openai.EmbeddingModel(e.model),
				Input: texts,
			}

			resp, err := e.client.CreateEmbeddings(ctx, req)
			if err != nil {
				// Check if error is retryable
				if !isRetryableError(err) {
					return &retry.NonRetryableError{Err: err}
				}
				return fmt.Errorf("OpenAI API error: %w", err)
			}

			// Extract vectors in order
			vectors := make([][]float32, len(texts))
			for _, data := range resp.Data {
				if data.Index >= len(vectors) {
					continue
				}
				vectors[data.Index] = data.Embedding
			}

			result = vectors
			return nil
		}, e.retryConfig)

		if err != nil {
			lastErr = err
			return err
		}
		return nil
	})

	if err != nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, err
	}

	return result, nil
}

// isRetryableError checks if an error is retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Network/timeout errors are retryable
	retryableKeywords := []string{"network", "timeout", "connection", "temporary", "unavailable", "rate limit", "429", "502", "503", "504"}
	for _, keyword := range retryableKeywords {
		if contains(errStr, keyword) {
			return true
		}
	}
	// Authentication/authorization errors are not retryable
	nonRetryableKeywords := []string{"authentication", "authorization", "invalid api key", "401", "403", "404"}
	for _, keyword := range nonRetryableKeywords {
		if contains(errStr, keyword) {
			return false
		}
	}
	return true // Default to retryable for unknown errors
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Dimensions returns the embedding vector dimensions.
func (e *OpenAIEmbedder) Dimensions() int {
	return e.dimensions
}

// Name returns the provider name.
func (e *OpenAIEmbedder) Name() string {
	return "openai"
}

