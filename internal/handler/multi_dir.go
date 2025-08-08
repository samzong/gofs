package handler

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/constants"
	"github.com/samzong/gofs/internal/filesystem"
	"github.com/samzong/gofs/internal/handler/templates"
)

// pathPool reduces string allocation overhead in path manipulation
var pathPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, constants.DefaultPathBufferSize)
		return &buf
	},
}

// generateContentETag creates a strong ETag based on content hash and version
func generateContentETag(content string) string {
	hash := sha256.Sum256([]byte(content))
	// Use first 16 bytes of hash for efficient ETag
	return fmt.Sprintf(`"gofs-%x"`, hash[:16])
}

// pathTrie implements a trie data structure for efficient path matching
type pathTrie struct {
	handler  *MountHandler
	children map[string]*pathTrie
	isEnd    bool
}

// newPathTrie creates a new path trie
func newPathTrie() *pathTrie {
	return &pathTrie{
		children: make(map[string]*pathTrie),
	}
}

// insert adds a path and its handler to the trie
func (t *pathTrie) insert(path string, handler *MountHandler) {
	node := t
	parts := strings.Split(strings.Trim(path, "/"), "/")

	for _, part := range parts {
		if part == "" {
			continue
		}
		if node.children[part] == nil {
			node.children[part] = newPathTrie()
		}
		node = node.children[part]
	}

	node.handler = handler
	node.isEnd = true
}

// findBestMatch searches for the longest prefix match in the trie
func (t *pathTrie) findBestMatch(path string) (*MountHandler, int) {
	node := t
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var bestHandler *MountHandler
	bestLen := 0
	currentLen := 0

	for _, part := range parts {
		if part == "" && len(parts) == 1 {
			// Root path case
			if node.handler != nil {
				return node.handler, 0
			}
			break
		}

		if part == "" {
			continue
		}

		if node.children[part] != nil {
			node = node.children[part]
			currentLen++
			if node.isEnd {
				bestHandler = node.handler
				bestLen = currentLen
			}
		} else {
			break
		}
	}

	return bestHandler, bestLen
}

// MultiDir handles multiple directory mounts with thread-safe operations
type MultiDir struct {
	mu         sync.RWMutex             // protects mounts map and mountOrder slice
	mounts     map[string]*MountHandler // path prefix -> handler (kept for compatibility)
	mountOrder []string                 // ordered list of mount paths for deterministic iteration
	trie       *pathTrie                // efficient path matching trie
	config     *config.Config
	logger     *slog.Logger
}

// MountHandler represents a single directory mount
type MountHandler struct {
	mount   config.DirMount
	fs      internal.FileSystem
	handler http.Handler
}

// NewMultiDir creates a new multi-directory handler with optimized path matching
func NewMultiDir(dirs []config.DirMount, cfg *config.Config, logger *slog.Logger) *MultiDir {
	mounts := make(map[string]*MountHandler)
	var mountOrder []string
	trie := newPathTrie()

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

		mountHandler := &MountHandler{
			mount:   mount,
			fs:      fs,
			handler: handler,
		}

		// Ensure path ends with / for proper prefix matching (legacy map)
		mountPath := mount.Path
		if !strings.HasSuffix(mountPath, "/") {
			mountPath += "/"
		}

		mounts[mountPath] = mountHandler
		mountOrder = append(mountOrder, mountPath)

		// Add to trie for efficient lookup
		trie.insert(mount.Path, mountHandler)

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
		trie:       trie,
		config:     cfg,
		logger:     logger,
	}
}

// ServeHTTP implements http.Handler with panic recovery
func (m *MultiDir) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Panic recovery middleware
	defer func() {
		if err := recover(); err != nil {
			m.logger.Error("Handler panic recovered",
				slog.Any("error", err),
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
				slog.String("remote_addr", r.RemoteAddr))

			// Return 500 error if headers haven't been written yet
			if w.Header().Get("Content-Type") == "" {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}
	}()

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
	m.mu.RLock()
	defer m.mu.RUnlock()

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

// findBestMatch finds the mount with the longest matching prefix using optimized trie (thread-safe)
func (m *MultiDir) findBestMatch(path string) *MountHandler {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use trie for O(log n) lookup instead of O(n) iteration
	if m.trie != nil {
		handler, _ := m.trie.findBestMatch(path)
		return handler
	}

	// Fallback to legacy method if trie is not available
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

// serveMountedPath handles request for a mounted directory with optimized path operations
func (m *MultiDir) serveMountedPath(w http.ResponseWriter, r *http.Request, mountHandler *MountHandler) {
	// Store original path for restoration
	originalPath := r.URL.Path

	// Get buffer from pool for efficient path manipulation
	bufPtr := pathPool.Get().(*[]byte)
	defer func() {
		*bufPtr = (*bufPtr)[:0] // Reset buffer
		pathPool.Put(bufPtr)    // Return pointer to pool
	}()

	// Efficiently strip mount prefix
	mountPrefix := strings.TrimSuffix(mountHandler.mount.Path, "/")

	newPath := originalPath
	if mountPrefix != "" && strings.HasPrefix(originalPath, mountPrefix) {
		newPath = originalPath[len(mountPrefix):]
	}
	if newPath == "" {
		newPath = "/"
	}

	r.URL.Path = newPath

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

// serveStaticCSS serves the advanced theme CSS with improved caching
func (m *MultiDir) serveStaticCSS(w http.ResponseWriter, r *http.Request) {
	// Generate strong ETag based on content
	etag := generateContentETag(templates.AdvancedCSS)

	// Set cache headers for CSS
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", constants.StaticAssetCacheMaxAge))
	w.Header().Set("ETag", etag)

	// Check if client has cached version
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write([]byte(templates.AdvancedCSS))
}

// serveStaticJS serves the advanced theme JavaScript with improved caching
func (m *MultiDir) serveStaticJS(w http.ResponseWriter, r *http.Request) {
	// Generate strong ETag based on content
	etag := generateContentETag(templates.AdvancedJS)

	// Set cache headers for JS
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", constants.StaticAssetCacheMaxAge))
	w.Header().Set("ETag", etag)

	// Check if client has cached version
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, _ = w.Write([]byte(templates.AdvancedJS))
}
