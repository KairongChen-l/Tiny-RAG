package job

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestQueue_ProgressUpdates(t *testing.T) {
	store := &mockStore{
		jobs: make(map[string]*Job),
		mu:   sync.RWMutex{},
	}

	updateCount := 0
	var updateMutex sync.Mutex

	handler := func(ctx context.Context, job *Job) error {
		// Simulate progress updates
		job.SetProgressWithStage(10, "stage1", "Stage 1")
		job.SetProgressWithStage(30, "stage2", "Stage 2")
		job.SetProgressWithStage(60, "stage3", "Stage 3")
		job.SetProgressWithStage(100, "complete", "Complete")
		return nil
	}

	queue := NewQueue(QueueConfig{
		Workers:   1,
		QueueSize: 10,
	}, store, handler, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue.Start(ctx)

	// Create a job with progress callback
	job := NewJob("test-job", TypeDocumentIngest, []byte("{}"))
	job.ProgressCallback = func(j *Job) {
		updateMutex.Lock()
		updateCount++
		updateMutex.Unlock()
		// Update store
		store.Update(context.Background(), j)
	}

	err := queue.Submit(ctx, job)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Wait for job to complete
	time.Sleep(100 * time.Millisecond)

	// Check that progress was updated
	updatedJob, _ := store.Get(context.Background(), job.ID)
	if updatedJob.Progress != 100 {
		t.Errorf("expected progress 100, got %d", updatedJob.Progress)
	}

	// Check that callback was called multiple times
	updateMutex.Lock()
	callCount := updateCount
	updateMutex.Unlock()

	if callCount < 3 {
		t.Errorf("expected at least 3 progress updates, got %d", callCount)
	}

	queue.Stop()
}

func TestQueue_ProgressUpdateFailure(t *testing.T) {
	store := &mockStore{
		jobs: make(map[string]*Job),
		mu:   sync.RWMutex{},
	}

	handler := func(ctx context.Context, job *Job) error {
		job.SetProgressWithStage(50, "processing", "Processing")
		return nil
	}

	queue := NewQueue(QueueConfig{
		Workers:   1,
		QueueSize: 10,
	}, store, handler, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	queue.Start(ctx)

	job := NewJob("test-job", TypeDocumentIngest, []byte("{}"))
	err := queue.Submit(ctx, job)
	if err != nil {
		t.Fatalf("failed to submit job: %v", err)
	}

	// Wait for job to complete
	time.Sleep(100 * time.Millisecond)

	// Job should still complete even if progress update fails
	updatedJob, _ := store.Get(context.Background(), job.ID)
	if updatedJob.Status != StatusCompleted {
		t.Errorf("expected status completed, got %s", updatedJob.Status)
	}

	queue.Stop()
}

type mockStore struct {
	jobs map[string]*Job
	mu   sync.RWMutex
}

func (m *mockStore) Create(ctx context.Context, job *Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *mockStore) Update(ctx context.Context, job *Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.jobs[job.ID]; !exists {
		return errors.New("job not found")
	}
	m.jobs[job.ID] = job
	return nil
}

func (m *mockStore) Get(ctx context.Context, id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, exists := m.jobs[id]
	if !exists {
		return nil, errors.New("job not found")
	}
	// Return a copy
	j := *job
	return &j, nil
}

func (m *mockStore) List(ctx context.Context, filter StoreFilter) ([]*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var jobs []*Job
	for _, job := range m.jobs {
		if filter.Type != "" && job.Type != filter.Type {
			continue
		}
		if filter.Status != "" && job.Status != filter.Status {
			continue
		}
		j := *job
		jobs = append(jobs, &j)
	}
	return jobs, nil
}
