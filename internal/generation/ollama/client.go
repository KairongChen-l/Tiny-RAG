// Package ollama provides Ollama local LLM client implementation.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
)

// Client implements the LLM interface using Ollama's local API.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// Config holds Ollama-specific configuration.
type Config struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// New creates a new Ollama LLM client.
func New(cfg Config) (*Client, error) {
	// Set defaults
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Model == "" {
		cfg.Model = "llama2"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300 * time.Second // Longer timeout for local models (5 minutes)
	}

	return &Client{
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// ollamaRequest represents the Ollama API request.
type ollamaRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Stream   bool                   `json:"stream"`
	Options  map[string]interface{} `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaResponse represents the Ollama API response.
type ollamaResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	TotalDuration   int64         `json:"total_duration"`
	LoadDuration    int64         `json:"load_duration"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

// Generate generates a response from a prompt.
func (c *Client) Generate(ctx context.Context, p *prompt.Prompt) (*generation.Response, error) {
	// Estimate token count (rough: 4 chars per token)
	// Limit context to avoid exceeding model's context window
	// Most Ollama models have 2048-4096 context, so we limit to ~1200 tokens for safety
	maxContextChars := 4800 // ~1200 tokens * 4 chars
	systemMsg := p.SystemMessage
	userMsg := p.UserMessage

	// Truncate if necessary
	if len(systemMsg) > maxContextChars/4 {
		systemMsg = systemMsg[:maxContextChars/4] + "..."
	}
	if len(userMsg) > maxContextChars*3/4 {
		userMsg = userMsg[:maxContextChars*3/4] + "..."
	}

	reqBody := ollamaRequest{
		Model: c.model,
		Messages: []ollamaMessage{
			{
				Role:    "system",
				Content: systemMsg,
			},
			{
				Role:    "user",
				Content: userMsg,
			},
		},
		Stream: false,
		// Add options to limit context size and improve performance
		Options: map[string]interface{}{
			"num_ctx":     2048, // Explicitly set context window
			"num_predict": 512,  // Limit response length to avoid long waits
			"temperature": 0.7,  // Set temperature
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// Log the error for debugging
		return nil, fmt.Errorf("Ollama API error (status %d): %s. Request model: %s, system msg length: %d, user msg length: %d",
			resp.StatusCode, string(body), c.model, len(systemMsg), len(userMsg))
	}

	var apiResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	answer := apiResp.Message.Content

	// Parse citations from response
	citations := prompt.ParseCitations(answer)

	// Estimate token count
	tokensUsed := apiResp.PromptEvalCount + apiResp.EvalCount

	return &generation.Response{
		Answer:       answer,
		TokensUsed:   tokensUsed,
		FinishReason: "stop",
		Citations:    citations,
	}, nil
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "ollama"
}
