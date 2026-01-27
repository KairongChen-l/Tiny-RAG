package ratelimit

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrRateLimitExceeded is returned when the rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// Limiter implements a token bucket rate limiter.
type Limiter struct {
	mu          sync.Mutex
	tokens      float64 // Current number of tokens
	capacity    float64 // Maximum number of tokens
	refillRate  float64 // Tokens added per second
	lastRefill  time.Time
}

// Config holds rate limiter configuration.
type Config struct {
	Rate    int           // Requests per duration
	Burst   int           // Maximum burst size (default: same as Rate)
	Window  time.Duration // Time window (default: 1 second)
}

// NewLimiter creates a new rate limiter.
func NewLimiter(cfg Config) *Limiter {
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.Rate
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Second
	}

	refillRate := float64(cfg.Rate) / cfg.Window.Seconds()

	return &Limiter{
		tokens:     float64(cfg.Burst),
		capacity:   float64(cfg.Burst),
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (l *Limiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN checks if N requests are allowed.
func (l *Limiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()

	// Refill tokens
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.capacity {
		l.tokens = l.capacity
	}
	l.lastRefill = now

	// Check if we have enough tokens
	if l.tokens >= float64(n) {
		l.tokens -= float64(n)
		return true
	}

	return false
}

// Wait waits until a request is allowed.
func (l *Limiter) Wait(ctx context.Context) error {
	return l.WaitN(ctx, 1)
}

// WaitN waits until N requests are allowed.
func (l *Limiter) WaitN(ctx context.Context, n int) error {
	for {
		if l.AllowN(n) {
			return nil
		}

		// Calculate wait time
		l.mu.Lock()
		needed := float64(n) - l.tokens
		waitTime := time.Duration(needed/l.refillRate) * time.Second
		l.mu.Unlock()

		if waitTime < 0 {
			waitTime = 0
		}
		if waitTime > time.Second {
			waitTime = time.Second
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue loop
		}
	}
}

