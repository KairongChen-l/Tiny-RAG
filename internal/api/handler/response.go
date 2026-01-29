package handler

import (
	"github.com/gin-gonic/gin"
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

// WriteJSON writes a JSON response using Gin context.
func WriteJSON(c *gin.Context, status int, data interface{}) {
	resp := Response{
		Success: status >= 200 && status < 300,
		Data:    data,
	}
	c.JSON(status, resp)
}

// WriteError writes an error response using Gin context.
func WriteError(c *gin.Context, status int, code, message string) {
	WriteErrorWithDetails(c, status, code, message, nil)
}

// WriteErrorWithDetails writes an error response with additional details using Gin context.
func WriteErrorWithDetails(c *gin.Context, status int, code, message string, details map[string]interface{}) {
	// Get request ID from context
	requestID := ""
	if id, exists := c.Get("request_id"); exists {
		if idStr, ok := id.(string); ok {
			requestID = idStr
		}
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
	}

	resp := Response{
		Success: false,
		Error:   errorInfo,
	}

	c.JSON(status, resp)
}

// WriteRetryableError writes a retryable error response.
func WriteRetryableError(c *gin.Context, status int, code, message string) {
	WriteErrorWithDetails(c, status, code, message, map[string]interface{}{
		"retryable": true,
	})
}

// WriteNonRetryableError writes a non-retryable error response.
func WriteNonRetryableError(c *gin.Context, status int, code, message string) {
	WriteErrorWithDetails(c, status, code, message, map[string]interface{}{
		"retryable": false,
	})
}

// isRetryableErrorCode checks if an error code indicates a retryable error.
func isRetryableErrorCode(code string) bool {
	// Timeout and service unavailable errors are typically retryable
	retryableCodes := map[string]bool{
		"TIMEOUT":             true,
		"SERVICE_UNAVAILABLE": true,
		"RATE_LIMIT":          true,
	}
	return retryableCodes[code]
}

// Common error codes
const (
	ErrCodeBadRequest         = "BAD_REQUEST"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeInternalError      = "INTERNAL_ERROR"
	ErrCodeValidation         = "VALIDATION_ERROR"
	ErrCodeUnsupportedFormat  = "UNSUPPORTED_FORMAT"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeRateLimit          = "RATE_LIMIT"
)
