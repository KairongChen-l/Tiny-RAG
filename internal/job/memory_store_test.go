package job

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_Create(t *testing.T) {
	store := NewMemoryStore()
	job := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	err := store.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to create duplicate
	err = store.Create(context.Background(), job)
	if err != ErrJobExists {
		t.Errorf("expected ErrJobExists, got %v", err)
	}
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore()
	job := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	store.Create(context.Background(), job)

	retrieved, err := store.Get(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != job.ID {
		t.Errorf("expected ID %s, got %s", job.ID, retrieved.ID)
	}

	// Get non-existent job
	_, err = store.Get(context.Background(), "non-existent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore()
	job := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	store.Create(context.Background(), job)

	job.Status = StatusCompleted
	err := store.Update(context.Background(), job)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, _ := store.Get(context.Background(), "test-id")
	if retrieved.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, retrieved.Status)
	}
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore()

	// Create multiple jobs
	for i := 0; i < 5; i++ {
		job := NewJob("job-"+string(rune(i)), TypeDocumentIngest, []byte("{}"))
		store.Create(context.Background(), job)
	}

	// List all
	jobs, err := store.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 5 {
		t.Errorf("expected 5 jobs, got %d", len(jobs))
	}

	// Filter by status
	jobs, _ = store.List(context.Background(), ListFilter{Status: StatusPending})
	if len(jobs) != 5 {
		t.Errorf("expected 5 pending jobs, got %d", len(jobs))
	}

	// Pagination
	jobs, _ = store.List(context.Background(), ListFilter{Limit: 2, Offset: 0})
	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	job := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	store.Create(context.Background(), job)

	err := store.Delete(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to get deleted job
	_, err = store.Get(context.Background(), "test-id")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

