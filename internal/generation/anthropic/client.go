// Package anthropic provides Anthropic Claude LLM client implementation.
package anthropic

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

// Client implements the LLM interface using Anthropic's API.
type Client struct {
	apiKey     string
	model      string
	maxTokens  int
	httpClient *http.Client
}

// Config holds Anthropic-specific configuration.
type Config struct {
	APIKey    string
	Model     string
	MaxTokens int
	Timeout   time.Duration
}

// New creates a new Anthropic LLM client.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	// Set defaults
	if cfg.Model == "" {
		cfg.Model = "claude-3-sonnet-20240229"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2048
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &Client{
		apiKey:    cfg.APIKey,
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

// anthropicRequest represents the Anthropic API request.
type anthropicRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	System    string            `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the Anthropic API response.
type anthropicResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []anthropicContent `json:"content"`
	StopReason   string `json:"stop_reason"`
	Usage        anthropicUsage `json:"usage"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Generate generates a response from a prompt.
func (c *Client) Generate(ctx context.Context, p *prompt.Prompt) (*generation.Response, error) {
	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    p.SystemMessage,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: p.UserMessage,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Anthropic API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract text content
	var answer string
	for _, content := range apiResp.Content {
		if content.Type == "text" {
			answer += content.Text
		}
	}

	// Parse citations from response
	citations := prompt.ParseCitations(answer)

	return &generation.Response{
		Answer:       answer,
		TokensUsed:   apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		FinishReason: apiResp.StopReason,
		Citations:    citations,
	}, nil
}

// GenerateStream generates a streaming response from a prompt.
func (c *Client) GenerateStream(ctx context.Context, p *prompt.Prompt) (<-chan generation.StreamChunk, error) {
	ch := make(chan generation.StreamChunk, 10)

	reqBody := anthropicRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		System:    p.SystemMessage,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: p.UserMessage,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages?stream=true", bytes.NewBuffer(jsonData))
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("Anthropic API error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		close(ch)
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(body))
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := json.NewDecoder(resp.Body)
		var totalTokens int

		for {
			var event map[string]interface{}
			if err := scanner.Decode(&event); err != nil {
				if err == io.EOF {
					ch <- generation.StreamChunk{
						Text:        "",
						Done:        true,
						TokensUsed:  totalTokens,
						FinishReason: "stop",
					}
					return
				}
				ch <- generation.StreamChunk{
					Text:        "",
					Done:        true,
					TokensUsed:  totalTokens,
					FinishReason: "error",
				}
				return
			}

			eventType, _ := event["type"].(string)
			if eventType == "content_block_delta" {
				if delta, ok := event["delta"].(map[string]interface{}); ok {
					if text, ok := delta["text"].(string); ok && text != "" {
						ch <- generation.StreamChunk{
							Text:        text,
							Done:        false,
							TokensUsed:  0,
							FinishReason: "",
						}
					}
				}
			} else if eventType == "message_stop" {
				if usage, ok := event["usage"].(map[string]interface{}); ok {
					if input, ok := usage["input_tokens"].(float64); ok {
						totalTokens += int(input)
					}
					if output, ok := usage["output_tokens"].(float64); ok {
						totalTokens += int(output)
					}
				}
				ch <- generation.StreamChunk{
					Text:        "",
					Done:        true,
					TokensUsed:  totalTokens,
					FinishReason: "stop",
				}
				return
			}
		}
	}()

	return ch, nil
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "anthropic"
}

