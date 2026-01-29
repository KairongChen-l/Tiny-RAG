package retrieval

import (
	"context"
	"fmt"
	"sort"
)

// HybridRetriever implements hybrid search combining vector and BM25 retrieval.
type HybridRetriever struct {
	vectorRetriever *VectorRetriever
	bm25Retriever   BM25RetrieverInterface
	fusionMethod    FusionMethod
	k               int // RRF parameter (default: 60)
}

// FusionMethod defines how to combine results from different retrievers.
type FusionMethod string

const (
	FusionRRF FusionMethod = "rrf" // Reciprocal Rank Fusion
	FusionAvg FusionMethod = "avg" // Average scores
	FusionMax FusionMethod = "max"  // Maximum score
)

// HybridRetrieverConfig holds configuration for hybrid retriever.
type HybridRetrieverConfig struct {
	VectorRetriever *VectorRetriever
	BM25Retriever   BM25RetrieverInterface
	FusionMethod    FusionMethod
	K               int // RRF parameter
}

// NewHybridRetriever creates a new hybrid retriever.
func NewHybridRetriever(cfg HybridRetrieverConfig) (*HybridRetriever, error) {
	if cfg.VectorRetriever == nil {
		return nil, fmt.Errorf("vector retriever is required")
	}
	if cfg.BM25Retriever == nil {
		return nil, fmt.Errorf("BM25 retriever is required")
	}
	if cfg.FusionMethod == "" {
		cfg.FusionMethod = FusionRRF
	}
	if cfg.K <= 0 {
		cfg.K = 60 // Default RRF parameter
	}

	return &HybridRetriever{
		vectorRetriever: cfg.VectorRetriever,
		bm25Retriever:   cfg.BM25Retriever,
		fusionMethod:    cfg.FusionMethod,
		k:               cfg.K,
	}, nil
}

// Retrieve performs hybrid retrieval combining vector and BM25 results.
func (h *HybridRetriever) Retrieve(ctx context.Context, query string, opts RetrieveOptions) (*RetrievalResult, error) {
	// Perform vector retrieval
	vectorResults, err := h.vectorRetriever.Retrieve(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("vector retrieval failed: %w", err)
	}

	// Perform BM25 retrieval
	bm25TopK := opts.TopK
	if opts.CandidateK > 0 {
		bm25TopK = opts.CandidateK
	}
	bm25Results, err := h.bm25Retriever.Search(ctx, query, bm25TopK)
	if err != nil {
		// BM25 might fail if not indexed, fall back to vector only
		return vectorResults, nil
	}

	// Fuse results
	fusedResults := h.fuseResults(vectorResults.Chunks, bm25Results, opts.TopK)

	return &RetrievalResult{
		Chunks:    fusedResults,
		QueryUsed: query,
	}, nil
}

// fuseResults fuses results from vector and BM25 retrievers.
func (h *HybridRetriever) fuseResults(vectorChunks, bm25Chunks []RetrievedChunk, topK int) []RetrievedChunk {
	switch h.fusionMethod {
	case FusionRRF:
		return h.fuseRRF(vectorChunks, bm25Chunks, topK)
	case FusionAvg:
		return h.fuseAverage(vectorChunks, bm25Chunks, topK)
	case FusionMax:
		return h.fuseMax(vectorChunks, bm25Chunks, topK)
	default:
		return h.fuseRRF(vectorChunks, bm25Chunks, topK)
	}
}

// fuseRRF fuses results using Reciprocal Rank Fusion (RRF).
// RRF score = sum(1 / (k + rank)) for each retriever
func (h *HybridRetriever) fuseRRF(vectorChunks, bm25Chunks []RetrievedChunk, topK int) []RetrievedChunk {
	// Build chunk map with RRF scores
	chunkMap := make(map[string]*RetrievedChunk)

	// Add vector results
	for rank, chunk := range vectorChunks {
		if existing, ok := chunkMap[chunk.ID]; ok {
			existing.Score += 1.0 / float32(h.k+rank+1)
		} else {
			chunkCopy := chunk
			chunkCopy.Score = 1.0 / float32(h.k+rank+1)
			chunkMap[chunk.ID] = &chunkCopy
		}
	}

	// Add BM25 results
	for rank, chunk := range bm25Chunks {
		if existing, ok := chunkMap[chunk.ID]; ok {
			existing.Score += 1.0 / float32(h.k+rank+1)
		} else {
			chunkCopy := chunk
			chunkCopy.Score = 1.0 / float32(h.k+rank+1)
			chunkMap[chunk.ID] = &chunkCopy
		}
	}

	// Convert to slice and sort
	results := make([]RetrievedChunk, 0, len(chunkMap))
	for _, chunk := range chunkMap {
		results = append(results, *chunk)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Trim to topK
	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}

	// Update citation IDs
	for i := range results {
		results[i].CitationID = i + 1
	}

	return results
}

