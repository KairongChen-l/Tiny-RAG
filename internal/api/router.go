package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/api/handler"
	"github.com/krc/rag/internal/ws"
	"github.com/krc/rag/pkg/ratelimit"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed static
var staticFS embed.FS

// Router holds API router and dependencies.
type Router struct {
	engine  *gin.Engine
	logger  *zap.Logger
	handler *handler.Handler
	limiter *ratelimit.Limiter // Optional rate limiter (nil if disabled)
	wsHandler *ws.Handler // Optional WebSocket handler
}

// RouterConfig holds router configuration.
type RouterConfig struct {
	Handler   *handler.Handler
	Logger    *zap.Logger
	Limiter   *ratelimit.Limiter // Optional: nil to disable rate limiting
	WSHandler *ws.Handler         // Optional: nil to disable WebSocket
}

// NewRouter creates a new API router.
// If limiter is nil, rate limiting will be disabled (useful for testing).
func NewRouter(cfg RouterConfig) *Router {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	r := &Router{
		engine:    gin.New(),
		logger:    cfg.Logger,
		handler:   cfg.Handler,
		limiter:   cfg.Limiter,
		wsHandler: cfg.WSHandler,
	}

	r.setupMiddleware()
	r.setupRoutes()
	r.setupStaticFiles()

	return r
}

// setupMiddleware configures global middleware.
func (r *Router) setupMiddleware() {
	// Recovery middleware
	r.engine.Use(gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		r.logger.Error("panic recovered",
			zap.Any("error", recovered),
			zap.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "internal server error",
			},
		})
		c.Abort()
	}))

	// Request ID middleware
	r.engine.Use(RequestIDMiddleware())

	// Logger middleware
	r.engine.Use(LoggerMiddleware(r.logger))

	// CORS middleware
	r.engine.Use(CORSMiddleware())

	// Content type validation middleware
	r.engine.Use(ValidateContentTypeMiddleware(r.logger))

	// Query parameter validation middleware
	r.engine.Use(ValidateQueryParamsMiddleware(r.logger))

	// Rate limiting middleware (only if limiter is configured)
	if r.limiter != nil {
		r.engine.Use(RateLimitMiddleware(r.limiter, r.logger))
		r.logger.Info("rate limiting enabled")
	} else {
		r.logger.Debug("rate limiting disabled")
	}
}

// setupRoutes configures API routes.
func (r *Router) setupRoutes() {
	// Prometheus metrics endpoint
	r.engine.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := r.engine.Group("/api/v1")
	{
		// Health check
		v1.GET("/health", r.handler.Health)

		// Document endpoints
		v1.POST("/documents", r.handler.UploadDocument)
		v1.POST("/documents/batch", r.handler.BatchUploadDocuments)
		v1.GET("/documents", r.handler.ListDocuments)
		v1.GET("/documents/stats", r.handler.GetDocumentStats)
		v1.DELETE("/documents/:id", r.handler.DeleteDocument)
		v1.POST("/documents/batch-delete", r.handler.BatchDeleteDocuments)
		v1.POST("/documents/:id/restore", r.handler.RestoreDocument)
		v1.GET("/documents/:id/versions", r.handler.ListDocumentVersions)
		v1.POST("/documents/:id/restore-version", r.handler.RestoreDocumentVersion)

		// Job endpoints
		v1.GET("/jobs/:id", r.handler.GetJob)

		// Query endpoints (single-turn)
		v1.POST("/query", r.handler.Query)

		// Conversation endpoints (multi-turn)
		v1.POST("/conversations", r.handler.CreateConversation)
		v1.GET("/conversations", r.handler.ListConversations)
		v1.GET("/conversations/:id", r.handler.GetConversation)
		v1.DELETE("/conversations/:id", r.handler.DeleteConversation)
		v1.POST("/conversations/:id/messages", r.handler.SendMessage)
	}

	// WebSocket endpoint (outside v1 group for direct access)
	if r.wsHandler != nil {
		r.engine.GET("/ws", func(c *gin.Context) {
			r.wsHandler.HandleWebSocket(c.Writer, c.Request)
		})
		r.logger.Info("WebSocket endpoint enabled at /ws")
	}
}

// setupStaticFiles serves the embedded static files for the web UI.
func (r *Router) setupStaticFiles() {
	// Load embedded static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		r.logger.Error("failed to load static files", zap.Error(err))
		return
	}

	// Serve index.html at root
	r.engine.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(staticContent, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "index.html not found")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})

	// Serve other static files
	r.engine.StaticFS("/static", http.FS(staticContent))
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.engine.ServeHTTP(w, req)
}
