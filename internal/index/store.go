// Package index provides vector storage and indexing functionality.
package index

import (
	"context"

	"github.com/krc/rag/internal/chunking"
)

// SearchOptions holds options for vector search.
type SearchOptions struct {
	TopK           int               // Number of results to return
	MinScore       float32           // Minimum similarity score threshold
	MetadataFilter map[string]string // Metadata filters
}

// SearchResult represents a search result with score.
type SearchResult struct {
	Chunk    chunking.Chunk
	Score    float32
	Citation int // Citation number (1-based)
}

// VectorStore defines the interface for vector storage operations.
type VectorStore interface {
	// Store stores chunks with their vectors.
	Store(ctx context.Context, chunks []chunking.ChunkWithVector) error

	// Search performs vector similarity search.
	Search(ctx context.Context, query []float32, opts SearchOptions) ([]SearchResult, error)

	// DeleteByDocument deletes all chunks for a document.
	DeleteByDocument(ctx context.Context, documentID string) error

	// GetDocumentHash returns the hash of a stored document.
	GetDocumentHash(ctx context.Context, documentID string) (string, error)

	// ReplaceDocument atomically replaces all chunks for a document.
	ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error

	// Close closes the store and releases resources.
	Close() error
}

