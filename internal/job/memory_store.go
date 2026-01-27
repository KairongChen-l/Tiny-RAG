package job

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrJobExists is returned when trying to create a job that already exists.
	ErrJobExists = errors.New("job already exists")
	// ErrJobNotFound is returned when a job is not found.
	ErrJobNotFound = errors.New("job not found")
)

// ListFilter holds filters for listing jobs.
type ListFilter struct {
	Status Status // Filter by status
	Type   Type   // Filter by type
	Limit  int    // Maximum number of jobs to return
	Offset int    // Number of jobs to skip
}

// MemoryStore is an in-memory implementation of Store for testing.
type MemoryStore struct {
	mu    sync.RWMutex
	jobs  map[string]*Job
	index []string // Maintain insertion order
}

// NewMemoryStore creates a new in-memory job store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		jobs:  make(map[string]*Job),
		index: make([]string, 0),
	}
}

// Create stores a new job.
func (m *MemoryStore) Create(ctx context.Context, j *Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[j.ID]; exists {
		return ErrJobExists
	}

	j.CreatedAt = time.Now()
	j.UpdatedAt = time.Now()
	m.jobs[j.ID] = j
	m.index = append(m.index, j.ID)

	return nil
}

// Get retrieves a job by ID.
func (m *MemoryStore) Get(ctx context.Context, id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return nil, ErrJobNotFound
	}

	// Return a copy to avoid race conditions
	jobCopy := *job
	return &jobCopy, nil
}

// Update updates an existing job.
func (m *MemoryStore) Update(ctx context.Context, j *Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[j.ID]; !exists {
		return ErrJobNotFound
	}

	j.UpdatedAt = time.Now()
	m.jobs[j.ID] = j

	return nil
}

// List returns all jobs matching the filter.
func (m *MemoryStore) List(ctx context.Context, filter ListFilter) ([]*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Job
	for _, id := range m.index {
		job := m.jobs[id]

		// Apply filters
		if filter.Status != "" && job.Status != filter.Status {
			continue
		}
		if filter.Type != "" && job.Type != filter.Type {
			continue
		}

		// Return a copy
		jobCopy := *job
		results = append(results, &jobCopy)
	}

	// Apply pagination
	if filter.Limit > 0 {
		start := filter.Offset
		if start > len(results) {
			start = len(results)
		}
		end := start + filter.Limit
		if end > len(results) {
			end = len(results)
		}
		if start < end {
			results = results[start:end]
		} else {
			results = []*Job{}
		}
	}

	return results, nil
}

// Delete removes a job.
func (m *MemoryStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[id]; !exists {
		return ErrJobNotFound
	}

	delete(m.jobs, id)
	// Remove from index
	for i, idxID := range m.index {
		if idxID == id {
			m.index = append(m.index[:i], m.index[i+1:]...)
			break
		}
	}

	return nil
}


