package performance

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// SlowQueryLogger logs slow queries for performance analysis.
type SlowQueryLogger struct {
	logger    *zap.Logger
	threshold time.Duration
}

// NewSlowQueryLogger creates a new slow query logger.
func NewSlowQueryLogger(logger *zap.Logger, threshold time.Duration) *SlowQueryLogger {
	return &SlowQueryLogger{
		logger:    logger,
		threshold: threshold,
	}
}

// Log logs a query if it exceeds the threshold.
func (s *SlowQueryLogger) Log(ctx context.Context, operation string, duration time.Duration, query string, args ...interface{}) {
	if duration > s.threshold {
		s.logger.Warn("slow query detected",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
			zap.String("query", query),
			zap.Any("args", args),
		)
	}
}

// LogWithError logs a query with error information.
func (s *SlowQueryLogger) LogWithError(ctx context.Context, operation string, duration time.Duration, err error, query string, args ...interface{}) {
	if duration > s.threshold {
		s.logger.Warn("slow query with error",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
			zap.Error(err),
			zap.String("query", query),
			zap.Any("args", args),
		)
	}
}

// Track tracks a query execution and logs if slow.
func (s *SlowQueryLogger) Track(ctx context.Context, operation, query string, fn func() error, args ...interface{}) error {
	start := time.Now()
	err := fn()
	duration := time.Since(start)

	if err != nil {
		s.LogWithError(ctx, operation, duration, err, query, args...)
	} else {
		s.Log(ctx, operation, duration, query, args...)
	}

	return err
}
