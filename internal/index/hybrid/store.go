package hybrid

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/index/mysql"
	"github.com/krc/rag/internal/index/qdrant"
)

// Store implements VectorStore using a hybrid approach:
// - MySQL + GORM: stores document/chunk metadata
// - Qdrant: stores vectors for similarity search
type Store struct {
	metaStore   *mysql.Store  // MySQL store for metadata
	vectorStore *qdrant.Store // Qdrant store for vectors
}

// Config holds hybrid store configuration.
type Config struct {
	MySQLDSN   string // MySQL DSN
	QdrantURL  string // Qdrant URL
	Collection string // Qdrant collection name
	Dimension  int    // Vector dimension
	QdrantKey  string // Optional Qdrant API key
}

// New creates a new hybrid store.
func New(cfg Config) (*Store, error) {
	// Initialize MySQL store for metadata
	metaStore, err := mysql.New(mysql.Config{
		DSN:      cfg.MySQLDSN,
		Dimension: cfg.Dimension,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MySQL store: %w", err)
	}

	// Initialize Qdrant store for vectors
	vectorStore, err := qdrant.New(qdrant.Config{
		URL:        cfg.QdrantURL,
		Collection: cfg.Collection,
		Dimension:  cfg.Dimension,
		APIKey:     cfg.QdrantKey,
	})
	if err != nil {
		metaStore.Close()
		return nil, fmt.Errorf("failed to initialize Qdrant store: %w", err)
	}

	return &Store{
		metaStore:   metaStore,
		vectorStore: vectorStore,
	}, nil
}

// Store stores chunks with their vectors.
// Metadata goes to MySQL, vectors go to Qdrant.
func (s *Store) Store(ctx context.Context, chunks []chunking.ChunkWithVector) error {
	// Store metadata in MySQL
	if err := s.metaStore.Store(ctx, chunks); err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	// Store vectors in Qdrant
	if err := s.vectorStore.Store(ctx, chunks); err != nil {
		return fmt.Errorf("failed to store vectors: %w", err)
	}

	return nil
}

// Search performs vector similarity search using Qdrant.
func (s *Store) Search(ctx context.Context, query []float32, opts index.SearchOptions) ([]index.SearchResult, error) {
	return s.vectorStore.Search(ctx, query, opts)
}

// StoreDocument stores document metadata in MySQL.
func (s *Store) StoreDocument(ctx context.Context, doc *index.StoredDocument) error {
	return s.metaStore.StoreDocument(ctx, doc)
}

// GetDocument retrieves document metadata from MySQL.
func (s *Store) GetDocument(ctx context.Context, id string) (*index.StoredDocument, error) {
	return s.metaStore.GetDocument(ctx, id)
}

// DeleteByDocument deletes chunks from both stores.
func (s *Store) DeleteByDocument(ctx context.Context, documentID string) error {
	// Delete from MySQL
	if err := s.metaStore.DeleteByDocument(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete from MySQL: %w", err)
	}

	// Delete from Qdrant
	if err := s.vectorStore.DeleteByDocument(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete from Qdrant: %w", err)
	}

	return nil
}

// GetDocumentHash returns document hash from MySQL.
func (s *Store) GetDocumentHash(ctx context.Context, documentID string) (string, error) {
	return s.metaStore.GetDocumentHash(ctx, documentID)
}

// ReplaceDocument atomically replaces chunks in both stores.
func (s *Store) ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error {
	// Replace in MySQL
	if err := s.metaStore.ReplaceDocument(ctx, documentID, chunks); err != nil {
		return fmt.Errorf("failed to replace in MySQL: %w", err)
	}

	// Delete old vectors from Qdrant
	if err := s.vectorStore.DeleteByDocument(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete old vectors: %w", err)
	}

	// Store new vectors in Qdrant
	if err := s.vectorStore.Store(ctx, chunks); err != nil {
		return fmt.Errorf("failed to store new vectors: %w", err)
	}

	return nil
}

// ListDocuments lists documents from MySQL.
func (s *Store) ListDocuments(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	return s.metaStore.ListDocuments(ctx, opts)
}

// GetStats returns statistics from MySQL.
func (s *Store) GetStats(ctx context.Context) (*index.Stats, error) {
	return s.metaStore.GetStats(ctx)
}

// Close closes both stores.
func (s *Store) Close() error {
	var errs []error
	if err := s.metaStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("MySQL close error: %w", err))
	}
	if err := s.vectorStore.Close(); err != nil {
		errs = append(errs, fmt.Errorf("Qdrant close error: %w", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// Version control methods (delegate to MySQL)

// StoreVersion stores a version snapshot.
func (s *Store) StoreVersion(ctx context.Context, documentID string, changeNote string) error {
	return s.metaStore.StoreVersion(ctx, documentID, changeNote)
}

// ListVersions returns all versions of a document.
func (s *Store) ListVersions(ctx context.Context, documentID string) ([]index.DocumentVersion, error) {
	return s.metaStore.ListVersions(ctx, documentID)
}

// RestoreVersion restores a document to a specific version.
func (s *Store) RestoreVersion(ctx context.Context, documentID string, version int) error {
	return s.metaStore.RestoreVersion(ctx, documentID, version)
}

// Soft delete methods (delegate to MySQL)

// SoftDeleteDocument marks a document as deleted.
func (s *Store) SoftDeleteDocument(ctx context.Context, documentID string) error {
	return s.metaStore.SoftDeleteDocument(ctx, documentID)
}

// RestoreDocument restores a soft-deleted document.
func (s *Store) RestoreDocument(ctx context.Context, documentID string) error {
	return s.metaStore.RestoreDocument(ctx, documentID)
}

// HardDeleteDocument permanently deletes a document.
func (s *Store) HardDeleteDocument(ctx context.Context, documentID string) error {
	// Delete from both stores
	if err := s.metaStore.HardDeleteDocument(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete from MySQL: %w", err)
	}
	if err := s.vectorStore.DeleteByDocument(ctx, documentID); err != nil {
		return fmt.Errorf("failed to delete from Qdrant: %w", err)
	}
	return nil
}

// DB returns the underlying GORM DB instance (for job store).
func (s *Store) DB() *gorm.DB {
	return s.metaStore.DB()
}

