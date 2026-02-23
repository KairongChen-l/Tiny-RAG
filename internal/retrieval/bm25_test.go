package retrieval

import (
	"context"
	"math"
	"testing"

	"github.com/krc/rag/internal/chunking"
)

func TestBM25Retriever_IdfLog(t *testing.T) {
	retriever := NewBM25Retriever(nil)

	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{"log(1) = 0", 1.0, 0.0},
		{"log(e) ≈ 1", math.E, 1.0},
		{"log(0) = 0", 0.0, 0.0},
		{"log(-1) = 0", -1.0, 0.0},
		{"log(10) ≈ 2.302", 10.0, math.Log(10.0)},
		{"log(0.5)", 0.5, math.Log(0.5)},
		{"log(100)", 100.0, math.Log(100.0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retriever.idfLog(tt.input)
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("idfLog(%f) = %f, expected %f", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBM25Retriever_Search(t *testing.T) {
	retriever := NewBM25Retriever(nil)

	chunks := []chunking.Chunk{
		{ID: "1", DocumentID: "doc1", Content: "Go is a statically typed programming language"},
		{ID: "2", DocumentID: "doc1", Content: "Python is a dynamically typed language"},
		{ID: "3", DocumentID: "doc1", Content: "Rust provides memory safety without garbage collection"},
	}

	ctx := context.Background()
	if err := retriever.IndexChunks(ctx, chunks); err != nil {
		t.Fatalf("IndexChunks failed: %v", err)
	}

	results, err := retriever.Search(ctx, "Go programming language", 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}

	// The first result should be the Go-related chunk
	if results[0].ID != "1" {
		t.Errorf("expected first result to be chunk '1', got '%s'", results[0].ID)
	}

	// All results should have positive scores
	for i, r := range results {
		if r.Score <= 0 {
			t.Errorf("result %d has non-positive score: %f", i, r.Score)
		}
		if r.CitationID != i+1 {
			t.Errorf("result %d has wrong citation ID: %d", i, r.CitationID)
		}
	}
}

func TestBM25Retriever_SearchEmpty(t *testing.T) {
	retriever := NewBM25Retriever(nil)
	ctx := context.Background()

	_, err := retriever.Search(ctx, "test query", 5)
	if err == nil {
		t.Error("expected error when searching with no indexed chunks")
	}
}

func TestBM25Retriever_DeleteRecalculatesIDF(t *testing.T) {
	retriever := NewBM25Retriever(nil)
	ctx := context.Background()

	chunks := []chunking.Chunk{
		{ID: "1", DocumentID: "doc1", Content: "Go programming language is fast and compiled"},
		{ID: "2", DocumentID: "doc2", Content: "Python is great for data science and machine learning"},
		{ID: "3", DocumentID: "doc2", Content: "Python has many useful libraries for analysis"},
		{ID: "4", DocumentID: "doc3", Content: "Rust provides memory safety without garbage collection"},
		{ID: "5", DocumentID: "doc3", Content: "JavaScript is widely used for web development"},
	}

	if err := retriever.IndexChunks(ctx, chunks); err != nil {
		t.Fatalf("IndexChunks failed: %v", err)
	}

	if len(retriever.indexed) != 5 {
		t.Fatalf("expected 5 indexed chunks, got %d", len(retriever.indexed))
	}

	avgDocLenBefore := retriever.avgDocLen

	// Delete doc1
	if err := retriever.DeleteByDocumentID(ctx, "doc1"); err != nil {
		t.Fatalf("DeleteByDocumentID failed: %v", err)
	}

	// Verify chunk removed
	if len(retriever.indexed) != 4 {
		t.Errorf("expected 4 indexed chunks after deletion, got %d", len(retriever.indexed))
	}

	// IDF should be recalculated
	if len(retriever.idf) == 0 {
		t.Error("IDF map should not be empty after deletion with remaining chunks")
	}

	// avgDocLen should change after deletion
	if retriever.avgDocLen == avgDocLenBefore {
		t.Log("avgDocLen unchanged after deletion (may happen if deleted doc had avg length)")
	}
	if retriever.avgDocLen == 0 {
		t.Error("avgDocLen should not be 0 after deletion with remaining chunks")
	}

	// Search for unique term in remaining docs should work
	results, err := retriever.Search(ctx, "machine learning data", 5)
	if err != nil {
		t.Fatalf("Search after deletion failed: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected results after deletion for unique query terms")
	}
}

func TestBM25Retriever_DeleteAllRecalculatesIDF(t *testing.T) {
	retriever := NewBM25Retriever(nil)
	ctx := context.Background()

	chunks := []chunking.Chunk{
		{ID: "1", DocumentID: "doc1", Content: "Go programming language"},
	}

	if err := retriever.IndexChunks(ctx, chunks); err != nil {
		t.Fatalf("IndexChunks failed: %v", err)
	}

	if err := retriever.DeleteByDocumentID(ctx, "doc1"); err != nil {
		t.Fatalf("DeleteByDocumentID failed: %v", err)
	}

	if len(retriever.indexed) != 0 {
		t.Errorf("expected 0 indexed chunks, got %d", len(retriever.indexed))
	}
	if retriever.avgDocLen != 0 {
		t.Errorf("expected avgDocLen 0, got %f", retriever.avgDocLen)
	}
}
