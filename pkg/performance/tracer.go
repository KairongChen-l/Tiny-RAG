package performance

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Tracer tracks performance of operations.
type Tracer struct {
	logger *zap.Logger
	start  time.Time
	stages map[string]time.Duration
}

// NewTracer creates a new performance tracer.
func NewTracer(logger *zap.Logger) *Tracer {
	return &Tracer{
		logger: logger,
		start:  time.Now(),
		stages: make(map[string]time.Duration),
	}
}

// StartStage starts timing a stage.
func (t *Tracer) StartStage(name string) func() {
	stageStart := time.Now()
	return func() {
		t.stages[name] = time.Since(stageStart)
	}
}

// Log logs the performance trace.
func (t *Tracer) Log(ctx context.Context, operation string) {
	total := time.Since(t.start)
	
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.Duration("total_duration", total),
	}

	for stage, duration := range t.stages {
		fields = append(fields, zap.Duration("stage_"+stage, duration))
	}

	t.logger.Info("performance trace", fields...)
}

// GetDuration returns the total duration.
func (t *Tracer) GetDuration() time.Duration {
	return time.Since(t.start)
}

// GetStageDuration returns the duration of a specific stage.
func (t *Tracer) GetStageDuration(stage string) time.Duration {
	return t.stages[stage]
}

