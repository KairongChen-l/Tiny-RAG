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

// ListOptions holds options for listing documents.
type ListOptions struct {
	Limit  int    // Maximum number of documents to return (default: 50, max: 1000)
	Offset int    // Number of documents to skip (default: 0)
	SortBy string // Sort field: "created_at", "updated_at", "title" (default: "created_at")
	Order  string // Sort order: "asc", "desc" (default: "desc")
}

// ListDocumentsResult holds the result of listing documents.
type ListDocumentsResult struct {
	Documents []StoredDocument
	Total     int // Total number of documents (before pagination)
	Limit     int
	Offset    int
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

	// ListDocuments returns stored documents with pagination and filtering.
	ListDocuments(ctx context.Context, opts ListOptions) (*ListDocumentsResult, error)

	// GetStats returns statistics about stored documents and chunks.
	GetStats(ctx context.Context) (*Stats, error)

	// Close closes the store and releases resources.
	Close() error
}

// Stats holds statistics about the vector store.
type Stats struct {
	TotalDocuments int64 `json:"total_documents"` // Total number of documents
	TotalChunks     int64 `json:"total_chunks"`     // Total number of chunks
	TotalSize       int64 `json:"total_size"`       // Total size in bytes (if available)
}
