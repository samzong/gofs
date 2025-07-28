// Package handler provides HTTP request handlers for the gofs file server.
package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
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
)

// File implements HTTP request handling for file system operations.
type File struct {
	fs     internal.FileSystem
	config *config.Config
}

// NewFile creates a new file handler with the given file system and configuration.
func NewFile(fs internal.FileSystem, cfg *config.Config) *File {
	return &File{
		fs:     fs,
		config: cfg,
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

	// Sort file list with directories first, then alphabetically
	sort.Slice(files, func(i, j int) bool {
		// Directories come first
		if files[i].IsDir() && !files[j].IsDir() {
			return true
		}
		if !files[i].IsDir() && files[j].IsDir() {
			return false
		}
		return files[i].Name() < files[j].Name()
	})

	// Check if JSON format is requested
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		h.renderJSON(w, path, files)
		return
	}

	// Render HTML response
	h.renderHTML(w, path, files)
}

// handleFile processes file requests and serves file content with appropriate headers.
func (h *File) handleFile(w http.ResponseWriter, _ *http.Request, path string) {
	file, err := h.fs.Open(path)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail the request as data may have been sent
			// In production, this should use structured logging
			fmt.Printf("Warning: Failed to close file %s: %v\n", path, closeErr)
		}
	}()

	// Get file info for size and security checks
	info, err := h.fs.Stat(path)
	if err != nil {
		http.Error(w, "Cannot stat file", http.StatusInternalServerError)
		return
	}

	// Enterprise security: Check file size limit (prevent DoS)
	if info.Size() > h.config.MaxFileSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Set security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Add Content Security Policy for enhanced security
	if h.config.EnableSecurity {
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
	}

	// Set filename in Content-Disposition header with proper escaping
	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))

	// Set MIME type
	mimeType := fileutil.DetectMimeType(path)
	w.Header().Set("Content-Type", mimeType)

	// Set content length for better client handling
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	// Enterprise optimization: Use buffered copy for large files
	if info.Size() > 1<<20 { // 1MB threshold
		buf := make([]byte, 64<<10) // 64KB buffer for large files
		if _, err := io.CopyBuffer(w, file, buf); err != nil {
			// Error already written to response, just log
			return
		}
	} else {
		// Use standard copy for small files (already optimized)
		if _, err := io.Copy(w, file); err != nil {
			// Error already written to response, just log
			return
		}
	}
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
func (h *File) renderHTML(w http.ResponseWriter, path string, files []internal.FileInfo) {
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
	}{
		Path:   "/" + path,
		Parent: path != "",
		Files:  items,
		CSS:    template.CSS(templates.StylesCSS), // Convert to template.CSS to bypass XSS protection
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Use pre-compiled template for better performance
	if err := templates.DirectoryTemplate.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}
