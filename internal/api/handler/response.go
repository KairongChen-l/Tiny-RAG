package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Response represents a standard API response.
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   interface{} `json:"error,omitempty"` // Can be ErrorInfo or EnhancedErrorInfo
}

// ErrorInfo holds error details.
type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// EnhancedErrorInfo holds enhanced error details with additional context.
type EnhancedErrorInfo struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
	Timestamp string                 `json:"timestamp,omitempty"`
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	resp := Response{
		Success: status >= 200 && status < 300,
		Data:    data,
	}

	json.NewEncoder(w).Encode(resp)
}

// WriteError writes an error response.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteErrorWithContext(w, nil, status, code, message, nil)
}

// RequestIDKey is the context key for request ID (duplicated here to avoid import cycle).
type RequestIDKey struct{}

// WriteErrorWithDetails writes an error response with additional details.
func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
	WriteErrorWithContext(w, nil, status, code, message, details)
}

// WriteErrorWithContext writes an error response with context for request ID extraction.
func WriteErrorWithContext(w http.ResponseWriter, ctx context.Context, status int, code, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Get request ID from context if available
	requestID := ""
	if ctx != nil {
		if id := ctx.Value(RequestIDKey{}); id != nil {
			if idStr, ok := id.(string); ok {
				requestID = idStr
			}
		}
	}
	if requestID == "" {
		requestID = uuid.New().String()
	}

	// Determine retryable status
	retryable := isRetryableErrorCode(code)
	if details != nil {
		if retryableVal, ok := details["retryable"].(bool); ok {
			retryable = retryableVal
		}
	}

	errorInfo := &EnhancedErrorInfo{
		Code:      code,
		Message:   message,
		Details:   details,
		Retryable: retryable,
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	resp := Response{
		Success: false,
		Error:   errorInfo,
	}

	json.NewEncoder(w).Encode(resp)
}

// WriteRetryableError writes a retryable error response.
func WriteRetryableError(w http.ResponseWriter, status int, code, message string) {
	WriteErrorWithDetails(w, status, code, message, map[string]interface{}{
		"retryable": true,
	})
}

// WriteNonRetryableError writes a non-retryable error response.
func WriteNonRetryableError(w http.ResponseWriter, status int, code, message string) {
	WriteErrorWithDetails(w, status, code, message, map[string]interface{}{
		"retryable": false,
	})
}

// isRetryableErrorCode checks if an error code indicates a retryable error.
func isRetryableErrorCode(code string) bool {
	// Timeout and service unavailable errors are typically retryable
	retryableCodes := map[string]bool{
		"TIMEOUT":            true,
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMIT":         true,
	}
	return retryableCodes[code]
}

// Common error codes
const (
	ErrCodeBadRequest        = "BAD_REQUEST"
	ErrCodeNotFound          = "NOT_FOUND"
	ErrCodeInternalError     = "INTERNAL_ERROR"
	ErrCodeValidation        = "VALIDATION_ERROR"
	ErrCodeUnsupportedFormat = "UNSUPPORTED_FORMAT"
	ErrCodeTimeout           = "TIMEOUT"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeRateLimit         = "RATE_LIMIT"
)

