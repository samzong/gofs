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
	"strings"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/pkg/fileutil"
)

// File implements HTTP request handling for file system operations.
type File struct {
	fs internal.FileSystem
}

// NewFile creates a new file handler with the given file system.
func NewFile(fs internal.FileSystem) *File {
	return &File{fs: fs}
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
func (h *File) handleFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := h.fs.Open(path)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info for size and security checks
	info, err := h.fs.Stat(path)
	if err != nil {
		http.Error(w, "Cannot stat file", http.StatusInternalServerError)
		return
	}

	// Enterprise security: Check file size limit (prevent DoS)
	const maxFileSize = 100 << 20 // 100MB limit
	if info.Size() > maxFileSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Set security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")

	// Set filename in Content-Disposition header with proper escaping
	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"",
		strings.ReplaceAll(filename, "\"", "\\\"")))

	// Set MIME type
	mimeType := fileutil.DetectMimeType(path)
	w.Header().Set("Content-Type", mimeType)

	// Set content length for better client handling
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	// Enterprise optimization: Use buffered copy for large files
	if info.Size() > 1<<20 { // 1MB threshold
		buf := make([]byte, 64<<10) // 64KB buffer for large files
		io.CopyBuffer(w, file, buf)
	} else {
		// Use standard copy for small files (already optimized)
		io.Copy(w, file)
	}
}

// renderJSON renders the file listing as JSON for API consumers.
func (h *File) renderJSON(w http.ResponseWriter, path string, files []internal.FileInfo) {
	type FileItem struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"isDir"`
		ModTime string `json:"modTime"`
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

	response := map[string]interface{}{
		"path":  path,
		"files": items,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// renderHTML renders the file listing as HTML for browser viewing.
func (h *File) renderHTML(w http.ResponseWriter, path string, files []internal.FileInfo) {
	const htmlTemplate = `<!DOCTYPE html>
<html><head><title>{{.Path}}</title></head>
<body><h1>{{.Path}}</h1><ul>
{{if .Parent}}<li><a href="../">📁 ..</a></li>{{end}}
{{range .Files}}<li><a href="{{.Name}}{{if .IsDir}}/{{end}}">{{if .IsDir}}📁{{else}}📄{{end}} {{.Name}}</a>{{if not .IsDir}} ({{.Size}}){{end}}</li>{{end}}
</ul></body></html>`

	type FileItem struct {
		Name  string
		IsDir bool
		Size  string
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
		Parent bool
		Files  []FileItem
	}{
		Path:   "/" + path,
		Parent: path != "",
		Files:  items,
	}

	tmpl, err := template.New("files").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}
