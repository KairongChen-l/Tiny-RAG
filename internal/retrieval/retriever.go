// Package retrieval provides document retrieval functionality.
package retrieval

import (
	"context"

	"github.com/krc/rag/internal/chunking"
)

// RetrieveOptions holds options for retrieval.
type RetrieveOptions struct {
	TopK               int               // Final number of results to return (default: 5)
	CandidateK         int               // Number of candidates before reranking (default: 20)
	MinScore           float32           // Minimum similarity score threshold
	MetadataFilter     map[string]string // Metadata filters
	EnableRerank       bool              // Enable reranking
	EnableQueryRewrite bool              // Enable query rewriting
	EnableQueryExpand  bool              // Enable query expansion
	MultiQueryCount    int               // Number of query variants for multi-query (0 = disabled)
	EnableDynamicTopK  bool              // Enable dynamic top-k selection based on score distribution
	DynamicTopKConfig  *DynamicTopKConfig // Configuration for dynamic top-k (nil = use defaults)
}

// DefaultRetrieveOptions returns default retrieval options.
func DefaultRetrieveOptions() RetrieveOptions {
	return RetrieveOptions{
		TopK:         5,
		CandidateK:   20,
		MinScore:     0.7,
		EnableRerank: false,
	}
}

// RetrievalResult holds the results of a retrieval operation.
type RetrievalResult struct {
	Chunks    []RetrievedChunk
	QueryUsed string // The actual query used (may be rewritten)
}

// RetrievedChunk represents a retrieved chunk with metadata.
type RetrievedChunk struct {
	chunking.Chunk
	Score      float32 // Similarity score
	CitationID int     // Citation number (1, 2, 3, ...)
}

// Retriever defines the interface for document retrieval.
type Retriever interface {
	// Retrieve performs retrieval based on query and options.
	Retrieve(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error)
}

// Reranker defines the interface for result reranking.
type Reranker interface {
	// Rerank reorders chunks based on relevance to query.
	Rerank(ctx context.Context, query string, chunks []RetrievedChunk) ([]RetrievedChunk, error)
}
