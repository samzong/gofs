package zipstream

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Options struct {
	CompressionLevel uint16
	MaxSize          int64
	BufferSize       int
}

func DefaultOptions() Options {
	return Options{
		CompressionLevel: zip.Store,
		MaxSize:          500 * 1024 * 1024,
		BufferSize:       32 * 1024,
	}
}

type BasicFileInfo interface {
	Name() string
	Size() int64
	IsDir() bool
	ModTime() time.Time
}

type FileEntry struct {
	Path   string
	Name   string
	Info   BasicFileInfo
	Reader io.ReadCloser
}

type Progress struct {
	TotalFiles     int
	ProcessedFiles int
	TotalBytes     int64
	ProcessedBytes int64
	CurrentFile    string
	StartTime      time.Time
	mu             sync.RWMutex
}

func (p *Progress) GetProgress() (processed, total int, bytes int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ProcessedFiles, p.TotalFiles, p.ProcessedBytes
}

type Writer struct {
	opts       Options
	writer     *zip.Writer
	progress   *Progress
	written    int64
	bufferPool *sync.Pool
}

func NewWriter(w io.Writer, opts Options) *Writer {
	if opts.BufferSize == 0 {
		opts.BufferSize = 32 * 1024
	}

	return &Writer{
		opts:   opts,
		writer: zip.NewWriter(w),
		progress: &Progress{
			StartTime: time.Now(),
		},
		bufferPool: &sync.Pool{
			New: func() any {
				buf := make([]byte, opts.BufferSize)
				return &buf
			},
		},
	}
}

func (zw *Writer) AddFile(entry FileEntry) error {
	if zw.opts.MaxSize > 0 && zw.written+entry.Info.Size() > zw.opts.MaxSize {
		return fmt.Errorf("exceeds maximum ZIP size of %d bytes", zw.opts.MaxSize)
	}

	zw.progress.mu.Lock()
	zw.progress.CurrentFile = entry.Name
	zw.progress.mu.Unlock()

	name := entry.Name
	if name == "" {
		name = filepath.Base(entry.Path)
	}

	header := &zip.FileHeader{
		Name:     name,
		Modified: entry.Info.ModTime(),
	}

	if entry.Info.IsDir() {
		header.Name = ensureTrailingSlash(header.Name)
	} else {
		// Prevent integer overflow: ensure file size is non-negative before conversion
		size := entry.Info.Size()
		if size < 0 {
			return fmt.Errorf("invalid file size %d for %s: file size cannot be negative", size, entry.Path)
		}
		header.UncompressedSize64 = uint64(size)
	}

	header.Method = zw.getCompressionMethod(header.Name)

	if entry.Info.IsDir() {
		header.Method = zip.Store
		_, err := zw.writer.CreateHeader(header)
		return err
	}

	w, err := zw.writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create file in zip: %w", err)
	}

	var reader io.ReadCloser
	if entry.Reader != nil {
		reader = entry.Reader
	} else {
		file, err := os.Open(entry.Path)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		reader = file
	}
	defer reader.Close()

	buf := zw.getBuffer()
	defer zw.putBuffer(buf)
	var written int64

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			nw, werr := w.Write(buf[:n])
			if werr != nil {
				return fmt.Errorf("write to zip: %w", werr)
			}
			written += int64(nw)
			zw.written += int64(nw)

			zw.progress.mu.Lock()
			zw.progress.ProcessedBytes += int64(nw)
			zw.progress.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
	}

	zw.progress.mu.Lock()
	zw.progress.ProcessedFiles++
	zw.progress.mu.Unlock()

	return nil
}

func (zw *Writer) AddDirectory(basePath string, zipPath string) error {
	return filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}

		entryName := filepath.Join(zipPath, relPath)
		if info.IsDir() {
			entryName = ensureTrailingSlash(entryName)
		}

		entryName = filepath.ToSlash(entryName)

		return zw.AddFile(FileEntry{
			Path: path,
			Name: entryName,
			Info: info,
		})
	})
}

func (zw *Writer) Close() error {
	return zw.writer.Close()
}

func (zw *Writer) Progress() *Progress {
	return zw.progress
}

func (zw *Writer) getCompressionMethod(filename string) uint16 {
	if zw.opts.CompressionLevel == zip.Store {
		return zip.Store
	}

	ext := strings.ToLower(filepath.Ext(filename))

	noCompress := map[string]bool{
		".zip": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true,
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
		".pdf": true, ".docx": true, ".xlsx": true, ".pptx": true,
		".apk": true, ".dmg": true, ".iso": true, ".deb": true, ".rpm": true,
	}

	if noCompress[ext] {
		return zip.Store
	}

	return zip.Deflate
}

func ensureTrailingSlash(path string) string {
	if !strings.HasSuffix(path, "/") {
		return path + "/"
	}
	return path
}

func (zw *Writer) getBuffer() []byte {
	if zw.bufferPool == nil {
		return make([]byte, zw.opts.BufferSize)
	}

	return *zw.bufferPool.Get().(*[]byte)
}

func (zw *Writer) putBuffer(buf []byte) {
	if zw.bufferPool == nil || buf == nil {
		return
	}

	buf = buf[:cap(buf)]
	zw.bufferPool.Put(&buf)
}

func StreamFiles(w io.Writer, files []FileEntry, opts Options) error {
	zw := NewWriter(w, opts)
	defer zw.Close()

	zw.progress.TotalFiles = len(files)

	for _, file := range files {
		if err := zw.AddFile(file); err != nil {
			return fmt.Errorf("add file %s: %w", file.Name, err)
		}
	}

	return nil
}

func StreamDirectory(w io.Writer, dirPath string, opts Options) error {
	zw := NewWriter(w, opts)
	defer zw.Close()

	var fileCount int
	_ = filepath.Walk(dirPath, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})
	zw.progress.TotalFiles = fileCount

	return zw.AddDirectory(dirPath, "")
}
