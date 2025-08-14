package filesystem

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samzong/gofs/internal"
)

func TestNewLocal(t *testing.T) {
	tests := []struct {
		name       string
		root       string
		showHidden bool
		expected   *Local
	}{
		{
			name:       "basic directory with hidden files disabled",
			root:       "/tmp/test",
			showHidden: false,
			expected: &Local{
				root:       "/tmp/test",
				showHidden: false,
			},
		},
		{
			name:       "basic directory with hidden files enabled",
			root:       "/tmp/test",
			showHidden: true,
			expected: &Local{
				root:       "/tmp/test",
				showHidden: true,
			},
		},
		{
			name:       "directory with trailing slash gets cleaned",
			root:       "/tmp/test/",
			showHidden: false,
			expected: &Local{
				root:       "/tmp/test",
				showHidden: false,
			},
		},
		{
			name:       "relative path gets cleaned",
			root:       "./test/../test",
			showHidden: false,
			expected: &Local{
				root:       "test",
				showHidden: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewLocal(tt.root, tt.showHidden)

			if result.root != tt.expected.root {
				t.Errorf("root = %v, want %v", result.root, tt.expected.root)
			}
			if result.showHidden != tt.expected.showHidden {
				t.Errorf("showHidden = %v, want %v", result.showHidden, tt.expected.showHidden)
			}
		})
	}
}

