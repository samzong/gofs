package handler

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/constants"
	"github.com/samzong/gofs/internal/handler/templates"
	"github.com/samzong/gofs/pkg/fileutil"
)

type UploadResponse struct {
	Success bool   `json:"success"`
	File    string `json:"file"`
	Size    int64  `json:"size"`
}

type FolderResponse struct {
	Success bool   `json:"success"`
	Folder  string `json:"folder"`
}

type DirectoryResponse struct {
	Path  string         `json:"path"`
	Files []FileItemJSON `json:"files"`
	Count int            `json:"count"`
}

type FileItemJSON struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"isDir"`
	ModTime time.Time `json:"modTime"`
}

type Middleware func(http.Handler) http.Handler

type csrfStore struct {
	mu     sync.RWMutex
	tokens map[string]time.Time
}

func newCSRFStore() *csrfStore {
	store := &csrfStore{
		tokens: make(map[string]time.Time),
	}
	go store.cleanup()
	return store
}

func (s *csrfStore) generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback for rare crypto/rand failures
		b = []byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix()))
	}
	token := base64.URLEncoding.EncodeToString(b)

	s.mu.Lock()
	s.tokens[token] = time.Now().Add(constants.CSRFTokenExpiry)
	s.mu.Unlock()

	return token
}

func (s *csrfStore) validateToken(token string) bool {
	if token == "" {
		return false
	}

	s.mu.RLock()
	expiry, exists := s.tokens[token]
	s.mu.RUnlock()

	if !exists || time.Now().After(expiry) {
		return false
	}

	s.mu.Lock()
	delete(s.tokens, token)
	s.mu.Unlock()

	return true
}

func (s *csrfStore) cleanup() {
	ticker := time.NewTicker(constants.CSRFCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for token, expiry := range s.tokens {
			if now.After(expiry) {
				delete(s.tokens, token)
			}
		}
		s.mu.Unlock()
	}
}

type RequestContext struct {
	StartTime  time.Time
	RequestID  string
	UserAgent  string
	RemoteAddr string
	Path       string
}

type AdvancedFile struct {
	fs         internal.FileSystem
	config     *config.Config
	logger     *slog.Logger
	csrfTokens *csrfStore
}

func NewAdvancedFile(fs internal.FileSystem, cfg *config.Config) *AdvancedFile {
	logger := slog.With(
		slog.String("handler", "advanced"),
		slog.String("theme", "advanced"),
	)

	return &AdvancedFile{
		fs:         fs,
		config:     cfg,
		logger:     logger,
		csrfTokens: newCSRFStore(),
	}
}

func (h *AdvancedFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var handler http.Handler = http.HandlerFunc(h.handleRequest)
	handler = h.loggingMiddleware(handler)
	handler = h.timeoutMiddleware(handler)
	handler = h.corsMiddleware(handler)
	handler.ServeHTTP(w, r)
}

func (h *AdvancedFile) handleRequest(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/static/theme.css":
		h.serveStaticCSS(w, r)
	case r.URL.Path == "/static/theme.js":
		h.serveStaticJS(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/"):
		h.handleAPI(w, r)
	default:
		h.handleFileRequest(w, r)
	}
}

func (h *AdvancedFile) handleAPI(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/csrf":
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h.handleGetCSRFToken(w, r)
	case "/api/upload":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Validate CSRF token for upload
		if !h.validateCSRFRequest(r) {
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
			return
		}
		h.handleUpload(w, r)
	case "/api/folder":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Validate CSRF token for folder creation
		if !h.validateCSRFRequest(r) {
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
			return
		}
		h.handleCreateFolder(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *AdvancedFile) handleGetCSRFToken(w http.ResponseWriter, r *http.Request) {
	token := h.csrfTokens.generateToken()
	response := map[string]string{"token": token}
	if err := writeJSON(w, response); err != nil {
		h.logger.Warn("Failed to write CSRF token response",
			slog.String("error", err.Error()))
	}
}

func (h *AdvancedFile) validateCSRFRequest(r *http.Request) bool {
	// Check header first
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		// Fallback to form value for multipart uploads
		token = r.FormValue("csrf_token")
	}

	// Additional security: Check Origin/Referer headers
	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

	// If Origin or Referer is present, verify it matches our host
	if origin != "" || referer != "" {
		host := r.Host
		expectedOrigin := fmt.Sprintf("http://%s", host)
		expectedOriginHTTPS := fmt.Sprintf("https://%s", host)

		if origin != "" && origin != expectedOrigin && origin != expectedOriginHTTPS {
			h.logger.Warn("CSRF: Origin mismatch",
				slog.String("origin", origin),
				slog.String("expected", expectedOrigin))
			return false
		}

		if referer != "" && !strings.HasPrefix(referer, expectedOrigin) && !strings.HasPrefix(referer, expectedOriginHTTPS) {
			h.logger.Warn("CSRF: Referer mismatch",
				slog.String("referer", referer),
				slog.String("expected", expectedOrigin))
			return false
		}
	}

	return h.csrfTokens.validateToken(token)
}

func (h *AdvancedFile) handleUpload(w http.ResponseWriter, r *http.Request) {
	file, header, err := h.parseUploadRequest(r)
	if err != nil {
		h.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if r.Context().Err() != nil {
		h.writeError(w, "Upload timeout", http.StatusRequestTimeout)
		return
	}

	filename := fileutil.SafePath(header.Filename)
	if filename == "" {
		h.writeError(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if err := h.saveUploadedFile(r.Context(), file, filename); err != nil {
		if r.Context().Err() != nil {
			h.writeError(w, "Upload timeout", http.StatusRequestTimeout)
			return
		}
		h.writeError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	h.logger.Info("File uploaded successfully",
		slog.String("filename", filename),
		slog.Int64("size", header.Size))

	response := UploadResponse{
		Success: true,
		File:    filename,
		Size:    header.Size,
	}
	if err := writeJSON(w, response); err != nil {
		h.logger.Warn("Failed to write JSON response for upload",
			slog.String("filename", filename),
			slog.String("error", err.Error()))
	}
}

func (h *AdvancedFile) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if r.Context().Err() != nil {
		h.writeError(w, "Request timeout", http.StatusRequestTimeout)
		return
	}

	folderName := fileutil.SafePath(req.Path)
	if folderName == "" {
		h.writeError(w, "Invalid folder name", http.StatusBadRequest)
		return
	}

	if err := h.fs.Mkdir(folderName, 0755); err != nil {
		h.writeError(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Folder created successfully",
		slog.String("folder", folderName))

	response := FolderResponse{
		Success: true,
		Folder:  folderName,
	}
	if err := writeJSON(w, response); err != nil {
		h.logger.Warn("Failed to write JSON response for folder creation",
			slog.String("folder", folderName),
			slog.String("error", err.Error()))
	}
}

func (h *AdvancedFile) handleFileRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	safePath := fileutil.SafePath(strings.TrimPrefix(path, "/"))

	info, err := h.fs.Stat(safePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		h.renderAdvancedDirectory(w, r, safePath)
		return
	}

	h.serveFile(w, r, safePath)
}

func (h *AdvancedFile) serveStaticCSS(w http.ResponseWriter, r *http.Request) {
	// Set cache headers for CSS
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", constants.StaticAssetCacheMaxAge))
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

func (h *AdvancedFile) serveStaticJS(w http.ResponseWriter, r *http.Request) {
	// Set cache headers for JS
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", constants.StaticAssetCacheMaxAge))
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

func (h *AdvancedFile) renderAdvancedDirectory(w http.ResponseWriter, r *http.Request, dirPath string) {
	files, err := h.fs.ReadDir(dirPath)
	if err != nil {
		http.Error(w, "Cannot read directory", http.StatusInternalServerError)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		h.renderJSON(w, dirPath, files)
		return
	}

	type FileItem struct {
		Name          string
		IsDir         bool
		Size          int64
		FormattedSize string
		FormattedTime string
	}

	type BreadcrumbItem struct {
		Name string
		Path string
	}

	var breadcrumbs []BreadcrumbItem
	if dirPath != "" && dirPath != "." {
		parts := strings.Split(strings.Trim(dirPath, "/"), "/")
		currentPath := ""
		for _, part := range parts {
			if part == "" {
				continue
			}
			// Use path.Join for proper URL path construction (always uses forward slashes)
			currentPath = path.Join(currentPath, part)
			breadcrumbs = append(breadcrumbs, BreadcrumbItem{
				Name: part,
				Path: "/" + currentPath,
			})
		}
	}

	var items []FileItem
	for _, file := range files {
		if !h.config.ShowHidden && strings.HasPrefix(file.Name(), ".") {
			continue
		}

		formattedSize := ""
		if !file.IsDir() {
			formattedSize = fileutil.FormatSize(file.Size())
		}

		items = append(items, FileItem{
			Name:          file.Name(),
			IsDir:         file.IsDir(),
			Size:          file.Size(),
			FormattedSize: formattedSize,
			FormattedTime: file.ModTime().Format("Jan 02, 2006"),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	data := struct {
		Path        string
		Parent      bool
		Files       []FileItem
		FileCount   int
		Breadcrumbs []BreadcrumbItem
	}{
		Path:        "/" + dirPath,
		Parent:      dirPath != "" && dirPath != ".",
		Files:       items,
		FileCount:   len(items),
		Breadcrumbs: breadcrumbs,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.AdvancedTemplate.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}

func (h *AdvancedFile) serveFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := h.fs.Open(path)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	info, err := h.fs.Stat(path)
	if err != nil {
		http.Error(w, "Cannot stat file", http.StatusInternalServerError)
		return
	}

	if info.Size() > h.config.MaxFileSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data:; font-src 'self'")

	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	w.Header().Set("Content-Type", fileutil.DetectMimeType(path))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	if _, err := io.Copy(w, file); err != nil {
		h.logger.Warn("Failed to serve file content",
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
}

func (h *AdvancedFile) renderJSON(w http.ResponseWriter, path string, files []internal.FileInfo) {
	var items []FileItemJSON
	for _, file := range files {
		if !h.config.ShowHidden && strings.HasPrefix(file.Name(), ".") {
			continue
		}
		items = append(items, FileItemJSON{
			Name:    file.Name(),
			Size:    file.Size(),
			IsDir:   file.IsDir(),
			ModTime: file.ModTime(),
		})
	}

	response := DirectoryResponse{
		Path:  path,
		Files: items,
		Count: len(items),
	}

	if err := writeJSON(w, response); err != nil {
		h.logger.Warn("Failed to encode JSON for directory listing",
			slog.String("path", path),
			slog.String("error", err.Error()))
	}
}

func (h *AdvancedFile) parseUploadRequest(r *http.Request) (multipart.File, *multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		return nil, nil, err
	}
	return r.FormFile("file")
}

func (h *AdvancedFile) saveUploadedFile(ctx context.Context, src multipart.File, filename string) error {
	dst, err := h.fs.Create(filename)
	if err != nil {
		return fmt.Errorf("creating file %q: %w", filename, err)
	}
	defer dst.Close()

	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(dst, src)
		done <- err
	}()

	select {
	case <-ctx.Done():
		_ = h.fs.Remove(filename)
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("copying file data: %w", err)
		}
		return nil
	}
}

func writeJSON[T any](w http.ResponseWriter, data T) error {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return fmt.Errorf("encoding JSON response: %w", err)
	}
	return nil
}

func (h *AdvancedFile) writeError(w http.ResponseWriter, message string, status int) {
	http.Error(w, message, status)
}

func (h *AdvancedFile) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func (h *AdvancedFile) timeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var timeout time.Duration

		switch {
		case strings.HasPrefix(r.URL.Path, "/api/upload"):
			timeout = constants.UploadTimeout
		case strings.HasPrefix(r.URL.Path, "/api/"):
			timeout = constants.DirectoryTimeout
		default:
			timeout = constants.FileServeTimeout
		}

		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func (h *AdvancedFile) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now()

		reqCtx := RequestContext{
			StartTime:  startTime,
			UserAgent:  r.UserAgent(),
			RemoteAddr: r.RemoteAddr,
			Path:       r.URL.Path,
		}

		h.logger.Info("Request started",
			slog.String("method", r.Method),
			slog.String("path", reqCtx.Path),
			slog.String("remote_addr", reqCtx.RemoteAddr),
			slog.String("user_agent", reqCtx.UserAgent))

		wrappedWriter := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(startTime)
		h.logger.Info("Request completed",
			slog.String("method", r.Method),
			slog.String("path", reqCtx.Path),
			slog.Int("status_code", wrappedWriter.statusCode),
			slog.Duration("duration", duration),
			slog.String("remote_addr", reqCtx.RemoteAddr))
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
