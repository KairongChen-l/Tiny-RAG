// Package repository provides data access abstractions for business logic.
package repository

import (
	"context"

	"github.com/krc/rag/internal/job"
)

// JobRepository provides job data access operations.
type JobRepository interface {
	// Create creates a new job.
	Create(ctx context.Context, j *job.Job) error

	// Get retrieves a job by ID.
	Get(ctx context.Context, id string) (*job.Job, error)

	// List lists jobs with filtering.
	List(ctx context.Context, status job.Status, limit int) ([]*job.Job, error)

	// Update updates an existing job.
	Update(ctx context.Context, j *job.Job) error

	// Delete deletes a job by ID.
	Delete(ctx context.Context, id string) error
}

// StoreJobRepository implements JobRepository using job.Store.
type StoreJobRepository struct {
	store job.Store
}

// NewStoreJobRepository creates a new job repository.
func NewStoreJobRepository(store job.Store) *StoreJobRepository {
	return &StoreJobRepository{
		store: store,
	}
}

// Create creates a new job.
func (r *StoreJobRepository) Create(ctx context.Context, j *job.Job) error {
	return r.store.Create(ctx, j)
}

// Get retrieves a job by ID.
func (r *StoreJobRepository) Get(ctx context.Context, id string) (*job.Job, error) {
	return r.store.Get(ctx, id)
}

// List lists jobs with filtering.
func (r *StoreJobRepository) List(ctx context.Context, status job.Status, limit int) ([]*job.Job, error) {
	filter := job.StoreFilter{
		Status: status,
		Limit:  limit,
	}
	return r.store.List(ctx, filter)
}

// Update updates an existing job.
func (r *StoreJobRepository) Update(ctx context.Context, j *job.Job) error {
	return r.store.Update(ctx, j)
}

// Delete deletes a job by ID.
// Note: job.Store interface doesn't have Delete method, so we mark the job as failed instead.
func (r *StoreJobRepository) Delete(ctx context.Context, id string) error {
	// Get the job first
	j, err := r.store.Get(ctx, id)
	if err != nil {
		return err
	}
	// Mark as failed (soft delete)
	j.Status = job.StatusFailed
	j.Error = "deleted"
	return r.store.Update(ctx, j)
}

