// Package generation provides LLM interaction functionality.
package generation

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/krc/rag/internal/prompt"
)

// Response represents an LLM response.
type Response struct {
	Answer       string // Generated answer
	TokensUsed   int    // Total tokens used
	FinishReason string // Reason for completion
	Citations    []int  // Parsed citation numbers
}

// LLMConfig holds common LLM configuration.
type LLMConfig struct {
	Model       string
	MaxTokens   int
	Temperature float32
	Timeout     time.Duration
}

// DefaultLLMConfig returns default LLM configuration.
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		Model:       "gpt-4",
		MaxTokens:   2048,
		Temperature: 0.7,
		Timeout:     60 * time.Second,
	}
}

// LLM defines the interface for LLM clients.
type LLM interface {
	// Generate generates a response from a prompt.
	Generate(ctx context.Context, p *prompt.Prompt) (*Response, error)

	// Name returns the provider name.
	Name() string
}

// LLMRegistry manages LLM providers.
type LLMRegistry struct {
	mu           sync.RWMutex
	providers    map[string]LLM
	defaultName  string
}

// NewLLMRegistry creates a new LLM registry.
func NewLLMRegistry() *LLMRegistry {
	return &LLMRegistry{
		providers: make(map[string]LLM),
	}
}

// Register adds an LLM provider to the registry.
func (r *LLMRegistry) Register(name string, llm LLM) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = llm
}

// SetDefault sets the default provider.
func (r *LLMRegistry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("provider not found: %s", name)
	}
	r.defaultName = name
	return nil
}

// Get returns a provider by name.
func (r *LLMRegistry) Get(name string) (LLM, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	llm, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return llm, nil
}

// GetDefault returns the default provider.
func (r *LLMRegistry) GetDefault() (LLM, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.defaultName == "" {
		return nil, fmt.Errorf("no default provider set")
	}
	return r.providers[r.defaultName], nil
}

// ListProviders returns all registered provider names.
func (r *LLMRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

