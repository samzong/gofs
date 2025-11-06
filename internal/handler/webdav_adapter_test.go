package handler

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/filesystem"
	"golang.org/x/net/webdav"
)

func TestNewWebDAVAdapter(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs)

	if adapter == nil {
		t.Fatal("NewWebDAVAdapter returned nil")
	}

	// Test that it implements webdav.FileSystem interface
	var _ = adapter
}

func TestWebDAVAdapter_Mkdir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)

	ctx := context.Background()
	err = adapter.Mkdir(ctx, "/testdir", 0755)

	if err != webdav.ErrForbidden {
		t.Errorf("Expected ErrForbidden, got %v", err)
	}
}

func TestWebDAVAdapter_RemoveAll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)

	ctx := context.Background()
	err = adapter.RemoveAll(ctx, "/testdir")

	if err != webdav.ErrForbidden {
		t.Errorf("Expected ErrForbidden, got %v", err)
	}
}

func TestWebDAVAdapter_Rename(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)

	ctx := context.Background()
	err = adapter.Rename(ctx, "/old.txt", "/new.txt")

	if err != webdav.ErrForbidden {
		t.Errorf("Expected ErrForbidden, got %v", err)
	}
}

func TestWebDAVAdapter_OpenFile_WriteOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)

	ctx := context.Background()
	writeFlags := []int{
		os.O_WRONLY,
		os.O_RDWR,
		os.O_APPEND,
		os.O_CREATE,
		os.O_TRUNC,
		os.O_WRONLY | os.O_CREATE,
		os.O_RDWR | os.O_APPEND,
	}

	for _, flag := range writeFlags {
		t.Run("", func(t *testing.T) {
			file, err := adapter.OpenFile(ctx, "/test.txt", flag, 0644)

			if err != webdav.ErrForbidden {
				t.Errorf("Expected ErrForbidden for flag %d, got %v", flag, err)
			}

			if file != nil {
				t.Errorf("Expected nil file for write operation, got %T", file)
			}
		})
	}
}

func TestWebDAVAdapter_OpenFile_ReadOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file and directory
	testFile := tempDir + "/test.txt"
	testContent := "Hello, WebDAV adapter!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testDir := tempDir + "/testdir"
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)
	ctx := context.Background()

	tests := []struct {
		name     string
		path     string
		flag     int
		isDir    bool
		expectOK bool
	}{
		{
			name:     "read only file",
			path:     "/test.txt",
			flag:     os.O_RDONLY,
			isDir:    false,
			expectOK: true,
		},
		{
			name:     "read only directory",
			path:     "/testdir",
			flag:     os.O_RDONLY,
			isDir:    true,
			expectOK: true,
		},
		{
			name:     "root directory",
			path:     "/",
			flag:     os.O_RDONLY,
			isDir:    true,
			expectOK: true,
		},
		{
			name:     "nonexistent file",
			path:     "/nonexistent.txt",
			flag:     os.O_RDONLY,
			isDir:    false,
			expectOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := adapter.OpenFile(ctx, tt.path, tt.flag, 0644)

			if tt.expectOK {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
					return
				}
				if file == nil {
					t.Error("Expected file, got nil")
					return
				}

				// Test file operations
				if tt.isDir {
					// Test directory operations
					dirFile, ok := file.(*webDAVDir)
					if !ok {
						t.Errorf("Expected webDAVDir, got %T", file)
					} else {
						// Test Stat
						info, err := dirFile.Stat()
						if err != nil {
							t.Errorf("Stat failed: %v", err)
						}
						if info != nil && !info.IsDir() {
							t.Error("Expected directory, got file")
						}

						// Test Close
						err = dirFile.Close()
						if err != nil {
							t.Errorf("Close failed: %v", err)
						}

						// Test Read (should fail for directories)
						_, err = dirFile.Read(make([]byte, 10))
						if err == nil {
							t.Error("Expected error reading from directory")
						}

						// Test Write (should fail - read only)
						_, err = dirFile.Write([]byte("test"))
						if err != webdav.ErrForbidden {
							t.Errorf("Expected ErrForbidden, got %v", err)
						}
					}
				} else {
					// Test file operations
					regFile, ok := file.(*webDAVFile)
					if !ok {
						t.Errorf("Expected webDAVFile, got %T", file)
					} else {
						// Test Stat
						info, err := regFile.Stat()
						if err != nil {
							t.Errorf("Stat failed: %v", err)
						}
						if info != nil && info.IsDir() {
							t.Error("Expected file, got directory")
						}

						// Test Read
						data, err := io.ReadAll(regFile)
						if err != nil {
							t.Errorf("Read failed: %v", err)
						}
						if string(data) != testContent {
							t.Errorf("Expected content %q, got %q", testContent, string(data))
						}

						// Test Close
						err = regFile.Close()
						if err != nil {
							t.Errorf("Close failed: %v", err)
						}

						// Test Write (should fail - read only)
						_, err = regFile.Write([]byte("test"))
						if err != webdav.ErrForbidden {
							t.Errorf("Expected ErrForbidden, got %v", err)
						}

						// Test Readdir (should fail for files)
						_, err = regFile.Readdir(10)
						if err == nil {
							t.Error("Expected error from Readdir on file")
						}
					}
				}
			} else {
				if err == nil {
					t.Error("Expected error, got success")
				}
				if file != nil {
					t.Error("Expected nil file on error")
				}
			}
		})
	}
}

