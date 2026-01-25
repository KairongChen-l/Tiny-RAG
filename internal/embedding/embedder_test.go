package embedding

import (
	"context"
	"testing"
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

