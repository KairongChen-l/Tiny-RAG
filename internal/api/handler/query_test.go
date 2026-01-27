package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/generation"
	"github.com/krc/rag/internal/prompt"
	"github.com/krc/rag/internal/retrieval"
	"github.com/krc/rag/pkg/cache"
	"github.com/krc/rag/pkg/config"
)

// mockRetriever is a mock retriever for testing.
type mockRetriever struct {
	result *retrieval.RetrievalResult
	err    error
}

func (m *mockRetriever) Retrieve(ctx context.Context, query string, opts retrieval.RetrieveOptions) (*retrieval.RetrievalResult, error) {
	return m.result, m.err
}

// mockPromptBuilder is a mock prompt builder for testing.
type mockPromptBuilder struct {
	prompt *prompt.Prompt
	err    error
}

func (m *mockPromptBuilder) Build(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, opts prompt.PromptOptions) (*prompt.Prompt, error) {
	return m.prompt, m.err
}

func (m *mockPromptBuilder) BuildWithHistory(ctx context.Context, query string, chunks []retrieval.RetrievedChunk, history string, opts prompt.PromptOptions) (*prompt.Prompt, error) {
	return m.prompt, m.err
}

// mockLLMClient is a mock LLM client for testing.
type mockLLMClient struct {
	response *generation.Response
	err      error
}

func (m *mockLLMClient) Generate(ctx context.Context, p *prompt.Prompt) (*generation.Response, error) {
	return m.response, m.err
}

func (m *mockLLMClient) Name() string {
	return "mock"
}

// mockLLMRegistry is a mock LLM registry for testing.
type mockLLMRegistry struct {
	client generation.LLM
}

func (m *mockLLMRegistry) GetDefault() (generation.LLM, error) {
	return m.client, nil
}

func (m *mockLLMRegistry) Get(name string) (generation.LLM, error) {
	return m.client, nil
}

func TestQuery_WithCache(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create cache
	queryCache := cache.NewMemoryCache(cache.MemoryCacheConfig{
		TTL:     1 * time.Hour,
		MaxSize: 100,
	})

	// Create handler with cache
	h := &Handler{
		logger:     logger,
		queryCache: queryCache,
		retriever: &mockRetriever{
			result: &retrieval.RetrievalResult{
				Chunks: []retrieval.RetrievedChunk{},
			},
		},
		promptBuilder: &mockPromptBuilder{
			prompt: &prompt.Prompt{
				SystemMessage: "system",
				UserMessage:   "user",
				Citations:     []prompt.CitationInfo{},
			},
		},
		llmRegistry: func() *generation.LLMRegistry {
			registry := generation.NewLLMRegistry()
			registry.Register("mock", &mockLLMClient{
				response: &generation.Response{
					Answer:     "cached answer",
					TokensUsed: 100,
				},
			})
			registry.SetDefault("mock")
			return registry
		}(),
		config: &config.Config{
			Prompt: config.PromptConfig{
				MaxContextTokens: 3000,
				IncludeCitations: true,
			},
		},
	}

	// First request - should not be cached
	req1 := QueryRequest{
		Query: "test query",
	}
	body1, _ := json.Marshal(req1)
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/query", bytes.NewReader(body1))

	h.Query(w1, r1)

	if w1.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w1.Code)
	}

	// Second request with same query - should be cached
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/query", bytes.NewReader(body1))

	h.Query(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w2.Code)
	}

	// Verify response is the same
	var resp1, resp2 QueryResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.Answer != resp2.Answer {
		t.Error("expected cached response to match original")
	}
}

func TestQuery_CacheKey_IncludesOptions(t *testing.T) {
	// Test that different options produce different cache keys
	key1 := cache.CacheKey("query", "test", map[string]interface{}{
		"top_k": 5,
	})
	key2 := cache.CacheKey("query", "test", map[string]interface{}{
		"top_k": 10,
	})

	if key1 == key2 {
		t.Error("expected different cache keys for different options")
	}
}
