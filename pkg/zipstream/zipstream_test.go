package zipstream

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWriter(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()

	w := NewWriter(&buf, opts)
	if w == nil {
		t.Fatal("NewWriter returned nil")
	}

	if w.opts.BufferSize != 32*1024 {
		t.Errorf("Expected buffer size 32KB, got %d", w.opts.BufferSize)
	}

	if w.opts.CompressionLevel != zip.Store {
		t.Errorf("Expected Store compression, got %d", w.opts.CompressionLevel)
	}

	if w.bufferPool == nil {
		t.Fatal("buffer pool should be initialized")
	}
}

func TestWriterBufferPool(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultOptions()
	opts.BufferSize = 1024

	w := NewWriter(&out, opts)

	buf := w.getBuffer()
	if len(buf) != opts.BufferSize {
		t.Fatalf("expected buffer of size %d, got %d", opts.BufferSize, len(buf))
	}

	w.putBuffer(buf)

	buf2 := w.getBuffer()
	if len(buf2) != opts.BufferSize {
		t.Fatalf("expected buffer of size %d after reuse, got %d", opts.BufferSize, len(buf2))
	}

	w.putBuffer(buf2)
}

func TestAddFile(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "test*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte("Hello, ZIP streaming!")
	if _, err := tmpfile.Write(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	info, err := os.Stat(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	w := NewWriter(&buf, DefaultOptions())

	err = w.AddFile(FileEntry{
		Path: tmpfile.Name(),
		Name: "test.txt",
		Info: info,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Verify ZIP content
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	if len(reader.File) != 1 {
		t.Fatalf("Expected 1 file in ZIP, got %d", len(reader.File))
	}

	if reader.File[0].Name != "test.txt" {
		t.Errorf("Expected filename 'test.txt', got '%s'", reader.File[0].Name)
	}

	// Read and verify content
	rc, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("Content mismatch: got %s, want %s", got, content)
	}
}

func TestCompressionMethod(t *testing.T) {
	tests := []struct {
		filename string
		opts     Options
		expected uint16
	}{
		{"file.txt", Options{CompressionLevel: zip.Store}, zip.Store},
		{"file.jpg", Options{CompressionLevel: zip.Deflate}, zip.Store},
		{"file.zip", Options{CompressionLevel: zip.Deflate}, zip.Store},
		{"file.mp4", Options{CompressionLevel: zip.Deflate}, zip.Store},
		{"file.txt", Options{CompressionLevel: zip.Deflate}, zip.Deflate},
		{"file.go", Options{CompressionLevel: zip.Deflate}, zip.Deflate},
	}

	for _, tt := range tests {
		w := NewWriter(nil, tt.opts)
		got := w.getCompressionMethod(tt.filename)
		if got != tt.expected {
			t.Errorf("getCompressionMethod(%s) = %d, want %d", tt.filename, got, tt.expected)
		}
	}
}

func TestMaxSizeLimit(t *testing.T) {
	var buf bytes.Buffer
	opts := Options{
		MaxSize:    100, // 100 bytes limit
		BufferSize: 32,
	}

	w := NewWriter(&buf, opts)

	// Create mock file info
	info := mockFileInfo{
		name:    "large.txt",
		size:    200, // Exceeds limit
		mode:    0644,
		modTime: time.Now(),
	}

	err := w.AddFile(FileEntry{
		Name:   "large.txt",
		Info:   info,
		Reader: io.NopCloser(bytes.NewReader(make([]byte, 200))),
	})

	if err == nil {
		t.Error("Expected error for exceeding size limit")
	}
}

func TestProgress(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, DefaultOptions())

	// Create a small file
	content := []byte("test content")
	info := mockFileInfo{
		name:    "test.txt",
		size:    int64(len(content)),
		mode:    0644,
		modTime: time.Now(),
	}

	w.progress.TotalFiles = 1

	err := w.AddFile(FileEntry{
		Name:   "test.txt",
		Info:   info,
		Reader: io.NopCloser(bytes.NewReader(content)),
	})
	if err != nil {
		t.Fatal(err)
	}

	processed, total, bytes := w.Progress().GetProgress()
	if processed != 1 {
		t.Errorf("Expected 1 processed file, got %d", processed)
	}
	if total != 1 {
		t.Errorf("Expected 1 total file, got %d", total)
	}
	if bytes != int64(len(content)) {
		t.Errorf("Expected %d bytes processed, got %d", len(content), bytes)
	}
}

func TestAddDirectory(t *testing.T) {
	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "ziptest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files
	files := map[string]string{
		"file1.txt":             "content1",
		"subdir/file2.txt":      "content2",
		"subdir/deep/file3.txt": "content3",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create ZIP
	var buf bytes.Buffer
	w := NewWriter(&buf, DefaultOptions())

	if err := w.AddDirectory(tmpDir, "test"); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Verify ZIP
	reader, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	// Check that all files are present
	fileMap := make(map[string]bool)
	for _, f := range reader.File {
		fileMap[f.Name] = true
	}

	expectedFiles := []string{
		"test/",
		"test/file1.txt",
		"test/subdir/",
		"test/subdir/file2.txt",
		"test/subdir/deep/",
		"test/subdir/deep/file3.txt",
	}

	for _, expected := range expectedFiles {
		if !fileMap[expected] {
			t.Errorf("Missing file in ZIP: %s", expected)
		}
	}
}

// mockFileInfo implements BasicFileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }
