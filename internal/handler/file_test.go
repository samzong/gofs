package handler

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
)

func TestFileHandler_RangeRequest(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "gofs-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with known content
	testContent := []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Setup the handler
	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100, // 100MB
		Theme:       "default",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	tests := []struct {
		name                string
		rangeHeader         string
		expectedStatus      int
		expectedBody        string
		expectedRangeHeader string
	}{
		{
			name:                "no range - full content",
			rangeHeader:         "",
			expectedStatus:      http.StatusOK,
			expectedBody:        string(testContent),
			expectedRangeHeader: "",
		},
		{
			name:                "first 10 bytes",
			rangeHeader:         "bytes=0-9",
			expectedStatus:      http.StatusPartialContent,
			expectedBody:        "0123456789",
			expectedRangeHeader: "bytes 0-9/62",
		},
		{
			name:                "middle 10 bytes",
			rangeHeader:         "bytes=10-19",
			expectedStatus:      http.StatusPartialContent,
			expectedBody:        "abcdefghij",
			expectedRangeHeader: "bytes 10-19/62",
		},
		{
			name:                "last 10 bytes",
			rangeHeader:         "bytes=-10",
			expectedStatus:      http.StatusPartialContent,
			expectedBody:        "QRSTUVWXYZ",
			expectedRangeHeader: "bytes 52-61/62",
		},
		{
			name:                "from byte 30 to end",
			rangeHeader:         "bytes=30-",
			expectedStatus:      http.StatusPartialContent,
			expectedBody:        "uvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			expectedRangeHeader: "bytes 30-61/62",
		},
		{
			name:                "invalid range - beyond file size",
			rangeHeader:         "bytes=100-200",
			expectedStatus:      http.StatusRequestedRangeNotSatisfiable,
			expectedBody:        "",
			expectedRangeHeader: "bytes */62",
		},
		{
			name:                "invalid range format - serve full content",
			rangeHeader:         "chunks=0-10",
			expectedStatus:      http.StatusOK,
			expectedBody:        string(testContent),
			expectedRangeHeader: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
			if tt.rangeHeader != "" {
				req.Header.Set("Range", tt.rangeHeader)
			}

			// Create response recorder
			recorder := httptest.NewRecorder()

			// Serve the request
			handler.ServeHTTP(recorder, req)

			// Get the response
			resp := recorder.Result()
			defer resp.Body.Close()

			// Check status code
			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			// Check Accept-Ranges header (should always be present for successful file requests)
			if tt.expectedStatus == http.StatusOK || tt.expectedStatus == http.StatusPartialContent {
				acceptRanges := resp.Header.Get("Accept-Ranges")
				if acceptRanges != "bytes" {
					t.Errorf("Expected Accept-Ranges: bytes, got %s", acceptRanges)
				}
			}

			// Check Content-Range header for partial content
			if tt.expectedStatus == http.StatusPartialContent || tt.expectedStatus == http.StatusRequestedRangeNotSatisfiable {
				contentRange := resp.Header.Get("Content-Range")
				if contentRange != tt.expectedRangeHeader {
					t.Errorf("Expected Content-Range: %s, got %s", tt.expectedRangeHeader, contentRange)
				}
			}

			// Check body content
			body, _ := io.ReadAll(resp.Body)
			if string(body) != tt.expectedBody {
				t.Errorf("Expected body: %q, got %q", tt.expectedBody, string(body))
			}
		})
	}
}

func TestFileHandler_RangeRequestLargeFile(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gofs-test-large-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a large test file (2MB)
	largeContent := make([]byte, 2*1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	testFile := filepath.Join(tempDir, "large.bin")
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	// Setup handler
	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 10 * 1024 * 1024, // 10MB
		Theme:       "default",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	// Test downloading different chunks
	tests := []struct {
		name        string
		rangeHeader string
		chunkSize   int
		startByte   int
	}{
		{
			name:        "first 1KB",
			rangeHeader: "bytes=0-1023",
			chunkSize:   1024,
			startByte:   0,
		},
		{
			name:        "middle 1KB",
			rangeHeader: "bytes=1048576-1049599",
			chunkSize:   1024,
			startByte:   1048576,
		},
		{
			name:        "last 1KB",
			rangeHeader: "bytes=-1024",
			chunkSize:   1024,
			startByte:   2*1024*1024 - 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/large.bin", nil)
			req.Header.Set("Range", tt.rangeHeader)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			// Should return partial content
			if resp.StatusCode != http.StatusPartialContent {
				t.Errorf("Expected status 206, got %d", resp.StatusCode)
			}

			// Check content length
			body, _ := io.ReadAll(resp.Body)
			if len(body) != tt.chunkSize {
				t.Errorf("Expected %d bytes, got %d", tt.chunkSize, len(body))
			}

			// Verify content matches expected bytes
			expectedContent := largeContent[tt.startByte : tt.startByte+tt.chunkSize]
			if !bytes.Equal(body, expectedContent) {
				t.Errorf("Content mismatch for range %s", tt.rangeHeader)
			}
		})
	}
}

