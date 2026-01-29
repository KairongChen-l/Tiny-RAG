package mysql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/pkg/database"
)

// Store implements VectorStore using MySQL + GORM for metadata and Qdrant for vectors.
// This is a hybrid approach: MySQL stores document/chunk metadata, vector store handles vectors.
type Store struct {
	db        *gorm.DB
	dimension int
	// Note: Vector storage is handled by a separate vector store (Qdrant)
	// This store only handles metadata storage
}

// Config holds MySQL store configuration.
type Config struct {
	DSN      string // MySQL DSN: "user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	Dimension int   // Vector dimension (for validation)
	// Optional GORM configuration
	MaxOpenConns    int           // Maximum open connections (default: 25)
	MaxIdleConns    int           // Maximum idle connections (default: 5)
	ConnMaxLifetime time.Duration // Connection max lifetime (default: 5 minutes)
	ConnMaxIdleTime time.Duration // Connection max idle time (default: 10 minutes)
}

// New creates a new MySQL store using GORM with optimized settings.
func New(cfg Config) (*Store, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("DSN is required")
	}

	// Create GORM connection with optimized settings
	db, err := database.NewGORM(database.Config{
		DSN:             cfg.DSN,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
		LogLevel:        0, // Silent, use zap for logging
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL: %w", err)
	}

	store := &Store{
		db:        db,
		dimension: cfg.Dimension,
	}

	// Auto migrate schema
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates database tables using GORM AutoMigrate.
func (s *Store) initSchema() error {
	// Auto migrate all models
	if err := s.db.AutoMigrate(
		&Document{},
		&Chunk{},
		&DocumentVersion{},
	); err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	return nil
}

// Store stores chunks with their vectors.
// Note: This implementation stores metadata in MySQL, but vectors should be stored separately.
// For a complete solution, use this with Qdrant for vector storage.
func (s *Store) Store(ctx context.Context, chunks []chunking.ChunkWithVector) error {
	if len(chunks) == 0 {
		return nil
	}

	// Use transaction helper for better error handling
	return database.TransactionWithContext(s.db, ctx, func(tx *gorm.DB) error {
		// Store chunks (metadata only, vectors stored separately)
		for _, chunk := range chunks {
			metadataJSON, _ := json.Marshal(chunk.Metadata)

			chunkModel := Chunk{
				ID:          chunk.ID,
				DocumentID:  chunk.DocumentID,
				Content:     chunk.Content,
				SectionPath: chunk.SectionPath,
				Position:    chunk.Position,
				Metadata:    string(metadataJSON),
				Hash:        chunk.Hash,
				Version:     1,
				CreatedAt:   time.Now(),
			}

			if err := tx.Save(&chunkModel).Error; err != nil {
				return fmt.Errorf("failed to store chunk %s: %w", chunk.ID, err)
			}
		}
		return nil
	})
}

// Search performs vector similarity search.
// Note: This is a placeholder. Actual vector search should be done via Qdrant.
// This method returns an error to indicate that vector search is not supported here.
func (s *Store) Search(ctx context.Context, query []float32, opts index.SearchOptions) ([]index.SearchResult, error) {
	return nil, fmt.Errorf("vector search not supported in MySQL store, use Qdrant or Elasticsearch")
}

// StoreDocument stores document metadata.
func (s *Store) StoreDocument(ctx context.Context, doc *index.StoredDocument) error {
	metadataJSON, _ := json.Marshal(doc.Metadata)

	document := Document{
		ID:        doc.ID,
		Source:    doc.Source,
		Title:     doc.Title,
		Format:    doc.Format,
		Hash:      doc.Hash,
		Metadata:  string(metadataJSON),
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}

	return s.db.WithContext(ctx).Save(&document).Error
}

// GetDocument retrieves a document by ID.
func (s *Store) GetDocument(ctx context.Context, id string) (*index.StoredDocument, error) {
	var doc Document
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&doc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("document not found: %s", id)
		}
		return nil, err
	}

	var metadata map[string]string
	if doc.Metadata != "" {
		json.Unmarshal([]byte(doc.Metadata), &metadata)
	}

	return &index.StoredDocument{
		ID:        doc.ID,
		Source:    doc.Source,
		Title:     doc.Title,
		Format:    doc.Format,
		Hash:      doc.Hash,
		Metadata:  metadata,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
	}, nil
}

