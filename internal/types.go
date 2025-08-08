// Package internal provides core interfaces and types for the gofs file server.
package internal

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"
)

// FileSystem defines the interface for file system operations.
// It provides abstraction over different storage backends.
type FileSystem interface {
	// Read operations

	// Open opens the named file for reading.
	Open(name string) (io.ReadCloser, error)

	// Stat returns file information for the named file.
	Stat(name string) (FileInfo, error)

	// ReadDir reads the directory and returns a list of directory entries.
	ReadDir(name string) ([]FileInfo, error)

	// Write operations

	// Create creates or truncates the named file for writing.
	Create(name string) (io.WriteCloser, error)

	// Mkdir creates a directory with the specified name and permission.
	Mkdir(name string, perm os.FileMode) error

	// Remove removes the named file or empty directory.
	Remove(name string) error
}

// FileInfo provides information about a file or directory.
type FileInfo interface {
	// Name returns the base name of the file.
	Name() string

	// Size returns the length in bytes for regular files.
	Size() int64

	// IsDir reports whether the file is a directory.
	IsDir() bool

	// ModTime returns the modification time.
	ModTime() time.Time
}

// APIError represents a structured error response for API operations.
type APIError struct {
	Details any    `json:"details,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new API error with the given code and message.
// Use method chaining to set optional fields.
func NewAPIError(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Status:  http.StatusInternalServerError, // Default status
	}
}

// WithStatus sets the HTTP status code for the error.
func (e *APIError) WithStatus(status int) *APIError {
	e.Status = status
	return e
}

// WithDetails adds additional details to the error.
func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

// MountInfo holds mount information for request context
type MountInfo struct {
	Path     string
	Name     string
	Readonly bool
}

type contextKey string

const mountInfoKey contextKey = "mount_info"

// WithMountInfo adds mount information to the context
func WithMountInfo(ctx context.Context, path, name string, readonly bool) context.Context {
	info := MountInfo{Path: path, Name: name, Readonly: readonly}
	return context.WithValue(ctx, mountInfoKey, info)
}

// GetMountInfo retrieves mount information from the context
func GetMountInfo(ctx context.Context) (MountInfo, bool) {
	info, ok := ctx.Value(mountInfoKey).(MountInfo)
	return info, ok
}