// fuseAverage fuses results by averaging normalized scores.
func (h *HybridRetriever) fuseAverage(vectorChunks, bm25Chunks []RetrievedChunk, topK int) []RetrievedChunk {
	// Normalize scores to [0, 1]
	vectorChunks = h.normalizeScores(vectorChunks)
	bm25Chunks = h.normalizeScores(bm25Chunks)

	// Build chunk map
	chunkMap := make(map[string]*RetrievedChunk)

	// Add vector results
	for _, chunk := range vectorChunks {
		if existing, ok := chunkMap[chunk.ID]; ok {
			existing.Score = (existing.Score + chunk.Score) / 2
		} else {
			chunkCopy := chunk
			chunkMap[chunk.ID] = &chunkCopy
		}
	}

	// Add BM25 results
	for _, chunk := range bm25Chunks {
		if existing, ok := chunkMap[chunk.ID]; ok {
			existing.Score = (existing.Score + chunk.Score) / 2
		} else {
			chunkCopy := chunk
			chunkMap[chunk.ID] = &chunkCopy
		}
	}

	// Convert to slice and sort
	results := make([]RetrievedChunk, 0, len(chunkMap))
	for _, chunk := range chunkMap {
		results = append(results, *chunk)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}

	for i := range results {
		results[i].CitationID = i + 1
	}

	return results
}

// fuseMax fuses results by taking maximum score.
func (h *HybridRetriever) fuseMax(vectorChunks, bm25Chunks []RetrievedChunk, topK int) []RetrievedChunk {
	// Normalize scores
	vectorChunks = h.normalizeScores(vectorChunks)
	bm25Chunks = h.normalizeScores(bm25Chunks)

	// Build chunk map
	chunkMap := make(map[string]*RetrievedChunk)

	// Add vector results
	for _, chunk := range vectorChunks {
		chunkCopy := chunk
		chunkMap[chunk.ID] = &chunkCopy
	}

	// Add BM25 results, taking max
	for _, chunk := range bm25Chunks {
		if existing, ok := chunkMap[chunk.ID]; ok {
			if chunk.Score > existing.Score {
				existing.Score = chunk.Score
			}
		} else {
			chunkCopy := chunk
			chunkMap[chunk.ID] = &chunkCopy
		}
	}

	// Convert to slice and sort
	results := make([]RetrievedChunk, 0, len(chunkMap))
	for _, chunk := range chunkMap {
		results = append(results, *chunk)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && topK < len(results) {
		results = results[:topK]
	}

	for i := range results {
		results[i].CitationID = i + 1
	}

	return results
}

// normalizeScores normalizes scores to [0, 1] range.
func (h *HybridRetriever) normalizeScores(chunks []RetrievedChunk) []RetrievedChunk {
	if len(chunks) == 0 {
		return chunks
	}

	// Find min and max scores
	minScore := chunks[0].Score
	maxScore := chunks[0].Score
	for _, chunk := range chunks {
		if chunk.Score < minScore {
			minScore = chunk.Score
		}
		if chunk.Score > maxScore {
			maxScore = chunk.Score
		}
	}

	// Normalize
	range_ := maxScore - minScore
	if range_ == 0 {
		// All scores are the same, set to 0.5
		for i := range chunks {
			chunks[i].Score = 0.5
		}
		return chunks
	}

	normalized := make([]RetrievedChunk, len(chunks))
	for i, chunk := range chunks {
		normalized[i] = chunk
		normalized[i].Score = (chunk.Score - minScore) / range_
	}

	return normalized
}