// DeleteByDocument deletes all chunks for a document.
func (s *Store) DeleteByDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Where("document_id = ?", documentID).Delete(&Chunk{}).Error
}

// GetDocumentHash returns the hash of a stored document.
func (s *Store) GetDocumentHash(ctx context.Context, documentID string) (string, error) {
	var doc Document
	if err := s.db.WithContext(ctx).Select("hash").Where("id = ?", documentID).First(&doc).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil // Document doesn't exist
		}
		return "", err
	}
	return doc.Hash, nil
}

// ReplaceDocument atomically replaces all chunks for a document.
func (s *Store) ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error {
	return database.TransactionWithContext(s.db, ctx, func(tx *gorm.DB) error {
		// Delete old chunks
		if err := tx.Where("document_id = ?", documentID).Delete(&Chunk{}).Error; err != nil {
			return fmt.Errorf("failed to delete old chunks: %w", err)
		}

		// Store new chunks
		for _, chunk := range chunks {
			metadataJSON, _ := json.Marshal(chunk.Metadata)
			chunkModel := Chunk{
				ID:          chunk.ID,
				DocumentID:  chunk.DocumentID,
				Content:     chunk.Content,
				SectionPath: chunk.SectionPath,
				Position:    chunk.Position,
				Metadata:    string(metadataJSON),
				Hash:        chunk.Hash,
				Version:     1,
				CreatedAt:   time.Now(),
			}
			if err := tx.Save(&chunkModel).Error; err != nil {
				return fmt.Errorf("failed to store chunk %s: %w", chunk.ID, err)
			}
		}
		return nil
	})
}

