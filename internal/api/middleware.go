package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Middleware wraps an http.Handler with additional functionality.
type Middleware func(http.Handler) http.Handler

// Chain chains multiple middlewares.
func Chain(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

// RequestIDKey is the context key for request ID.
type RequestIDKey struct{}

// Logger returns a logging middleware.
func Logger(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			// Get request ID from context
			requestID := r.Context().Value(RequestIDKey{}).(string)

			logger.Info("request",
				zap.String("request_id", requestID),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", wrapped.status),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

// RequestID adds a unique request ID to each request.
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if request ID is already in header
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Add to response header
			w.Header().Set("X-Request-ID", requestID)

			// Add to context
			ctx := context.WithValue(r.Context(), RequestIDKey{}, requestID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateQueryParams validates query parameters for common endpoints.
func ValidateQueryParams(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate limit parameter
			if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
				limit, err := strconv.Atoi(limitStr)
				if err != nil || limit <= 0 || limit > 1000 {
					writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be between 1 and 1000")
					return
				}
			}

			// Validate offset parameter
			if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
				offset, err := strconv.Atoi(offsetStr)
				if err != nil || offset < 0 {
					writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "offset must be a non-negative integer")
					return
				}
			}

			// Validate sort_by parameter
			if sortBy := r.URL.Query().Get("sort_by"); sortBy != "" {
				validSortFields := map[string]bool{
					"created_at": true,
					"updated_at": true,
					"title":      true,
				}
				if !validSortFields[sortBy] {
					writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "sort_by must be one of: created_at, updated_at, title")
					return
				}
			}

			// Validate order parameter
			if order := r.URL.Query().Get("order"); order != "" {
				if order != "asc" && order != "desc" {
					writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "order must be 'asc' or 'desc'")
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateContentType validates Content-Type header for POST/PUT requests.
func ValidateContentType(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate for POST/PUT/PATCH requests with body
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				if r.Body != nil && r.ContentLength > 0 {
					contentType := r.Header.Get("Content-Type")
					// Allow multipart/form-data for file uploads
					if !strings.HasPrefix(contentType, "application/json") &&
						!strings.HasPrefix(contentType, "multipart/form-data") &&
						!strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
						writeErrorResponse(w, http.StatusBadRequest, "VALIDATION_ERROR", "Content-Type must be application/json, multipart/form-data, or application/x-www-form-urlencoded")
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Recoverer returns a panic recovery middleware.
func Recoverer(logger *zap.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("panic recovered",
						zap.Any("error", err),
						zap.String("path", r.URL.Path),
					)
					writeErrorResponse(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// writeErrorResponse writes an error response (internal use in middleware).
func writeErrorResponse(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

// CORS returns a CORS middleware.
func CORS() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
