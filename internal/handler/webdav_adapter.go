package handler

import (
	"context"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"github.com/samzong/gofs/internal"
	"golang.org/x/net/webdav"
)

// webDAVAdapter adapts our FileSystem interface to webdav.FileSystem
type webDAVAdapter struct {
	fs internal.FileSystem
}

// NewWebDAVAdapter creates a new WebDAV file system adapter
func NewWebDAVAdapter(fs internal.FileSystem) webdav.FileSystem {
	return &webDAVAdapter{fs: fs}
}

// Mkdir implements webdav.FileSystem (read-only, returns error)
func (w *webDAVAdapter) Mkdir(_ context.Context, _ string, _ os.FileMode) error {
	return webdav.ErrForbidden
}

// OpenFile implements webdav.FileSystem
func (w *webDAVAdapter) OpenFile(_ context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	// Only allow read operations
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0 {
		return nil, webdav.ErrForbidden
	}

	// Clean and validate path
	cleanPath := path.Clean(name)
	if cleanPath == "/" {
		cleanPath = "."
	} else {
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}

	// Get file info first
	info, err := w.fs.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	// If it's a directory, return a directory file
	if info.IsDir() {
		return &webDAVDir{
			adapter: w,
			path:    cleanPath,
			info:    info,
		}, nil
	}

	// Open regular file for reading
	reader, err := w.fs.Open(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	return &webDAVFile{
		ReadCloser: reader,
		info:       info,
		path:       cleanPath,
	}, nil
}

// RemoveAll implements webdav.FileSystem (read-only, returns error)
func (w *webDAVAdapter) RemoveAll(_ context.Context, _ string) error {
	return webdav.ErrForbidden
}

// Rename implements webdav.FileSystem (read-only, returns error)
func (w *webDAVAdapter) Rename(_ context.Context, _, _ string) error {
	return webdav.ErrForbidden
}

// Stat implements webdav.FileSystem
func (w *webDAVAdapter) Stat(_ context.Context, name string) (os.FileInfo, error) {
	// Clean and validate path
	cleanPath := path.Clean(name)
	if cleanPath == "/" {
		cleanPath = "."
	} else {
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}

	info, err := w.fs.Stat(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	return &webDAVFileInfo{FileInfo: info}, nil
}

// webDAVFile wraps a file for WebDAV access
type webDAVFile struct {
	io.ReadCloser
	info internal.FileInfo
	path string
}

// Write implements webdav.File (read-only, returns error)
func (f *webDAVFile) Write([]byte) (int, error) {
	return 0, webdav.ErrForbidden
}

// Seek implements webdav.File
func (f *webDAVFile) Seek(_ int64, _ int) (int64, error) {
	// Most read operations don't need seek, return error
	return 0, errors.New("seek not supported")
}

// Readdir implements webdav.File (returns error for regular files)
func (f *webDAVFile) Readdir(_ int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

// Stat implements webdav.File
func (f *webDAVFile) Stat() (os.FileInfo, error) {
	return &webDAVFileInfo{FileInfo: f.info}, nil
}

// webDAVDir represents a directory for WebDAV access
type webDAVDir struct {
	adapter *webDAVAdapter
	path    string
	info    internal.FileInfo
	entries []os.FileInfo
	pos     int
}

// Close implements webdav.File
func (d *webDAVDir) Close() error {
	return nil
}

// Read implements webdav.File (returns error for directories)
func (d *webDAVDir) Read([]byte) (int, error) {
	return 0, errors.New("is a directory")
}

// Write implements webdav.File (read-only, returns error)
func (d *webDAVDir) Write([]byte) (int, error) {
	return 0, webdav.ErrForbidden
}

// Seek implements webdav.File
func (d *webDAVDir) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		d.pos = 0
		return 0, nil
	}
	return 0, errors.New("seek not supported")
}

// Readdir implements webdav.File
func (d *webDAVDir) Readdir(count int) ([]os.FileInfo, error) {
	// Load entries if not already loaded
	if d.entries == nil {
		entries, err := d.adapter.fs.ReadDir(d.path)
		if err != nil {
			return nil, err
		}

		d.entries = make([]os.FileInfo, len(entries))
		for i, entry := range entries {
			d.entries[i] = &webDAVFileInfo{FileInfo: entry}
		}
	}

	// If no entries at all, return empty slice
	if len(d.entries) == 0 {
		return []os.FileInfo{}, nil
	}

	// Check if we've read all entries
	if d.pos >= len(d.entries) {
		return nil, io.EOF
	}

	// Handle count parameter
	if count <= 0 {
		// Return all remaining entries
		result := d.entries[d.pos:]
		d.pos = len(d.entries)
		return result, nil
	}

	// Return up to count entries
	end := d.pos + count
	if end > len(d.entries) {
		end = len(d.entries)
	}

	result := d.entries[d.pos:end]
	d.pos = end

	return result, nil
}

// Stat implements webdav.File
func (d *webDAVDir) Stat() (os.FileInfo, error) {
	return &webDAVFileInfo{FileInfo: d.info}, nil
}

// webDAVFileInfo wraps our FileInfo to implement os.FileInfo
type webDAVFileInfo struct {
	internal.FileInfo
}

// Mode implements os.FileInfo
func (i *webDAVFileInfo) Mode() os.FileMode {
	if i.IsDir() {
		return os.ModeDir | 0755
	}
	return 0644
}

// Sys implements os.FileInfo
func (i *webDAVFileInfo) Sys() any {
	return nil
}
