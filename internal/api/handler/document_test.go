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

func TestListDocuments(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	w := httptest.NewRecorder()

	// Call handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.ListDocuments(c)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}

	// Extract documents from response
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp.Data)
	}

	documents, ok := respData["documents"].([]interface{})
	if !ok {
		t.Fatalf("expected documents to be array, got %T", respData["documents"])
	}

	if len(documents) != 2 {
		t.Errorf("expected 2 documents, got %d", len(documents))
	}

	// Verify document fields (order may vary, so check both)
	doc1 := documents[0].(map[string]interface{})
	doc2 := documents[1].(map[string]interface{})

	// Check that both documents are present
	ids := []string{doc1["id"].(string), doc2["id"].(string)}
	if ids[0] != "doc1" && ids[0] != "doc2" {
		t.Errorf("unexpected document id: %v", ids[0])
	}
	if ids[1] != "doc1" && ids[1] != "doc2" {
		t.Errorf("unexpected document id: %v", ids[1])
	}
	if ids[0] == ids[1] {
		t.Errorf("duplicate document ids: %v", ids)
	}

	// Verify titles
	titles := []string{doc1["title"].(string), doc2["title"].(string)}
	if titles[0] != "Test Document 1" && titles[0] != "Test Document 2" {
		t.Errorf("unexpected document title: %v", titles[0])
	}
	if titles[1] != "Test Document 1" && titles[1] != "Test Document 2" {
		t.Errorf("unexpected document title: %v", titles[1])
	}
}

func TestListDocuments_Empty(t *testing.T) {
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

	// Create handler
	logger, _ := zap.NewDevelopment()
	h := &Handler{
		vectorStore: store,
		logger:      logger,
	}

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	w := httptest.NewRecorder()

	// Call handler
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.ListDocuments(c)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Errorf("expected success=true, got false")
	}

	// Extract documents from response
	respData, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be map, got %T", resp.Data)
	}

	documents, ok := respData["documents"].([]interface{})
	if !ok {
		t.Fatalf("expected documents to be array, got %T", respData["documents"])
	}

	if len(documents) != 0 {
		t.Errorf("expected 0 documents, got %d", len(documents))
	}
}
