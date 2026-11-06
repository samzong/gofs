package filesystem

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/pkg/fileutil"
)

type Local struct {
	root       string
	showHidden bool
}

func NewLocal(root string, showHidden bool) *Local {
	return &Local{
		root:       filepath.Clean(root),
		showHidden: showHidden,
	}
}

func (fs *Local) Root() string {
	return fs.root
}

func (fs *Local) Open(name string) (io.ReadCloser, error) {
	rel := fs.relPath(name)
	if rel == "" {
		return nil, &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Invalid file path",
			Status:  http.StatusBadRequest,
		}
	}

	var file *os.File
	err := fs.inRoot(func(root *os.Root) (err error) {
		file, err = root.Open(rel)
		return err
	})
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
	rel := fs.relPath(name)
	if rel == "" {
		return nil, &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Invalid file path",
			Status:  http.StatusBadRequest,
		}
	}

	var info os.FileInfo
	err := fs.inRoot(func(root *os.Root) (err error) {
		info, err = root.Stat(rel)
		return err
	})
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
	rel := fs.relPath(name)
	if rel == "" {
		return nil, &internal.APIError{
			Code:    "INVALID_PATH",
			Message: "Invalid directory path",
			Status:  http.StatusBadRequest,
		}
	}

	var result []internal.FileInfo
	err := fs.inRoot(func(root *os.Root) error {
		dir, err := root.Open(rel)
		if err != nil {
			return err
		}
		defer dir.Close()

		entries, err := dir.ReadDir(-1)
		if err != nil {
			return err
		}
		slices.SortFunc(entries, func(a, b os.DirEntry) int {
			return strings.Compare(a.Name(), b.Name())
		})

		result = make([]internal.FileInfo, 0, len(entries))
		for _, entry := range entries {
			if !fs.showHidden && isHidden(entry.Name()) {
				continue
			}

			info, err := root.Lstat(filepath.Join(rel, entry.Name()))
			if err != nil {
				continue
			}
			result = append(result, &localFileInfo{FileInfo: info})
		}
		return nil
	})
	if err != nil {
		return nil, &internal.APIError{
			Code:    "DIRECTORY_READ_ERROR",
			Message: "Unable to read directory contents",
			Status:  http.StatusForbidden,
		}
	}

	return result, nil
}

// Create creates or truncates the named file for writing.
// The returned io.WriteCloser must be closed after use.
func (fs *Local) Create(name string) (io.WriteCloser, error) {
	rel := fs.relPath(name)
	if rel == "" {
		return nil, fmt.Errorf("invalid path: %s", name)
	}

	var file *os.File
	err := fs.inRoot(func(root *os.Root) (err error) {
		file, err = root.Create(rel)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("creating file %q: %w", name, err)
	}
	return file, nil
}

// Mkdir creates a directory with the specified name and permission.
func (fs *Local) Mkdir(name string, perm os.FileMode) error {
	rel := fs.relPath(name)
	if rel == "" {
		return fmt.Errorf("invalid path: %s", name)
	}

	if err := fs.inRoot(func(root *os.Root) error { return root.Mkdir(rel, perm) }); err != nil {
		return fmt.Errorf("creating directory %q: %w", name, err)
	}
	return nil
}

// Remove removes the named file or empty directory.
func (fs *Local) Remove(name string) error {
	rel := fs.relPath(name)
	if rel == "" {
		return fmt.Errorf("invalid path: %s", name)
	}

	if err := fs.inRoot(func(root *os.Root) error { return root.Remove(rel) }); err != nil {
		return fmt.Errorf("removing %q: %w", name, err)
	}
	return nil
}

func (fs *Local) relPath(name string) string {
	if name == "" {
		name = "."
	}
	return fileutil.SafePath(name)
}

func (fs *Local) inRoot(op func(*os.Root) error) error {
	root, err := os.OpenRoot(fs.root)
	if err != nil {
		return err
	}
	defer root.Close()
	return op(root)
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

// ReadonlyFileSystem wraps a FileSystem to make it read-only
type ReadonlyFileSystem struct {
	internal.FileSystem
}

// NewReadonly creates a read-only wrapper around a FileSystem
func NewReadonly(fs internal.FileSystem) *ReadonlyFileSystem {
	return &ReadonlyFileSystem{FileSystem: fs}
}

// Create is disabled for read-only filesystem
func (r *ReadonlyFileSystem) Create(name string) (io.WriteCloser, error) {
	return nil, fmt.Errorf("read-only filesystem: cannot create %s", name)
}

// Mkdir is disabled for read-only filesystem
func (r *ReadonlyFileSystem) Mkdir(name string, _ os.FileMode) error {
	return fmt.Errorf("read-only filesystem: cannot create directory %s", name)
}

// Remove is disabled for read-only filesystem
func (r *ReadonlyFileSystem) Remove(name string) error {
	return fmt.Errorf("read-only filesystem: cannot remove %s", name)
}
