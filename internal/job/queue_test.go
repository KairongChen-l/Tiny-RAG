package job

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
)

// TestQueueErrorHandling tests that job errors are properly handled and persisted
func TestQueueErrorHandling(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockStore := &mockJobStore{
		jobs: make(map[string]*Job),
	}

	// Create a handler that always fails
	failingHandler := func(ctx context.Context, job *Job) error {
		return errors.New("simulated processing error")
	}

	queue := NewQueue(QueueConfig{
		Workers:   1,
		QueueSize: 10,
	}, mockStore, failingHandler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue.Start(ctx)
	defer queue.Stop()

	// Create and submit a job
	job := NewJob("test-job-1", TypeDocumentIngest, []byte(`{"test": "data"}`))
	if err := queue.Submit(ctx, job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify job was persisted with error
	storedJob, err := mockStore.Get(ctx, "test-job-1")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	if storedJob.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, storedJob.Status)
	}

	if storedJob.Error == "" {
		t.Error("expected error message to be set")
	}

	if storedJob.Error != "simulated processing error" {
		t.Errorf("expected error 'simulated processing error', got '%s'", storedJob.Error)
	}
}

// TestQueueProgressUpdates tests that job progress is properly updated during processing
func TestQueueProgressUpdates(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockStore := &mockJobStore{
		jobs: make(map[string]*Job),
	}

	// Create a handler that updates progress
	progressHandler := func(ctx context.Context, job *Job) error {
		job.SetProgress(25)
		time.Sleep(10 * time.Millisecond)
		job.SetProgress(50)
		time.Sleep(10 * time.Millisecond)
		job.SetProgress(75)
		time.Sleep(10 * time.Millisecond)
		job.SetProgress(100)
		return nil
	}

	queue := NewQueue(QueueConfig{
		Workers:   1,
		QueueSize: 10,
	}, mockStore, progressHandler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue.Start(ctx)
	defer queue.Stop()

	// Create and submit a job
	job := NewJob("test-job-2", TypeDocumentIngest, []byte(`{"test": "data"}`))
	if err := queue.Submit(ctx, job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify job was completed
	storedJob, err := mockStore.Get(ctx, "test-job-2")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}

	if storedJob.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, storedJob.Status)
	}

	if storedJob.Progress != 100 {
		t.Errorf("expected progress 100, got %d", storedJob.Progress)
	}
}

// TestQueueContextCancellation tests that jobs respect context cancellation
func TestQueueContextCancellation(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockStore := &mockJobStore{
		jobs: make(map[string]*Job),
	}

	// Create a handler that takes a long time
	longRunningHandler := func(ctx context.Context, job *Job) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	}

	queue := NewQueue(QueueConfig{
		Workers:   1,
		QueueSize: 10,
	}, mockStore, longRunningHandler, logger)

	ctx, cancel := context.WithCancel(context.Background())
	queue.Start(ctx)

	// Create and submit a job
	job := NewJob("test-job-3", TypeDocumentIngest, []byte(`{"test": "data"}`))
	if err := queue.Submit(ctx, job); err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Cancel context immediately
	cancel()

	// Wait a bit for cancellation to propagate
	time.Sleep(50 * time.Millisecond)

	queue.Stop()
}

// mockJobStore is a simple in-memory job store for testing
type mockJobStore struct {
	jobs map[string]*Job
}

func (m *mockJobStore) Create(ctx context.Context, job *Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobStore) Update(ctx context.Context, job *Job) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *mockJobStore) Get(ctx context.Context, id string) (*Job, error) {
	job, ok := m.jobs[id]
	if !ok {
		return nil, errors.New("job not found")
	}
	// Return a copy to avoid race conditions
	jobCopy := *job
	return &jobCopy, nil
}

func (m *mockJobStore) List(ctx context.Context, filter StoreFilter) ([]*Job, error) {
	var jobs []*Job
	for _, job := range m.jobs {
		if filter.Type != "" && job.Type != filter.Type {
			continue
		}
		if filter.Status != "" && job.Status != filter.Status {
			continue
		}
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	return jobs, nil
}

