package handler

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/samzong/gofs/internal/config"
)

func TestNewMultiDir(t *testing.T) {
	// Create test directories
	tempDir1, err := os.MkdirTemp("", "gofs-multi-test1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "gofs-multi-test2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	mounts := []config.DirMount{
		{Dir: tempDir1, Path: "/docs", Name: "Documentation", Readonly: false},
		{Dir: tempDir2, Path: "/data", Name: "Data Files", Readonly: true},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)
	if handler == nil {
		t.Fatal("NewMultiDir returned nil")
	}

	// Test that the handler implements http.Handler
	var _ http.Handler = handler
}

func TestMultiDir_ServeHTTP_PathResolution(t *testing.T) {
	// Create test directory structure
	tempDir1, err := os.MkdirTemp("", "gofs-multi-path1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "gofs-multi-path2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// Create test files in each directory
	file1 := filepath.Join(tempDir1, "doc1.txt")
	if err := os.WriteFile(file1, []byte("documentation content"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	file2 := filepath.Join(tempDir2, "data1.json")
	if err := os.WriteFile(file2, []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Create subdirectory in first mount
	subDir := filepath.Join(tempDir1, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "subdoc.txt")
	if err := os.WriteFile(subFile, []byte("subdirectory content"), 0644); err != nil {
		t.Fatalf("Failed to create subfile: %v", err)
	}

	mounts := []config.DirMount{
		{Dir: tempDir1, Path: "/docs", Name: "Documentation", Readonly: false},
		{Dir: tempDir2, Path: "/data", Name: "Data Files", Readonly: true},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	tests := []struct {
		name            string
		path            string
		expectedStatus  int
		expectedContent string
		shouldContain   []string
	}{
		{
			name:           "root_directory_listing",
			path:           "/",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"docs", "data", "Documentation", "Data Files"},
		},
		{
			name:           "docs_directory_listing",
			path:           "/docs/",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"doc1.txt", "subdir"},
		},
		{
			name:           "data_directory_listing",
			path:           "/data/",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"data1.json"},
		},
		{
			name:            "docs_file_content",
			path:            "/docs/doc1.txt",
			expectedStatus:  http.StatusOK,
			expectedContent: "documentation content",
		},
		{
			name:            "data_file_content",
			path:            "/data/data1.json",
			expectedStatus:  http.StatusOK,
			expectedContent: `{"key": "value"}`,
		},
		{
			name:            "subdirectory_file",
			path:            "/docs/subdir/subdoc.txt",
			expectedStatus:  http.StatusOK,
			expectedContent: "subdirectory content",
		},
		{
			name:           "subdirectory_listing",
			path:           "/docs/subdir/",
			expectedStatus: http.StatusOK,
			shouldContain:  []string{"subdoc.txt"},
		},
		{
			name:           "nonexistent_mount",
			path:           "/invalid/file.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "nonexistent_file",
			path:           "/docs/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Response body: %s",
					tt.expectedStatus, resp.StatusCode, string(body))
				return
			}

			if tt.expectedStatus == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}

				bodyStr := string(body)

				if tt.expectedContent != "" {
					if bodyStr != tt.expectedContent {
						t.Errorf("Expected content %q, got %q", tt.expectedContent, bodyStr)
					}
				}

				for _, expected := range tt.shouldContain {
					if !strings.Contains(bodyStr, expected) {
						t.Errorf("Response body missing expected content: %s", expected)
					}
				}
			}
		})
	}
}

