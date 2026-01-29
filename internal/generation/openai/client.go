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

// GenerateStream generates a streaming response from a prompt.
func (c *Client) GenerateStream(ctx context.Context, p *prompt.Prompt) (<-chan generation.StreamChunk, error) {
	ch := make(chan generation.StreamChunk, 10)

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
		Stream:      true,
	}

	stream, err := c.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("OpenAI stream error: %w", err)
	}

	go func() {
		defer close(ch)
		defer stream.Close()

		var fullText string
		var totalTokens int

		for {
			response, err := stream.Recv()
			if err != nil {
				if err.Error() == "stream is done" {
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

			if len(response.Choices) > 0 {
				delta := response.Choices[0].Delta.Content
				if delta != "" {
					fullText += delta
					ch <- generation.StreamChunk{
						Text:        delta,
						Done:        false,
						TokensUsed:  0, // Will be set at the end
						FinishReason: "",
					}
				}

				if response.Choices[0].FinishReason != "" {
					// Get final token count from usage if available
					if response.Usage.TotalTokens > 0 {
						totalTokens = response.Usage.TotalTokens
					}
					ch <- generation.StreamChunk{
						Text:        "",
						Done:        true,
						TokensUsed:  totalTokens,
						FinishReason: string(response.Choices[0].FinishReason),
					}
					return
				}
			}
		}
	}()

	return ch, nil
}

// Name returns the provider name.
func (c *Client) Name() string {
	return "openai"
}

