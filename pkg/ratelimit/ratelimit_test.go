package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	limiter := NewLimiter(Config{
		Rate:   10,
		Burst:  10,
		Window: time.Second,
	})

	// Should allow first 10 requests
	for i := 0; i < 10; i++ {
		if !limiter.Allow() {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 11th request should be denied
	if limiter.Allow() {
		t.Error("11th request should be denied")
	}
}

func TestLimiter_Refill(t *testing.T) {
	limiter := NewLimiter(Config{
		Rate:   10,
		Burst:  10,
		Window: time.Second,
	})

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		limiter.Allow()
	}

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should allow at least 1 more request
	if !limiter.Allow() {
		t.Error("request should be allowed after refill")
	}
}

func TestLimiter_Wait(t *testing.T) {
	limiter := NewLimiter(Config{
		Rate:   10,
		Burst:  10,
		Window: time.Second,
	})

	ctx := context.Background()

	// Exhaust tokens
	for i := 0; i < 10; i++ {
		limiter.Allow()
	}

	// Wait should eventually succeed
	start := time.Now()
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	elapsed := time.Since(start)

	// Should have waited at least some time
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected to wait, but only waited %v", elapsed)
	}
}

