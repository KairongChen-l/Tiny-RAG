package kimi

import (
	"errors"
	"testing"
)

func TestClient_Generate_WithRetry(t *testing.T) {
	// This test requires a valid API key, so we'll skip it in CI
	// In a real scenario, we would mock the HTTP client
	t.Skip("requires API key and network access")
}

func TestClient_RetryableError(t *testing.T) {
	// Test that network errors are retryable
	err := errors.New("network error")
	if !isRetryableError(err) {
		t.Error("network error should be retryable")
	}
}

func TestClient_NonRetryableError(t *testing.T) {
	// Test that authentication errors are not retryable
	err := errors.New("authentication failed")
	if isRetryableError(err) {
		t.Error("authentication error should not be retryable")
	}
}

func TestClient_RetryableError_RateLimit(t *testing.T) {
	// Test that rate limit errors are retryable
	err := errors.New("rate limit exceeded 429")
	if !isRetryableError(err) {
		t.Error("rate limit error should be retryable")
	}
}

func TestClient_NonRetryableError_InvalidKey(t *testing.T) {
	// Test that invalid API key errors are not retryable
	err := errors.New("invalid api key")
	if isRetryableError(err) {
		t.Error("invalid API key error should not be retryable")
	}
}
