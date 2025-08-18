package handler

import (
	"archive/zip"
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
	"strings"
	"sync"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/constants"
	"github.com/samzong/gofs/internal/handler/templates"
	"github.com/samzong/gofs/internal/middleware"
	"github.com/samzong/gofs/pkg/fileutil"
	"github.com/samzong/gofs/pkg/httprange"
	"github.com/samzong/gofs/pkg/zipstream"
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
	fs           internal.FileSystem
	config       *config.Config
	logger       *slog.Logger
	csrfTokens   *csrfStore
	zipSemaphore chan struct{}
}

func NewAdvancedFile(fs internal.FileSystem, cfg *config.Config) *AdvancedFile {
	logger := slog.With(
		slog.String("handler", "advanced"),
		slog.String("theme", "advanced"),
	)

	return &AdvancedFile{
		fs:           fs,
		config:       cfg,
		logger:       logger,
		csrfTokens:   newCSRFStore(),
		zipSemaphore: make(chan struct{}, 3),
	}
}

func (h *AdvancedFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var handler http.Handler = http.HandlerFunc(h.handleRequest)

	// Build middleware chain
	securityConfig := middleware.SecurityConfig{
		EnableSecurity: h.config.EnableSecurity,
		ContentSecurityPolicy: "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; font-src 'self'",
	}
	handler = middleware.SecurityHeaders(securityConfig)(handler)
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
		if !h.validateCSRFRequest(r) {
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
			return
		}
		h.handleCreateFolder(w, r)
	case "/api/zip":
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !h.validateCSRFRequest(r) {
			http.Error(w, "Invalid or missing CSRF token", http.StatusForbidden)
			return
		}
		h.handleZipDownload(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *AdvancedFile) handleGetCSRFToken(w http.ResponseWriter, r *http.Request) {
	token := h.csrfTokens.generateToken()
	response := map[string]string{"token": token}
	if err := middleware.WriteJSON(w, response); err != nil {
		h.logger.Warn("Failed to write CSRF token response",
			slog.String("error", err.Error()))
	}
}

func (h *AdvancedFile) validateCSRFRequest(r *http.Request) bool {
	token := r.Header.Get("X-CSRF-Token")
	if token == "" {
		token = r.FormValue("csrf_token")
	}

	origin := r.Header.Get("Origin")
	referer := r.Header.Get("Referer")

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
		middleware.WriteJSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	if r.Context().Err() != nil {
		middleware.WriteJSONError(w, "Upload timeout", http.StatusRequestTimeout)
		return
	}

	filename := fileutil.SafePath(header.Filename)
	if filename == "" {
		middleware.WriteJSONError(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	if err := h.saveUploadedFile(r.Context(), file, filename); err != nil {
		if r.Context().Err() != nil {
			middleware.WriteJSONError(w, "Upload timeout", http.StatusRequestTimeout)
			return
		}
		middleware.WriteJSONError(w, "Failed to save file", http.StatusInternalServerError)
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
	if err := middleware.WriteJSON(w, response); err != nil {
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
		middleware.WriteJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if r.Context().Err() != nil {
		middleware.WriteJSONError(w, "Request timeout", http.StatusRequestTimeout)
		return
	}

	folderName := fileutil.SafePath(req.Path)
	if folderName == "" {
		middleware.WriteJSONError(w, "Invalid folder name", http.StatusBadRequest)
		return
	}

	if err := h.fs.Mkdir(folderName, 0755); err != nil {
		middleware.WriteJSONError(w, "Failed to create folder", http.StatusInternalServerError)
		return
	}

	h.logger.Info("Folder created successfully",
		slog.String("folder", folderName))

	response := FolderResponse{
		Success: true,
		Folder:  folderName,
	}
	if err := middleware.WriteJSON(w, response); err != nil {
		h.logger.Warn("Failed to write JSON response for folder creation",
			slog.String("folder", folderName),
			slog.String("error", err.Error()))
	}
}

type ZipRequest struct {
	Paths []string `json:"paths"`
	Name  string   `json:"name"`
}

func (h *AdvancedFile) handleZipDownload(w http.ResponseWriter, r *http.Request) {
	select {
	case h.zipSemaphore <- struct{}{}:
		defer func() { <-h.zipSemaphore }()
	default:
		h.logger.Warn("Too many concurrent ZIP downloads")
		http.Error(w, "Too many concurrent downloads, please try again later", http.StatusTooManyRequests)
		return
	}

	var req ZipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		middleware.WriteJSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.Paths) == 0 {
		middleware.WriteJSONError(w, "No files selected", http.StatusBadRequest)
		return
	}

	var entries []zipstream.FileEntry
	var totalSize int64

	for _, p := range req.Paths {
		safePath := middleware.SafeRequestPath(p)
		if safePath == "" {
			continue
		}

		info, err := h.fs.Stat(safePath)
		if err != nil {
			h.logger.Debug("File not found for ZIP",
				slog.String("path", safePath),
				slog.String("error", err.Error()))
			continue
		}

		if info.IsDir() {
			dirSize, fileCount := h.calculateDirSize(safePath)
			totalSize += dirSize
			h.logger.Debug("Adding directory to ZIP",
				slog.String("path", safePath),
				slog.Int64("size", dirSize),
				slog.Int("files", fileCount))
		} else {
			totalSize += info.Size()
			entries = append(entries, zipstream.FileEntry{
				Path: safePath,
				Name: filepath.Base(safePath),
				Info: info,
			})
		}
	}

	if len(entries) == 0 && len(req.Paths) > 0 {
		for _, p := range req.Paths {
			safePath := middleware.SafeRequestPath(p)
			info, err := h.fs.Stat(safePath)
			if err == nil && info.IsDir() {
				h.collectDirFiles(safePath, safePath, &entries)
			}
		}
	}

	if len(entries) == 0 {
		middleware.WriteJSONError(w, "No valid files to download", http.StatusBadRequest)
		return
	}

	zipName := req.Name
	if zipName == "" {
		if len(entries) == 1 {
			zipName = strings.TrimSuffix(entries[0].Name, filepath.Ext(entries[0].Name)) + ".zip"
		} else {
			zipName = fmt.Sprintf("download_%d.zip", time.Now().Unix())
		}
	}
	if !strings.HasSuffix(zipName, ".zip") {
		zipName += ".zip"
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", zipName))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	h.logger.Info("Starting ZIP download",
		slog.String("filename", zipName),
		slog.Int("file_count", len(entries)),
		slog.Int64("total_size", totalSize))

	opts := zipstream.Options{
		CompressionLevel: zip.Store,
		MaxSize:          500 * 1024 * 1024,
		BufferSize:       32 * 1024,
	}

	zw := zipstream.NewWriter(w, opts)
	defer zw.Close()

	for _, entry := range entries {
		file, err := h.fs.Open(entry.Path)
		if err != nil {
			h.logger.Warn("Failed to open file for ZIP",
				slog.String("path", entry.Path),
				slog.String("error", err.Error()))
			continue
		}

		entry.Reader = file
		if err := zw.AddFile(entry); err != nil {
			if closeErr := file.Close(); closeErr != nil {
				h.logger.Warn("Failed to close file after ZIP error",
					slog.String("path", entry.Path),
					slog.String("close_error", closeErr.Error()))
			}
			h.logger.Warn("Failed to add file to ZIP",
				slog.String("path", entry.Path),
				slog.String("error", err.Error()))
			continue
		}
		if err := file.Close(); err != nil {
			h.logger.Warn("Failed to close file after ZIP processing",
				slog.String("path", entry.Path),
				slog.String("error", err.Error()))
		}
	}

	h.logger.Info("ZIP download completed",
		slog.String("filename", zipName),
		slog.Int("files_processed", len(entries)))
}

func (h *AdvancedFile) calculateDirSize(dirPath string) (int64, int) {
	var totalSize int64
	var fileCount int

	files, err := h.fs.ReadDir(dirPath)
	if err != nil {
		return 0, 0
	}

	for _, file := range files {
		if !h.config.ShowHidden && strings.HasPrefix(file.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(dirPath, file.Name())
		if file.IsDir() {
			size, count := h.calculateDirSize(fullPath)
			totalSize += size
			fileCount += count
		} else {
			totalSize += file.Size()
			fileCount++
		}
	}

	return totalSize, fileCount
}

func (h *AdvancedFile) collectDirFiles(basePath, currentPath string, entries *[]zipstream.FileEntry) {
	files, err := h.fs.ReadDir(currentPath)
	if err != nil {
		h.logger.Warn("Failed to read directory for ZIP",
			slog.String("path", currentPath),
			slog.String("error", err.Error()))
		return
	}

	for _, file := range files {
		if !h.config.ShowHidden && strings.HasPrefix(file.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(currentPath, file.Name())

		relPath, err := filepath.Rel(basePath, fullPath)
		if err != nil {
			relPath = file.Name()
		}

		if file.IsDir() {
			h.collectDirFiles(basePath, fullPath, entries)
		} else {
			*entries = append(*entries, zipstream.FileEntry{
				Path: fullPath,
				Name: relPath,
				Info: file,
			})
		}
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

	safePath := middleware.SafeRequestPath(path)

	// Check for path traversal attempts - SafeRequestPath returns empty string for dangerous paths
	if safePath == "" && path != "/" {
		http.Error(w, "Bad Request: Invalid path", http.StatusBadRequest)
		return
	}

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
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", constants.StaticAssetCacheMaxAge))
	w.Header().Set("ETag", fmt.Sprintf(`"%x"`, templates.AdvancedCSS))

	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == fmt.Sprintf(`"%x"`, templates.AdvancedCSS) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	_, _ = w.Write([]byte(templates.AdvancedCSS))
}

func (h *AdvancedFile) serveStaticJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", constants.StaticAssetCacheMaxAge))
	w.Header().Set("ETag", fmt.Sprintf(`"%x"`, templates.AdvancedJS))

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

	rangeHeader := r.Header.Get("Range")
	rng, err := httprange.ParseRange(rangeHeader, info.Size())
	if err != nil {
		if err == httprange.ErrUnsatisfiableRange {
			httprange.WriteRangeNotSatisfiable(w, info.Size())
			return
		}
		rng = nil
	}

	mimeType := fileutil.DetectMimeType(path)
	filename := filepath.Base(path)

	seeker, seekable := file.(io.ReadSeeker)
	if !seekable && rng != nil {
		h.logger.Debug("File doesn't support seeking, serving full content",
			slog.String("path", path),
			slog.String("component", "advanced_file_handler"),
		)
		rng = nil
	}

	if rng != nil {
		h.logger.Debug("Serving partial content",
			slog.String("path", path),
			slog.Int64("start", rng.Start),
			slog.Int64("end", rng.End),
			slog.Int64("length", rng.Length),
			slog.String("component", "advanced_file_handler"),
		)

		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))

		if err := httprange.ServeContent(w, seeker, rng, info.Size(), mimeType); err != nil {
			h.logger.Warn("Error serving partial content",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("component", "advanced_file_handler"),
			)
		}
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))

		if err := httprange.ServeFullContent(w, file, info.Size(), mimeType); err != nil {
			h.logger.Warn("Error serving full content",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("component", "advanced_file_handler"),
			)
		}
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

	if err := middleware.WriteJSON(w, response); err != nil {
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
