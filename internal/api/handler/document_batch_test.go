package handler

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/index"
	"github.com/krc/rag/internal/ingestion"
	"github.com/krc/rag/internal/job"
	"github.com/krc/rag/internal/metrics"
	"github.com/krc/rag/pkg/cache"
	"github.com/krc/rag/pkg/config"
)

func TestBatchUploadDocuments_MultipleFiles(t *testing.T) {
	// Create test handler
	h := createTestHandler(t)

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

	// Call handler
	h.BatchUploadDocuments(w, req)

	// Check response
	if w.Code != http.StatusOK && w.Code != http.StatusMultiStatus {
		t.Errorf("expected status 200 or 207, got %d", w.Code)
	}

	var response BatchUploadDocumentsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.Total != 2 {
		t.Errorf("expected 2 files, got %d", response.Total)
	}
}

func TestBatchUploadDocuments_ZIPFile(t *testing.T) {
	// This test requires actual ZIP file creation
	// For now, we'll test the error case
	h := createTestHandler(t)

	// Create request with invalid ZIP
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("zip", "test.zip")
	part.Write([]byte("not a zip file"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/v1/documents/batch", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()

	h.BatchUploadDocuments(w, req)

	// Should return error for invalid ZIP
	if w.Code == http.StatusOK {
		t.Error("expected error for invalid ZIP file")
	}
}

func TestBatchDeleteDocuments(t *testing.T) {
	h := createTestHandler(t)

	reqBody := BatchDeleteDocumentsRequest{
		DocumentIDs: []string{"doc1", "doc2"},
		Hard:        false,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/documents/batch/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()

	h.BatchDeleteDocuments(w, req)

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

// createTestHandler creates a test handler with minimal dependencies
func createTestHandler(t *testing.T) *Handler {
	logger := zap.NewNop()
	parserRegistry := ingestion.NewParserRegistry()
	parserRegistry.Register("text", ingestion.NewTextParser())
	parserRegistry.Register("markdown", ingestion.NewMarkdownParser())

	// Create mock vector store
	mockStore := &mockVectorStore{}

	// Create job queue and store
	jobStore := job.NewMemoryStore()
	jobQueue := job.NewQueue(job.QueueConfig{
		Workers:   1,
		QueueSize: 10,
	}, jobStore)

	metrics := metrics.NewMetrics()

	// Create minimal prompt builder
	promptBuilder := prompt.NewBuilder(prompt.Config{
		MaxContextTokens: 3000,
		IncludeCitations: true,
	})

	// Create minimal handler config
	cfg := Config{
		Config:         &config.Config{},
		Logger:         logger,
		ParserRegistry: parserRegistry,
		VectorStore:    mockStore,
		JobQueue:       jobQueue,
		JobStore:       jobStore,
		Metrics:        metrics,
		QueryCache:     cache.NewMemoryCache(),
		PromptBuilder:  promptBuilder,
	}

	return New(cfg)
}

// mockVectorStore is a minimal mock for testing
type mockVectorStore struct{}

func (m *mockVectorStore) Store(ctx context.Context, chunks []interface{}) error {
	return nil
}

func (m *mockVectorStore) Search(ctx context.Context, query []float32, opts index.SearchOptions) ([]index.SearchResult, error) {
	return nil, nil
}

func (m *mockVectorStore) DeleteByDocument(ctx context.Context, documentID string) error {
	return nil
}

func (m *mockVectorStore) StoreDocument(ctx context.Context, doc *index.StoredDocument) error {
	return nil
}

func (m *mockVectorStore) GetDocument(ctx context.Context, id string) (*index.StoredDocument, error) {
	return nil, nil
}

func (m *mockVectorStore) ListDocuments(ctx context.Context, opts index.ListOptions) (*index.ListDocumentsResult, error) {
	return &index.ListDocumentsResult{
		Documents: []index.StoredDocument{},
		Total:     0,
		Limit:     opts.Limit,
		Offset:    opts.Offset,
	}, nil
}

func (m *mockVectorStore) GetStats(ctx context.Context) (*index.Stats, error) {
	return &index.Stats{}, nil
}

func (m *mockVectorStore) GetDocumentHash(ctx context.Context, documentID string) (string, error) {
	return "", nil
}

func (m *mockVectorStore) ReplaceDocument(ctx context.Context, documentID string, chunks []interface{}) error {
	return nil
}

func (m *mockVectorStore) SoftDeleteDocument(ctx context.Context, documentID string) error {
	return nil
}

func (m *mockVectorStore) RestoreDocument(ctx context.Context, documentID string) error {
	return nil
}

func (m *mockVectorStore) HardDeleteDocument(ctx context.Context, documentID string) error {
	return nil
}

func (m *mockVectorStore) StoreVersion(ctx context.Context, documentID string, changeNote string) error {
	return nil
}

func (m *mockVectorStore) ListVersions(ctx context.Context, documentID string) ([]index.DocumentVersion, error) {
	return nil, nil
}

func (m *mockVectorStore) RestoreVersion(ctx context.Context, documentID string, version int) error {
	return nil
}
