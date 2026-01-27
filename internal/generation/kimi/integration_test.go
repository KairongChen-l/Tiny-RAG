// +build integration

package kimi

import (
	"context"
	"os"
	"testing"

	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
)

// TestKimiClient_Integration tests the full integration with Kimi API.
// This test requires a valid KIMI_API_KEY environment variable.
func TestKimiClient_Integration(t *testing.T) {
	apiKey := os.Getenv("KIMI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: KIMI_API_KEY environment variable not set")
	}

	client, err := New(Config{
		APIKey:      apiKey,
		Model:       "moonshot-v1-8k",
		MaxTokens:   2048,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name        string
		prompt      *prompt.Prompt
		expectError bool
		validate    func(t *testing.T, resp *generation.Response)
	}{
		{
			name: "simple question",
			prompt: &prompt.Prompt{
				SystemMessage: "You are a helpful assistant.",
				UserMessage:   "What is 2+2? Answer with just the number.",
				TokenCount:    0,
			},
			expectError: false,
			validate: func(t *testing.T, resp *generation.Response) {
				if resp.Answer == "" {
					t.Error("Expected non-empty answer")
				}
				if resp.TokensUsed == 0 {
					t.Error("Expected non-zero tokens used")
				}
				t.Logf("Answer: %s", resp.Answer)
				t.Logf("Tokens used: %d", resp.TokensUsed)
			},
		},
		{
			name: "RAG context question",
			prompt: &prompt.Prompt{
				SystemMessage: "You are a helpful assistant. Answer questions based on the provided context.",
				UserMessage: `Context:
Document 1: RAG stands for Retrieval-Augmented Generation. It combines retrieval and generation.

Question: What does RAG stand for?`,
				TokenCount: 0,
			},
			expectError: false,
			validate: func(t *testing.T, resp *generation.Response) {
				if resp.Answer == "" {
					t.Error("Expected non-empty answer")
				}
				// Answer should mention RAG
				if len(resp.Answer) < 10 {
					t.Error("Expected a meaningful answer")
				}
				t.Logf("Answer: %s", resp.Answer)
			},
		},
		{
			name: "citation parsing",
			prompt: &prompt.Prompt{
				SystemMessage: "You are a helpful assistant. Use citations like [citation:1] when referencing sources.",
				UserMessage: `Context:
[citation:1] RAG is a technique that combines retrieval and generation.

Question: Explain RAG and cite your source.`,
				TokenCount: 0,
			},
			expectError: false,
			validate: func(t *testing.T, resp *generation.Response) {
				if resp.Answer == "" {
					t.Error("Expected non-empty answer")
				}
				// Check if citations are parsed
				if resp.Citations != nil && len(resp.Citations) > 0 {
					t.Logf("Found %d citations: %v", len(resp.Citations), resp.Citations)
				}
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := client.Generate(ctx, tt.prompt)
			if (err != nil) != tt.expectError {
				t.Errorf("Generate() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !tt.expectError && resp != nil && tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

// TestKimiClient_ErrorHandling tests error handling with invalid API key.
func TestKimiClient_ErrorHandling(t *testing.T) {
	client, err := New(Config{
		APIKey: "invalid-api-key-12345",
		Model:  "moonshot-v1-8k",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	p := &prompt.Prompt{
		SystemMessage: "Test",
		UserMessage:   "Test",
		TokenCount:    0,
	}

	_, err = client.Generate(ctx, p)
	if err == nil {
		t.Error("Expected error with invalid API key")
	} else {
		t.Logf("Got expected error: %v", err)
	}
}

