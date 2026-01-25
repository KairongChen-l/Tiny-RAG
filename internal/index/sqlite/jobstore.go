package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/krc/rag/internal/job"
)

// JobStore implements job.Store using SQLite.
type JobStore struct {
	db *sql.DB
}

// NewJobStore creates a new SQLite job store.
func NewJobStore(db *sql.DB) *JobStore {
	return &JobStore{db: db}
}

// Create creates a new job record.
func (s *JobStore) Create(ctx context.Context, j *job.Job) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (id, type, status, progress, error, result, payload, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, j.ID, j.Type, j.Status, j.Progress, j.Error, j.Result, j.Payload, j.CreatedAt, j.UpdatedAt)
	
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

// Update updates a job record.
func (s *JobStore) Update(ctx context.Context, j *job.Job) error {
	j.UpdatedAt = time.Now()
	
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs SET status = ?, progress = ?, error = ?, result = ?, updated_at = ?
		WHERE id = ?
	`, j.Status, j.Progress, j.Error, j.Result, j.UpdatedAt, j.ID)
	
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}
	return nil
}

// Get retrieves a job by ID.
func (s *JobStore) Get(ctx context.Context, id string) (*job.Job, error) {
	var j job.Job
	var jobType, status string
	
	err := s.db.QueryRowContext(ctx, `
		SELECT id, type, status, progress, error, result, payload, created_at, updated_at
		FROM jobs WHERE id = ?
	`, id).Scan(
		&j.ID, &jobType, &status, &j.Progress, &j.Error, &j.Result, &j.Payload, &j.CreatedAt, &j.UpdatedAt,
	)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	
	j.Type = job.Type(jobType)
	j.Status = job.Status(status)
	
	return &j, nil
}

// List lists jobs with optional filtering.
func (s *JobStore) List(ctx context.Context, filter job.StoreFilter) ([]*job.Job, error) {
	query := "SELECT id, type, status, progress, error, result, payload, created_at, updated_at FROM jobs WHERE 1=1"
	args := []interface{}{}
	
	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, string(filter.Type))
	}
	
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, string(filter.Status))
	}
	
	query += " ORDER BY created_at DESC"
	
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}
	
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()
	
	var jobs []*job.Job
	for rows.Next() {
		var j job.Job
		var jobType, status string
		
		if err := rows.Scan(
			&j.ID, &jobType, &status, &j.Progress, &j.Error, &j.Result, &j.Payload, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		
		j.Type = job.Type(jobType)
		j.Status = job.Status(status)
		jobs = append(jobs, &j)
	}
	
	return jobs, rows.Err()
}