// ListDocuments returns stored documents with pagination and filtering.
func (s *Store) ListDocuments(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	query := s.db.WithContext(ctx).Model(&Document{})

	// Exclude soft-deleted unless requested
	if !opts.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	}

	// Apply filters
	if len(opts.FilterBy) > 0 {
		for key, value := range opts.FilterBy {
			query = query.Where("JSON_EXTRACT(metadata, ?) = ?", fmt.Sprintf("$.%s", key), value)
		}
	}

	// Full-text search
	if opts.SearchQuery != "" {
		query = query.Where("title LIKE ? OR source LIKE ?", "%"+opts.SearchQuery+"%", "%"+opts.SearchQuery+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// Apply pagination and sorting
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	sortBy := opts.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	order := opts.Order
	if order == "" {
		order = "desc"
	}

	var documents []Document
	if err := query.Order(fmt.Sprintf("%s %s", sortBy, order)).
		Limit(limit).
		Offset(opts.Offset).
		Find(&documents).Error; err != nil {
		return nil, err
	}

	// Convert to StoredDocument
	result := make([]index.StoredDocument, len(documents))
	for i, doc := range documents {
		var metadata map[string]string
		if doc.Metadata != "" {
			json.Unmarshal([]byte(doc.Metadata), &metadata)
		}

		result[i] = index.StoredDocument{
			ID:        doc.ID,
			Source:    doc.Source,
			Title:     doc.Title,
			Format:    doc.Format,
			Hash:      doc.Hash,
			Metadata:  metadata,
			CreatedAt: doc.CreatedAt,
			UpdatedAt: doc.UpdatedAt,
		}
	}

	return &index.ListDocumentsResult{
		Documents: result,
		Total:     int(total),
		Limit:     limit,
		Offset:    opts.Offset,
	}, nil
}

// GetStats returns statistics about stored documents and chunks.
func (s *Store) GetStats(ctx context.Context) (*index.Stats, error) {
	var docCount, chunkCount int64

	if err := s.db.WithContext(ctx).Model(&Document{}).Where("deleted_at IS NULL").Count(&docCount).Error; err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Model(&Chunk{}).Count(&chunkCount).Error; err != nil {
		return nil, err
	}

	return &index.Stats{
		TotalDocuments: docCount,
		TotalChunks:    chunkCount,
		TotalSize:      0, // Size calculation not implemented
	}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Version control methods

// StoreVersion stores a version snapshot of a document.
func (s *Store) StoreVersion(ctx context.Context, documentID string, changeNote string) error {
	// Get document hash
	hash, err := s.GetDocumentHash(ctx, documentID)
	if err != nil {
		return err
	}

	// Count chunks
	var chunkCount int64
	if err := s.db.WithContext(ctx).Model(&Chunk{}).Where("document_id = ?", documentID).Count(&chunkCount).Error; err != nil {
		return err
	}

	// Get next version number
	var maxVersion int
	if err := s.db.WithContext(ctx).Model(&DocumentVersion{}).
		Where("document_id = ?", documentID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion).Error; err != nil {
		return err
	}

	version := DocumentVersion{
		DocumentID: documentID,
		Version:    maxVersion + 1,
		Hash:       hash,
		ChunkCount: int(chunkCount),
		ChangeNote: changeNote,
		CreatedAt:  time.Now(),
	}

	return s.db.WithContext(ctx).Create(&version).Error
}

// ListVersions returns all versions of a document.
func (s *Store) ListVersions(ctx context.Context, documentID string) ([]index.DocumentVersion, error) {
	var versions []DocumentVersion
	if err := s.db.WithContext(ctx).Where("document_id = ?", documentID).
		Order("version DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}

	result := make([]index.DocumentVersion, len(versions))
	for i, v := range versions {
		result[i] = index.DocumentVersion{
			Version:    v.Version,
			DocumentID: v.DocumentID,
			Hash:       v.Hash,
			ChunkCount: v.ChunkCount,
			CreatedAt:  v.CreatedAt,
			CreatedBy:  v.CreatedBy,
			ChangeNote: v.ChangeNote,
		}
	}

	return result, nil
}

// RestoreVersion restores a document to a specific version.
func (s *Store) RestoreVersion(ctx context.Context, documentID string, version int) error {
	var docVersion DocumentVersion
	if err := s.db.WithContext(ctx).
		Where("document_id = ? AND version = ?", documentID, version).
		First(&docVersion).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("version not found")
		}
		return err
	}

	// Update document hash to match version
	return s.db.WithContext(ctx).Model(&Document{}).
		Where("id = ?", documentID).
		Update("hash", docVersion.Hash).Error
}

// Soft delete methods

// SoftDeleteDocument marks a document as deleted.
func (s *Store) SoftDeleteDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Model(&Document{}).
		Where("id = ?", documentID).
		Update("deleted_at", time.Now()).Error
}

// RestoreDocument restores a soft-deleted document.
func (s *Store) RestoreDocument(ctx context.Context, documentID string) error {
	return s.db.WithContext(ctx).Model(&Document{}).
		Where("id = ?", documentID).
		Update("deleted_at", nil).Error
}

// HardDeleteDocument permanently deletes a document.
func (s *Store) HardDeleteDocument(ctx context.Context, documentID string) error {
	return database.TransactionWithContext(s.db, ctx, func(tx *gorm.DB) error {
		// Delete chunks
		if err := tx.Where("document_id = ?", documentID).Delete(&Chunk{}).Error; err != nil {
			return fmt.Errorf("failed to delete chunks: %w", err)
		}

		// Delete versions
		if err := tx.Where("document_id = ?", documentID).Delete(&DocumentVersion{}).Error; err != nil {
			return fmt.Errorf("failed to delete versions: %w", err)
		}

		// Delete document (hard delete, ignore soft delete)
		if err := tx.Unscoped().Where("id = ?", documentID).Delete(&Document{}).Error; err != nil {
			return fmt.Errorf("failed to delete document: %w", err)
		}

		return nil
	})
}

// DB returns the underlying GORM DB instance (for job store).
func (s *Store) DB() *gorm.DB {
	return s.db
}

