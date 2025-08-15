package handler

import (
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/samzong/gofs/internal/middleware"
	"github.com/samzong/gofs/pkg/fileutil"
	"github.com/samzong/gofs/pkg/httprange"
)

type File struct {
	fs     internal.FileSystem
	config *config.Config
	logger *slog.Logger
}

func NewFile(fs internal.FileSystem, cfg *config.Config, logger *slog.Logger) *File {
	return &File{
		fs:     fs,
		config: cfg,
		logger: logger,
	}
}

func (h *File) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Apply security headers
	securityConfig := middleware.SecurityConfig{
		EnableSecurity: h.config.EnableSecurity,
	}
	middleware.SecurityHeaders(securityConfig)(http.HandlerFunc(h.handleGet)).ServeHTTP(w, r)
}

func (h *File) handleGet(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "" {
		path = "/"
	}

	safePath := middleware.SafeRequestPath(path)

	info, err := h.fs.Stat(safePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		h.handleDirectory(w, r, safePath)
		return
	}

	h.handleFile(w, r, safePath)
}

func (h *File) handleDirectory(w http.ResponseWriter, r *http.Request, path string) {
	files, err := h.fs.ReadDir(path)
	if err != nil {
		http.Error(w, "Cannot read directory", http.StatusInternalServerError)
		return
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir() && !files[j].IsDir() {
			return true
		}
		if !files[i].IsDir() && files[j].IsDir() {
			return false
		}
		return strings.ToLower(files[i].Name()) < strings.ToLower(files[j].Name())
	})

	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		h.renderJSON(w, path, files)
		return
	}

	h.renderHTML(w, path, files, h.config.Theme)
}

func (h *File) handleFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := h.fs.Open(path)
	if err != nil {
		http.Error(w, "Cannot open file", http.StatusInternalServerError)
		return
	}
	defer h.closeFile(file, path)

	info, err := h.fs.Stat(path)
	if err != nil {
		http.Error(w, "Cannot stat file", http.StatusInternalServerError)
		return
	}

	if info.Size() > h.config.MaxFileSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	// Generate ETag based on content hash if file supports seeking
	var etag string
	if seeker, ok := file.(io.ReadSeeker); ok {
		var err error
		etag, err = h.generateContentETag(seeker)
		if err != nil {
			h.logger.Warn("Failed to generate content-based ETag",
				slog.String("path", path),
				slog.String("error", err.Error()),
			)
			// Use fallback ETag based on file metadata
			etag = fmt.Sprintf(`"gofs-%x-%x-%x"`,
				[]byte(path),
				info.Size(),
				info.ModTime().Unix())
		}
	} else {
		// Use fallback ETag for non-seekable files
		etag = fmt.Sprintf(`"gofs-%x-%x-%x"`,
			[]byte(path),
			info.Size(),
			info.ModTime().Unix())
	}

	// Check If-None-Match header for conditional requests
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
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

	seeker, seekable := file.(io.ReadSeeker)
	if !seekable && rng != nil {
		h.logger.Debug("File doesn't support seeking, serving full content",
			slog.String("path", path),
			slog.String("component", "file_handler"),
		)
		rng = nil
	}

	if rng != nil {
		h.logger.Debug("Serving partial content",
			slog.String("path", path),
			slog.Int64("start", rng.Start),
			slog.Int64("end", rng.End),
			slog.Int64("length", rng.Length),
			slog.String("component", "file_handler"),
		)

		filename := filepath.Base(path)
		w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
		w.Header().Set("ETag", etag)

		if err := httprange.ServeContent(w, seeker, rng, info.Size(), mimeType); err != nil {
			h.logger.Warn("Error serving partial content",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("component", "file_handler"),
			)
		}
	} else {
		h.setFileHeaders(w, path, info, etag)
		if err := httprange.ServeFullContent(w, file, info.Size(), mimeType); err != nil {
			h.logger.Warn("Error serving full content",
				slog.String("path", path),
				slog.String("error", err.Error()),
				slog.String("component", "file_handler"),
			)
		}
	}
}

func (h *File) closeFile(file io.ReadCloser, path string) {
	if err := file.Close(); err != nil {
		h.logger.Warn("File close failed",
			slog.String("path", path),
			slog.String("error", err.Error()),
			slog.String("component", "file_handler"),
		)
	}
}

func (h *File) setFileHeaders(w http.ResponseWriter, path string, info internal.FileInfo, etag string) {
	filename := filepath.Base(path)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	w.Header().Set("Content-Type", fileutil.DetectMimeType(path))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("ETag", etag)
}

func (h *File) generateContentETag(file io.ReadSeeker) (string, error) {
	// Save current position
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return "", err
	}

	// Seek to beginning
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	// Calculate SHA-256 hash
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	// Restore original position
	if _, err := file.Seek(currentPos, io.SeekStart); err != nil {
		return "", err
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	return fmt.Sprintf(`"%s"`, hash), nil
}

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
		CSS    template.CSS
		Theme  string
	}{
		Path:   "/" + path,
		Parent: path != "",
		Files:  items,
		CSS:    template.CSS(templates.GetThemeCSS(theme)),
		Theme:  theme,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := templates.DirectoryTemplate.Execute(w, data); err != nil {
		http.Error(w, "Template execution error", http.StatusInternalServerError)
		return
	}
}
