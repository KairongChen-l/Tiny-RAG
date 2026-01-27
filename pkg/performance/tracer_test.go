package performance

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestTracer_Stages(t *testing.T) {
	logger := zap.NewNop()
	tracer := NewTracer(logger)

	// Track multiple stages
	stage1 := tracer.StartStage("stage1")
	time.Sleep(10 * time.Millisecond)
	stage1()

	stage2 := tracer.StartStage("stage2")
	time.Sleep(20 * time.Millisecond)
	stage2()

	// Check durations
	if tracer.GetStageDuration("stage1") < 10*time.Millisecond {
		t.Error("stage1 duration too short")
	}
	if tracer.GetStageDuration("stage2") < 20*time.Millisecond {
		t.Error("stage2 duration too short")
	}

	// Log trace
	tracer.Log(context.Background(), "test_operation")
}

func TestTracer_GetDuration(t *testing.T) {
	logger := zap.NewNop()
	tracer := NewTracer(logger)

	time.Sleep(50 * time.Millisecond)
	duration := tracer.GetDuration()

	if duration < 50*time.Millisecond {
		t.Errorf("expected duration >= 50ms, got %v", duration)
	}
}

