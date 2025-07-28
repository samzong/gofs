// Package filesystem provides file system abstractions for the gofs server.
package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/samzong/gofs/internal"
)

// Local implements the FileSystem interface for local file system access.
type Local struct {
	root string
}

// NewLocal creates a new local file system instance.
func NewLocal(root string) *Local {
	return &Local{
		root: filepath.Clean(root),
	}
}

// Root returns the root directory being served.
func (fs *Local) Root() string {
	return fs.root
}

// Open opens the file at the given path for reading.
func (fs *Local) Open(name string) (io.ReadCloser, error) {
	fullPath, err := fs.safePath(name)
	if err != nil {
		return nil, err
	}
	// #nosec G304 - path is validated by safePath function
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", fullPath, err)
	}
	return file, nil
}

// Stat returns file information for the given path.
func (fs *Local) Stat(name string) (internal.FileInfo, error) {
	fullPath, err := fs.safePath(name)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %q: %w", fullPath, err)
	}

	return &localFileInfo{FileInfo: info}, nil
}

// ReadDir reads the directory and returns a list of directory entries.
func (fs *Local) ReadDir(name string) ([]internal.FileInfo, error) {
	fullPath, err := fs.safePath(name)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", fullPath, err)
	}

	result := make([]internal.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}
		result = append(result, &localFileInfo{FileInfo: info})
	}

	return result, nil
}

// safePath ensures the path is safe and prevents directory traversal attacks.
func (fs *Local) safePath(name string) (string, error) {
	if name == "" {
		name = "."
	}

	// Clean the path
	name = filepath.Clean("/" + name)

	// Remove leading slash
	name = strings.TrimPrefix(name, "/")

	// Build full path
	fullPath := filepath.Join(fs.root, name)

	// Ensure path is within root directory
	if !strings.HasPrefix(fullPath, fs.root) {
		return "", &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Path outside of root directory",
		}
	}

	return fullPath, nil
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
