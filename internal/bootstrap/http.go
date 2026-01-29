package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/krc/rag/internal/api"
)

// HTTPServer wraps HTTP server with graceful shutdown support.
type HTTPServer struct {
	server *http.Server
	logger *zap.Logger
}

// NewHTTPServer creates a new HTTP server wrapper.
func NewHTTPServer(port int, router *api.Router, readTimeout, writeTimeout time.Duration, logger *zap.Logger) *HTTPServer {
	return &HTTPServer{
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      router,
			ReadTimeout:  readTimeout,
			WriteTimeout: writeTimeout,
		},
		logger: logger,
	}
}

// Start starts the HTTP server in a goroutine.
func (s *HTTPServer) Start() error {
	go func() {
		s.logger.Info("starting HTTP server", zap.String("addr", s.server.Addr))
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()
	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server...")
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("HTTP server shutdown error", zap.Error(err))
		return err
	}
	s.logger.Info("HTTP server stopped")
	return nil
}

// Server returns the underlying HTTP server.
func (s *HTTPServer) Server() *http.Server {
	return s.server
}


