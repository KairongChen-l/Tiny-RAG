// Package index provides vector storage and indexing functionality.
package index

import (
	"context"
	"time"

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
	Limit          int               // Maximum number of documents to return (default: 50, max: 1000)
	Offset         int               // Number of documents to skip (default: 0)
	SortBy         string            // Sort field: "created_at", "updated_at", "title" (default: "created_at")
	Order          string            // Sort order: "asc", "desc" (default: "desc")
	IncludeDeleted bool              // Include soft-deleted documents (default: false)
	SearchQuery    string            // Full-text search query (searches in title and content)
	FilterBy       map[string]string // Metadata filters (key-value pairs)
}

// ListDocumentsResult holds the result of listing documents.
type ListDocumentsResult struct {
	Documents []StoredDocument
	Total     int // Total number of documents (before pagination)
	Limit     int
	Offset    int
}

// DocumentVersion represents a version of a document.
type DocumentVersion struct {
	Version    int       `json:"version"`
	DocumentID string    `json:"document_id"`
	Hash       string    `json:"hash"`
	ChunkCount int       `json:"chunk_count"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by,omitempty"`  // Optional: user who created this version
	ChangeNote string    `json:"change_note,omitempty"` // Optional: note about what changed
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

	// Version control methods (optional - store may not support versioning)
	// StoreVersion stores a version snapshot of a document.
	StoreVersion(ctx context.Context, documentID string, changeNote string) error

	// ListVersions returns all versions of a document.
	ListVersions(ctx context.Context, documentID string) ([]DocumentVersion, error)

	// RestoreVersion restores a document to a specific version.
	RestoreVersion(ctx context.Context, documentID string, version int) error

	// Soft delete methods
	// SoftDeleteDocument marks a document as deleted without removing it.
	SoftDeleteDocument(ctx context.Context, documentID string) error

	// RestoreDocument restores a soft-deleted document.
	RestoreDocument(ctx context.Context, documentID string) error

	// HardDeleteDocument permanently deletes a document (including soft-deleted ones).
	HardDeleteDocument(ctx context.Context, documentID string) error
}

// Stats holds statistics about the vector store.
type Stats struct {
	TotalDocuments int64 `json:"total_documents"` // Total number of documents
	TotalChunks    int64 `json:"total_chunks"`    // Total number of chunks
	TotalSize      int64 `json:"total_size"`      // Total size in bytes (if available)
}
