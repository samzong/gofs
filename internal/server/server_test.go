package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/middleware"
)

func TestHealthCheckMiddleware(t *testing.T) {
	// Create a test handler that returns 404 for non-health endpoints
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Not Found")
	})

	// Wrap with health check middleware
	handler := healthCheckMiddleware(testHandler)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "health check endpoint /healthz",
			path:           "/healthz",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "health check endpoint /readyz",
			path:           "/readyz",
			expectedStatus: http.StatusOK,
			expectedBody:   "OK",
		},
		{
			name:           "non-health endpoint",
			path:           "/files",
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Not Found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if w.Body.String() != tt.expectedBody {
				t.Errorf("Expected body %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// Create a simple test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	// Create a logger (we can't easily test the output, but we can test it doesn't panic)
	logger := slog.Default()
	handler := loggingMiddleware(logger)(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// This should not panic
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body %q, got %q", "OK", w.Body.String())
	}
}

func TestResponseWriter(t *testing.T) {
	// Test the responseWriter wrapper
	w := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Test default status code
	if rw.statusCode != http.StatusOK {
		t.Errorf("Expected default status code %d, got %d", http.StatusOK, rw.statusCode)
	}

	// Test writing header
	rw.WriteHeader(http.StatusNotFound)
	if rw.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, rw.statusCode)
	}

	// Test writing body
	_, _ = rw.Write([]byte("test"))
	if w.Body.String() != "test" {
		t.Errorf("Expected body %q, got %q", "test", w.Body.String())
	}
}

func TestNew(t *testing.T) {
	cfg, err := config.New(8080, "localhost", ".", "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.Default()

	tests := []struct {
		name           string
		authMiddleware *middleware.BasicAuth
		logger         *slog.Logger
	}{
		{
			name:           "without auth middleware",
			authMiddleware: nil,
			logger:         logger,
		},
		{
			name:           "with nil logger",
			authMiddleware: nil,
			logger:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := New(cfg, testHandler, nil, tt.authMiddleware, tt.logger)

			if server == nil {
				t.Error("New() returned nil server")
				return
			}

			if server.config != cfg {
				t.Error("Server config not set correctly")
			}

			if server.handler == nil {
				t.Error("Server handler not set")
			}

			if server.logger == nil {
				t.Error("Server logger not set")
			}
		})
	}
}

func TestNewWithAuth(t *testing.T) {
	cfg, err := config.New(8080, "localhost", ".", "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authMiddleware, err := middleware.NewBasicAuth("test", "user", "password")
	if err != nil {
		t.Fatalf("Failed to create auth middleware: %v", err)
	}

	logger := slog.Default()
	server := New(cfg, testHandler, nil, authMiddleware, logger)

	if server == nil {
		t.Error("New() returned nil server")
		return
	}

	// Test that the middleware chain is working by making a request
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	server.handler.ServeHTTP(w, req)

	// Should return 401 without proper auth
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestServerShutdown(t *testing.T) {
	cfg, err := config.New(0, "localhost", ".", "default", false, nil) // Port 0 for random port
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	logger := slog.Default()
	server := New(cfg, testHandler, nil, nil, logger)

	// Test shutdown on nil server
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = server.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown() with nil server returned error: %v", err)
	}
}

func TestServerStartsOnRandomPort(t *testing.T) {
	cfg, err := config.New(0, "127.0.0.1", t.TempDir(), "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}
	if cfg.Port != 0 {
		t.Fatalf("expected configured port 0, got %d", cfg.Port)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	srv := New(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), nil, nil, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	var addr string
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		srv.mu.Lock()
		if srv.listener != nil {
			addr = srv.listener.Addr().String()
		}
		srv.mu.Unlock()
		if addr != "" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if addr == "" {
		t.Fatal("server did not create a listener")
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "0" {
		t.Fatalf("listener address = %q, error = %v", addr, err)
	}

	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("health check: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if !strings.Contains(logs.String(), `"address":"`+addr+`"`) {
		t.Fatalf("listener address missing from logs: %s", logs.String())
	}
}

func TestConcurrentRequests(t *testing.T) {
	cfg, err := config.New(8080, "localhost", ".", "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	logger := slog.Default()
	server := New(cfg, testHandler, nil, nil, logger)

	// Test concurrent requests
	const numConcurrent = 10
	done := make(chan bool, numConcurrent)

	for i := range numConcurrent {
		go func(id int) {
			req := httptest.NewRequest("GET", fmt.Sprintf("/%d", id), nil)
			w := httptest.NewRecorder()
			server.handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Concurrent request %d: expected status 200, got %d", id, w.Code)
			}
			done <- true
		}(i)
	}

	// Wait for all concurrent requests to complete
	for range numConcurrent {
		<-done
	}
}
