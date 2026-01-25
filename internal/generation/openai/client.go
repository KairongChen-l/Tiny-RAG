// Package openai provides OpenAI LLM client implementation.
package openai

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"

	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
)

// Client implements the LLM interface using OpenAI's API.
type Client struct {
	client *openai.Client
	config generation.LLMConfig
}

// Config holds OpenAI-specific configuration.
type Config struct {
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float32
	BaseURL     string // Optional custom base URL
}

// New creates a new OpenAI LLM client.
func New(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	// Set defaults
	if cfg.Model == "" {
		cfg.Model = "gpt-4"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2048
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}

	// Create client config
	clientConfig := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		clientConfig.BaseURL = cfg.BaseURL
	}

	return &Client{
		client: openai.NewClientWithConfig(clientConfig),
		config: generation.LLMConfig{
			Model:       cfg.Model,
			MaxTokens:   cfg.MaxTokens,
			Temperature: cfg.Temperature,
		},
	}, nil
}

// Generate generates a response from a prompt.
func (c *Client) Generate(ctx context.Context, p *prompt.Prompt) (*generation.Response, error) {
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
		return nil, fmt.Errorf("OpenAI API error: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response choices returned")
	}

	answer := resp.Choices[0].Message.Content
	finishReason := string(resp.Choices[0].FinishReason)

	// Parse citations from response
	citations := prompt.ParseCitations(answer)

	return &generation.Response{
		Answer:       answer,
		TokensUsed:   resp.Usage.TotalTokens,
		FinishReason: finishReason,
		Citations:    citations,
	}, nil
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "openai"
}