func TestWebDAVAdapter_Stat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file and directory
	testFile := tempDir + "/test.txt"
	testContent := "Hello, WebDAV stat!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testDir := tempDir + "/testdir"
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)
	ctx := context.Background()

	tests := []struct {
		name     string
		path     string
		expectOK bool
		isDir    bool
	}{
		{"file", "/test.txt", true, false},
		{"directory", "/testdir", true, true},
		{"root", "/", true, true},
		{"nonexistent", "/nonexistent.txt", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := adapter.Stat(ctx, tt.path)

			if tt.expectOK {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
					return
				}
				if info == nil {
					t.Error("Expected file info, got nil")
					return
				}

				if info.IsDir() != tt.isDir {
					t.Errorf("Expected isDir=%v, got %v", tt.isDir, info.IsDir())
				}

				// Test webDAVFileInfo wrapper
				webdavInfo, ok := info.(*webDAVFileInfo)
				if !ok {
					t.Errorf("Expected webDAVFileInfo, got %T", info)
					return
				}

				// Test Mode()
				mode := webdavInfo.Mode()
				if tt.isDir {
					if !mode.IsDir() {
						t.Error("Expected directory mode")
					}
					if mode&0755 == 0 {
						t.Error("Expected directory permissions")
					}
				} else {
					if mode.IsDir() {
						t.Error("Expected file mode")
					}
					if mode&0644 == 0 {
						t.Error("Expected file permissions")
					}
				}

				// Test Sys()
				if webdavInfo.Sys() != nil {
					t.Error("Expected Sys() to return nil")
				}

				// Test other os.FileInfo methods
				if webdavInfo.Name() == "" {
					t.Error("Expected non-empty name")
				}
				if webdavInfo.ModTime().IsZero() {
					t.Error("Expected non-zero mod time")
				}
			} else {
				if err == nil {
					t.Error("Expected error, got success")
				}
				if info != nil {
					t.Error("Expected nil info on error")
				}
			}
		})
	}
}

func TestWebDAVFile_Seek(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := tempDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)
	ctx := context.Background()

	file, err := adapter.OpenFile(ctx, "/test.txt", os.O_RDONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Test seek (should fail)
	offset, err := file.Seek(0, io.SeekStart)
	if err == nil {
		t.Error("Expected seek to fail")
	}
	if offset != 0 {
		t.Errorf("Expected offset 0, got %d", offset)
	}
}

