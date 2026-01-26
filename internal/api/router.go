package api

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/krc/rag/internal/api/handler"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed static
var staticFS embed.FS

// Router holds API router and dependencies.
type Router struct {
	mux     *chi.Mux
	logger  *zap.Logger
	handler *handler.Handler
}

// NewRouter creates a new API router.
func NewRouter(h *handler.Handler, logger *zap.Logger) *Router {
	r := &Router{
		mux:     chi.NewRouter(),
		logger:  logger,
		handler: h,
	}

	r.setupMiddleware()
	r.setupRoutes()
	r.setupStaticFiles()

	return r
}

// setupMiddleware configures global middleware.
func (r *Router) setupMiddleware() {
	r.mux.Use(Chain(
		Recoverer(r.logger),
		Logger(r.logger),
		CORS(),
	))
}

// setupRoutes configures API routes.
func (r *Router) setupRoutes() {
	// Prometheus metrics endpoint
	r.mux.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.mux.Route("/api/v1", func(router chi.Router) {
		// Health check
		router.Get("/health", r.handler.Health)

		// Document endpoints
		router.Post("/documents", r.handler.UploadDocument)
		router.Get("/documents", r.handler.ListDocuments)
		router.Delete("/documents/{id}", r.handler.DeleteDocument)

		// Job endpoints
		router.Get("/jobs/{id}", r.handler.GetJob)

		// Query endpoints (single-turn)
		router.Post("/query", r.handler.Query)

		// Conversation endpoints (multi-turn)
		router.Post("/conversations", r.handler.CreateConversation)
		router.Get("/conversations", r.handler.ListConversations)
		router.Get("/conversations/{id}", r.handler.GetConversation)
		router.Delete("/conversations/{id}", r.handler.DeleteConversation)
		router.Post("/conversations/{id}/messages", r.handler.SendMessage)
	})
}

// setupStaticFiles serves the embedded static files for the web UI.
func (r *Router) setupStaticFiles() {
	// Serve embedded static files
	staticContent, err := fs.Sub(staticFS, "static")
	if err != nil {
		r.logger.Error("failed to load static files", zap.Error(err))
		return
	}

	// Serve index.html at root
	r.mux.Get("/", func(w http.ResponseWriter, req *http.Request) {
		data, err := fs.ReadFile(staticContent, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	// Serve other static files
	fileServer := http.FileServer(http.FS(staticContent))
	r.mux.Handle("/static/*", http.StripPrefix("/static/", fileServer))
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
