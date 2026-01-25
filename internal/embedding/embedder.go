// Package embedding provides text embedding functionality.
package embedding

import (
	"context"
	"fmt"
	"sync"
)

// Embedder defines the interface for text embedding.
type Embedder interface {
	// Embed generates an embedding vector for a single text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embedding vectors for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the embedding vector dimensions.
	Dimensions() int

	// Name returns the provider name.
	Name() string
}

// EmbedderConfig holds common embedding configuration.
type EmbedderConfig struct {
	Model      string
	Dimensions int
	BatchSize  int
}

// EmbedderRegistry manages embedding providers.
type EmbedderRegistry struct {
	mu        sync.RWMutex
	providers map[string]Embedder
	default_  string
}

// NewEmbedderRegistry creates a new embedder registry.
func NewEmbedderRegistry() *EmbedderRegistry {
	return &EmbedderRegistry{
		providers: make(map[string]Embedder),
	}
}

// Register adds an embedder to the registry.
func (r *EmbedderRegistry) Register(name string, embedder Embedder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = embedder
}

// SetDefault sets the default embedder.
func (r *EmbedderRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("embedder not found: %s", name)
	}
	r.default_ = name
	return nil
}

// Get returns an embedder by name.
func (r *EmbedderRegistry) Get(name string) (Embedder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	embedder, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("embedder not found: %s", name)
	}
	return embedder, nil
}

// GetDefault returns the default embedder.
func (r *EmbedderRegistry) GetDefault() (Embedder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.default_ == "" {
		return nil, fmt.Errorf("no default embedder set")
	}
	return r.providers[r.default_], nil
}

