package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetryWithExponentialBackoff_Success(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	err := RetryWithExponentialBackoff(context.Background(), fn, RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithExponentialBackoff_MaxAttempts(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("persistent error")
	}

	err := RetryWithExponentialBackoff(context.Background(), fn, RetryConfig{
		MaxAttempts: 3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	if err == nil {
		t.Error("expected error after max attempts")
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryWithExponentialBackoff_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	attempts := 0
	fn := func() error {
		attempts++
		return errors.New("error")
	}

	err := RetryWithExponentialBackoff(ctx, fn, RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	if err == nil {
		t.Error("expected error due to context cancellation")
	}
	if attempts > 1 {
		t.Errorf("expected at most 1 attempt, got %d", attempts)
	}
}

func TestRetryWithExponentialBackoff_NonRetryableError(t *testing.T) {
	attempts := 0
	fn := func() error {
		attempts++
		return &NonRetryableError{Err: errors.New("permanent error")}
	}

	err := RetryWithExponentialBackoff(context.Background(), fn, RetryConfig{
		MaxAttempts: 5,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
	})

	if err == nil {
		t.Error("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

