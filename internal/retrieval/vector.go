package retrieval

import (
	"context"
	"fmt"
	"sort"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/embedding"
	"github.com/krc/rag/internal/index"
)

// VectorRetriever implements Retriever using vector similarity search.
type VectorRetriever struct {
	store         index.VectorStore
	embedder      embedding.Embedder
	reranker      Reranker      // optional
	queryRewriter QueryRewriter // optional
	parentStore   index.VectorStore // optional: separate store holding parent chunks for parent-child retrieval
}

// VectorRetrieverConfig holds configuration for the vector retriever.
type VectorRetrieverConfig struct {
	Store         index.VectorStore
	Embedder      embedding.Embedder
	Reranker      Reranker      // optional, can be nil
	QueryRewriter QueryRewriter // optional, can be nil
	ParentStore   index.VectorStore // optional: store for parent chunks in parent-child indexing
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
		store:         cfg.Store,
		embedder:      cfg.Embedder,
		reranker:      cfg.Reranker,
		queryRewriter: cfg.QueryRewriter,
		parentStore:   cfg.ParentStore,
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

	// Rewrite query if rewriter is available
	actualQuery := query
	if r.queryRewriter != nil {
		rewritten, err := r.queryRewriter.Rewrite(ctx, query)
		if err == nil && rewritten != "" {
			actualQuery = rewritten
		}
	}

	// Embed the query
	queryVector, err := r.embedder.Embed(ctx, actualQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Determine how many candidates to retrieve
	searchTopK := opts.TopK
	if opts.EnableRerank && r.reranker != nil {
		searchTopK = opts.CandidateK // Retrieve more for reranking
	}

	// Inject tenant filter into metadata when TenantID is specified.
	// Copy the filter map to avoid mutating the caller's data.
	if opts.TenantID != "" {
		filterCopy := make(map[string]string, len(opts.MetadataFilter)+1)
		for k, v := range opts.MetadataFilter {
			filterCopy[k] = v
		}
		filterCopy["tenant_id"] = opts.TenantID
		opts.MetadataFilter = filterCopy
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

	// Apply dynamic top-k selection or static TopK trimming
	if opts.EnableDynamicTopK {
		cfg := DefaultDynamicTopKConfig()
		if opts.DynamicTopKConfig != nil {
			cfg = *opts.DynamicTopKConfig
		}
		selector := NewDynamicTopKSelector(cfg)
		result := selector.Select(chunks)
		chunks = result.Chunks
	} else if len(chunks) > opts.TopK {
		chunks = chunks[:opts.TopK]
	}

	return &RetrievalResult{
		Chunks:    chunks,
		QueryUsed: actualQuery,
	}, nil
}

// SetReranker sets or updates the reranker.
func (r *VectorRetriever) SetReranker(reranker Reranker) {
	r.reranker = reranker
}

// RetrieveWithParentContext performs fine-grained retrieval using child chunks
// and then looks up their parent chunks to restore full context. Each returned
// chunk is a parent chunk carrying the best matching child's similarity score.
// Duplicate parents (matched by multiple children) are deduplicated, keeping the
// highest score. A parentStore must be configured; otherwise an error is returned.
func (r *VectorRetriever) RetrieveWithParentContext(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error) {
	if r.parentStore == nil {
		return nil, fmt.Errorf("parent store is required for parent-child retrieval")
	}

	// Step 1: retrieve child chunks from the primary store
	childResult, err := r.Retrieve(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("child retrieval failed: %w", err)
	}

	// Step 2: collect unique parent IDs and keep the best score per parent
	type parentMatch struct {
		parentID string
		score    float32
	}
	bestByParent := make(map[string]float32)
	for _, c := range childResult.Chunks {
		pid := c.Chunk.ParentChunkID
		if pid == "" {
			// Chunk has no parent; treat it as its own context
			pid = c.Chunk.ID
		}
		if score, exists := bestByParent[pid]; !exists || c.Score > score {
			bestByParent[pid] = c.Score
		}
	}

	// Step 3: look up parent chunks and build results
	parentChunks, err := r.lookupParentChunks(ctx, bestByParent)
	if err != nil {
		return nil, fmt.Errorf("parent lookup failed: %w", err)
	}

	// Trim to TopK
	if opts.TopK > 0 && len(parentChunks) > opts.TopK {
		parentChunks = parentChunks[:opts.TopK]
	}

	// Assign citation IDs
	for i := range parentChunks {
		parentChunks[i].CitationID = i + 1
	}

	return &RetrievalResult{
		Chunks:    parentChunks,
		QueryUsed: childResult.QueryUsed,
	}, nil
}

// lookupParentChunks searches the parentStore for each parent ID using a
// zero-vector search filtered by chunk ID metadata. Results are sorted by
// descending score inherited from the best matching child.
func (r *VectorRetriever) lookupParentChunks(ctx context.Context, bestByParent map[string]float32) ([]RetrievedChunk, error) {
	var results []RetrievedChunk

	for parentID, score := range bestByParent {
		searchOpts := index.SearchOptions{
			TopK:           1,
			MetadataFilter: map[string]string{"chunk_id": parentID},
		}

		found, err := r.parentStore.Search(ctx, nil, searchOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to look up parent chunk %s: %w", parentID, err)
		}
		if len(found) > 0 {
			results = append(results, RetrievedChunk{
				Chunk: found[0].Chunk,
				Score: score,
			})
		}
	}

	// Sort by score descending
	sortRetrievedChunks(results)

	return results, nil
}

// sortRetrievedChunks sorts chunks by score in descending order.
func sortRetrievedChunks(chunks []RetrievedChunk) {
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})
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
