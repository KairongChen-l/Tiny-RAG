package job

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// RedisQueue implements Queue using Redis (asynq).
type RedisQueue struct {
	client  *asynq.Client
	server  *asynq.Server
	mux     *asynq.ServeMux
	handler Handler
	store   Store
	logger  *zap.Logger
	cfg     RedisQueueConfig
}

// RedisQueueConfig holds configuration for Redis queue.
type RedisQueueConfig struct {
	RedisAddr     string        // Redis address (default: "localhost:6379")
	RedisPassword string        // Redis password (optional)
	RedisDB       int           // Redis database (default: 0)
	Workers       int           // Number of workers (default: 3)
	Concurrency   int           // Concurrency per worker (default: 10)
	RetryDelay    time.Duration // Retry delay (default: 5 minutes)
	MaxRetries    int           // Maximum retries (default: 3)
}

// NewRedisQueue creates a new Redis-based job queue.
func NewRedisQueue(cfg RedisQueueConfig, store Store, handler Handler, logger *zap.Logger) (*RedisQueue, error) {
	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "localhost:6379"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 5 * time.Minute
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}

	// Create Redis client
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	client := asynq.NewClient(redisOpt)

	// Create server
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.Concurrency,
		Queues: map[string]int{
			"critical": 6,
			"default":  3,
			"low":      1,
		},
		RetryDelayFunc: func(n int, e error, task *asynq.Task) time.Duration {
			return cfg.RetryDelay
		},
	})

	mux := asynq.NewServeMux()

	queue := &RedisQueue{
		client:  client,
		server:  server,
		mux:     mux,
		handler: handler,
		store:   store,
		logger:  logger,
		cfg:     cfg,
	}

	// Register handler
	mux.HandleFunc("document_ingest", queue.handleTask)

	return queue, nil
}

// Start starts the Redis queue server.
func (q *RedisQueue) Start(ctx context.Context) error {
	// Start server in goroutine
	go func() {
		if err := q.server.Run(q.mux); err != nil {
			q.logger.Error("redis queue server error", zap.Error(err))
		}
	}()

	q.logger.Info("redis queue started", zap.String("addr", q.cfg.RedisAddr))
	return nil
}

// Stop stops the Redis queue server.
func (q *RedisQueue) Stop() {
	q.server.Shutdown()
	q.client.Close()
	q.logger.Info("redis queue stopped")
}

// Submit submits a job to the queue.
func (q *RedisQueue) Submit(ctx context.Context, job *Job) error {
	// Persist job first
	if q.store != nil {
		if err := q.store.Create(ctx, job); err != nil {
			return fmt.Errorf("failed to create job: %w", err)
		}
	}

	// Determine queue priority (default to "default")
	queueName := "default"
	// Priority can be added to Job struct in the future
	// For now, use default queue

	// Create asynq task
	task := asynq.NewTask(string(job.Type), job.Payload)

	// Submit task
	opts := []asynq.Option{
		asynq.Queue(queueName),
		asynq.MaxRetry(q.cfg.MaxRetries),
		asynq.TaskID(job.ID), // Use job ID as task ID
	}

	info, err := q.client.Enqueue(task, opts...)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	q.logger.Debug("job submitted to redis queue",
		zap.String("job_id", job.ID),
		zap.String("queue", queueName),
		zap.String("task_id", info.ID),
	)

	return nil
}

// handleTask handles an asynq task.
func (q *RedisQueue) handleTask(ctx context.Context, t *asynq.Task) error {
	// Try to find job from store by matching payload
	var job *Job
	if q.store != nil {
		// List recent pending jobs and find matching one
		recentJobs, _ := q.store.List(ctx, StoreFilter{
			Type:   Type(t.Type()),
			Status: StatusPending,
			Limit:  100,
		})
		// Find job with matching payload (simple byte comparison)
		for _, j := range recentJobs {
			if len(j.Payload) == len(t.Payload()) {
				// Simple match - in production, use hash comparison
				match := true
				for i := 0; i < len(j.Payload) && i < 100; i++ { // Compare first 100 bytes
					if j.Payload[i] != t.Payload()[i] {
						match = false
						break
					}
				}
				if match {
					job = j
					break
				}
			}
		}
	}
	
	// If not found, create new job from task
	if job == nil {
		job = &Job{
			ID:      "", // Will be generated if needed
			Type:    Type(t.Type()),
			Payload: t.Payload(),
			Status:  StatusProcessing,
		}
	}

	logger := q.logger.With(zap.String("job_id", job.ID), zap.String("job_type", string(job.Type)))

	// Update status to processing
	job.SetProcessing()
	if q.store != nil {
		if err := q.store.Update(ctx, job); err != nil {
			logger.Error("failed to update job status", zap.Error(err))
		}
	}

	logger.Info("processing job from redis queue")

	// Execute handler
	if err := q.handler(ctx, job); err != nil {
		logger.Error("job failed", zap.Error(err))
		job.SetFailed(err)
		if q.store != nil {
			q.store.Update(ctx, job)
		}
		return err // Return error to trigger retry
	}

	logger.Info("job completed")
	job.SetCompleted("")
	if q.store != nil {
		if err := q.store.Update(ctx, job); err != nil {
			logger.Error("failed to update job status", zap.Error(err))
		}
	}

	return nil
}

// getMaxRetries returns the maximum number of retries.
func (q *RedisQueue) getMaxRetries() int {
	return 3 // Default
}

