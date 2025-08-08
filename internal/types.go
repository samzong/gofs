package internal

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"
)

type FileSystem interface {
	Open(name string) (io.ReadCloser, error)
	Stat(name string) (FileInfo, error)
	ReadDir(name string) ([]FileInfo, error)
	Create(name string) (io.WriteCloser, error)
	Mkdir(name string, perm os.FileMode) error
	Remove(name string) error
}

type FileInfo interface {
	Name() string
	Size() int64
	IsDir() bool
	ModTime() time.Time
}

type APIError struct {
	Details any    `json:"details,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}

func NewAPIError(code, message string) *APIError {
	return &APIError{
		Code:    code,
		Message: message,
		Status:  http.StatusInternalServerError,
	}
}

func (e *APIError) WithStatus(status int) *APIError {
	e.Status = status
	return e
}

func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

type MountInfo struct {
	Path     string
	Name     string
	Readonly bool
}

type contextKey string

const mountInfoKey contextKey = "mount_info"

func WithMountInfo(ctx context.Context, path, name string, readonly bool) context.Context {
	info := MountInfo{Path: path, Name: name, Readonly: readonly}
	return context.WithValue(ctx, mountInfoKey, info)
}

func GetMountInfo(ctx context.Context) (MountInfo, bool) {
	info, ok := ctx.Value(mountInfoKey).(MountInfo)
	return info, ok
}
