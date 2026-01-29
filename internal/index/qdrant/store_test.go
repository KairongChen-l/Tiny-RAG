package qdrant

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
)

// getTestQdrantURL returns Qdrant URL from environment or default.
func getTestQdrantURL() string {
	url := os.Getenv("QDRANT_URL")
	if url == "" {
		url = "http://localhost:6333"
	}
	return url
}

// TestStore_DeleteByDocument tests document deletion in Qdrant.
func TestStore_DeleteByDocument(t *testing.T) {
	// Skip if Qdrant is not available
	url := getTestQdrantURL()
	store, err := New(Config{
		URL:        url,
		Collection: "test_delete_" + time.Now().Format("20060102150405"),
		Dimension:  4,
	})
	if err != nil {
		t.Skipf("Skipping test: failed to connect to Qdrant at %s: %v", url, err)
	}

	ctx := context.Background()

	// Store a document with chunks
	docID := uuid.New().String()
	chunk1ID := uuid.New().String()
	chunk2ID := uuid.New().String()
	chunks := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:         chunk1ID,
				DocumentID: docID,
				Content:    "Test content 1",
				Position:   0,
				Hash:       "hash1",
				Metadata: map[string]string{
					"source": "test.md",
					"title":  "Test Document",
				},
			},
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
		},
		{
			Chunk: chunking.Chunk{
				ID:         chunk2ID,
				DocumentID: docID,
				Content:    "Test content 2",
				Position:   1,
				Hash:       "hash2",
				Metadata: map[string]string{
					"source": "test.md",
					"title":  "Test Document",
				},
			},
			Vector: []float32{0.5, 0.6, 0.7, 0.8},
		},
	}

	// Store chunks
	if err := store.Store(ctx, chunks); err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// Verify chunks exist by searching
	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, index.SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("Expected to find chunks after storing")
	}

	// Delete document
	if err := store.DeleteByDocument(ctx, docID); err != nil {
		t.Fatalf("DeleteByDocument() failed: %v", err)
	}

	// Verify chunks are deleted
	results, err = store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, index.SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() failed after deletion: %v", err)
	}

	// Check that chunks from deleted document are not in results
	for _, result := range results {
		if result.Chunk.DocumentID == docID {
			t.Errorf("Found chunk from deleted document: %s", result.Chunk.ID)
		}
	}
}

// TestStore_DeleteByDocument_NonExistent tests deletion of non-existent document.
func TestStore_DeleteByDocument_NonExistent(t *testing.T) {
	url := getTestQdrantURL()
	store, err := New(Config{
		URL:        url,
		Collection: "test_delete_nonexistent_" + time.Now().Format("20060102150405"),
		Dimension:  4,
	})
	if err != nil {
		t.Skipf("Skipping test: failed to connect to Qdrant at %s: %v", url, err)
	}

	ctx := context.Background()

	// Delete non-existent document should not error
	err = store.DeleteByDocument(ctx, "non-existent-doc")
	if err != nil {
		t.Errorf("DeleteByDocument() should not error for non-existent document: %v", err)
	}
}

