// Package internal provides core interfaces and types for the gofs file server.
package internal

import (
	"io"
	"net/http"
	"time"
)

// FileSystem defines the interface for file system operations.
// It provides abstraction over different storage backends.
type FileSystem interface {
	// Open opens the named file for reading.
	Open(name string) (io.ReadCloser, error)

	// Stat returns file information for the named file.
	Stat(name string) (FileInfo, error)

	// ReadDir reads the directory and returns a list of directory entries.
	ReadDir(name string) ([]FileInfo, error)
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
	Code    string `json:"code"`              // Error code for programmatic handling
	Message string `json:"message"`           // Human-readable error message
	Details any    `json:"details,omitempty"` // Additional error details
	Status  int    `json:"-"`                 // HTTP status code (not serialized)
}

// Error implements the error interface for APIError.
func (e *APIError) Error() string {
	return e.Message
}

// NewAPIError creates a new API error with the given code and message.
func NewAPIError(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Status:  http.StatusInternalServerError, // Default status
	}
}

// NewAPIErrorWithDetails creates a new API error with additional details.
func NewAPIErrorWithDetails(code, message string, details any) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Details: details,
		Status:  http.StatusInternalServerError, // Default status
	}
}

// NewAPIErrorWithStatus creates a new API error with a specific HTTP status.
func NewAPIErrorWithStatus(code, message string, status int) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}
