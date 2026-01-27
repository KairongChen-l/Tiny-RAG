package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/api/handler"
	"github.com/krc/rag/pkg/ratelimit"
)

func TestRouter_WithoutRateLimit(t *testing.T) {
	// Test that router works without rate limiter (for testing scenarios)
	logger := zap.NewNop()
	mockHandler := &handler.Handler{}

	router := NewRouter(RouterConfig{
		Handler: mockHandler,
		Logger:  logger,
		Limiter: nil, // No rate limiter - should work fine
	})

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Should not panic or fail
	router.ServeHTTP(w, req)

	// Router should be created successfully
	if router == nil {
		t.Error("router should not be nil")
	}
}

func TestRouter_WithRateLimit(t *testing.T) {
	// Test that router works with rate limiter
	logger := zap.NewNop()
	mockHandler := &handler.Handler{}

	limiter := ratelimit.NewLimiter(ratelimit.Config{
		Rate:   10,
		Burst:  10,
		Window: 1,
	})

	router := NewRouter(RouterConfig{
		Handler: mockHandler,
		Logger:  logger,
		Limiter: limiter,
	})

	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	w := httptest.NewRecorder()

	// Should not panic
	router.ServeHTTP(w, req)

	if router == nil {
		t.Error("router should not be nil")
	}
}

func TestRouter_RateLimitDisabledByDefault(t *testing.T) {
	// Verify that rate limiting is disabled when limiter is nil
	logger := zap.NewNop()
	mockHandler := &handler.Handler{}

	router := NewRouter(RouterConfig{
		Handler: mockHandler,
		Logger:  logger,
		Limiter: nil, // Explicitly nil - rate limiting should be disabled
	})

	// Make multiple requests - should all succeed (no rate limiting)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// All requests should succeed (no 429 errors)
		if w.Code == http.StatusTooManyRequests {
			t.Errorf("request %d should not be rate limited when limiter is nil", i+1)
		}
	}
}
