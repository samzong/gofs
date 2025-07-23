package filesystem

import (
	"io"
	"io/fs"
	"time"

	"github.com/samzong/gofs/internal"
)

// LocalFileInfo implements the FileInfo interface for local file information.
type LocalFileInfo struct {
	name    string
	size    int64
	mode    string
	modTime time.Time
	isDir   bool
	path    string
}

// Name returns the file name.
func (f *LocalFileInfo) Name() string { return f.name }

// Size returns the file size in bytes.
func (f *LocalFileInfo) Size() int64 { return f.size }

// Mode returns the file mode as a string.
func (f *LocalFileInfo) Mode() string { return f.mode }

// ModTime returns the file modification time.
func (f *LocalFileInfo) ModTime() time.Time { return f.modTime }

// IsDir returns true if the file is a directory.
func (f *LocalFileInfo) IsDir() bool { return f.isDir }

// Path returns the file path.
func (f *LocalFileInfo) Path() string { return f.path }

// LocalFile implements the File interface for local files.
type LocalFile struct {
	io.ReadSeekCloser
	info internal.FileInfo
}

// Stat returns the file information for this file.
func (f *LocalFile) Stat() (internal.FileInfo, error) {
	return f.info, nil
}

// NewLocalFileInfo creates a LocalFileInfo from fs.FileInfo.
func NewLocalFileInfo(info fs.FileInfo, path string) internal.FileInfo {
	mode := info.Mode().String()
	return &LocalFileInfo{
		name:    info.Name(),
		size:    info.Size(),
		mode:    mode,
		modTime: info.ModTime(),
		isDir:   info.IsDir(),
		path:    path,
	}
}
