package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/krc/rag/pkg/ratelimit"
)

// RequestIDKey is the context key for request ID.
const RequestIDKey = "request_id"

// RequestIDMiddleware adds a unique request ID to each request.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if request ID is already in header
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Add to response header
		c.Header("X-Request-ID", requestID)

		// Add to context
		c.Set(RequestIDKey, requestID)
		c.Next()
	}
}

// LoggerMiddleware returns a logging middleware.
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Get request ID from context
		requestID, _ := c.Get(RequestIDKey)
		requestIDStr := ""
		if id, ok := requestID.(string); ok {
			requestIDStr = id
		}

		logger.Info("request",
			zap.String("request_id", requestIDStr),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("duration", time.Since(start)),
			zap.String("remote_addr", c.ClientIP()),
		)
	}
}

// ValidateQueryParamsMiddleware validates query parameters for common endpoints.
func ValidateQueryParamsMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate limit parameter
		if limitStr := c.Query("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil || limit <= 0 || limit > 1000 {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "limit must be between 1 and 1000",
					},
				})
				c.Abort()
				return
			}
		}

		// Validate offset parameter
		if offsetStr := c.Query("offset"); offsetStr != "" {
			offset, err := strconv.Atoi(offsetStr)
			if err != nil || offset < 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "offset must be a non-negative integer",
					},
				})
				c.Abort()
				return
			}
		}

		// Validate sort_by parameter
		if sortBy := c.Query("sort_by"); sortBy != "" {
			validSortFields := map[string]bool{
				"created_at": true,
				"updated_at": true,
				"title":      true,
			}
			if !validSortFields[sortBy] {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "sort_by must be one of: created_at, updated_at, title",
					},
				})
				c.Abort()
				return
			}
		}

		// Validate order parameter
		if order := c.Query("order"); order != "" {
			if order != "asc" && order != "desc" {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error": gin.H{
						"code":    "VALIDATION_ERROR",
						"message": "order must be 'asc' or 'desc'",
					},
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// ValidateContentTypeMiddleware validates Content-Type header for POST/PUT requests.
func ValidateContentTypeMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only validate for POST/PUT/PATCH requests with body
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			if c.Request.Body != nil && c.Request.ContentLength > 0 {
				contentType := c.GetHeader("Content-Type")
				// Allow multipart/form-data for file uploads
				if !strings.HasPrefix(contentType, "application/json") &&
					!strings.HasPrefix(contentType, "multipart/form-data") &&
					!strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
					c.JSON(http.StatusBadRequest, gin.H{
						"success": false,
						"error": gin.H{
							"code":    "VALIDATION_ERROR",
							"message": "Content-Type must be application/json, multipart/form-data, or application/x-www-form-urlencoded",
						},
					})
					c.Abort()
					return
				}
			}
		}

		c.Next()
	}
}

// CORSMiddleware returns a CORS middleware.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// RateLimitMiddleware returns a rate limiting middleware.
func RateLimitMiddleware(limiter *ratelimit.Limiter, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow() {
			logger.Warn("rate limit exceeded",
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.String("ip", c.ClientIP()),
			)
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error": gin.H{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "rate limit exceeded",
				},
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