func TestFileHandler_MultipleRangeNotSupported(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gofs-test-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testContent := []byte("test content")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Setup handler
	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024,
		Theme:       "default",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	// Test multiple ranges (not supported, should return full content)
	req := httptest.NewRequest(http.MethodGet, "/test.txt", nil)
	req.Header.Set("Range", "bytes=0-4,7-11")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	resp := recorder.Result()
	defer resp.Body.Close()

	// Should return full content (200 OK) since multiple ranges are not supported
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 for unsupported multiple ranges, got %d", resp.StatusCode)
	}

	// Should still have Accept-Ranges header
	acceptRanges := resp.Header.Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		t.Errorf("Expected Accept-Ranges: bytes, got %s", acceptRanges)
	}

	// Should receive full content
	body, _ := io.ReadAll(resp.Body)
	if string(body) != string(testContent) {
		t.Errorf("Expected full content, got %q", string(body))
	}
}

func TestFileHandler_DirectoryListing(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "gofs-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and subdirectories
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	file1 := filepath.Join(tempDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	file2 := filepath.Join(tempDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Create hidden file
	hiddenFile := filepath.Join(tempDir, ".hidden")
	if err := os.WriteFile(hiddenFile, []byte("hidden"), 0644); err != nil {
		t.Fatalf("Failed to create hidden file: %v", err)
	}

	tests := []struct {
		name             string
		showHidden       bool
		theme            string
		accept           string
		expectedType     string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:             "html_default_theme",
			showHidden:       false,
			theme:            "default",
			accept:           "text/html",
			expectedType:     "text/html",
			shouldContain:    []string{"file1.txt", "file2.txt", "subdir"},
			shouldNotContain: []string{".hidden"},
		},
		{
			name:             "html_classic_theme",
			showHidden:       false,
			theme:            "classic",
			accept:           "text/html",
			expectedType:     "text/html",
			shouldContain:    []string{"file1.txt", "file2.txt", "subdir"},
			shouldNotContain: []string{".hidden"},
		},
		{
			name:         "json_response",
			showHidden:   false,
			theme:        "default",
			accept:       "application/json",
			expectedType: "application/json",
			shouldContain: []string{
				`"name":"file1.txt"`, `"name":"file2.txt"`, `"name":"subdir"`, `"isDir":true`, `"isDir":false`,
			},
			shouldNotContain: []string{`"name":".hidden"`},
		},
		{
			name:             "show_hidden_files",
			showHidden:       true,
			theme:            "default",
			accept:           "text/html",
			expectedType:     "text/html",
			shouldContain:    []string{"file1.txt", "file2.txt", "subdir", ".hidden"},
			shouldNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := filesystem.NewLocal(tempDir, tt.showHidden)
			cfg := &config.Config{
				MaxFileSize: 1024 * 1024,
				Theme:       tt.theme,
				ShowHidden:  tt.showHidden,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := NewFile(fs, cfg, logger)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept", tt.accept)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, tt.expectedType) {
				t.Errorf("Expected content type %s, got %s", tt.expectedType, contentType)
			}

			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)

			for _, expected := range tt.shouldContain {
				if !strings.Contains(bodyStr, expected) {
					t.Errorf("Response body should contain %q", expected)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(bodyStr, notExpected) {
					t.Errorf("Response body should not contain %q", notExpected)
				}
			}
		})
	}
}