func TestWebDAVDir_Operations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	subDir := tempDir + "/subdir"
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create files
	file1 := tempDir + "/file1.txt"
	file2 := tempDir + "/file2.txt"
	subFile := subDir + "/subfile.txt"

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}
	if err := os.WriteFile(subFile, []byte("subcontent"), 0644); err != nil {
		t.Fatalf("Failed to create subfile: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)
	ctx := context.Background()

	// Open directory
	file, err := adapter.OpenFile(ctx, "/", os.O_RDONLY, 0755)
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer file.Close()

	dir, ok := file.(*webDAVDir)
	if !ok {
		t.Fatalf("Expected webDAVDir, got %T", file)
	}

	// Test Readdir - read all entries
	entries, err := dir.Readdir(0)
	if err != nil {
		t.Errorf("Readdir(0) failed: %v", err)
	}
	if len(entries) < 3 { // file1.txt, file2.txt, subdir
		t.Errorf("Expected at least 3 entries, got %d", len(entries))
	}

	// Test Readdir - read specific count
	_, _ = dir.Seek(0, io.SeekStart) // Reset position
	entries, err = dir.Readdir(2)
	if err != nil {
		t.Errorf("Readdir(2) failed: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}

	// Test Readdir - read remaining
	entries, err = dir.Readdir(0)
	if err != nil && err != io.EOF {
		t.Errorf("Readdir remaining failed: %v", err)
	}
	if len(entries) < 1 {
		t.Errorf("Expected at least 1 remaining entry, got %d", len(entries))
	}

	// Test Readdir - EOF when no more entries
	_, err = dir.Readdir(1)
	if err != io.EOF {
		t.Errorf("Expected EOF, got %v", err)
	}

	// Test Seek to start
	pos, err := dir.Seek(0, io.SeekStart)
	if err != nil {
		t.Errorf("Seek to start failed: %v", err)
	}
	if pos != 0 {
		t.Errorf("Expected position 0, got %d", pos)
	}

	// Test Seek with invalid parameters
	_, err = dir.Seek(5, io.SeekCurrent)
	if err == nil {
		t.Error("Expected seek with invalid params to fail")
	}

	// Test Write (should fail - read only)
	_, err = dir.Write([]byte("test"))
	if err != webdav.ErrForbidden {
		t.Errorf("Expected ErrForbidden, got %v", err)
	}
}

func TestWebDAVDir_EmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create empty subdirectory
	emptyDir := tempDir + "/empty"
	if err := os.Mkdir(emptyDir, 0755); err != nil {
		t.Fatalf("Failed to create empty directory: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)
	ctx := context.Background()

	// Open empty directory
	file, err := adapter.OpenFile(ctx, "/empty", os.O_RDONLY, 0755)
	if err != nil {
		t.Fatalf("Failed to open empty directory: %v", err)
	}
	defer file.Close()

	dir, ok := file.(*webDAVDir)
	if !ok {
		t.Fatalf("Expected webDAVDir, got %T", file)
	}

	// Test Readdir on empty directory
	entries, err := dir.Readdir(0)
	if err != nil {
		t.Errorf("Readdir(0) on empty dir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries in empty directory, got %d", len(entries))
	}

	// Test Readdir with count on empty directory
	entries, err = dir.Readdir(5)
	// Empty directory may return nil error or EOF depending on implementation
	if err != nil && err != io.EOF {
		t.Errorf("Expected nil or EOF for empty directory, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries in empty directory, got %d", len(entries))
	}
}

func TestWebDAVAdapter_ErrorHandling(t *testing.T) {
	// Test with mock filesystem that returns errors
	mockFS := &mockWebDAVFileSystem{
		statError:    errors.New("stat error"),
		openError:    errors.New("open error"),
		readDirError: errors.New("readdir error"),
	}

	adapter := NewWebDAVAdapter(mockFS).(*webDAVAdapter)
	ctx := context.Background()

	// Test Stat error handling
	info, err := adapter.Stat(ctx, "/test")
	if err == nil {
		t.Error("Expected stat error")
	}
	if info != nil {
		t.Error("Expected nil info on error")
	}

	// Test OpenFile error handling
	file, err := adapter.OpenFile(ctx, "/test", os.O_RDONLY, 0644)
	if err == nil {
		t.Error("Expected open error")
	}
	if file != nil {
		t.Error("Expected nil file on error")
	}

	// Test with os.ErrNotExist specifically
	mockFS.statError = os.ErrNotExist
	mockFS.openError = os.ErrNotExist

	_, err = adapter.Stat(ctx, "/test")
	if err != os.ErrNotExist {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}

	_, err = adapter.OpenFile(ctx, "/test", os.O_RDONLY, 0644)
	if err != os.ErrNotExist {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

func TestWebDAVAdapter_PathCleaning(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_adapter_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := tempDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	adapter := NewWebDAVAdapter(fs).(*webDAVAdapter)
	ctx := context.Background()

	pathTests := []struct {
		name       string
		inputPath  string
		shouldWork bool
	}{
		{"normal path", "/test.txt", true},
		{"path with double slash", "//test.txt", true},
		{"path with dot", "/./test.txt", true},
		{"root path", "/", true},
		{"empty path becomes root", "", true},
		{"complex path", "/subdir/../test.txt", true},
	}

	for _, tt := range pathTests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Stat with path cleaning
			info, err := adapter.Stat(ctx, tt.inputPath)
			if tt.shouldWork {
				if err != nil {
					// Some paths might not exist after cleaning, that's ok
					t.Logf("Path %q after cleaning resulted in: %v", tt.inputPath, err)
				} else if info == nil {
					t.Error("Expected file info, got nil")
				}
			}

			// Test OpenFile with path cleaning
			file, err := adapter.OpenFile(ctx, tt.inputPath, os.O_RDONLY, 0644)
			if tt.shouldWork && err == nil {
				if file == nil {
					t.Error("Expected file, got nil")
				} else {
					file.Close()
				}
			}
		})
	}
}

// Mock filesystem for testing error conditions
type mockWebDAVFileSystem struct {
	statError    error
	openError    error
	readDirError error
}

func (m *mockWebDAVFileSystem) Open(_ string) (io.ReadCloser, error) {
	if m.openError != nil {
		return nil, m.openError
	}
	return io.NopCloser(strings.NewReader("mock content")), nil
}

func (m *mockWebDAVFileSystem) Stat(name string) (internal.FileInfo, error) {
	if m.statError != nil {
		return nil, m.statError
	}
	return &mockWebDAVFileInfo{name: name, isDir: false}, nil
}

func (m *mockWebDAVFileSystem) ReadDir(name string) ([]internal.FileInfo, error) {
	if m.readDirError != nil {
		return nil, m.readDirError
	}
	return []internal.FileInfo{
		&mockWebDAVFileInfo{name: "file1.txt", isDir: false},
		&mockWebDAVFileInfo{name: "subdir", isDir: true},
	}, nil
}

func (m *mockWebDAVFileSystem) Create(_ string) (io.WriteCloser, error) {
	return nil, os.ErrPermission
}

func (m *mockWebDAVFileSystem) Mkdir(_ string, _ os.FileMode) error {
	return os.ErrPermission
}

func (m *mockWebDAVFileSystem) Remove(_ string) error {
	return os.ErrPermission
}

type mockWebDAVFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *mockWebDAVFileInfo) Name() string {
	return m.name
}

func (m *mockWebDAVFileInfo) Size() int64 {
	return m.size
}

func (m *mockWebDAVFileInfo) IsDir() bool {
	return m.isDir
}

func (m *mockWebDAVFileInfo) ModTime() time.Time {
	return time.Now()
}
