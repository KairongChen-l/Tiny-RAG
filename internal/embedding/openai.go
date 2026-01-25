package embedding

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

// OpenAIEmbedder implements Embedder using OpenAI's API.
type OpenAIEmbedder struct {
	client     *openai.Client
	model      string
	dimensions int
	batchSize  int
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

// embedBatch processes a single batch of texts.
func (e *OpenAIEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	req := openai.EmbeddingRequest{
		Model: openai.EmbeddingModel(e.model),
		Input: texts,
	}

	resp, err := e.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	// Extract vectors in order
	vectors := make([][]float32, len(texts))
	for _, data := range resp.Data {
		if data.Index >= len(vectors) {
			continue
		}
		vectors[data.Index] = data.Embedding
	}

	return vectors, nil
}

// Dimensions returns the embedding vector dimensions.
func (e *OpenAIEmbedder) Dimensions() int {
	return e.dimensions
}

// Name returns the provider name.
func (e *OpenAIEmbedder) Name() string {
	return "openai"
}

