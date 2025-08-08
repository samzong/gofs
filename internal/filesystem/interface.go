package filesystem

import (
	"io"
	"io/fs"
	"time"

	"github.com/samzong/gofs/internal"
)

type LocalFileInfo struct {
	modTime time.Time
	name    string
	mode    string
	path    string
	size    int64
	isDir   bool
}

func (f *LocalFileInfo) Name() string       { return f.name }
func (f *LocalFileInfo) Size() int64        { return f.size }
func (f *LocalFileInfo) Mode() string       { return f.mode }
func (f *LocalFileInfo) ModTime() time.Time { return f.modTime }
func (f *LocalFileInfo) IsDir() bool        { return f.isDir }
func (f *LocalFileInfo) Path() string       { return f.path }

type LocalFile struct {
	io.ReadSeekCloser
	info internal.FileInfo
}

func (f *LocalFile) Stat() (internal.FileInfo, error) {
	return f.info, nil
}

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
