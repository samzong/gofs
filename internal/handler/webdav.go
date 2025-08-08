package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"golang.org/x/net/webdav"
)

// WebDAV handles WebDAV protocol requests for read-only file access
type WebDAV struct {
	handler *webdav.Handler
	config  *config.Config
	logger  *slog.Logger
	prefix  string
}

// NewWebDAV creates a new WebDAV handler
func NewWebDAV(fs internal.FileSystem, cfg *config.Config, logger *slog.Logger) *WebDAV {
	// Create WebDAV adapter
	adapter := NewWebDAVAdapter(fs)

	// Configure WebDAV handler with read-only lock system
	handler := &webdav.Handler{
		FileSystem: adapter,
		LockSystem: webdav.NewMemLS(), // Memory-based lock system
		Logger: func(r *http.Request, err error) {
			if err != nil {
				logger.Debug("webdav operation",
					"method", r.Method,
					"path", r.URL.Path,
					"error", err)
			}
		},
		Prefix: "/dav",
	}

	return &WebDAV{
		handler: handler,
		config:  cfg,
		logger:  logger,
		prefix:  "/dav",
	}
}

// ServeHTTP implements http.Handler interface
func (w *WebDAV) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	// Block write operations for extra safety
	switch r.Method {
	case "PUT", "DELETE", "MKCOL", "COPY", "MOVE", "PROPPATCH", "LOCK", "UNLOCK":
		w.logger.Warn("write operation attempted",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr)
		http.Error(rw, "Method Not Allowed - Read Only", http.StatusMethodNotAllowed)
		return
	}

	// Add WebDAV headers for client compatibility
	rw.Header().Set("DAV", "1, 2")
	rw.Header().Set("MS-Author-Via", "DAV")

	// Handle OPTIONS for Windows clients
	if r.Method == "OPTIONS" {
		rw.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND")
		rw.Header().Set("Public", "OPTIONS, GET, HEAD, PROPFIND")
		rw.WriteHeader(http.StatusOK)
		return
	}

	// Limit PROPFIND depth to prevent resource exhaustion
	if r.Method == "PROPFIND" && r.Header.Get("Depth") == "infinity" {
		r.Header.Set("Depth", "1")
	}

	// Verify path prefix
	if !strings.HasPrefix(r.URL.Path, w.prefix) {
		http.NotFound(rw, r)
		return
	}

	// Delegate to WebDAV handler
	w.handler.ServeHTTP(rw, r)
}