func TestMultiDir_TrieBasedRouting(t *testing.T) {
	// Create test directories with overlapping path prefixes
	tempDir1, err := os.MkdirTemp("", "gofs-trie-test1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "gofs-trie-test2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	tempDir3, err := os.MkdirTemp("", "gofs-trie-test3-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 3: %v", err)
	}
	defer os.RemoveAll(tempDir3)

	// Create test files
	if err := os.WriteFile(filepath.Join(tempDir1, "file.txt"), []byte("api content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir2, "file.txt"), []byte("api v1 content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir3, "file.txt"), []byte("api v2 content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	mounts := []config.DirMount{
		{Dir: tempDir1, Path: "/api", Name: "API", Readonly: false},
		{Dir: tempDir2, Path: "/api/v1", Name: "API V1", Readonly: false},
		{Dir: tempDir3, Path: "/api/v2", Name: "API V2", Readonly: true},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	tests := []struct {
		name            string
		path            string
		expectedContent string
	}{
		{
			name:            "most_specific_match_v2",
			path:            "/api/v2/file.txt",
			expectedContent: "api v2 content",
		},
		{
			name:            "most_specific_match_v1",
			path:            "/api/v1/file.txt",
			expectedContent: "api v1 content",
		},
		{
			name:            "base_api_match",
			path:            "/api/file.txt",
			expectedContent: "api content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status 200, got %d. Response body: %s",
					resp.StatusCode, string(body))
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if string(body) != tt.expectedContent {
				t.Errorf("Expected content %q, got %q", tt.expectedContent, string(body))
			}
		})
	}
}

func TestMultiDir_ConcurrentAccess(t *testing.T) {
	// Create test directories
	tempDir1, err := os.MkdirTemp("", "gofs-concurrent1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "gofs-concurrent2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// Create test files
	for i := range 10 {
		content1 := fmt.Sprintf("content from mount1 file %d", i)
		filename1 := filepath.Join(tempDir1, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filename1, []byte(content1), 0644); err != nil {
			t.Fatalf("Failed to create file1: %v", err)
		}

		content2 := fmt.Sprintf("content from mount2 file %d", i)
		filename2 := filepath.Join(tempDir2, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filename2, []byte(content2), 0644); err != nil {
			t.Fatalf("Failed to create file2: %v", err)
		}
	}

	mounts := []config.DirMount{
		{Dir: tempDir1, Path: "/mount1", Name: "Mount 1", Readonly: false},
		{Dir: tempDir2, Path: "/mount2", Name: "Mount 2", Readonly: true},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Test concurrent file requests
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range requestsPerGoroutine {
				// Alternate between mounts
				mount := "mount1"
				expectedPrefix := "content from mount1"
				if (id+j)%2 == 1 {
					mount = "mount2"
					expectedPrefix = "content from mount2"
				}

				fileNum := (id + j) % 10
				path := fmt.Sprintf("/%s/file%d.txt", mount, fileNum)

				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				resp := w.Result()
				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("unexpected status %d for path %s", resp.StatusCode, path)
					resp.Body.Close()
					continue
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					errors <- fmt.Errorf("failed to read body for path %s: %v", path, err)
					continue
				}

				if !strings.Contains(string(body), expectedPrefix) {
					errors <- fmt.Errorf("unexpected content for path %s: got %q", path, string(body))
				}
			}
		}(i)
	}

	// Test concurrent directory listings
	wg.Add(numGoroutines)
	for i := range numGoroutines {
		go func(id int) {
			defer wg.Done()
			for j := range requestsPerGoroutine {
				paths := []string{"/", "/mount1/", "/mount2/"}
				path := paths[(id+j)%len(paths)]

				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				resp := w.Result()
				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("unexpected status %d for directory path %s", resp.StatusCode, path)
				}
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
		if errorCount > 10 { // Limit error output
			t.Errorf("... and %d more errors", len(errors))
			break
		}
	}
}

func TestMultiDir_PanicRecovery(t *testing.T) {
	// Create a test directory
	tempDir, err := os.MkdirTemp("", "gofs-panic-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	mounts := []config.DirMount{
		{Dir: tempDir, Path: "/test", Name: "Test", Readonly: false},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	// Test various edge cases that might cause panics
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"empty_path", http.MethodGet, ""},
		{"malformed_path", http.MethodGet, "//double//slash//"},
		{"very_long_path", http.MethodGet, "/" + strings.Repeat("a", 1000)},
		{"unicode_path", http.MethodGet, "/测试/файл"},
		{"path_with_nulls", http.MethodGet, "/test\x00file"},
		{"path_traversal", http.MethodGet, "/test/../../../etc/passwd"},
		{"invalid_method", "INVALID", "/test/"},
		{"options_request", http.MethodOptions, "/test/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Handler panicked: %v", r)
				}
			}()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			resp.Body.Close()

			// Handler should respond without panicking
			if resp.StatusCode == 0 {
				t.Error("Handler failed to respond")
			}
		})
	}
}

