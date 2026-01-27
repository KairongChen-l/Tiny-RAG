package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
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
	store := &JobStore{db: db}
	// Initialize schema
	if err := store.initSchema(); err != nil {
		// Log error but don't fail - schema might already exist
		fmt.Printf("Warning: failed to initialize job schema: %v\n", err)
	}
	return store
}

// initSchema creates the jobs table if it doesn't exist.
func (s *JobStore) initSchema() error {
	// Create table with new columns (using ALTER TABLE for existing databases)
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			type TEXT NOT NULL,
			status TEXT NOT NULL,
			progress INTEGER DEFAULT 0,
			current_stage TEXT,
			stage_message TEXT,
			progress_history TEXT,
			error TEXT,
			result TEXT,
			payload BLOB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
		CREATE INDEX IF NOT EXISTS idx_jobs_type_status ON jobs(type, status);
	`)
	if err != nil {
		return err
	}

	// Add new columns to existing table if they don't exist
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we check first
	_, _ = s.db.Exec(`ALTER TABLE jobs ADD COLUMN current_stage TEXT`)
	_, _ = s.db.Exec(`ALTER TABLE jobs ADD COLUMN stage_message TEXT`)
	_, _ = s.db.Exec(`ALTER TABLE jobs ADD COLUMN progress_history TEXT`)

	return nil
}

// Create creates a new job record.
func (s *JobStore) Create(ctx context.Context, j *job.Job) error {
	historyJSON, _ := json.Marshal(j.ProgressHistory)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (id, type, status, progress, current_stage, stage_message, progress_history, error, result, payload, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, j.ID, j.Type, j.Status, j.Progress, j.CurrentStage, j.StageMessage, string(historyJSON), j.Error, j.Result, j.Payload, j.CreatedAt, j.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

// Update updates a job record.
func (s *JobStore) Update(ctx context.Context, j *job.Job) error {
	j.UpdatedAt = time.Now()
	historyJSON, _ := json.Marshal(j.ProgressHistory)

	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs SET status = ?, progress = ?, current_stage = ?, stage_message = ?, progress_history = ?, error = ?, result = ?, updated_at = ?
		WHERE id = ?
	`, j.Status, j.Progress, j.CurrentStage, j.StageMessage, string(historyJSON), j.Error, j.Result, j.UpdatedAt, j.ID)

	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}
	return nil
}

// Get retrieves a job by ID.
func (s *JobStore) Get(ctx context.Context, id string) (*job.Job, error) {
	var j job.Job
	var jobType, status string
	var currentStage, stageMessage, historyJSON sql.NullString

	err := s.db.QueryRowContext(ctx, `
		SELECT id, type, status, progress, current_stage, stage_message, progress_history, error, result, payload, created_at, updated_at
		FROM jobs WHERE id = ?
	`, id).Scan(
		&j.ID, &jobType, &status, &j.Progress, &currentStage, &stageMessage, &historyJSON, &j.Error, &j.Result, &j.Payload, &j.CreatedAt, &j.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	j.Type = job.Type(jobType)
	j.Status = job.Status(status)

	if currentStage.Valid {
		j.CurrentStage = currentStage.String
	}
	if stageMessage.Valid {
		j.StageMessage = stageMessage.String
	}
	if historyJSON.Valid && historyJSON.String != "" {
		json.Unmarshal([]byte(historyJSON.String), &j.ProgressHistory)
	}

	return &j, nil
}

// List lists jobs with optional filtering.
func (s *JobStore) List(ctx context.Context, filter job.StoreFilter) ([]*job.Job, error) {
	query := "SELECT id, type, status, progress, current_stage, stage_message, progress_history, error, result, payload, created_at, updated_at FROM jobs WHERE 1=1"
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
		var currentStage, stageMessage, historyJSON sql.NullString

		if err := rows.Scan(
			&j.ID, &jobType, &status, &j.Progress, &currentStage, &stageMessage, &historyJSON, &j.Error, &j.Result, &j.Payload, &j.CreatedAt, &j.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		j.Type = job.Type(jobType)
		j.Status = job.Status(status)

		if currentStage.Valid {
			j.CurrentStage = currentStage.String
		}
		if stageMessage.Valid {
			j.StageMessage = stageMessage.String
		}
		if historyJSON.Valid && historyJSON.String != "" {
			json.Unmarshal([]byte(historyJSON.String), &j.ProgressHistory)
		}

		jobs = append(jobs, &j)
	}

	return jobs, rows.Err()
}
