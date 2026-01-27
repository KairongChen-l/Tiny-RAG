package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	// Should allow calls in closed state
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected closed state, got %v", cb.State())
	}
}

func TestCircuitBreaker_OpenState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          100 * time.Millisecond,
	})

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected open state, got %v", cb.State())
	}

	// Should reject calls immediately in open state
	err := cb.Execute(func() error {
		return nil
	})

	if err == nil {
		t.Error("expected error in open state")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open circuit
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	// Wait for timeout to enter half-open
	time.Sleep(60 * time.Millisecond)

	// Execute a call to trigger state update (updateState is called in Execute)
	// This should transition from open to half-open
	_ = cb.Execute(func() error {
		return nil
	})

	if cb.State() != StateHalfOpen {
		t.Errorf("expected half-open state, got %v", cb.State())
	}

	// Success should close circuit
	cb.Execute(func() error {
		return nil
	})
	cb.Execute(func() error {
		return nil
	})

	if cb.State() != StateClosed {
		t.Errorf("expected closed state after success, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          50 * time.Millisecond,
	})

	// Open circuit
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return errors.New("error")
		})
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Failure in half-open should open again
	err := cb.Execute(func() error {
		return errors.New("error")
	})

	if err == nil {
		t.Error("expected error")
	}
	if cb.State() != StateOpen {
		t.Errorf("expected open state after failure in half-open, got %v", cb.State())
	}
}

