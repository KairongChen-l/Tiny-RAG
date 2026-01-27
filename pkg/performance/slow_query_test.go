package performance

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestSlowQueryLogger_Log(t *testing.T) {
	logger := zap.NewNop()
	threshold := 100 * time.Millisecond
	sql := NewSlowQueryLogger(logger, threshold)

	// Fast query should not log
	sql.Log(context.Background(), "test", 50*time.Millisecond, "SELECT * FROM test")

	// Slow query should log
	sql.Log(context.Background(), "test", 200*time.Millisecond, "SELECT * FROM test")
}

func TestSlowQueryLogger_Track(t *testing.T) {
	logger := zap.NewNop()
	threshold := 100 * time.Millisecond
	sql := NewSlowQueryLogger(logger, threshold)

	// Fast operation
	err := sql.Track(context.Background(), "test", "SELECT * FROM test", func() error {
		time.Sleep(50 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Slow operation
	err = sql.Track(context.Background(), "test", "SELECT * FROM test", func() error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