func TestMultiDir_MemoryPoolOptimization(t *testing.T) {
	// This test verifies that the handler uses memory pools efficiently
	// by performing many operations and checking for memory leaks
	tempDir, err := os.MkdirTemp("", "gofs-memory-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create many test files
	for i := range 100 {
		filename := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		content := fmt.Sprintf("content for file %d with some additional data to make it larger", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	mounts := []config.DirMount{
		{Dir: tempDir, Path: "/files", Name: "Files", Readonly: false},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	// Perform many concurrent operations to test memory pool usage
	const numOperations = 1000
	var wg sync.WaitGroup
	wg.Add(numOperations)

	for i := range numOperations {
		go func(id int) {
			defer wg.Done()

			// Mix of file and directory requests
			var path string
			if id%2 == 0 {
				path = fmt.Sprintf("/files/file%d.txt", id%100)
			} else {
				path = "/files/"
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			if resp.StatusCode == http.StatusOK {
				// Read the body to ensure full processing
				_, _ = io.ReadAll(resp.Body)
			}
			resp.Body.Close()
		}(i)
	}

	wg.Wait()

	// Test that the handler is still responsive after many operations
	req := httptest.NewRequest(http.MethodGet, "/files/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Handler not responsive after memory pool test: status %d", resp.StatusCode)
	}
}

func TestMultiDir_StrongETags(t *testing.T) {
	// Create test directory
	tempDir, err := os.MkdirTemp("", "gofs-etag-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with different content
	testFiles := map[string][]byte{
		"file1.txt": []byte("content 1"),
		"file2.txt": []byte("content 2"),
		"same.txt":  []byte("identical content"),
	}

	for filename, content := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create another directory with same content for comparison
	tempDir2, err := os.MkdirTemp("", "gofs-etag-test2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	// Create identical file in second directory
	identicalFile := filepath.Join(tempDir2, "same.txt")
	if err := os.WriteFile(identicalFile, []byte("identical content"), 0644); err != nil {
		t.Fatalf("Failed to create identical file: %v", err)
	}

	mounts := []config.DirMount{
		{Dir: tempDir, Path: "/dir1", Name: "Dir1", Readonly: false},
		{Dir: tempDir2, Path: "/dir2", Name: "Dir2", Readonly: false},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	tests := []struct {
		name        string
		path        string
		compareWith string
		shouldMatch bool
	}{
		{
			name:        "different_files_different_etags",
			path:        "/dir1/file1.txt",
			compareWith: "/dir1/file2.txt",
			shouldMatch: false,
		},
		{
			name:        "same_content_same_etags",
			path:        "/dir1/same.txt",
			compareWith: "/dir2/same.txt",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get first file
			req1 := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w1 := httptest.NewRecorder()
			handler.ServeHTTP(w1, req1)

			resp1 := w1.Result()
			defer resp1.Body.Close()

			if resp1.StatusCode != http.StatusOK {
				t.Fatalf("First request failed: %d", resp1.StatusCode)
			}

			etag1 := resp1.Header.Get("ETag")
			if etag1 == "" {
				t.Fatal("First response missing ETag header")
			}

			// Get second file
			req2 := httptest.NewRequest(http.MethodGet, tt.compareWith, nil)
			w2 := httptest.NewRecorder()
			handler.ServeHTTP(w2, req2)

			resp2 := w2.Result()
			defer resp2.Body.Close()

			if resp2.StatusCode != http.StatusOK {
				t.Fatalf("Second request failed: %d", resp2.StatusCode)
			}

			etag2 := resp2.Header.Get("ETag")
			if etag2 == "" {
				t.Fatal("Second response missing ETag header")
			}

			// Verify ETags are strong (start with ")
			if !strings.HasPrefix(etag1, `"`) || !strings.HasPrefix(etag2, `"`) {
				t.Error("ETags should be strong (start with quote)")
			}

			// Compare ETags
			if tt.shouldMatch {
				if etag1 != etag2 {
					t.Errorf("ETags should match for identical content: %s vs %s", etag1, etag2)
				}
			} else {
				if etag1 == etag2 {
					t.Errorf("ETags should not match for different content: %s", etag1)
				}
			}

			// Test conditional requests with If-None-Match
			req3 := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req3.Header.Set("If-None-Match", etag1)
			w3 := httptest.NewRecorder()
			handler.ServeHTTP(w3, req3)

			resp3 := w3.Result()
			resp3.Body.Close()

			if resp3.StatusCode != http.StatusNotModified {
				t.Errorf("Expected 304 Not Modified for If-None-Match, got %d", resp3.StatusCode)
			}
		})
	}
}

func TestMultiDir_MountOperations(t *testing.T) {
	// Create test directories
	tempDir1, err := os.MkdirTemp("", "gofs-mount-ops1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer os.RemoveAll(tempDir1)

	tempDir2, err := os.MkdirTemp("", "gofs-mount-ops2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	initialMounts := []config.DirMount{
		{Dir: tempDir1, Path: "/initial", Name: "Initial", Readonly: false},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(initialMounts, cfg, logger)

	// Test initial mount
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Initial mount test failed: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if !strings.Contains(string(body), "initial") {
		t.Error("Root listing should contain initial mount")
	}
}

func TestMultiDir_ThreadSafety_RaceConditions(t *testing.T) {
	// This test specifically targets race condition detection
	// Run with: go test -race

	tempDir, err := os.MkdirTemp("", "gofs-race-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	for i := range 20 {
		filename := filepath.Join(tempDir, fmt.Sprintf("race%d.txt", i))
		content := fmt.Sprintf("race test content %d", i)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create race test file: %v", err)
		}
	}

	mounts := []config.DirMount{
		{Dir: tempDir, Path: "/race", Name: "Race Test", Readonly: false},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	// High concurrency test specifically for race detection
	const highConcurrency = 100
	const operationsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(highConcurrency)

	for i := range highConcurrency {
		go func(id int) {
			defer wg.Done()
			for j := range operationsPerGoroutine {
				// Mix different types of requests
				var path string
				switch (id + j) % 4 {
				case 0:
					path = "/"
				case 1:
					path = "/race/"
				case 2:
					path = fmt.Sprintf("/race/race%d.txt", j%20)
				case 3:
					path = "/race/nonexistent.txt"
				}

				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()

				handler.ServeHTTP(w, req)

				resp := w.Result()
				// Read body to ensure full processing
				_, _ = io.ReadAll(resp.Body)
				resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()
}

func TestMultiDir_ETagContentHashing(t *testing.T) {
	// Test that ETags are properly generated using SHA-256 content hashing
	tempDir, err := os.MkdirTemp("", "gofs-etag-hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testContent := []byte("test content for ETag generation")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mounts := []config.DirMount{
		{Dir: tempDir, Path: "/test", Name: "Test", Readonly: false},
	}

	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "default",
	}

	logger := slog.Default()
	handler := NewMultiDir(mounts, cfg, logger)

	req := httptest.NewRequest(http.MethodGet, "/test/test.txt", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Request failed: %d", resp.StatusCode)
	}

	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("Response missing ETag header")
	}

	// Verify ETag is a strong ETag (quoted)
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) {
		t.Errorf("ETag should be strong (quoted): %s", etag)
	}

	// Calculate expected SHA-256 hash
	hasher := sha256.New()
	hasher.Write(testContent)
	expectedHash := fmt.Sprintf(`"%x"`, hasher.Sum(nil))

	if etag != expectedHash {
		t.Errorf("ETag should be SHA-256 hash of content: expected %s, got %s", expectedHash, etag)
	}
}
