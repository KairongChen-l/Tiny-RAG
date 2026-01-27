// Package job provides async job processing functionality.
package job

import (
	"context"
	"time"
)

// Status represents job status.
type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// Type represents job type.
type Type string

const (
	TypeDocumentIngest Type = "document_ingest"
)

// ProgressEntry represents a single progress update entry.
type ProgressEntry struct {
	Progress     int       // Progress percentage (0-100)
	Stage        string    // Current stage identifier (e.g., "parsing", "chunking", "embedding")
	StageMessage string    // Human-readable stage message
	Timestamp    time.Time // When this progress was recorded
}

// Job represents an async job.
type Job struct {
	ID               string          // Unique identifier
	Type             Type            // Job type
	Status           Status          // Current status
	Progress         int             // Progress percentage (0-100)
	CurrentStage     string          // Current stage identifier
	StageMessage     string          // Current stage message
	ProgressHistory  []ProgressEntry // History of progress updates (optional, for detailed tracking)
	Error            string          // Error message if failed
	Result           string          // Result data (JSON)
	CreatedAt        time.Time       // Creation timestamp
	UpdatedAt        time.Time       // Last update timestamp
	Payload          []byte          // Job payload (JSON)
	ProgressCallback func(*Job)      // Optional callback for progress updates
}

// NewJob creates a new job with pending status.
func NewJob(id string, jobType Type, payload []byte) *Job {
	now := time.Now()
	return &Job{
		ID:        id,
		Type:      jobType,
		Status:    StatusPending,
		Progress:  0,
		CreatedAt: now,
		UpdatedAt: now,
		Payload:   payload,
	}
}

// SetProcessing marks the job as processing.
func (j *Job) SetProcessing() {
	j.Status = StatusProcessing
	j.UpdatedAt = time.Now()
}

// SetProgress updates the job progress.
func (j *Job) SetProgress(progress int) {
	j.Progress = progress
	j.UpdatedAt = time.Now()

	// Call progress callback if set
	if j.ProgressCallback != nil {
		j.ProgressCallback(j)
	}
}

// SetProgressWithStage updates the job progress with stage information.
func (j *Job) SetProgressWithStage(progress int, stage string, message string) {
	j.Progress = progress
	j.CurrentStage = stage
	j.StageMessage = message
	j.UpdatedAt = time.Now()

	// Add to progress history (keep last 20 entries)
	entry := ProgressEntry{
		Progress:     progress,
		Stage:        stage,
		StageMessage: message,
		Timestamp:    time.Now(),
	}
	j.ProgressHistory = append(j.ProgressHistory, entry)
	if len(j.ProgressHistory) > 20 {
		j.ProgressHistory = j.ProgressHistory[len(j.ProgressHistory)-20:]
	}

	// Call progress callback if set
	if j.ProgressCallback != nil {
		j.ProgressCallback(j)
	}
}

// SetCompleted marks the job as completed.
func (j *Job) SetCompleted(result string) {
	j.Status = StatusCompleted
	j.Progress = 100
	j.Result = result
	j.UpdatedAt = time.Now()
}

// SetFailed marks the job as failed.
func (j *Job) SetFailed(err error) {
	j.Status = StatusFailed
	j.Error = err.Error()
	j.UpdatedAt = time.Now()
}

// Store defines the interface for job persistence.
type Store interface {
	// Create creates a new job record.
	Create(ctx context.Context, job *Job) error

	// Update updates a job record.
	Update(ctx context.Context, job *Job) error

	// Get retrieves a job by ID.
	Get(ctx context.Context, id string) (*Job, error)

	// List lists jobs with optional filtering.
	List(ctx context.Context, filter StoreFilter) ([]*Job, error)
}

// StoreFilter holds job list filter options.
type StoreFilter struct {
	Type   Type
	Status Status
	Limit  int
	Offset int
}
