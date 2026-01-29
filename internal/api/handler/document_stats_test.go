package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/index/sqlite"
)

func TestGetDocumentStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Create in-memory SQLite store
	store, err := sqlite.New(sqlite.Config{
		Path:      ":memory:",
		Dimension: 384,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create test documents
	ctx := context.Background()
	testDocs := []*index.StoredDocument{
		{
			ID:        "doc1",
			Source:    "test1.md",
			Title:     "Test Document 1",
			Format:    "markdown",
			Hash:      "hash1",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc2",
			Source:    "test2.txt",
			Title:     "Test Document 2",
			Format:    "text",
			Hash:      "hash2",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, doc := range testDocs {
		if err := store.StoreDocument(ctx, doc); err != nil {
			t.Fatalf("failed to store document: %v", err)
		}
	}

	// Create handler
	logger, _ := zap.NewDevelopment()
	h := &Handler{
		vectorStore: store,
		logger:      logger,
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/stats", nil)
	w := httptest.NewRecorder()

	// Call handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.GetDocumentStats(c)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}

	stats, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be map, got %T. Data: %+v", resp.Data, resp.Data)
	}

	// Check required fields
	requiredFields := []string{"total_documents", "total_chunks", "total_size"}
	for _, field := range requiredFields {
		if _, ok := stats[field]; !ok {
			t.Errorf("expected field %s in stats. Stats: %+v", field, stats)
		}
	}

	// Check total_documents
	if totalDocs, ok := stats["total_documents"].(float64); ok {
		if int(totalDocs) != 2 {
			t.Errorf("expected total_documents=2, got %v", totalDocs)
		}
	} else {
		t.Errorf("expected total_documents to be float64, got %T. Value: %v", stats["total_documents"], stats["total_documents"])
	}
}

func TestGetDocumentStats_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// Create empty store
	store, err := sqlite.New(sqlite.Config{
		Path:      ":memory:",
		Dimension: 384,
	})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create handler
	logger, _ := zap.NewDevelopment()
	h := &Handler{
		vectorStore: store,
		logger:      logger,
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents/stats", nil)
	w := httptest.NewRecorder()

	// Call handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.GetDocumentStats(c)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}

	stats, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be map, got %T. Data: %+v", resp.Data, resp.Data)
	}

	// Check that all counts are 0
	if totalDocs, ok := stats["total_documents"].(float64); ok {
		if int(totalDocs) != 0 {
			t.Errorf("expected total_documents=0, got %v", totalDocs)
		}
	} else {
		t.Errorf("expected total_documents to be float64, got %T. Value: %v", stats["total_documents"], stats["total_documents"])
	}
}