func TestFileHandler_SecurityHeaders(t *testing.T) {
	// Create a temporary directory and file
	tempDir, err := os.MkdirTemp("", "gofs-security-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize:    1024 * 1024,
		Theme:          "default",
		EnableSecurity: true,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	tests := []struct {
		name    string
		path    string
		headers map[string]string
	}{
		{
			name: "file_request_security_headers",
			path: "/test.txt",
			headers: map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
				"Content-Security-Policy": "default-src 'self'",
			},
		},
		{
			name: "directory_request_security_headers",
			path: "/",
			headers: map[string]string{
				"X-Content-Type-Options":  "nosniff",
				"X-Frame-Options":         "DENY",
				"Referrer-Policy":         "strict-origin-when-cross-origin",
				"Content-Security-Policy": "default-src 'self'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			for headerName, expectedValue := range tt.headers {
				actualValue := resp.Header.Get(headerName)
				if actualValue != expectedValue {
					t.Errorf("Expected %s header %q, got %q", headerName, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestFileHandler_ErrorHandling(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gofs-error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a large test file for size limit testing
	largeContent := make([]byte, 2*1024*1024) // 2MB
	largeFile := filepath.Join(tempDir, "large.txt")
	if err := os.WriteFile(largeFile, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024, // 1MB limit
		Theme:       "default",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	tests := []struct {
		name           string
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "file_not_found",
			path:           "/nonexistent.txt",
			method:         http.MethodGet,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "file_too_large",
			path:           "/large.txt",
			method:         http.MethodGet,
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
		{
			name:           "method_not_allowed",
			path:           "/",
			method:         http.MethodPost,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid_method_put",
			path:           "/",
			method:         http.MethodPut,
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "invalid_method_delete",
			path:           "/",
			method:         http.MethodDelete,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}
		})
	}
}

func TestFileHandler_ContentTypes(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "gofs-content-type-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with different extensions
	testFiles := map[string]struct {
		content     []byte
		contentType string
	}{
		"test.txt":  {[]byte("text content"), "text/plain"},
		"test.html": {[]byte("<html>content</html>"), "text/html"},
		"test.json": {[]byte(`{"key": "value"}`), "application/json"},
		"test.css":  {[]byte("body { color: red; }"), "text/css"},
		"test.js":   {[]byte("console.log('test');"), "text/javascript"},
		"image.png": {[]byte("PNG fake content"), "image/png"},
		"unknown":   {[]byte("unknown content"), "application/octet-stream"},
	}

	for filename, fileData := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, fileData.content, 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filename, err)
		}
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024,
		Theme:       "default",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	for filename, fileData := range testFiles {
		t.Run("content_type_"+filename, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+filename, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, fileData.contentType) {
				t.Errorf("Expected content type %s, got %s", fileData.contentType, contentType)
			}

			// Verify Content-Disposition header for file downloads
			contentDisposition := resp.Header.Get("Content-Disposition")
			if !strings.Contains(contentDisposition, "inline") {
				t.Errorf("Expected Content-Disposition to contain 'inline', got %s", contentDisposition)
			}
			if !strings.Contains(contentDisposition, filename) {
				t.Errorf("Expected Content-Disposition to contain filename %s, got %s", filename, contentDisposition)
			}
		})
	}
}

func TestFileHandler_PathSecurity(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "gofs-path-security-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "safe.txt")
	if err := os.WriteFile(testFile, []byte("safe content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024,
		Theme:       "default",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewFile(fs, cfg, logger)

	// Test various path traversal attempts - these should be sanitized by SafePath
	tests := []struct {
		name           string
		path           string
		expectedStatus int
		description    string
	}{
		{"path_traversal_unix", "../../../etc/passwd", http.StatusOK, "Sanitized to empty string, shows root directory"},
		{
			"path_traversal_windows", "..\\..\\..\\windows\\system32\\config\\sam",
			http.StatusOK, "Sanitized to empty string, shows root directory",
		},
		{"absolute_path", "/etc/passwd", http.StatusNotFound, "Becomes etc/passwd which doesn't exist"},
		{"relative_traversal", "./../safe.txt", http.StatusOK, "Sanitized to empty string, shows root directory"},
		{
			"complex_traversal", "./safe.txt/../../etc/passwd",
			http.StatusOK, "Sanitized to empty string, shows root directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+tt.path, nil)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, req)

			resp := recorder.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d for path %q (%s), got %d", tt.expectedStatus, tt.path, tt.description, resp.StatusCode)
			}
		})
	}

	// Verify that safe path still works
	t.Run("safe_path_works", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/safe.txt", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, req)

		resp := recorder.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200 for safe path, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		if string(body) != "safe content" {
			t.Errorf("Expected safe content, got %q", string(body))
		}
	})
}
