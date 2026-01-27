package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/krc/rag/internal/job"
)

func TestGetJob_WithProgressStages(t *testing.T) {
	// Create a mock job store
	mockStore := &mockJobStore{
		jobs: make(map[string]*job.Job),
	}

	// Create a job with progress stages
	j := job.NewJob("test-job", job.TypeDocumentIngest, []byte("{}"))
	j.SetProgressWithStage(10, "parsing", "Parsing document")
	j.SetProgressWithStage(30, "chunking", "Chunking document")
	j.SetProgressWithStage(60, "embedding", "Generating embeddings")
	mockStore.jobs["test-job"] = j

	// Create handler
	h := &Handler{
		jobStore: mockStore,
	}

	// Create chi router and set up route
	r := chi.NewRouter()
	r.Get("/api/v1/jobs/{id}", h.GetJob)
	
	// Create request
	req := httptest.NewRequest("GET", "/api/v1/jobs/test-job", nil)
	w := httptest.NewRecorder()
	
	// Serve request through router
	r.ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
		t.Logf("response body: %s", w.Body.String())
		// If it's a 400 or 404, the job ID might not be extracted correctly
		// Let's check if the issue is with chi router setup
		return
	}

	var response JobResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify response fields
	if response.ID != "test-job" {
		t.Errorf("expected job ID 'test-job', got '%s'", response.ID)
	}
	if response.Progress != 60 {
		t.Errorf("expected progress 60, got %d", response.Progress)
	}
	if response.CurrentStage != "embedding" {
		t.Errorf("expected stage 'embedding', got '%s'", response.CurrentStage)
	}
	if response.StageMessage != "Generating embeddings" {
		t.Errorf("expected message 'Generating embeddings', got '%s'", response.StageMessage)
	}

	// Check progress history
	if len(response.ProgressHistory) != 3 {
		t.Errorf("expected 3 progress entries, got %d", len(response.ProgressHistory))
	}

	// Check last entry
	last := response.ProgressHistory[len(response.ProgressHistory)-1]
	if last.Progress != 60 {
		t.Errorf("expected last progress 60, got %d", last.Progress)
	}
	if last.Stage != "embedding" {
		t.Errorf("expected last stage 'embedding', got '%s'", last.Stage)
	}
}

type mockJobStore struct {
	jobs map[string]*job.Job
}

func (m *mockJobStore) Create(ctx context.Context, j *job.Job) error {
	m.jobs[j.ID] = j
	return nil
}

func (m *mockJobStore) Update(ctx context.Context, j *job.Job) error {
	m.jobs[j.ID] = j
	return nil
}

func (m *mockJobStore) Get(ctx context.Context, id string) (*job.Job, error) {
	j, exists := m.jobs[id]
	if !exists {
		return nil, &jobNotFoundError{id: id}
	}
	// Return a copy
	jCopy := *j
	return &jCopy, nil
}

func (m *mockJobStore) List(ctx context.Context, filter job.StoreFilter) ([]*job.Job, error) {
	var jobs []*job.Job
	for _, j := range m.jobs {
		if filter.Type != "" && j.Type != filter.Type {
			continue
		}
		if filter.Status != "" && j.Status != filter.Status {
			continue
		}
		jCopy := *j
		jobs = append(jobs, &jCopy)
	}
	return jobs, nil
}

type jobNotFoundError struct {
	id string
}

func (e *jobNotFoundError) Error() string {
	return "job not found: " + e.id
}
