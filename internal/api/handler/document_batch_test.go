package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/chunking"
	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
	"github.com/krc/rag/internal/metrics"
)

var (
	testMetricsOnce sync.Once
	testMetrics     *metrics.Metrics
)

func TestBatchUploadDocuments_MultipleFiles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create test handler
	h := createTestHandler()

	// Create multipart form with multiple files
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add first file
	part1, _ := writer.CreateFormFile("files", "test1.txt")
	part1.Write([]byte("Test content 1"))

	// Add second file
	part2, _ := writer.CreateFormFile("files", "test2.md")
	part2.Write([]byte("# Test Markdown"))

	writer.Close()

	// Create request
	req := httptest.NewRequest("POST", "/api/v1/documents/batch", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// Call handler
	h.BatchUploadDocuments(c)

	// Check response
	if w.Code != http.StatusOK && w.Code != http.StatusMultiStatus {
		t.Errorf("expected status 200 or 207, got %d", w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got false: %s", w.Body.String())
	}
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data map, got %T", resp.Data)
	}
	total, _ := data["total"].(float64)
	if int(total) != 2 {
		t.Errorf("expected total=2, got %v", total)
	}

	if mw, ok := h.jobQueue.(*mockWorker); ok {
		if len(mw.submitted) != 2 {
			t.Fatalf("expected 2 submitted jobs, got %d", len(mw.submitted))
		}
	}
}

func TestBatchUploadDocuments_ZIPFile(t *testing.T) {
	// This test requires actual ZIP file creation
	// For now, we'll test the error case
	gin.SetMode(gin.TestMode)
	h := createTestHandler()

	// Create request with invalid ZIP
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("zip", "test.zip")
	part.Write([]byte("not a zip file"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/documents/batch", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.BatchUploadDocuments(c)

	// Should return error for invalid ZIP
	if w.Code == http.StatusOK {
		t.Error("expected error for invalid ZIP file")
	}
}

func TestBatchDeleteDocuments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := createTestHandler()

	reqBody := BatchDeleteDocumentsRequest{
		DocumentIDs: []string{"doc1", "doc2"},
		Hard:        false,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/documents/batch/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.BatchDeleteDocuments(c)

	// Should process the request
	if w.Code != http.StatusOK && w.Code != http.StatusMultiStatus {
		t.Errorf("expected status 200 or 207, got %d", w.Code)
	}
}

// Helper function to create multipart form with multiple files
func createMultipartForm(files map[string][]byte) (*bytes.Buffer, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for filename, content := range files {
		part, err := writer.CreateFormFile("files", filename)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(content); err != nil {
			return nil, "", err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, "", err
	}

	return body, writer.FormDataContentType(), nil
}

// createTestHandler creates a test handler with minimal dependencies.
func createTestHandler() *Handler {
	logger := zap.NewNop()
	parserRegistry := ingestion.NewParserRegistry()
	parserRegistry.Register(ingestion.NewTextParser())
	parserRegistry.Register(ingestion.NewMarkdownParser())

	// Create mock vector store (for batch delete)
	mockStore := &mockVectorStore{}

	mw := &mockWorker{}
	return &Handler{
		logger:         logger,
		parserRegistry: parserRegistry,
		vectorStore:    mockStore,
		jobQueue:       mw,
		metrics: func() *metrics.Metrics {
			testMetricsOnce.Do(func() {
				testMetrics = metrics.NewMetrics()
			})
			return testMetrics
		}(),
	}
}

type mockWorker struct {
	submitted []*job.Job
}

func (m *mockWorker) Submit(ctx context.Context, j *job.Job) error {
	m.submitted = append(m.submitted, j)
	return nil
}
func (m *mockWorker) Start(ctx context.Context) error { return nil }
func (m *mockWorker) Stop()                           {}

// mockVectorStore implements index.VectorStore for tests.
type mockVectorStore struct{}

func (m *mockVectorStore) Store(ctx context.Context, chunks []chunking.ChunkWithVector) error { return nil }
func (m *mockVectorStore) Search(ctx context.Context, query []float32, opts index.SearchOptions) ([]index.SearchResult, error) {
	return nil, nil
}
func (m *mockVectorStore) DeleteByDocument(ctx context.Context, documentID string) error { return nil }
func (m *mockVectorStore) GetDocumentHash(ctx context.Context, documentID string) (string, error) { return "", nil }
func (m *mockVectorStore) ReplaceDocument(ctx context.Context, documentID string, chunks []chunking.ChunkWithVector) error {
	return nil
}
func (m *mockVectorStore) ListDocuments(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	return &index.ListDocumentsResult{Documents: nil, Total: 0, Limit: opts.Limit, Offset: opts.Offset}, nil
}
func (m *mockVectorStore) GetStats(ctx context.Context) (*index.Stats, error) { return &index.Stats{}, nil }
func (m *mockVectorStore) Close() error                                        { return nil }
func (m *mockVectorStore) StoreVersion(ctx context.Context, documentID string, changeNote string) error {
	return nil
}
func (m *mockVectorStore) ListVersions(ctx context.Context, documentID string) ([]index.DocumentVersion, error) {
	return nil, nil
}
func (m *mockVectorStore) RestoreVersion(ctx context.Context, documentID string, version int) error { return nil }
func (m *mockVectorStore) SoftDeleteDocument(ctx context.Context, documentID string) error          { return nil }
func (m *mockVectorStore) RestoreDocument(ctx context.Context, documentID string) error             { return nil }
func (m *mockVectorStore) HardDeleteDocument(ctx context.Context, documentID string) error          { return nil }
func (m *mockVectorStore) StoreDocument(ctx context.Context, doc *index.StoredDocument) error      { return nil }
