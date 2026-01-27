package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
)

func TestStore_StoreVersion(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	store, err := New(Config{
		Path:      ":memory:",
		Dimension: 1536,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a document first
	docID := "test-doc-1"
	doc := &index.StoredDocument{
		ID:        docID,
		Source:    "test.md",
		Title:     "Test Document",
		Format:    "markdown",
		Hash:      "hash1",
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatalf("failed to store document: %v", err)
	}

	// Store some chunks
	chunks := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:         "chunk-1",
				DocumentID: docID,
				Content:    "Test content 1",
			},
			Vector: make([]float32, 1536),
		},
	}
	if err := store.Store(ctx, chunks); err != nil {
		t.Fatalf("failed to store chunks: %v", err)
	}

	// Store version
	if err := store.StoreVersion(ctx, docID, "Initial version"); err != nil {
		t.Fatalf("failed to store version: %v", err)
	}

	// List versions
	versions, err := store.ListVersions(ctx, docID)
	if err != nil {
		t.Fatalf("failed to list versions: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}

	if versions[0].Version != 1 {
		t.Errorf("expected version 1, got %d", versions[0].Version)
	}

	if versions[0].Hash != "hash1" {
		t.Errorf("expected hash 'hash1', got '%s'", versions[0].Hash)
	}

	if versions[0].ChunkCount != 1 {
		t.Errorf("expected chunk count 1, got %d", versions[0].ChunkCount)
	}
}

func TestStore_RestoreVersion(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	store, err := New(Config{
		Path:      ":memory:",
		Dimension: 1536,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Create a document
	docID := "test-doc-2"
	doc := &index.StoredDocument{
		ID:        docID,
		Source:    "test.md",
		Title:     "Test Document",
		Format:    "markdown",
		Hash:      "hash1",
		Metadata:  make(map[string]string),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatalf("failed to store document: %v", err)
	}

	// Store version
	if err := store.StoreVersion(ctx, docID, "Version 1"); err != nil {
		t.Fatalf("failed to store version: %v", err)
	}

	// Update document hash
	doc.Hash = "hash2"
	doc.UpdatedAt = time.Now()
	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatalf("failed to update document: %v", err)
	}

	// Restore to version 1
	if err := store.RestoreVersion(ctx, docID, 1); err != nil {
		t.Fatalf("failed to restore version: %v", err)
	}

	// Verify document hash was restored
	restoredDoc, err := store.GetDocument(ctx, docID)
	if err != nil {
		t.Fatalf("failed to get document: %v", err)
	}

	if restoredDoc.Hash != "hash1" {
		t.Errorf("expected hash 'hash1' after restore, got '%s'", restoredDoc.Hash)
	}
}

