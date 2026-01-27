package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
)

// Store implements VectorStore using SQLite with sqlite-vec extension.
type Store struct {
	db        *sql.DB
	dimension int
	mu        sync.RWMutex
}

// Config holds SQLite store configuration.
type Config struct {
	Path      string // Database file path
	Dimension int    // Vector dimension (e.g., 1536 for OpenAI)
}

// New creates a new SQLite vector store.
func New(cfg Config) (*Store, error) {
	// Ensure directory exists
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database with sqlite-vec extension enabled
	db, err := sql.Open("sqlite3", cfg.Path+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &Store{
		db:        db,
		dimension: cfg.Dimension,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the database tables.
func (s *Store) initSchema() error {
	// Create main schema
	if _, err := s.db.Exec(Schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Try to create vector table (will fail if sqlite-vec not available)
	// This is optional - we can still work without vectors for testing
	vecSchema := VectorTableSchema(s.dimension)
	if _, err := s.db.Exec(vecSchema); err != nil {
		// Log warning but don't fail - sqlite-vec might not be available
		// In production, you'd want to handle this differently
		fmt.Printf("Warning: Could not create vector table (sqlite-vec may not be loaded): %v\n", err)
	}

	return nil
}

// Store stores chunks with their vectors.
func (s *Store) Store(ctx context.Context, chunks []chunking.ChunkWithVector) error {
	if len(chunks) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Prepare statements
	chunkStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (id, document_id, content, section_path, position, metadata, hash, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(id) DO UPDATE SET
			content = excluded.content,
			section_path = excluded.section_path,
			position = excluded.position,
			metadata = excluded.metadata,
			hash = excluded.hash,
			version = version + 1
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare chunk statement: %w", err)
	}
	defer chunkStmt.Close()

	// Insert chunks
	for _, chunk := range chunks {
		metadata, _ := json.Marshal(chunk.Metadata)

		_, err := chunkStmt.ExecContext(ctx,
			chunk.ID,
			chunk.DocumentID,
			chunk.Content,
			chunk.SectionPath,
			chunk.Position,
			string(metadata),
			chunk.Hash,
		)
		if err != nil {
			return fmt.Errorf("failed to insert chunk %s: %w", chunk.ID, err)
		}

		// Try to insert vector (may fail if sqlite-vec not available)
		if len(chunk.Vector) > 0 {
			// Check if chunk_vectors table exists first to avoid repeated warnings
			var tableExists int
			_ = tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='chunk_vectors'`,
			).Scan(&tableExists)

			if tableExists > 0 {
				_, err = tx.ExecContext(ctx,
					`INSERT OR REPLACE INTO chunk_vectors (chunk_id, embedding) VALUES (?, ?)`,
					chunk.ID, vectorToBlob(chunk.Vector),
				)
				if err != nil {
					// Only log if it's not a "table doesn't exist" error
					if err.Error() != "no such table: chunk_vectors" {
						fmt.Printf("Warning: Could not store vector for chunk %s: %v\n", chunk.ID, err)
					}
				}
			}
		}
	}

	return tx.Commit()
}

// Search performs vector similarity search.
func (s *Store) Search(ctx context.Context, query []float32, opts index.SearchOptions) ([]index.SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Try vector search first
	results, err := s.vectorSearch(ctx, query, topK, opts)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// Fallback to returning all chunks (for testing without sqlite-vec)
	return s.fallbackSearch(ctx, topK, opts)
}

// vectorSearch performs actual vector similarity search.
func (s *Store) vectorSearch(ctx context.Context, query []float32, topK int, opts index.SearchOptions) ([]index.SearchResult, error) {
	// Build query with optional filters
	baseQuery := `
		SELECT c.id, c.document_id, c.content, c.section_path, c.position, c.metadata, c.hash,
		       vec_distance_cosine(v.embedding, ?) as distance
		FROM chunks c
		JOIN chunk_vectors v ON c.id = v.chunk_id
	`

	args := []interface{}{vectorToBlob(query)}

	// Add metadata filters if present
	if len(opts.MetadataFilter) > 0 {
		for key, value := range opts.MetadataFilter {
			baseQuery += fmt.Sprintf(" AND json_extract(c.metadata, '$.%s') = ?", key)
			args = append(args, value)
		}
	}

	baseQuery += " ORDER BY distance ASC LIMIT ?"
	args = append(args, topK)

	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []index.SearchResult
	citationNum := 1

	for rows.Next() {
		var (
			id, docID, content, sectionPath, hash string
			position                              int
			metadataStr                           string
			distance                              float64
		)

		if err := rows.Scan(&id, &docID, &content, &sectionPath, &position, &metadataStr, &hash, &distance); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		// Parse metadata
		var metadata map[string]string
		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &metadata)
		}

		// Convert distance to similarity score (cosine distance to similarity)
		score := float32(1.0 - distance)

		// Apply minimum score filter
		if opts.MinScore > 0 && score < opts.MinScore {
			continue
		}

		results = append(results, index.SearchResult{
			Chunk: chunking.Chunk{
				ID:          id,
				DocumentID:  docID,
				Content:     content,
				SectionPath: sectionPath,
				Position:    position,
				Metadata:    metadata,
				Hash:        hash,
			},
			Score:    score,
			Citation: citationNum,
		})
		citationNum++
	}

	return results, rows.Err()
}

// fallbackSearch returns chunks without vector search (for testing).
func (s *Store) fallbackSearch(ctx context.Context, topK int, opts index.SearchOptions) ([]index.SearchResult, error) {
	query := `SELECT id, document_id, content, section_path, position, metadata, hash FROM chunks LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []index.SearchResult
	citationNum := 1

	for rows.Next() {
		var (
			id, docID, content, sectionPath, hash string
			position                              int
			metadataStr                           sql.NullString
		)

		if err := rows.Scan(&id, &docID, &content, &sectionPath, &position, &metadataStr, &hash); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var metadata map[string]string
		if metadataStr.Valid && metadataStr.String != "" {
			json.Unmarshal([]byte(metadataStr.String), &metadata)
		}

		results = append(results, index.SearchResult{
			Chunk: chunking.Chunk{
				ID:          id,
				DocumentID:  docID,
				Content:     content,
				SectionPath: sectionPath,
				Position:    position,
				Metadata:    metadata,
				Hash:        hash,
			},
			Score:    0.5, // Placeholder score
			Citation: citationNum,
		})
		citationNum++
	}

	return results, rows.Err()
}

// SoftDeleteDocument marks a document as deleted without removing it.
func (s *Store) SoftDeleteDocument(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		"UPDATE documents SET deleted_at = ? WHERE id = ?",
		time.Now(), documentID,
	)
	if err != nil {
		return fmt.Errorf("failed to soft delete document: %w", err)
	}

	return nil
}

// RestoreDocument restores a soft-deleted document.
func (s *Store) RestoreDocument(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx,
		"UPDATE documents SET deleted_at = NULL WHERE id = ?",
		documentID,
	)
	if err != nil {
		return fmt.Errorf("failed to restore document: %w", err)
	}

	return nil
}

// HardDeleteDocument permanently deletes a document (including soft-deleted ones).
func (s *Store) HardDeleteDocument(ctx context.Context, documentID string) error {
	// This is the same as DeleteByDocument - permanently delete
	return s.DeleteByDocument(ctx, documentID)
}

// DeleteByDocument deletes all chunks for a document (hard delete).
func (s *Store) DeleteByDocument(ctx context.Context, documentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get chunk IDs for vector deletion
	rows, err := tx.QueryContext(ctx, "SELECT id FROM chunks WHERE document_id = ?", documentID)
	if err != nil {
		return err
	}

	var chunkIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		chunkIDs = append(chunkIDs, id)
	}
	rows.Close()

	// Delete vectors
	for _, id := range chunkIDs {
		tx.ExecContext(ctx, "DELETE FROM chunk_vectors WHERE chunk_id = ?", id)
	}

	// Delete chunks (cascades from documents due to foreign key)
	if _, err := tx.ExecContext(ctx, "DELETE FROM chunks WHERE document_id = ?", documentID); err != nil {
		return err
	}

	// Delete document
	if _, err := tx.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", documentID); err != nil {
		return err
	}

	return tx.Commit()
}

// GetDocumentHash returns the hash of a stored document.
func (s *Store) GetDocumentHash(ctx context.Context, documentID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var hash string
	err := s.db.QueryRowContext(ctx,
		"SELECT hash FROM documents WHERE id = ?",
		documentID,
	).Scan(&hash)

	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// ReplaceDocument atomically replaces all chunks for a document.
func (s *Store) ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get existing chunk IDs
	rows, err := tx.QueryContext(ctx, "SELECT id FROM chunks WHERE document_id = ?", documentID)
	if err != nil {
		return err
	}

	var oldChunkIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		oldChunkIDs = append(oldChunkIDs, id)
	}
	rows.Close()

	// Insert new chunks
	chunkStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks (id, document_id, content, section_path, position, metadata, hash, version)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
	`)
	if err != nil {
		return err
	}
	defer chunkStmt.Close()

	for _, chunk := range chunks {
		metadata, _ := json.Marshal(chunk.Metadata)

		if _, err := chunkStmt.ExecContext(ctx,
			chunk.ID, chunk.DocumentID, chunk.Content, chunk.SectionPath,
			chunk.Position, string(metadata), chunk.Hash,
		); err != nil {
			return err
		}

		// Insert vector
		if len(chunk.Vector) > 0 {
			tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO chunk_vectors (chunk_id, embedding) VALUES (?, ?)`,
				chunk.ID, vectorToBlob(chunk.Vector),
			)
		}
	}

	// Delete old chunks and vectors
	for _, id := range oldChunkIDs {
		tx.ExecContext(ctx, "DELETE FROM chunk_vectors WHERE chunk_id = ?", id)
		tx.ExecContext(ctx, "DELETE FROM chunks WHERE id = ?", id)
	}

	// Update document timestamp
	if _, err := tx.ExecContext(ctx,
		"UPDATE documents SET updated_at = ? WHERE id = ?",
		time.Now(), documentID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// StoreDocument stores or updates a document record.
func (s *Store) StoreDocument(ctx context.Context, doc *index.StoredDocument) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadata, _ := json.Marshal(doc.Metadata)

	var deletedAt interface{}
	if doc.DeletedAt != nil {
		deletedAt = doc.DeletedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO documents (id, source, title, format, hash, metadata, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source = excluded.source,
			title = excluded.title,
			format = excluded.format,
			hash = excluded.hash,
			metadata = excluded.metadata,
			updated_at = excluded.updated_at,
			deleted_at = excluded.deleted_at
	`, doc.ID, doc.Source, doc.Title, doc.Format, doc.Hash, string(metadata), doc.CreatedAt, doc.UpdatedAt, deletedAt)
	if err != nil {
		return err
	}

	// Update FTS index
	// Get document content from chunks for FTS
	var content string
	s.db.QueryRowContext(ctx,
		"SELECT GROUP_CONCAT(content, ' ') FROM chunks WHERE document_id = ?",
		doc.ID,
	).Scan(&content)

	// Update or insert into FTS table
	s.db.ExecContext(ctx, `
		INSERT INTO documents_fts (id, title, content) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, content = excluded.content
	`, doc.ID, doc.Title, content)

	return nil
}

// GetDocument retrieves a document by ID.
func (s *Store) GetDocument(ctx context.Context, id string) (*index.StoredDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var doc index.StoredDocument
	var metadataStr sql.NullString
	var title sql.NullString

	var deletedAt sql.NullTime
	err := s.db.QueryRowContext(ctx,
		"SELECT id, source, title, format, hash, metadata, created_at, updated_at, deleted_at FROM documents WHERE id = ?",
		id,
	).Scan(&doc.ID, &doc.Source, &title, &doc.Format, &doc.Hash, &metadataStr, &doc.CreatedAt, &doc.UpdatedAt, &deletedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if title.Valid {
		doc.Title = title.String
	}
	if metadataStr.Valid && metadataStr.String != "" {
		json.Unmarshal([]byte(metadataStr.String), &doc.Metadata)
	}
	if deletedAt.Valid {
		doc.DeletedAt = &deletedAt.Time
	}

	return &doc, nil
}

// ListDocuments returns stored documents with pagination and filtering.
func (s *Store) ListDocuments(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Set defaults
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.SortBy == "" {
		opts.SortBy = "created_at"
	}
	if opts.Order == "" {
		opts.Order = "desc"
	}

	// Validate sort field
	sortField := "created_at"
	switch opts.SortBy {
	case "title", "updated_at", "created_at":
		sortField = opts.SortBy
	default:
		sortField = "created_at"
	}

	// Validate order
	order := "DESC"
	if opts.Order == "asc" {
		order = "ASC"
	}

	// Build WHERE clause
	whereClauses := []string{}
	args := []interface{}{}

	if !opts.IncludeDeleted {
		whereClauses = append(whereClauses, "deleted_at IS NULL")
	}

	// Apply metadata filters
	if len(opts.FilterBy) > 0 {
		for key, value := range opts.FilterBy {
			whereClauses = append(whereClauses, "json_extract(metadata, '$."+key+"') = ?")
			args = append(args, value)
		}
	}

	// Build WHERE clause string
	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Apply full-text search if provided
	if opts.SearchQuery != "" {
		// Use FTS5 for full-text search
		ftsWhere := fmt.Sprintf(
			"id IN (SELECT id FROM documents_fts WHERE documents_fts MATCH ?)",
		)
		if whereClause != "" {
			whereClause += " AND " + ftsWhere
		} else {
			whereClause = "WHERE " + ftsWhere
		}
		args = append([]interface{}{opts.SearchQuery}, args...)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM documents " + whereClause
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Build query with pagination, filters, and search
	query := fmt.Sprintf(
		"SELECT id, source, title, format, hash, metadata, created_at, updated_at, deleted_at FROM documents %s ORDER BY %s %s LIMIT ? OFFSET ?",
		whereClause, sortField, order,
	)
	args = append(args, opts.Limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []index.StoredDocument
	for rows.Next() {
		var doc index.StoredDocument
		var metadataStr sql.NullString
		var title sql.NullString

		var deletedAt sql.NullTime
		if err := rows.Scan(&doc.ID, &doc.Source, &title, &doc.Format, &doc.Hash, &metadataStr, &doc.CreatedAt, &doc.UpdatedAt, &deletedAt); err != nil {
			return nil, err
		}

		if title.Valid {
			doc.Title = title.String
		}
		if metadataStr.Valid && metadataStr.String != "" {
			json.Unmarshal([]byte(metadataStr.String), &doc.Metadata)
		}
		if deletedAt.Valid {
			doc.DeletedAt = &deletedAt.Time
		}

		docs = append(docs, doc)
	}

	return &index.ListDocumentsResult{
		Documents: docs,
		Total:     total,
		Limit:     opts.Limit,
		Offset:    opts.Offset,
	}, rows.Err()
}

// Close closes the database connection.
// GetStats returns statistics about stored documents and chunks.
func (s *Store) GetStats(ctx context.Context) (*index.Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &index.Stats{}

	// Count documents
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM documents").Scan(&stats.TotalDocuments)
	if err != nil {
		return nil, fmt.Errorf("failed to count documents: %w", err)
	}

	// Count chunks
	err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM chunks").Scan(&stats.TotalChunks)
	if err != nil {
		return nil, fmt.Errorf("failed to count chunks: %w", err)
	}

	// Calculate total size (sum of content lengths)
	err = s.db.QueryRowContext(ctx, "SELECT COALESCE(SUM(LENGTH(content)), 0) FROM chunks").Scan(&stats.TotalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total size: %w", err)
	}

	return stats, nil
}

// StoreVersion stores a version snapshot of a document.
func (s *Store) StoreVersion(ctx context.Context, documentID string, changeNote string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current document hash and chunk count
	var hash string
	var chunkCount int
	err := s.db.QueryRowContext(ctx,
		"SELECT hash, (SELECT COUNT(*) FROM chunks WHERE document_id = ?) FROM documents WHERE id = ?",
		documentID, documentID,
	).Scan(&hash, &chunkCount)
	if err == sql.ErrNoRows {
		return fmt.Errorf("document not found: %s", documentID)
	}
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Get next version number
	var maxVersion int
	err = s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version), 0) FROM document_versions WHERE document_id = ?",
		documentID,
	).Scan(&maxVersion)
	if err != nil {
		return fmt.Errorf("failed to get max version: %w", err)
	}

	nextVersion := maxVersion + 1

	// Store version record
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO document_versions (document_id, version, hash, chunk_count, change_note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, documentID, nextVersion, hash, chunkCount, changeNote, time.Now())
	if err != nil {
		return fmt.Errorf("failed to store version: %w", err)
	}

	return nil
}

// ListVersions returns all versions of a document.
func (s *Store) ListVersions(ctx context.Context, documentID string) ([]index.DocumentVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT version, document_id, hash, chunk_count, change_note, created_at, created_by
		FROM document_versions
		WHERE document_id = ?
		ORDER BY version DESC
	`, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer rows.Close()

	var versions []index.DocumentVersion
	for rows.Next() {
		var v index.DocumentVersion
		var changeNote, createdBy sql.NullString

		err := rows.Scan(&v.Version, &v.DocumentID, &v.Hash, &v.ChunkCount, &changeNote, &v.CreatedAt, &createdBy)
		if err != nil {
			return nil, fmt.Errorf("failed to scan version: %w", err)
		}

		if changeNote.Valid {
			v.ChangeNote = changeNote.String
		}
		if createdBy.Valid {
			v.CreatedBy = createdBy.String
		}

		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// RestoreVersion restores a document to a specific version.
// Note: This is a simplified implementation that only restores the document hash.
// A full implementation would need to restore chunks as well, which requires storing chunk snapshots.
func (s *Store) RestoreVersion(ctx context.Context, documentID string, version int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get version info
	var hash string
	var chunkCount int
	err := s.db.QueryRowContext(ctx, `
		SELECT hash, chunk_count FROM document_versions
		WHERE document_id = ? AND version = ?
	`, documentID, version).Scan(&hash, &chunkCount)
	if err == sql.ErrNoRows {
		return fmt.Errorf("version %d not found for document %s", version, documentID)
	}
	if err != nil {
		return fmt.Errorf("failed to get version: %w", err)
	}

	// Update document hash to match the version
	// Note: This is a simplified restore - in production, you'd want to restore chunks too
	_, err = s.db.ExecContext(ctx, `
		UPDATE documents SET hash = ?, updated_at = ? WHERE id = ?
	`, hash, time.Now(), documentID)
	if err != nil {
		return fmt.Errorf("failed to restore document: %w", err)
	}

	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection.
func (s *Store) DB() *sql.DB {
	return s.db
}

// vectorToBlob converts a float32 slice to bytes for SQLite storage.
func vectorToBlob(v []float32) []byte {
	// Simple binary encoding: 4 bytes per float32, little-endian
	b := make([]byte, len(v)*4)
	for i, f := range v {
		bits := math.Float32bits(f)
		b[i*4] = byte(bits)
		b[i*4+1] = byte(bits >> 8)
		b[i*4+2] = byte(bits >> 16)
		b[i*4+3] = byte(bits >> 24)
	}
	return b
}

// blobToVector converts bytes back to float32 slice.
func blobToVector(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		bits := uint32(b[i*4]) | uint32(b[i*4+1])<<8 | uint32(b[i*4+2])<<16 | uint32(b[i*4+3])<<24
		v[i] = math.Float32frombits(bits)
	}
	return v
}
