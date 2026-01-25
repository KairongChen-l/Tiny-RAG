package retrieval

import (
	"context"
	"fmt"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/index"
)

// VectorRetriever implements Retriever using vector similarity search.
type VectorRetriever struct {
	store    index.VectorStore
	embedder embedding.Embedder
	reranker Reranker // optional
}

// VectorRetrieverConfig holds configuration for the vector retriever.
type VectorRetrieverConfig struct {
	Store    index.VectorStore
	Embedder embedding.Embedder
	Reranker Reranker // optional, can be nil
}

// NewVectorRetriever creates a new vector-based retriever.
func NewVectorRetriever(cfg VectorRetrieverConfig) (*VectorRetriever, error) {
	if cfg.Store == nil {
		return nil, fmt.Errorf("vector store is required")
	}
	if cfg.Embedder == nil {
		return nil, fmt.Errorf("embedder is required")
	}

	return &VectorRetriever{
		store:    cfg.Store,
		embedder: cfg.Embedder,
		reranker: cfg.Reranker,
	}, nil
}

// Retrieve performs retrieval based on query and options.
func (r *VectorRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error) {
	// Apply defaults
	if opts.TopK <= 0 {
		opts.TopK = 5
	}
	if opts.CandidateK <= 0 {
		opts.CandidateK = 20
	}

	// Embed the query
	queryVector, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Determine how many candidates to retrieve
	searchTopK := opts.TopK
	if opts.EnableRerank && r.reranker != nil {
		searchTopK = opts.CandidateK // Retrieve more for reranking
	}

	// Search the vector store
	searchOpts := index.SearchOptions{
		TopK:           searchTopK,
		MinScore:       opts.MinScore,
		MetadataFilter: opts.MetadataFilter,
	}

	results, err := r.store.Search(ctx, queryVector, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Convert to RetrievedChunks
	chunks := make([]RetrievedChunk, len(results))
	for i, result := range results {
		chunks[i] = RetrievedChunk{
			Chunk:      result.Chunk,
			Score:      result.Score,
			CitationID: i + 1, // 1-based citation IDs
		}
	}

	// Apply reranking if enabled
	if opts.EnableRerank && r.reranker != nil && len(chunks) > 0 {
		chunks, err = r.reranker.Rerank(ctx, query, chunks)
		if err != nil {
			// Log but don't fail - fall back to original order
			fmt.Printf("Warning: Reranking failed: %v\n", err)
		}

		// Update citation IDs after reranking
		for i := range chunks {
			chunks[i].CitationID = i + 1
		}
	}

	// Trim to final TopK
	if len(chunks) > opts.TopK {
		chunks = chunks[:opts.TopK]
	}

	return &RetrievalResult{
		Chunks:    chunks,
		QueryUsed: query,
	}, nil
}

// SetReranker sets or updates the reranker.
func (r *VectorRetriever) SetReranker(reranker Reranker) {
	r.reranker = reranker
}

// MockRetriever is a simple retriever for testing that returns predefined chunks.
type MockRetriever struct {
	chunks []RetrievedChunk
}

// NewMockRetriever creates a mock retriever with predefined chunks.
func NewMockRetriever(chunks []chunking.Chunk) *MockRetriever {
	retrieved := make([]RetrievedChunk, len(chunks))
	for i, c := range chunks {
		retrieved[i] = RetrievedChunk{
			Chunk:      c,
			Score:      0.9 - float32(i)*0.1, // Decreasing scores
			CitationID: i + 1,
		}
	}
	return &MockRetriever{chunks: retrieved}
}

// Retrieve returns the predefined chunks.
func (r *MockRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error) {
	chunks := r.chunks
	if opts.TopK > 0 && opts.TopK < len(chunks) {
		chunks = chunks[:opts.TopK]
	}
	return &RetrievalResult{
		Chunks:    chunks,
		QueryUsed: query,
	}, nil
}

