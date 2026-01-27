package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/krc/rag/internal/job"
	_ "github.com/mattn/go-sqlite3"
)

func TestJobStore_ProgressStages(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	// Create a job with progress stages
	j := job.NewJob("test-id", job.TypeDocumentIngest, []byte("{}"))
	j.SetProgressWithStage(10, "parsing", "Parsing document")
	j.SetProgressWithStage(30, "chunking", "Chunking document")
	j.SetProgressWithStage(60, "embedding", "Generating embeddings")

	// Create job
	ctx := context.Background()
	if err := store.Create(ctx, j); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Retrieve job
	retrieved, err := store.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	// Check progress and stage
	if retrieved.Progress != 60 {
		t.Errorf("expected progress 60, got %d", retrieved.Progress)
	}
	if retrieved.CurrentStage != "embedding" {
		t.Errorf("expected stage 'embedding', got '%s'", retrieved.CurrentStage)
	}
	if retrieved.StageMessage != "Generating embeddings" {
		t.Errorf("expected message 'Generating embeddings', got '%s'", retrieved.StageMessage)
	}

	// Check progress history
	if len(retrieved.ProgressHistory) != 3 {
		t.Errorf("expected 3 progress entries, got %d", len(retrieved.ProgressHistory))
	}
}

func TestJobStore_UpdateProgress(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	// Create a job
	j := job.NewJob("test-id", job.TypeDocumentIngest, []byte("{}"))
	ctx := context.Background()
	if err := store.Create(ctx, j); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Update progress with stage
	j.SetProgressWithStage(50, "processing", "Processing document")
	if err := store.Update(ctx, j); err != nil {
		t.Fatalf("failed to update job: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	if retrieved.Progress != 50 {
		t.Errorf("expected progress 50, got %d", retrieved.Progress)
	}
	if retrieved.CurrentStage != "processing" {
		t.Errorf("expected stage 'processing', got '%s'", retrieved.CurrentStage)
	}
}

func TestJobStore_BackwardCompatibility(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	store := NewJobStore(db)

	// Create a job without stages (backward compatibility)
	j := job.NewJob("test-id", job.TypeDocumentIngest, []byte("{}"))
	j.SetProgress(25) // Use old method

	ctx := context.Background()
	if err := store.Create(ctx, j); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	// Retrieve and verify
	retrieved, err := store.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	if retrieved.Progress != 25 {
		t.Errorf("expected progress 25, got %d", retrieved.Progress)
	}
	// Stage fields should be empty for old jobs
	if retrieved.CurrentStage != "" {
		t.Errorf("expected empty stage, got '%s'", retrieved.CurrentStage)
	}
}

