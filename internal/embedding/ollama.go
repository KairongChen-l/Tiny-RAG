package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaEmbedder implements Embedder using Ollama's local API.
type OllamaEmbedder struct {
	baseURL    string
	model      string
	dimensions int
	client     *http.Client
}

// OllamaConfig holds Ollama embedder configuration.
type OllamaConfig struct {
	BaseURL    string // e.g., "http://localhost:11434"
	Model      string // e.g., "nomic-embed-text"
	Dimensions int    // e.g., 768
	Timeout    time.Duration
}

// NewOllamaEmbedder creates a new Ollama embedder.
func NewOllamaEmbedder(cfg OllamaConfig) (*OllamaEmbedder, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "nomic-embed-text"
	}
	if cfg.Dimensions == 0 {
		cfg.Dimensions = 768 // Default for nomic-embed-text
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &OllamaEmbedder{
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// ollamaEmbedRequest represents the Ollama embedding API request.
type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbedResponse represents the Ollama embedding API response.
type ollamaEmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

// Embed generates an embedding vector for a single text.
func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := ollamaEmbedRequest{
		Model:  e.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", e.baseURL+"/api/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		// Provide more context about connection errors
		return nil, fmt.Errorf("Ollama API error (model: %s, base_url: %s): %w", e.model, e.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (model: %s): returned status %d: %s", e.model, resp.StatusCode, string(body))
	}

	var embedResp ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&embedResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert float64 to float32
	vector := make([]float32, len(embedResp.Embedding))
	for i, v := range embedResp.Embedding {
		vector[i] = float32(v)
	}

	return vector, nil
}

// EmbedBatch generates embedding vectors for multiple texts.
// Ollama doesn't have native batch support, so we process sequentially.
func (e *OllamaEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))

	for i, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		vector, err := e.Embed(ctx, text)
		if err != nil {
			// Include text preview in error for debugging (truncate if too long)
			textPreview := text
			if len(textPreview) > 50 {
				textPreview = textPreview[:50] + "..."
			}
			return nil, fmt.Errorf("failed to embed text %d/%d (preview: %q): %w", i+1, len(texts), textPreview, err)
		}
		vectors[i] = vector
	}

	return vectors, nil
}

// Dimensions returns the embedding vector dimensions.
func (e *OllamaEmbedder) Dimensions() int {
	return e.dimensions
}

// Name returns the provider name.
func (e *OllamaEmbedder) Name() string {
	return "ollama"
}
