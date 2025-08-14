package filesystem

import (
	"io/fs"
	"testing"
	"time"
)

func TestLocalFileInfo_Methods(t *testing.T) {
	modTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)

	tests := []struct {
		name         string
		info         *LocalFileInfo
		expectedName string
		expectedSize int64
		expectedMode string
		expectedPath string
		expectedDir  bool
	}{
		{
			name: "regular file",
			info: &LocalFileInfo{
				name:    "test.txt",
				size:    1024,
				mode:    "-rw-r--r--",
				modTime: modTime,
				isDir:   false,
				path:    "/path/to/test.txt",
			},
			expectedName: "test.txt",
			expectedSize: 1024,
			expectedMode: "-rw-r--r--",
			expectedPath: "/path/to/test.txt",
			expectedDir:  false,
		},
		{
			name: "directory",
			info: &LocalFileInfo{
				name:    "docs",
				size:    0,
				mode:    "drwxr-xr-x",
				modTime: modTime,
				isDir:   true,
				path:    "/path/to/docs",
			},
			expectedName: "docs",
			expectedSize: 0,
			expectedMode: "drwxr-xr-x",
			expectedPath: "/path/to/docs",
			expectedDir:  true,
		},
		{
			name: "empty filename",
			info: &LocalFileInfo{
				name:    "",
				size:    0,
				mode:    "-rw-r--r--",
				modTime: modTime,
				isDir:   false,
				path:    "",
			},
			expectedName: "",
			expectedSize: 0,
			expectedMode: "-rw-r--r--",
			expectedPath: "",
			expectedDir:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.Name(); got != tt.expectedName {
				t.Errorf("Name() = %v, want %v", got, tt.expectedName)
			}
			if got := tt.info.Size(); got != tt.expectedSize {
				t.Errorf("Size() = %v, want %v", got, tt.expectedSize)
			}
			if got := tt.info.Mode(); got != tt.expectedMode {
				t.Errorf("Mode() = %v, want %v", got, tt.expectedMode)
			}
			if got := tt.info.ModTime(); !got.Equal(modTime) {
				t.Errorf("ModTime() = %v, want %v", got, modTime)
			}
			if got := tt.info.IsDir(); got != tt.expectedDir {
				t.Errorf("IsDir() = %v, want %v", got, tt.expectedDir)
			}
			if got := tt.info.Path(); got != tt.expectedPath {
				t.Errorf("Path() = %v, want %v", got, tt.expectedPath)
			}
		})
	}
}

func TestNewLocalFileInfo(t *testing.T) {
	modTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)

	tests := []struct {
		name     string
		fileInfo mockFileInfo
		path     string
		expected *LocalFileInfo
	}{
		{
			name: "regular file",
			fileInfo: mockFileInfo{
				name:    "test.txt",
				size:    1024,
				mode:    0644,
				modTime: modTime,
				isDir:   false,
			},
			path: "/path/to/test.txt",
			expected: &LocalFileInfo{
				name:    "test.txt",
				size:    1024,
				mode:    "-rw-r--r--",
				modTime: modTime,
				isDir:   false,
				path:    "/path/to/test.txt",
			},
		},
		{
			name: "directory",
			fileInfo: mockFileInfo{
				name:    "testdir",
				size:    0,
				mode:    0755 | fs.ModeDir,
				modTime: modTime,
				isDir:   true,
			},
			path: "/path/to/testdir",
			expected: &LocalFileInfo{
				name:    "testdir",
				size:    0,
				mode:    "drwxr-xr-x",
				modTime: modTime,
				isDir:   true,
				path:    "/path/to/testdir",
			},
		},
		{
			name: "executable file",
			fileInfo: mockFileInfo{
				name:    "script.sh",
				size:    512,
				mode:    0755,
				modTime: modTime,
				isDir:   false,
			},
			path: "/usr/bin/script.sh",
			expected: &LocalFileInfo{
				name:    "script.sh",
				size:    512,
				mode:    "-rwxr-xr-x",
				modTime: modTime,
				isDir:   false,
				path:    "/usr/bin/script.sh",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewLocalFileInfo(tt.fileInfo, tt.path)

			localInfo, ok := result.(*LocalFileInfo)
			if !ok {
				t.Fatalf("Expected *LocalFileInfo, got %T", result)
			}

			if localInfo.Name() != tt.expected.Name() {
				t.Errorf("Name() = %v, want %v", localInfo.Name(), tt.expected.Name())
			}
			if localInfo.Size() != tt.expected.Size() {
				t.Errorf("Size() = %v, want %v", localInfo.Size(), tt.expected.Size())
			}
			if localInfo.Mode() != tt.expected.Mode() {
				t.Errorf("Mode() = %v, want %v", localInfo.Mode(), tt.expected.Mode())
			}
			if !localInfo.ModTime().Equal(tt.expected.ModTime()) {
				t.Errorf("ModTime() = %v, want %v", localInfo.ModTime(), tt.expected.ModTime())
			}
			if localInfo.IsDir() != tt.expected.IsDir() {
				t.Errorf("IsDir() = %v, want %v", localInfo.IsDir(), tt.expected.IsDir())
			}
			if localInfo.Path() != tt.expected.Path() {
				t.Errorf("Path() = %v, want %v", localInfo.Path(), tt.expected.Path())
			}
		})
	}
}

func TestLocalFile_Stat(t *testing.T) {
	modTime := time.Date(2023, 12, 25, 10, 30, 45, 0, time.UTC)

	fileInfo := &LocalFileInfo{
		name:    "test.txt",
		size:    1024,
		mode:    "-rw-r--r--",
		modTime: modTime,
		isDir:   false,
		path:    "/path/to/test.txt",
	}

	localFile := &LocalFile{
		info: fileInfo,
	}

	result, err := localFile.Stat()
	if err != nil {
		t.Errorf("Stat() returned unexpected error: %v", err)
	}

	if result != fileInfo {
		t.Errorf("Stat() = %v, want %v", result, fileInfo)
	}

	if result.Name() != "test.txt" {
		t.Errorf("Stat().Name() = %v, want %v", result.Name(), "test.txt")
	}
	if result.Size() != 1024 {
		t.Errorf("Stat().Size() = %v, want %v", result.Size(), 1024)
	}
	if !result.ModTime().Equal(modTime) {
		t.Errorf("Stat().ModTime() = %v, want %v", result.ModTime(), modTime)
	}
	if result.IsDir() != false {
		t.Errorf("Stat().IsDir() = %v, want %v", result.IsDir(), false)
	}

	// Test LocalFileInfo specific methods by casting
	localInfo, ok := result.(*LocalFileInfo)
	if !ok {
		t.Fatalf("Expected *LocalFileInfo, got %T", result)
	}
	if localInfo.Mode() != "-rw-r--r--" {
		t.Errorf("Mode() = %v, want %v", localInfo.Mode(), "-rw-r--r--")
	}
	if localInfo.Path() != "/path/to/test.txt" {
		t.Errorf("Path() = %v, want %v", localInfo.Path(), "/path/to/test.txt")
	}
}

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (m mockFileInfo) Name() string       { return m.name }
func (m mockFileInfo) Size() int64        { return m.size }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return m.modTime }
func (m mockFileInfo) IsDir() bool        { return m.isDir }
func (m mockFileInfo) Sys() any           { return nil }
