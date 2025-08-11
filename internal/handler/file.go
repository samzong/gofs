// Package handler provides HTTP request handlers for the gofs file server.
package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/handler/templates"
	"github.com/samzong/gofs/pkg/fileutil"
	"github.com/samzong/gofs/pkg/httprange"
)

// File implements HTTP request handling for file system operations.
type File struct {
	fs     internal.FileSystem
	config *config.Config
	logger *slog.Logger
}

// NewFile creates a new file handler with the given file system and configuration.
func NewFile(fs internal.FileSystem, cfg *config.Config, logger *slog.Logger) *File {
	return &File{
		fs:     fs,
		config: cfg,
		logger: logger,
	}
}

// ServeHTTP handles incoming HTTP requests for file operations.
// It only supports GET requests and returns Method Not Allowed for others.
func (h *File) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only support GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.handleGet(w, r)
}

// handleGet processes GET requests for files and directories.
func (h *File) handleGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	// Safe path handling to prevent directory traversal
	safePath := fileutil.SafePath(strings.TrimPrefix(path, "/"))

	// Get file information
	info, err := h.fs.Stat(safePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Handle directories by showing file listing
	if info.IsDir() {
		h.handleDirectory(w, r, safePath)
		return
	}

	// Handle files by serving the content
	h.handleFile(w, r, safePath)
}

// handleDirectory processes directory requests and returns file listings.
// It supports both HTML and JSON responses based on the Accept header.
func (h *File) handleDirectory(w http.ResponseWriter, r *http.Request, path string) {
	files, err := h.fs.ReadDir(path)
	if err != nil {
		http.Error(w, "Cannot read directory", http.StatusInternalServerError)
		return
	}

	// Sort file list with directories first, then alphabetically (case-insensitive)
	sort.Slice(files, func(i, j int) bool {
		// Directories come first
		if files[i].IsDir() && !files[j].IsDir() {
			return true
		}
		if !files[i].IsDir() && files[j].IsDir() {
			return false
		}
		// Case-insensitive alphabetical sorting within same type
		return strings.ToLower(files[i].Name()) < strings.ToLower(files[j].Name())
	})

	// Check if JSON format is requested
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		h.renderJSON(w, path, files)
		return
	}

	// Render HTML response with configured theme
	h.renderHTML(w, path, files, h.config.Theme)
}

// handleFile processes file requests and serves file content with appropriate headers.
// It supports HTTP Range requests for resumable downloads according to RFC 7233.
func (h *File) handleFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := h.fs.Open(path)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer h.closeFile(file, path)

	// Get file info for size and security checks
	info, err := h.fs.Stat(path)
	if err != nil {
		http.Error(w, "Cannot stat file", http.StatusInternalServerError)
		return
	}

	// Check file size limit (prevent DoS)
	if info.Size() > h.config.MaxFileSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Set security headers
	h.setSecurityHeaders(w)

	// Parse Range header if present
	rangeHeader := r.Header.Get("Range")
	rng, err := httprange.ParseRange(rangeHeader, info.Size())
	if err != nil {
		if err == httprange.ErrUnsatisfiableRange {
			// Send 416 Range Not Satisfiable
			httprange.WriteRangeNotSatisfiable(w, info.Size())
			return
		}
		// For other errors (invalid format, multiple ranges), ignore and serve full content
		rng = nil
	}

	// Determine MIME type
	mimeType := fileutil.DetectMimeType(path)

	// Check if the file is seekable
	seeker, seekable := file.(io.ReadSeeker)
	if !seekable && rng != nil {
		// File doesn't support seeking, serve full content
		h.logger.Debug("File doesn't support seeking, serving full content",
			slog.String("path", path),
			slog.String("component", "file_handler"),
		)
		rng = nil
	}

	// Serve content based on range request
	if rng != nil {
		// Serve partial content
		h.logger.Debug("Serving partial content",
			slog.String("path", path),
			slog.Int64("start", rng.Start),
			slog.Int64("end", rng.End),
			slog.Int64("length", rng.Length),
			slog.String("component", "file_handler"),
		)

		// Set filename header for partial content
		filename := filepath.Base(path)
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))

		if err := httprange.ServeContent(w, seeker, rng, info.Size(), mimeType); err != nil {
			h.logger.Warn("Error serving partial content",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("component", "file_handler"),
			)
		}
	} else {
		// Serve full content
		h.setFileHeaders(w, path, info)
		if err := httprange.ServeFullContent(w, file, info.Size(), mimeType); err != nil {
			h.logger.Warn("Error serving full content",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("component", "file_handler"),
			)
		}
	}
}

// closeFile handles safe file closing with error logging.
func (h *File) closeFile(file io.ReadCloser, path string) {
	if err := file.Close(); err != nil {
		h.logger.Warn("File close failed",
			slog.String("path", path),
			slog.String("error", err.Error()),
			slog.String("component", "file_handler"),
		)
	}
}

// setSecurityHeaders sets security-related HTTP headers.
func (h *File) setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	if h.config.EnableSecurity {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
	}
}

// setFileHeaders sets file-specific HTTP headers.
func (h *File) setFileHeaders(w http.ResponseWriter, path string, info internal.FileInfo) {
	// Set filename in Content-Disposition header
	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))

	// Set MIME type
	w.Header().Set("Content-Type", fileutil.DetectMimeType(path))

	// Set content length
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
}

// renderJSON renders the file listing as JSON for API consumers.
func (h *File) renderJSON(w http.ResponseWriter, path string, files []internal.FileInfo) {
	type FileItem struct {
		Name    string `json:"name"`
		ModTime string `json:"modTime"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"isDir"`
	}

	items := make([]FileItem, 0, len(files))
	for _, file := range files {
		items = append(items, FileItem{
			Name:    file.Name(),
			Size:    file.Size(),
			IsDir:   file.IsDir(),
			ModTime: file.ModTime().Format(time.RFC3339),
		})
	}

	response := map[string]any{
		"path":  path,
		"files": items,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
		return
	}
}

// renderHTML renders the file listing as HTML for browser viewing.
func (h *File) renderHTML(w http.ResponseWriter, path string, files []internal.FileInfo, theme string) {
	type FileItem struct {
		Name  string
		Size  string
		IsDir bool
	}

	items := make([]FileItem, 0, len(files))
	for _, file := range files {
		size := ""
		if !file.IsDir() {
			size = fileutil.FormatSize(file.Size())
		}
		items = append(items, FileItem{
			Name:  file.Name(),
			IsDir: file.IsDir(),
			Size:  size,
		})
	}

	data := struct {
		Path   string
		Files  []FileItem
		Parent bool
		CSS    template.CSS // Inject CSS styles as safe CSS type
		Theme  string       // Current theme name for potential use in template
	}{
		Path:   "/" + path,
		Parent: path != "",
		Files:  items,
		CSS:    template.CSS(templates.GetThemeCSS(theme)), // Use theme-specific CSS
		Theme:  theme,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Use pre-compiled template for better performance
	if err := templates.DirectoryTemplate.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}
