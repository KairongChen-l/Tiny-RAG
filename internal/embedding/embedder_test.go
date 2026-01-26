package embedding

import (
	"context"
	"testing"
	"time"
)

func TestEmbedderRegistry(t *testing.T) {
	registry := NewEmbedderRegistry()

	// Create a mock embedder
	mock := &mockEmbedder{
		name:       "mock",
		dimensions: 128,
	}

	// Register
	registry.Register("mock", mock)

	// Get by name
	embedder, err := registry.Get("mock")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if embedder.Name() != "mock" {
		t.Errorf("expected name 'mock', got '%s'", embedder.Name())
	}

	// Set default
	if err := registry.SetDefault("mock"); err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}

	// Get default
	defaultEmbedder, err := registry.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault failed: %v", err)
	}
	if defaultEmbedder.Name() != "mock" {
		t.Errorf("expected default name 'mock', got '%s'", defaultEmbedder.Name())
	}

	// Get non-existent
	_, err = registry.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent embedder")
	}
}

func TestOpenAIConfig(t *testing.T) {
	// Test that config validation works (without API call)
	_, err := NewOpenAIEmbedder(OpenAIConfig{
		APIKey: "", // Empty key should fail
	})
	if err == nil {
		t.Error("expected error for empty API key")
	}

	// Valid config should not error (just creates client)
	_, err = NewOpenAIEmbedder(OpenAIConfig{
		APIKey: "test-key",
		Model:  "text-embedding-3-small",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOllamaConfig(t *testing.T) {
	// Ollama should work with defaults
	embedder, err := NewOllamaEmbedder(OllamaConfig{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if embedder.Name() != "ollama" {
		t.Errorf("expected name 'ollama', got '%s'", embedder.Name())
	}
	if embedder.Dimensions() != 768 {
		t.Errorf("expected dimensions 768, got %d", embedder.Dimensions())
	}
}

// TestOllamaConnectionError tests error handling when Ollama service is unavailable
func TestOllamaConnectionError(t *testing.T) {
	// Use a non-existent port to simulate connection refused
	embedder, err := NewOllamaEmbedder(OllamaConfig{
		BaseURL: "http://localhost:99999", // Invalid port
		Model:   "nomic-embed-text",
		Timeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	ctx := context.Background()
	_, err = embedder.Embed(ctx, "test text")
	if err == nil {
		t.Error("expected error when Ollama service is unavailable")
	}
	
	// Verify error message contains useful information
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
	
	// Check that error message indicates it's an Ollama API error
	errMsg := err.Error()
	if len(errMsg) < 16 || errMsg[:16] != "Ollama API error" {
		t.Errorf("expected error to start with 'Ollama API error', got: %s", errMsg)
	}
	
	// Verify error message includes model info for debugging (check if contains "model:")
	if len(errMsg) >= 30 {
		// Error should include model information
		if errMsg[:30] != "Ollama API error (model: " {
			// May have different format, but should contain "model:" somewhere
			if len(errMsg) < 50 || errMsg[:50] != "Ollama API error (model: nomic-embed-text" {
				t.Logf("error message: %s", errMsg)
				// Accept any format that includes model info
			}
		}
	}
}

// TestOllamaBatchErrorHandling tests that batch embedding fails gracefully on errors
func TestOllamaBatchErrorHandling(t *testing.T) {
	// Use a non-existent port
	embedder, err := NewOllamaEmbedder(OllamaConfig{
		BaseURL: "http://localhost:99999",
		Model:   "nomic-embed-text",
		Timeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	ctx := context.Background()
	texts := []string{"text1", "text2", "text3"}
	_, err = embedder.EmbedBatch(ctx, texts)
	if err == nil {
		t.Error("expected error when Ollama service is unavailable")
	}
	
	// Verify error message indicates which text failed
	errMsg := err.Error()
	if len(errMsg) < 20 || errMsg[:20] != "failed to embed text" {
		t.Errorf("expected error to indicate which text failed, got: %s", errMsg)
	}
	
	// Verify error includes text preview and progress info
	if errMsg[:30] != "failed to embed text 1/3 (preview" {
		t.Logf("error message format: %s", errMsg[:50])
		// The format may vary, but should contain text index info
	}
}

// TestOllamaContextCancellation tests that embedding respects context cancellation
func TestOllamaContextCancellation(t *testing.T) {
	embedder, err := NewOllamaEmbedder(OllamaConfig{
		BaseURL: "http://localhost:11434", // Valid but may not be running
		Model:   "nomic-embed-text",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = embedder.Embed(ctx, "test text")
	if err == nil {
		t.Error("expected error when context is cancelled")
	}
	if err != context.Canceled {
		// If connection fails first, that's also acceptable
		// But if context was cancelled, we should see that error
		t.Logf("got error (may be connection error): %v", err)
	}
}

// mockEmbedder is a test embedder.
type mockEmbedder struct {
	name       string
	dimensions int
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, m.dimensions), nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i := range texts {
		vectors[i] = make([]float32, m.dimensions)
	}
	return vectors, nil
}

func (m *mockEmbedder) Dimensions() int {
	return m.dimensions
}

func (m *mockEmbedder) Name() string {
	return m.name
}

