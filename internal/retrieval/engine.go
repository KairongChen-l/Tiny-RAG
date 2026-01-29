package retrieval

import (
	"context"
)

// SearchEngine provides a unified interface for search operations.
// It abstracts over different retrieval strategies (vector, BM25, hybrid).
type SearchEngine interface {
	// Search performs a search query and returns results.
	Search(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error)
}

// UnifiedSearchEngine is a unified search engine that combines multiple retrieval strategies.
type UnifiedSearchEngine struct {
	retriever Retriever
}

// NewUnifiedSearchEngine creates a new unified search engine.
func NewUnifiedSearchEngine(retriever Retriever) *UnifiedSearchEngine {
	return &UnifiedSearchEngine{retriever: retriever}
}

// Search performs a search query.
func (e *UnifiedSearchEngine) Search(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error) {
	return e.retriever.Retrieve(ctx, query, opts)
}

