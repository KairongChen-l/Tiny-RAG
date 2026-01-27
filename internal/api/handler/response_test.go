package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteError_StandardFormat(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, ErrCodeValidation, "invalid parameter")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success=false")
	}

	if resp.Error == nil {
		t.Fatal("expected error field")
	}

	// Unmarshal error info
	errorBytes, _ := json.Marshal(resp.Error)
	var errorInfo EnhancedErrorInfo
	if err := json.Unmarshal(errorBytes, &errorInfo); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errorInfo.Code != ErrCodeValidation {
		t.Errorf("expected error code %s, got %s", ErrCodeValidation, errorInfo.Code)
	}

	if errorInfo.Message != "invalid parameter" {
		t.Errorf("expected error message 'invalid parameter', got %s", errorInfo.Message)
	}
}

func TestWriteError_WithDetails(t *testing.T) {
	w := httptest.NewRecorder()
	WriteErrorWithDetails(w, http.StatusInternalServerError, ErrCodeInternalError, "operation failed", map[string]interface{}{
		"request_id": "test-123",
		"retryable":  true,
	})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error field")
	}

	// Check if details are included
	errorMap, ok := resp.Error.(map[string]interface{})
	if !ok {
		// Try to unmarshal as EnhancedErrorInfo
		errorBytes, _ := json.Marshal(resp.Error)
		var errorInfo EnhancedErrorInfo
		if err := json.Unmarshal(errorBytes, &errorInfo); err != nil {
			t.Fatalf("failed to unmarshal error: %v", err)
		}
		errorMap = map[string]interface{}{
			"code":      errorInfo.Code,
			"message":   errorInfo.Message,
			"details":   errorInfo.Details,
			"retryable": errorInfo.Retryable,
		}
	}

	details, ok := errorMap["details"].(map[string]interface{})
	if !ok {
		t.Fatal("expected details field")
	}

	if details["request_id"] != "test-123" {
		t.Errorf("expected request_id 'test-123', got %v", details["request_id"])
	}

	if details["retryable"] != true {
		t.Errorf("expected retryable true, got %v", details["retryable"])
	}
}

func TestWriteError_RetryableError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteRetryableError(w, http.StatusServiceUnavailable, ErrCodeInternalError, "service temporarily unavailable")

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Unmarshal error info
	errorBytes, _ := json.Marshal(resp.Error)
	var errorInfo EnhancedErrorInfo
	if err := json.Unmarshal(errorBytes, &errorInfo); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if !errorInfo.Retryable {
		t.Error("expected retryable=true")
	}
}

func TestWriteError_NonRetryableError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteNonRetryableError(w, http.StatusBadRequest, ErrCodeValidation, "invalid request format")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Unmarshal error info
	errorBytes, _ := json.Marshal(resp.Error)
	var errorInfo EnhancedErrorInfo
	if err := json.Unmarshal(errorBytes, &errorInfo); err != nil {
		t.Fatalf("failed to unmarshal error: %v", err)
	}

	if errorInfo.Retryable {
		t.Error("expected retryable=false")
	}
}