func TestLocal_Root(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		expected string
	}{
		{
			name:     "simple path",
			root:     "/tmp/test",
			expected: "/tmp/test",
		},
		{
			name:     "path with trailing slash",
			root:     "/tmp/test/",
			expected: "/tmp/test",
		},
		{
			name:     "relative path",
			root:     "./test",
			expected: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewLocal(tt.root, false)
			if got := fs.Root(); got != tt.expected {
				t.Errorf("Root() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLocal_Open(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gofs-local-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := []byte("Hello, World!")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := NewLocal(tempDir, false)

	tests := []struct {
		name      string
		filename  string
		wantError bool
		errorCode string
	}{
		{
			name:      "valid file",
			filename:  "test.txt",
			wantError: false,
		},
		{
			name:      "nonexistent file",
			filename:  "nonexistent.txt",
			wantError: true,
			errorCode: "FILE_ACCESS_ERROR",
		},
		{
			name:      "path traversal attempt",
			filename:  "../../../etc/passwd",
			wantError: true,
			errorCode: "INVALID_PATH",
		},
		{
			name:      "null byte injection",
			filename:  "test\x00.txt",
			wantError: true,
			errorCode: "INVALID_PATH",
		},
		{
			name:      "empty filename",
			filename:  "",
			wantError: true,
			errorCode: "INVALID_PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := fs.Open(tt.filename)

			if tt.wantError {
				if err == nil {
					t.Errorf("Open() expected error but got none")
					if reader != nil {
						reader.Close()
					}
					return
				}

				apiErr, ok := err.(*internal.APIError)
				if !ok {
					t.Errorf("Expected APIError, got %T: %v", err, err)
					return
				}

				if apiErr.Code != tt.errorCode {
					t.Errorf("Expected error code %v, got %v", tt.errorCode, apiErr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("Open() unexpected error: %v", err)
					return
				}

				if reader == nil {
					t.Errorf("Open() returned nil reader")
					return
				}

				// Test reading the content
				content, err := io.ReadAll(reader)
				if err != nil {
					t.Errorf("Failed to read content: %v", err)
				} else if string(content) != string(testContent) {
					t.Errorf("Content mismatch: got %q, want %q", string(content), string(testContent))
				}

				reader.Close()
			}
		})
	}
}

func TestLocal_PathTraversalSecurity(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "gofs-security-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a file outside the temp directory to test traversal prevention
	parentDir := filepath.Dir(tempDir)
	secretFile := filepath.Join(parentDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret data"), 0644); err != nil {
		t.Fatalf("Failed to create secret file: %v", err)
	}
	defer os.Remove(secretFile)

	fs := NewLocal(tempDir, false)

	maliciousPaths := []string{
		"../secret.txt",
		"../../secret.txt",
		"../../../etc/passwd",
		"..\\..\\windows\\system32\\config\\sam",
		"test/../../../etc/passwd",
		"./../../secret.txt",
		"test/../../secret.txt",
	}

	for _, malPath := range maliciousPaths {
		t.Run("path_traversal_"+malPath, func(t *testing.T) {
			reader, err := fs.Open(malPath)

			if err == nil {
				reader.Close()
				t.Errorf("Path traversal attack succeeded for path: %v", malPath)
				return
			}

			apiErr, ok := err.(*internal.APIError)
			if !ok {
				t.Errorf("Expected APIError for malicious path %v, got %T: %v", malPath, err, err)
				return
			}

			if apiErr.Code != "INVALID_PATH" {
				t.Errorf("Expected INVALID_PATH error for %v, got %v", malPath, apiErr.Code)
			}
		})
	}
}

func TestLocal_NullByteAndControlCharacters(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-null-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := NewLocal(tempDir, false)

	maliciousPaths := []string{
		"test\x00.txt",
		"file\x00",
		"\x00secret.txt",
		"test\r\nheader: injection",
		"file\x01.txt",
		"test\x1f.txt",
		"file\x7f.txt",
	}

	for _, malPath := range maliciousPaths {
		t.Run("null_byte_"+strings.ReplaceAll(malPath, "\x00", "NULL"), func(t *testing.T) {
			reader, err := fs.Open(malPath)

			if err == nil {
				reader.Close()
				t.Errorf("Null byte/control character attack succeeded for path: %q", malPath)
				return
			}

			apiErr, ok := err.(*internal.APIError)
			if !ok {
				t.Errorf("Expected APIError for malicious path %q, got %T: %v", malPath, err, err)
				return
			}

			if apiErr.Code != "INVALID_PATH" {
				t.Errorf("Expected INVALID_PATH error for %q, got %v", malPath, apiErr.Code)
			}
		})
	}
}

func TestLocal_Stat(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gofs-stat-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file and directory
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	testDir := filepath.Join(tempDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	fs := NewLocal(tempDir, false)

	tests := []struct {
		name      string
		filename  string
		wantError bool
		errorCode string
		isDir     bool
		size      int64
	}{
		{
			name:      "regular file",
			filename:  "test.txt",
			wantError: false,
			isDir:     false,
			size:      int64(len(testContent)),
		},
		{
			name:      "directory",
			filename:  "testdir",
			wantError: false,
			isDir:     true,
			size:      0,
		},
		{
			name:      "nonexistent file",
			filename:  "nonexistent.txt",
			wantError: true,
			errorCode: "FILE_ACCESS_ERROR",
		},
		{
			name:      "path traversal attempt",
			filename:  "../../../etc/passwd",
			wantError: true,
			errorCode: "INVALID_PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fs.Stat(tt.filename)

			if tt.wantError {
				if err == nil {
					t.Errorf("Stat() expected error but got none")
					return
				}

				apiErr, ok := err.(*internal.APIError)
				if !ok {
					t.Errorf("Expected APIError, got %T: %v", err, err)
					return
				}

				if apiErr.Code != tt.errorCode {
					t.Errorf("Expected error code %v, got %v", tt.errorCode, apiErr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("Stat() unexpected error: %v", err)
					return
				}

				if info == nil {
					t.Errorf("Stat() returned nil info")
					return
				}

				if info.IsDir() != tt.isDir {
					t.Errorf("IsDir() = %v, want %v", info.IsDir(), tt.isDir)
				}

				if !tt.isDir && info.Size() != tt.size {
					t.Errorf("Size() = %v, want %v", info.Size(), tt.size)
				}

				if info.Name() != tt.filename {
					t.Errorf("Name() = %v, want %v", info.Name(), tt.filename)
				}
			}
		})
	}
}

func TestLocal_ReadDir(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gofs-readdir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	testFiles := []string{"file1.txt", "file2.txt", ".hidden", "subdir"}
	for i, name := range testFiles {
		path := filepath.Join(tempDir, name)
		if name == "subdir" {
			if err := os.Mkdir(path, 0755); err != nil {
				t.Fatalf("Failed to create directory %v: %v", name, err)
			}
		} else {
			content := []byte("content " + string(rune('0'+i)))
			if err := os.WriteFile(path, content, 0644); err != nil {
				t.Fatalf("Failed to create file %v: %v", name, err)
			}
		}
	}

	tests := []struct {
		name         string
		showHidden   bool
		dirname      string
		wantError    bool
		errorCode    string
		expectHidden bool
		minEntries   int
	}{
		{
			name:         "show hidden files enabled",
			showHidden:   true,
			dirname:      "",
			wantError:    false,
			expectHidden: true,
			minEntries:   4, // file1.txt, file2.txt, .hidden, subdir
		},
		{
			name:         "show hidden files disabled",
			showHidden:   false,
			dirname:      "",
			wantError:    false,
			expectHidden: false,
			minEntries:   3, // file1.txt, file2.txt, subdir
		},
		{
			name:       "nonexistent directory",
			showHidden: false,
			dirname:    "nonexistent",
			wantError:  true,
			errorCode:  "FILE_ACCESS_ERROR",
		},
		{
			name:       "path traversal attempt",
			showHidden: false,
			dirname:    "../../../etc",
			wantError:  true,
			errorCode:  "INVALID_PATH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := NewLocal(tempDir, tt.showHidden)
			entries, err := fs.ReadDir(tt.dirname)

			if tt.wantError {
				if err == nil {
					t.Errorf("ReadDir() expected error but got none")
					return
				}

				apiErr, ok := err.(*internal.APIError)
				if !ok {
					t.Errorf("Expected APIError, got %T: %v", err, err)
					return
				}

				if apiErr.Code != tt.errorCode {
					t.Errorf("Expected error code %v, got %v", tt.errorCode, apiErr.Code)
				}
			} else {
				if err != nil {
					t.Errorf("ReadDir() unexpected error: %v", err)
					return
				}

				if len(entries) < tt.minEntries {
					t.Errorf("Expected at least %d entries, got %d", tt.minEntries, len(entries))
				}

				hasHidden := false
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".") {
						hasHidden = true
						break
					}
				}

				if tt.expectHidden && !hasHidden {
					t.Errorf("Expected hidden files to be included, but none found")
				} else if !tt.expectHidden && hasHidden {
					t.Errorf("Expected hidden files to be excluded, but found some")
				}
			}
		})
	}
}

func TestLocal_ConcurrentAccess(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gofs-concurrent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	for i := range 10 {
		filename := filepath.Join(tempDir, "file"+string(rune('0'+i))+".txt")
		content := []byte("content " + string(rune('0'+i)))
		if err := os.WriteFile(filename, content, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	fs := NewLocal(tempDir, false)

	// Test concurrent file operations
	const numGoroutines = 20
	const operationsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*operationsPerGoroutine)

	// Concurrent Open operations
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range operationsPerGoroutine {
				filename := "file" + string(rune('0'+(id+j)%10)) + ".txt"
				reader, err := fs.Open(filename)
				if err != nil {
					errors <- err
					continue
				}

				content, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					errors <- err
					continue
				}

				expectedContent := "content " + string(rune('0'+(id+j)%10))
				if string(content) != expectedContent {
					errors <- err
				}
			}
		}(i)
	}

	// Concurrent Stat operations
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range operationsPerGoroutine {
				filename := "file" + string(rune('0'+(id+j)%10)) + ".txt"
				info, err := fs.Stat(filename)
				if err != nil {
					errors <- err
					continue
				}
				if info.Name() != filename {
					errors <- err
				}
			}
		}(i)
	}

	// Concurrent ReadDir operations
	wg.Add(numGoroutines)
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range operationsPerGoroutine {
				entries, err := fs.ReadDir("")
				if err != nil {
					errors <- err
					continue
				}
				if len(entries) < 10 {
					errors <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}
}

func TestLocal_LargeFileHandling(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gofs-large-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a moderately large test file (1MB)
	largeFile := filepath.Join(tempDir, "large.txt")
	largeContent := make([]byte, 1024*1024) // 1MB
	for i := range largeContent {
		largeContent[i] = byte('A' + (i % 26))
	}

	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	fs := NewLocal(tempDir, false)

	// Test opening and reading large file
	reader, err := fs.Open("large.txt")
	if err != nil {
		t.Fatalf("Failed to open large file: %v", err)
	}
	defer reader.Close()

	// Read in chunks to test streaming
	buffer := make([]byte, 64*1024) // 64KB chunks
	totalRead := 0
	for {
		n, err := reader.Read(buffer)
		totalRead += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Error reading large file: %v", err)
		}
	}

	if totalRead != len(largeContent) {
		t.Errorf("Expected to read %d bytes, got %d", len(largeContent), totalRead)
	}

	// Test Stat on large file
	info, err := fs.Stat("large.txt")
	if err != nil {
		t.Fatalf("Failed to stat large file: %v", err)
	}

	if info.Size() != int64(len(largeContent)) {
		t.Errorf("Expected file size %d, got %d", len(largeContent), info.Size())
	}
}
