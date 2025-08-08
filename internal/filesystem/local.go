// Package filesystem provides file system abstractions for the gofs server.
package filesystem

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/pkg/fileutil"
)

// Local implements the FileSystem interface for local file system access.
type Local struct {
	root       string
	showHidden bool
}

// NewLocal creates a new local file system instance.
func NewLocal(root string, showHidden bool) *Local {
	return &Local{
		root:       filepath.Clean(root),
		showHidden: showHidden,
	}
}

// Root returns the root directory being served.
func (fs *Local) Root() string {
	return fs.root
}

// Open opens the file at the given path for reading.
func (fs *Local) Open(name string) (io.ReadCloser, error) {
	fullPath := fs.getFullPath(name)
	if fullPath == "" {
		return nil, &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Invalid file path",
			Status:  http.StatusBadRequest,
		}
	}

	// Verify symlink safety
	if err := fs.verifySymlinkSafety(fullPath); err != nil {
		return nil, err
	}

	// #nosec G304 - path is validated by fileutil.SafePath
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, &internal.APIError{
			Code:    "FILE_ACCESS_ERROR",
			Message: "Unable to access requested file",
			Status:  http.StatusNotFound,
		}
	}
	return file, nil
}

// Stat returns file information for the given path.
func (fs *Local) Stat(name string) (internal.FileInfo, error) {
	fullPath := fs.getFullPath(name)
	if fullPath == "" {
		return nil, &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Invalid file path",
			Status:  http.StatusBadRequest,
		}
	}

	// Verify symlink safety
	if err := fs.verifySymlinkSafety(fullPath); err != nil {
		return nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, &internal.APIError{
			Code:    "FILE_STAT_ERROR",
			Message: "Unable to get file information",
			Status:  http.StatusNotFound,
		}
	}

	return &localFileInfo{FileInfo: info}, nil
}

// ReadDir reads the directory and returns a list of directory entries.
func (fs *Local) ReadDir(name string) ([]internal.FileInfo, error) {
	fullPath := fs.getFullPath(name)
	if fullPath == "" {
		return nil, &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Invalid directory path",
			Status:  http.StatusBadRequest,
		}
	}

	// Verify symlink safety
	if err := fs.verifySymlinkSafety(fullPath); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, &internal.APIError{
			Code:    "DIRECTORY_READ_ERROR",
			Message: "Unable to read directory contents",
			Status:  http.StatusForbidden,
		}
	}

	result := make([]internal.FileInfo, 0, len(entries))
	for _, entry := range entries {
		// Filter hidden files if showHidden is false
		if !fs.showHidden && isHidden(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}
		result = append(result, &localFileInfo{FileInfo: info})
	}

	return result, nil
}

// Create creates or truncates the named file for writing.
// The returned io.WriteCloser must be closed after use.
func (fs *Local) Create(name string) (io.WriteCloser, error) {
	path := fs.getFullPath(name)
	if path == "" {
		return nil, fmt.Errorf("invalid path: %s", name)
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating file %q: %w", path, err)
	}
	return file, nil
}

// Mkdir creates a directory with the specified name and permission.
func (fs *Local) Mkdir(name string, perm os.FileMode) error {
	path := fs.getFullPath(name)
	if path == "" {
		return fmt.Errorf("invalid path: %s", name)
	}

	if err := os.Mkdir(path, perm); err != nil {
		return fmt.Errorf("creating directory %q: %w", path, err)
	}
	return nil
}

// Remove removes the named file or empty directory.
func (fs *Local) Remove(name string) error {
	path := fs.getFullPath(name)
	if path == "" {
		return fmt.Errorf("invalid path: %s", name)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing %q: %w", path, err)
	}
	return nil
}

// getFullPath converts a request path to a full filesystem path.
// It uses fileutil.SafePath for validation and returns empty string if invalid.
func (fs *Local) getFullPath(name string) string {
	if name == "" {
		name = "."
	}

	// Use fileutil.SafePath for validation
	safeName := fileutil.SafePath(name)
	if safeName == "" {
		return ""
	}

	// Build full path
	fullPath := filepath.Join(fs.root, safeName)

	// Final safety check: ensure path is within root
	if !strings.HasPrefix(fullPath, fs.root) {
		return ""
	}

	return fullPath
}

// verifySymlinkSafety checks if a symlink points outside the root directory.
func (fs *Local) verifySymlinkSafety(fullPath string) error {
	info, err := os.Lstat(fullPath)
	if err != nil {
		// File doesn't exist yet, which is fine for operations like Create
		if os.IsNotExist(err) {
			return nil
		}
		return &internal.APIError{
			Code:    "FILE_ACCESS_ERROR",
			Message: "Unable to check file status",
			Status:  http.StatusInternalServerError,
		}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		// Resolve the symlink and ensure it points within the root
		resolved, err := filepath.EvalSymlinks(fullPath)
		if err != nil {
			return &internal.APIError{
				Code:    "SYMLINK_ERROR",
				Message: "Unable to resolve symlink",
				Status:  http.StatusForbidden,
			}
		}

		// Clean and check if resolved path is within root
		resolved = filepath.Clean(resolved)
		if !strings.HasPrefix(resolved, fs.root) {
			return &internal.APIError{
				Code:    "SYMLINK_ATTACK",
				Message: "Symlink points outside root directory",
				Status:  http.StatusForbidden,
			}
		}
	}

	return nil
}

// isHidden checks if a file or directory is hidden.
func isHidden(name string) bool {
	return fileutil.IsHidden(name)
}

// localFileInfo implements internal.FileInfo for os.FileInfo.
type localFileInfo struct {
	os.FileInfo
}

// Name returns the base name of the file.
func (fi *localFileInfo) Name() string {
	return fi.FileInfo.Name()
}

// Size returns the length in bytes for regular files.
func (fi *localFileInfo) Size() int64 {
	return fi.FileInfo.Size()
}

// IsDir reports whether the file is a directory.
func (fi *localFileInfo) IsDir() bool {
	return fi.FileInfo.IsDir()
}

// ModTime returns the modification time.
func (fi *localFileInfo) ModTime() time.Time {
	return fi.FileInfo.ModTime()
}
