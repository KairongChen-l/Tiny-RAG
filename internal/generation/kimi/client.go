// Package kimi provides Kimi (Moonshot AI) LLM client implementation.
package kimi

import (
	"context"
	"fmt"
	"time"

	"github.com/sashabaranov/go-openai"

	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/pkg/circuitbreaker"
	"github.com/krc/rag/pkg/retry"
)

// Client implements the LLM interface using Kimi's API.
type Client struct {
	client         *openai.Client
	config         generation.LLMConfig
	circuitBreaker *circuitbreaker.CircuitBreaker
	retryConfig    retry.RetryConfig
}

// Config holds Kimi-specific configuration.
type Config struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float32
	BaseURL     string // Optional custom base URL
}

// New creates a new Kimi LLM client.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Kimi API key is required")
	}

	// Set defaults
	if cfg.Model == "" {
		cfg.Model = "moonshot-v1-8k" // Default Kimi model
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096 // Kimi supports longer context
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.moonshot.cn/v1" // Kimi API base URL
	}

	// Create client config with Kimi base URL
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	clientConfig.BaseURL = cfg.BaseURL

	return &Client{
		client: openai.NewClientWithConfig(clientConfig),
		config: generation.LLMConfig{
			Model:       cfg.Model,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
		},
		circuitBreaker: circuitbreaker.NewCircuitBreaker(circuitbreaker.DefaultCircuitBreakerConfig()),
		retryConfig: retry.RetryConfig{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     2 * time.Second,
			Multiplier:   2.0,
		},
	}, nil
}

// Generate generates a response from a prompt with retry and circuit breaker.
func (c *Client) Generate(ctx context.Context, p *prompt.Prompt) (*generation.Response, error) {
	var result *generation.Response
	var lastErr error

	// Use circuit breaker to protect against cascading failures
	err := c.circuitBreaker.Execute(func() error {
		// Use retry with exponential backoff
		err := retry.RetryWithExponentialBackoff(ctx, func() error {
			req := openai.ChatCompletionRequest{
				Model: c.config.Model,
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: p.SystemMessage,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: p.UserMessage,
					},
				},
				MaxTokens:   c.config.MaxTokens,
				Temperature: c.config.Temperature,
			}

			resp, err := c.client.CreateChatCompletion(ctx, req)
			if err != nil {
				// Check if error is retryable
				if !isRetryableError(err) {
					return &retry.NonRetryableError{Err: err}
				}
				return fmt.Errorf("Kimi API error: %w", err)
			}

			if len(resp.Choices) == 0 {
				return &retry.NonRetryableError{Err: fmt.Errorf("no response choices returned")}
			}

			answer := resp.Choices[0].Message.Content
			finishReason := string(resp.Choices[0].FinishReason)

			// Parse citations from response
			citations := prompt.ParseCitations(answer)

			result = &generation.Response{
				Answer:       answer,
				TokensUsed:   resp.Usage.TotalTokens,
				FinishReason: finishReason,
				Citations:    citations,
			}
			return nil
		}, c.retryConfig)

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

// Name returns the provider name.
func (c *Client) Name() string {
	return "kimi"
}
