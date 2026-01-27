package job

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Handler processes a job.
type Handler func(ctx context.Context, job *Job) error

// Queue manages async job processing.
type Queue struct {
	jobs       chan *Job
	workers    int
	handler    Handler
	store      Store
	logger     *zap.Logger
	wg         sync.WaitGroup
	cancelFunc context.CancelFunc
}

// QueueConfig holds queue configuration.
type QueueConfig struct {
	Workers   int
	QueueSize int
}

// NewQueue creates a new job queue.
func NewQueue(cfg QueueConfig, store Store, handler Handler, logger *zap.Logger) *Queue {
	return &Queue{
		jobs:    make(chan *Job, cfg.QueueSize),
		workers: cfg.Workers,
		handler: handler,
		store:   store,
		logger:  logger,
	}
}

// Start starts the worker pool.
func (q *Queue) Start(ctx context.Context) {
	ctx, q.cancelFunc = context.WithCancel(ctx)

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx, i)
	}

	q.logger.Info("job queue started", zap.Int("workers", q.workers))
}

// Stop stops the worker pool gracefully.
func (q *Queue) Stop() {
	if q.cancelFunc != nil {
		q.cancelFunc()
	}
	close(q.jobs)
	q.wg.Wait()
	q.logger.Info("job queue stopped")
}

// Submit submits a job to the queue.
func (q *Queue) Submit(ctx context.Context, job *Job) error {
	// Persist job first
	if q.store != nil {
		if err := q.store.Create(ctx, job); err != nil {
			return fmt.Errorf("failed to create job: %w", err)
		}
	}

	// Submit to queue (non-blocking with timeout)
	select {
	case q.jobs <- job:
		q.logger.Debug("job submitted", zap.String("job_id", job.ID))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("job queue is full")
	}
}

// worker processes jobs from the queue.
func (q *Queue) worker(ctx context.Context, id int) {
	defer q.wg.Done()

	q.logger.Debug("worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-ctx.Done():
			q.logger.Debug("worker stopping", zap.Int("worker_id", id))
			return
		case job, ok := <-q.jobs:
			if !ok {
				return
			}
			q.processJob(ctx, job)
		}
	}
}

// processJob processes a single job.
func (q *Queue) processJob(ctx context.Context, job *Job) {
	logger := q.logger.With(zap.String("job_id", job.ID), zap.String("job_type", string(job.Type)))

	// Set up progress callback to persist updates
	originalCallback := job.ProgressCallback
	job.ProgressCallback = func(j *Job) {
		// Call original callback if set
		if originalCallback != nil {
			originalCallback(j)
		}

		// Persist progress update (with error handling to not block processing)
		if q.store != nil {
			if err := q.store.Update(ctx, j); err != nil {
				logger.Warn("failed to update job progress", zap.Error(err), zap.Int("progress", j.Progress))
			}
		}
	}

	// Update status to processing
	job.SetProcessing()
	if q.store != nil {
		if err := q.store.Update(ctx, job); err != nil {
			logger.Error("failed to update job status", zap.Error(err))
		}
	}

	logger.Info("processing job")

	// Execute handler
	if err := q.handler(ctx, job); err != nil {
		logger.Error("job failed", zap.Error(err))
		job.SetFailed(err)
	} else {
		logger.Info("job completed")
		job.SetCompleted("")
	}

	// Persist final status
	if q.store != nil {
		if err := q.store.Update(ctx, job); err != nil {
			logger.Error("failed to update job status", zap.Error(err))
		}
	}

	// Restore original callback
	job.ProgressCallback = originalCallback
}
