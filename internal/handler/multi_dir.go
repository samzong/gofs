package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
	"github.com/samzong/gofs/internal/handler/templates"
)

// MultiDir handles multiple directory mounts
type MultiDir struct {
	mounts     map[string]*MountHandler // path prefix -> handler
	mountOrder []string                 // ordered list of mount paths for deterministic iteration
	config     *config.Config
	logger     *slog.Logger
}

// MountHandler represents a single directory mount
type MountHandler struct {
	mount   config.DirMount
	fs      internal.FileSystem
	handler http.Handler
}

// NewMultiDir creates a new multi-directory handler
func NewMultiDir(dirs []config.DirMount, cfg *config.Config, logger *slog.Logger) *MultiDir {
	mounts := make(map[string]*MountHandler)
	var mountOrder []string

	for _, mount := range dirs {
		// Create filesystem
		var fs internal.FileSystem = filesystem.NewLocal(mount.Dir, cfg.ShowHidden)
		if mount.Readonly {
			fs = filesystem.NewReadonly(fs)
		}

		// Create handler based on theme
		var handler http.Handler
		if cfg.Theme == "advanced" {
			handler = NewAdvancedFile(fs, cfg)
		} else {
			handler = NewFile(fs, cfg, logger)
		}

		// Ensure path ends with / for proper prefix matching
		mountPath := mount.Path
		if !strings.HasSuffix(mountPath, "/") {
			mountPath += "/"
		}

		mounts[mountPath] = &MountHandler{
			mount:   mount,
			fs:      fs,
			handler: handler,
		}
		mountOrder = append(mountOrder, mountPath)

		logger.Info("Directory mounted",
			slog.String("path", mount.Path),
			slog.String("dir", mount.Dir),
			slog.Bool("readonly", mount.Readonly),
			slog.String("name", mount.Name),
		)
	}

	return &MultiDir{
		mounts:     mounts,
		mountOrder: mountOrder,
		config:     cfg,
		logger:     logger,
	}
}

// ServeHTTP implements http.Handler
func (m *MultiDir) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle static assets for advanced theme
	if r.URL.Path == "/static/theme.css" || r.URL.Path == "/static/theme.js" {
		m.handleStaticAssets(w, r)
		return
	}

	// Handle root path
	if r.URL.Path == "/" {
		m.handleRoot(w, r)
		return
	}

	// Find best matching mount
	mountHandler := m.findBestMatch(r.URL.Path)
	if mountHandler == nil {
		http.NotFound(w, r)
		return
	}

	m.serveMountedPath(w, r, mountHandler)
}

// handleRoot serves the root path - redirect to first mount
func (m *MultiDir) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Always redirect to first mount (deterministically) for all themes
	if len(m.mountOrder) > 0 {
		firstMountPath := m.mountOrder[0]
		if mount, exists := m.mounts[firstMountPath]; exists {
			http.Redirect(w, r, mount.mount.Path, http.StatusFound)
			return
		}
	}
	http.NotFound(w, r)
}

// findBestMatch finds the mount with the longest matching prefix
func (m *MultiDir) findBestMatch(path string) *MountHandler {
	var bestMatch *MountHandler
	var bestLen int

	for prefix, handler := range m.mounts {
		// Handle exact path match (e.g., /cmd matches /cmd/)
		mountPath := strings.TrimSuffix(prefix, "/")
		if path == mountPath || strings.HasPrefix(path, prefix) {
			prefixLen := len(mountPath)
			if prefixLen > bestLen {
				bestMatch = handler
				bestLen = prefixLen
			}
		}
	}
	return bestMatch
}

// serveMountedPath handles request for a mounted directory
func (m *MultiDir) serveMountedPath(w http.ResponseWriter, r *http.Request, mountHandler *MountHandler) {
	// Strip mount prefix and adjust path
	originalPath := r.URL.Path
	mountPrefix := strings.TrimSuffix(mountHandler.mount.Path, "/")
	r.URL.Path = strings.TrimPrefix(r.URL.Path, mountPrefix)
	if r.URL.Path == "" {
		r.URL.Path = "/"
	}

	// Add mount context and serve
	mount := mountHandler.mount
	ctx := internal.WithMountInfo(r.Context(), mount.Path, mount.Name, mount.Readonly)
	r = r.WithContext(ctx)
	mountHandler.handler.ServeHTTP(w, r)
	r.URL.Path = originalPath // Restore for potential reuse
}

// handleStaticAssets serves static CSS and JS files for the advanced theme
func (m *MultiDir) handleStaticAssets(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/static/theme.css":
		m.serveStaticCSS(w, r)
	case "/static/theme.js":
		m.serveStaticJS(w, r)
	default:
		http.NotFound(w, r)
	}
}

// serveStaticCSS serves the advanced theme CSS
func (m *MultiDir) serveStaticCSS(w http.ResponseWriter, r *http.Request) {
	// Set cache headers for CSS (cache for 1 hour)
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
	w.Header().Set("ETag", fmt.Sprintf(`"%x"`, templates.AdvancedCSS))

	// Check if client has cached version
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == fmt.Sprintf(`"%x"`, templates.AdvancedCSS) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write([]byte(templates.AdvancedCSS))
}

// serveStaticJS serves the advanced theme JavaScript
func (m *MultiDir) serveStaticJS(w http.ResponseWriter, r *http.Request) {
	// Set cache headers for JS (cache for 1 hour)
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600, immutable")
	w.Header().Set("ETag", fmt.Sprintf(`"%x"`, templates.AdvancedJS))

	// Check if client has cached version
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == fmt.Sprintf(`"%x"`, templates.AdvancedJS) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write([]byte(templates.AdvancedJS))
}