// TestStore_DeleteByDocument_MultipleDocuments tests that deletion only affects target document.
func TestStore_DeleteByDocument_MultipleDocuments(t *testing.T) {
	url := getTestQdrantURL()
	store, err := New(Config{
		URL:        url,
		Collection: "test_delete_multiple_" + time.Now().Format("20060102150405"),
		Dimension:  4,
	})
	if err != nil {
		t.Skipf("Skipping test: failed to connect to Qdrant at %s: %v", url, err)
	}

	ctx := context.Background()

	// Store two documents
	doc1ID := uuid.New().String()
	doc2ID := uuid.New().String()
	chunk1Doc1ID := uuid.New().String()
	chunk1Doc2ID := uuid.New().String()

	chunks1 := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:         chunk1Doc1ID,
				DocumentID: doc1ID,
				Content:    "Document 1 content",
				Position:   0,
				Hash:       "hash1",
			},
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
		},
	}

	chunks2 := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:         chunk1Doc2ID,
				DocumentID: doc2ID,
				Content:    "Document 2 content",
				Position:   0,
				Hash:       "hash2",
			},
			Vector: []float32{0.5, 0.6, 0.7, 0.8},
		},
	}

	if err := store.Store(ctx, chunks1); err != nil {
		t.Fatalf("Store() failed for doc1: %v", err)
	}
	if err := store.Store(ctx, chunks2); err != nil {
		t.Fatalf("Store() failed for doc2: %v", err)
	}

	// Delete only doc1
	if err := store.DeleteByDocument(ctx, doc1ID); err != nil {
		t.Fatalf("DeleteByDocument() failed: %v", err)
	}

	// Verify doc1 chunks are deleted
	results, err := store.Search(ctx, []float32{0.1, 0.2, 0.3, 0.4}, index.SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() failed: %v", err)
	}
	for _, result := range results {
		if result.Chunk.DocumentID == doc1ID {
			t.Errorf("Found chunk from deleted document doc1: %s", result.Chunk.ID)
		}
	}

	// Verify doc2 chunks still exist
	results, err = store.Search(ctx, []float32{0.5, 0.6, 0.7, 0.8}, index.SearchOptions{TopK: 10})
	if err != nil {
		t.Fatalf("Search() failed: %v", err)
	}
	foundDoc2 := false
	for _, result := range results {
		if result.Chunk.DocumentID == doc2ID {
			foundDoc2 = true
			break
		}
	}
	if !foundDoc2 {
		t.Error("Document 2 chunks should still exist after deleting document 1")
	}
}

// TestStore_ListDocuments_AfterDeletion tests that deleted documents don't appear in list.
func TestStore_ListDocuments_AfterDeletion(t *testing.T) {
	url := getTestQdrantURL()
	store, err := New(Config{
		URL:        url,
		Collection: "test_list_after_delete_" + time.Now().Format("20060102150405"),
		Dimension:  4,
	})
	if err != nil {
		t.Skipf("Skipping test: failed to connect to Qdrant at %s: %v", url, err)
	}

	ctx := context.Background()

	// Store document
	docID := uuid.New().String()
	chunkID := uuid.New().String()
	now := time.Now()
	doc := &index.StoredDocument{
		ID:        docID,
		Source:    "test.md",
		Title:     "Test Document",
		Format:    "markdown",
		Hash:      "test-hash",
		CreatedAt: now,
		UpdatedAt: now,
		Metadata: map[string]string{
			"title": "Test Document",
		},
	}

	if err := store.StoreDocument(ctx, doc); err != nil {
		t.Fatalf("StoreDocument() failed: %v", err)
	}

	chunks := []chunking.ChunkWithVector{
		{
			Chunk: chunking.Chunk{
				ID:         chunkID,
				DocumentID: docID,
				Content:    "Test content",
				Position:   0,
				Hash:       "hash1",
				Metadata: map[string]string{
					"source": "test.md",
					"title":  "Test Document",
				},
			},
			Vector: []float32{0.1, 0.2, 0.3, 0.4},
		},
	}

	if err := store.Store(ctx, chunks); err != nil {
		t.Fatalf("Store() failed: %v", err)
	}

	// List documents - should include our document
	docs, err := store.ListDocuments(ctx, index.ListOptions{})
	if err != nil {
		t.Fatalf("ListDocuments() failed: %v", err)
	}

	found := false
	for _, d := range docs.Documents {
		if d.ID == docID {
			found = true
			break
		}
	}
	if !found {
		t.Error("Document should appear in list before deletion")
	}

	// Delete document
	if err := store.DeleteByDocument(ctx, docID); err != nil {
		t.Fatalf("DeleteByDocument() failed: %v", err)
	}

	// List documents again - should not include deleted document
	docs, err = store.ListDocuments(ctx, index.ListOptions{})
	if err != nil {
		t.Fatalf("ListDocuments() failed after deletion: %v", err)
	}

	for _, d := range docs.Documents {
		if d.ID == docID {
			t.Error("Deleted document should not appear in list")
		}
	}
}

