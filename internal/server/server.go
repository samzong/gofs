// Package server provides HTTP server implementation for the gofs file server.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/middleware"
)

// Server implements a lightweight HTTP file server with graceful shutdown support.
type Server struct {
	config        *config.Config
	handler       http.Handler
	webdavHandler http.Handler
	server        *http.Server
	listener      net.Listener
	logger        *slog.Logger
	mu            sync.RWMutex
}

// healthCheckMiddleware wraps a handler to add health check endpoint.
func healthCheckMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz", "/readyz":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "OK")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware provides simple HTTP request logging using slog
func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the ResponseWriter to capture status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Info("HTTP request",
				slog.String("method", r.Method),
				slog.String("path", fmt.Sprintf("%q", r.URL.Path)),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", duration),
			)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// New creates a new HTTP server instance with the given configuration and handler.
// The authMiddleware parameter is optional; if nil, no authentication is required.
// The webdavHandler parameter is optional; if provided, WebDAV will be enabled on /dav path.
func New(cfg *config.Config, handler http.Handler, webdavHandler http.Handler,
	authMiddleware *middleware.BasicAuth, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	componentLogger := logger.With(slog.String("component", "server"))

	// Build simple middleware chain for the main handler
	var finalHandler = handler

	// Add health check middleware (first in chain)
	finalHandler = healthCheckMiddleware(finalHandler)

	// Add authentication middleware if provided
	if authMiddleware != nil {
		finalHandler = authMiddleware.Middleware(finalHandler)
	}

	// Add HTTP request logging middleware (last in chain)
	finalHandler = loggingMiddleware(componentLogger)(finalHandler)

	// Apply middleware to WebDAV handler if provided
	var finalWebDAVHandler http.Handler
	if webdavHandler != nil {
		finalWebDAVHandler = webdavHandler
		if authMiddleware != nil {
			finalWebDAVHandler = authMiddleware.Middleware(finalWebDAVHandler)
		}
		finalWebDAVHandler = loggingMiddleware(componentLogger)(finalWebDAVHandler)
	}

	// Create a router if WebDAV is enabled
	var rootHandler http.Handler
	if finalWebDAVHandler != nil {
		mux := http.NewServeMux()
		mux.Handle("/dav/", finalWebDAVHandler)
		mux.Handle("/", finalHandler)
		rootHandler = mux
	} else {
		rootHandler = finalHandler
	}

	componentLogger.Info("Server initialized",
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.String("dir", cfg.Dir),
		slog.Bool("auth_enabled", authMiddleware != nil),
		slog.Bool("webdav_enabled", webdavHandler != nil),
	)

	return &Server{
		config:        cfg,
		handler:       rootHandler,
		webdavHandler: finalWebDAVHandler,
		logger:        componentLogger,
	}
}

// Start starts the HTTP server and begins accepting connections.
// This method blocks until the server is shut down or an error occurs.
func (s *Server) Start() error {
	// Create listener
	addr := s.config.Address()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.logger.Error("Failed to create listener",
			slog.String("address", addr),
			slog.Any("error", err),
		)
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.logger.Info("Server listener created",
		slog.String("address", addr),
		slog.String("network", "tcp"),
	)

	s.mu.Lock()
	s.listener = listener
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.handler,
		ReadTimeout:  time.Duration(s.config.RequestTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.RequestTimeout) * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorLog:     nil, // Disable default logging in favor of structured logging
	}
	s.mu.Unlock()

	s.logger.Info("Server starting",
		slog.String("address", addr),
		slog.Duration("read_timeout", time.Duration(s.config.RequestTimeout)*time.Second),
		slog.Duration("write_timeout", time.Duration(s.config.RequestTimeout)*time.Second),
		slog.Duration("idle_timeout", 120*time.Second),
	)

	// Start serving
	if err := s.server.Serve(listener); err != nil {
		s.logger.Error("Server serve error",
			slog.String("address", addr),
			slog.Any("error", err),
		)
		return fmt.Errorf("server failed to serve: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// It waits for active connections to close within the provided context timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.server == nil {
		s.logger.Warn("Shutdown called on nil server")
		return nil
	}

	s.logger.Info("Server shutdown initiated")

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server shutdown failed", slog.Any("error", err))
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("Server shutdown completed")
	return nil
}
