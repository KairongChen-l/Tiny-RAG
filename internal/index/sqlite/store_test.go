package sqlite

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
)

func TestStore_Basic(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "rag-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Create store
	store, err := New(Config{
		Path:      tmpFile.Name(),
		Dimension: 1536,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// Test document storage
	doc := &index.StoredDocument{
		ID:        "doc1",
		Source:    "/path/to/test.md",
		Title:     "Test Document",
		Format:    "markdown",
		Hash:      "abc123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatalf("StoreDocument failed: %v", err)
	}

	// Test document retrieval
	retrieved, err := store.GetDocument(ctx, "doc1")
	if err != nil {
		t.Fatalf("GetDocument failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetDocument returned nil")
	}
	if retrieved.ID != "doc1" {
		t.Errorf("expected doc ID 'doc1', got '%s'", retrieved.ID)
	}

	// Test hash retrieval
	hash, err := store.GetDocumentHash(ctx, "doc1")
	if err != nil {
		t.Fatalf("GetDocumentHash failed: %v", err)
	}
	if hash != "abc123" {
		t.Errorf("expected hash 'abc123', got '%s'", hash)
	}
}

func TestStore_Chunks(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "rag-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	store, err := New(Config{
		Path:      tmpFile.Name(),
		Dimension: 4, // Small dimension for testing
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store document first
	doc := &index.StoredDocument{
		ID:        "doc1",
		Source:    "/path/to/test.md",
		Title:     "Test Document",
		Format:    "markdown",
		Hash:      "abc123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatalf("StoreDocument failed: %v", err)
	}

	// Store chunks
	chunks := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:          "chunk1",
				DocumentID:  "doc1",
				Content:     "This is test content 1",
				SectionPath: "Section1",
				Position:    0,
				Metadata:    map[string]string{"key": "value1"},
				Hash:        "hash1",
			},
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
		},
		{
			Chunk: chunking.Chunk{
				ID:          "chunk2",
				DocumentID:  "doc1",
				Content:     "This is test content 2",
				SectionPath: "Section2",
				Position:    1,
				Metadata:    map[string]string{"key": "value2"},
				Hash:        "hash2",
			},
			Vector: []float32{0.5, 0.6, 0.7, 0.8},
		},
	}

	if err := store.Store(ctx, chunks); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Test fallback search (since sqlite-vec won't be available in tests)
	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, index.SearchOptions{
		TopK: 10,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestStore_DeleteByDocument(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "rag-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	store, err := New(Config{
		Path:      tmpFile.Name(),
		Dimension: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store document and chunks
	doc := &index.StoredDocument{
		ID:        "doc1",
		Source:    "/path/to/test.md",
		Title:     "Test Document",
		Format:    "markdown",
		Hash:      "abc123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatal(err)
	}

	chunks := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:         "chunk1",
				DocumentID: "doc1",
				Content:    "Test content",
				Position:   0,
				Hash:       "hash1",
			},
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
		},
	}
	if err := store.Store(ctx, chunks); err != nil {
		t.Fatal(err)
	}

	// Delete document
	if err := store.DeleteByDocument(ctx, "doc1"); err != nil {
		t.Fatalf("DeleteByDocument failed: %v", err)
	}

	// Verify document is deleted
	retrieved, err := store.GetDocument(ctx, "doc1")
	if err != nil {
		t.Fatal(err)
	}
	if retrieved != nil {
		t.Error("document should have been deleted")
	}

	// Verify chunks are deleted
	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, index.SearchOptions{TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after deletion, got %d", len(results))
	}
}

func TestStore_ListDocuments(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "rag-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	store, err := New(Config{
		Path:      tmpFile.Name(),
		Dimension: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store multiple documents
	for i := 1; i <= 3; i++ {
		doc := &index.StoredDocument{
			ID:        "doc" + itoa(i),
			Source:    "/path/to/test" + itoa(i) + ".md",
			Title:     "Test Document " + itoa(i),
			Format:    "markdown",
			Hash:      "hash" + itoa(i),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := store.StoreDocument(ctx, doc); err != nil {
			t.Fatal(err)
		}
	}

	// List documents
	docs, err := store.ListDocuments(ctx)
	if err != nil {
		t.Fatalf("ListDocuments failed: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("expected 3 documents, got %d", len(docs))
	}
}

