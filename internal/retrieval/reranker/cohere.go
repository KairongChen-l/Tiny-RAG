package reranker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/krc/rag/internal/retrieval"
)

// CohereReranker implements Reranker using Cohere's rerank HTTP API.
type CohereReranker struct {
	apiKey     string
	model      string
	topN       int
	httpClient *http.Client
	baseURL    string
}

// Config holds Cohere reranker configuration.
type Config struct {
	APIKey string // Cohere API key
	Model  string // Model name (default: "rerank-multilingual-v3.0")
	TopN   int    // Maximum number of results to return (default: 10)
}

// NewCohereReranker creates a new Cohere reranker.
func NewCohereReranker(cfg Config) (*CohereReranker, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Cohere API key is required")
	}
	if cfg.Model == "" {
		cfg.Model = "rerank-multilingual-v3.0" // Default to multilingual
	}
	if cfg.TopN <= 0 {
		cfg.TopN = 10
	}

	return &CohereReranker{
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		topN:       cfg.TopN,
		baseURL:    "https://api.cohere.ai/v1",
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// rerankRequest represents the Cohere rerank API request.
type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// rerankResponse represents the Cohere rerank API response.
type rerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

// Rerank reorders chunks based on relevance to query using Cohere's rerank API.
func (r *CohereReranker) Rerank(ctx context.Context, query string, chunks []retrieval.RetrievedChunk) ([]retrieval.RetrievedChunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}

	// Prepare documents for reranking
	documents := make([]string, len(chunks))
	for i, chunk := range chunks {
		documents[i] = chunk.Content
	}

	// Build request
	reqBody := rerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: documents,
		TopN:      r.topN,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := r.baseURL + "/rerank"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	// Execute request
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cohere rerank API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Cohere rerank API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var rerankResp rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Map results back to chunks
	rerankedChunks := make([]retrieval.RetrievedChunk, 0, len(rerankResp.Results))
	for _, result := range rerankResp.Results {
		index := result.Index
		if index < 0 || index >= len(chunks) {
			continue // Skip invalid indices
		}

		chunk := chunks[index]
		// Update score with rerank score
		chunk.Score = float32(result.RelevanceScore)
		rerankedChunks = append(rerankedChunks, chunk)
	}

	// Sort by score descending
	sort.Slice(rerankedChunks, func(i, j int) bool {
		return rerankedChunks[i].Score > rerankedChunks[j].Score
	})

	return rerankedChunks, nil
}

