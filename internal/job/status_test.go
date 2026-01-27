package job

import (
	"testing"
	"time"
)

func TestJob_ProgressStages(t *testing.T) {
	j := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	// Test setting progress with stage
	j.SetProgressWithStage(10, "parsing", "Parsing document")
	if j.Progress != 10 {
		t.Errorf("expected progress 10, got %d", j.Progress)
	}
	if j.CurrentStage != "parsing" {
		t.Errorf("expected stage 'parsing', got '%s'", j.CurrentStage)
	}
	if j.StageMessage != "Parsing document" {
		t.Errorf("expected message 'Parsing document', got '%s'", j.StageMessage)
	}

	// Test updating progress
	j.SetProgressWithStage(50, "chunking", "Chunking document")
	if j.Progress != 50 {
		t.Errorf("expected progress 50, got %d", j.Progress)
	}
	if j.CurrentStage != "chunking" {
		t.Errorf("expected stage 'chunking', got '%s'", j.CurrentStage)
	}
}

func TestJob_ProgressHistory(t *testing.T) {
	j := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	// Set multiple progress updates
	j.SetProgressWithStage(10, "parsing", "Parsing document")
	j.SetProgressWithStage(30, "chunking", "Chunking document")
	j.SetProgressWithStage(60, "embedding", "Generating embeddings")

	// Check that progress history is maintained
	if len(j.ProgressHistory) != 3 {
		t.Errorf("expected 3 progress updates, got %d", len(j.ProgressHistory))
	}

	// Check last entry
	last := j.ProgressHistory[len(j.ProgressHistory)-1]
	if last.Progress != 60 {
		t.Errorf("expected last progress 60, got %d", last.Progress)
	}
	if last.Stage != "embedding" {
		t.Errorf("expected last stage 'embedding', got '%s'", last.Stage)
	}
}

func TestJob_UpdateProgress(t *testing.T) {
	j := NewJob("test-id", TypeDocumentIngest, []byte("{}"))

	// Test that UpdatedAt is updated
	oldTime := j.UpdatedAt
	time.Sleep(10 * time.Millisecond)
	j.SetProgress(25)
	if !j.UpdatedAt.After(oldTime) {
		t.Error("expected UpdatedAt to be updated")
	}
}

