// Package server provides HTTP server implementation for the gofs file server.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/samzong/gofs/internal/config"
)

// Server implements a lightweight HTTP file server with graceful shutdown support.
type Server struct {
	config   *config.Config
	handler  http.Handler
	server   *http.Server
	listener net.Listener
	mu       sync.RWMutex
}

// New creates a new HTTP server instance with the given configuration and handler.
func New(cfg *config.Config, handler http.Handler) *Server {
	return &Server{
		config:  cfg,
		handler: handler,
	}
}

// Start starts the HTTP server and begins accepting connections.
// This method blocks until the server is shut down or an error occurs.
func (s *Server) Start() error {
	ctx := context.Background()

	// Create listener
	addr := s.config.Address()
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.mu.Lock()
	s.listener = listener
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}
	s.mu.Unlock()

	// Start serving
	return s.server.Serve(listener)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// It waits for active connections to close within the provided context timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.server == nil {
		return nil
	}

	return s.server.Shutdown(ctx)
}
