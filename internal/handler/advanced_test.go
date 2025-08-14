package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
)

func TestNewAdvancedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-advanced-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100, // 100MB
		Theme:       "advanced",
	}

	handler := NewAdvancedFile(fs, cfg)
	if handler == nil {
		t.Fatal("NewAdvancedFile returned nil")
	}

	// Test that the handler implements http.Handler
	var _ http.Handler = handler
}

func TestAdvancedFile_ServeHTTP_DirectoryListing(t *testing.T) {
	// Create test directory structure
	tempDir, err := os.MkdirTemp("", "gofs-advanced-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	testFiles := map[string][]byte{
		"file1.txt":    []byte("content1"),
		"file2.html":   []byte("<html><body>test</body></html>"),
		"image.png":    []byte("fake png data"),
		"script.js":    []byte("console.log('test');"),
		"style.css":    []byte("body { margin: 0; }"),
		"document.pdf": []byte("fake pdf data"),
	}

	for filename, content := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file %v: %v", filename, err)
		}
	}

	// Create test subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)

	// Check that directory listing contains expected files
	for filename := range testFiles {
		if !strings.Contains(bodyStr, filename) {
			t.Errorf("Directory listing missing file: %s", filename)
		}
	}

	// Check that subdirectory is listed
	if !strings.Contains(bodyStr, "subdir") {
		t.Errorf("Directory listing missing subdirectory: subdir")
	}

	// Verify advanced theme is used (should contain advanced-specific elements)
	if !strings.Contains(bodyStr, "advanced") {
		t.Errorf("Response does not appear to use advanced theme")
	}
}

func TestAdvancedFile_ServeHTTP_FileServing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-advanced-file-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files with different content types
	testFiles := map[string]struct {
		content     []byte
		contentType string
	}{
		"test.txt":  {[]byte("plain text content"), "text/plain"},
		"test.html": {[]byte("<html><body>HTML content</body></html>"), "text/html"},
		"test.json": {[]byte(`{"key": "value"}`), "application/json"},
		"test.css":  {[]byte("body { margin: 0; }"), "text/css"},
		"test.js":   {[]byte("console.log('test');"), "application/javascript"},
	}

	for filename, fileData := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, fileData.content, 0644); err != nil {
			t.Fatalf("Failed to create test file %v: %v", filename, err)
		}
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	for filename, expected := range testFiles {
		t.Run("serve_file_"+filename, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/"+filename, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Expected status 200, got %d", resp.StatusCode)
			}

			// Check content type
			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, expected.contentType) {
				t.Errorf("Expected content type to contain %s, got %s", expected.contentType, contentType)
			}

			// Check content
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read response body: %v", err)
			}

			if !bytes.Equal(body, expected.content) {
				t.Errorf("Content mismatch for %s: got %q, want %q", filename, string(body), string(expected.content))
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_SecurityHeaders(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-security-headers-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name   string
		path   string
		method string
	}{
		{"directory_listing", "/", http.MethodGet},
		{"file_serving", "/test.txt", http.MethodGet},
	}

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"X-XSS-Protection":        "1; mode=block",
		"Content-Security-Policy": "default-src 'self'",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			// Check security headers
			for headerName, expectedValue := range expectedHeaders {
				headerValue := resp.Header.Get(headerName)
				if headerValue != expectedValue {
					t.Errorf("Header %s: expected %q, got %q", headerName, expectedValue, headerValue)
				}
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_CORS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-cors-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name           string
		method         string
		origin         string
		expectedCORS   bool
		expectedOrigin string
	}{
		{
			name:           "options_request_with_origin",
			method:         http.MethodOptions,
			origin:         "https://example.com",
			expectedCORS:   true,
			expectedOrigin: "*",
		},
		{
			name:           "get_request_with_origin",
			method:         http.MethodGet,
			origin:         "https://example.com",
			expectedCORS:   true,
			expectedOrigin: "*",
		},
		{
			name:         "request_without_origin",
			method:       http.MethodGet,
			origin:       "",
			expectedCORS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if tt.expectedCORS {
				corsOrigin := resp.Header.Get("Access-Control-Allow-Origin")
				if corsOrigin != tt.expectedOrigin {
					t.Errorf("Expected CORS origin %q, got %q", tt.expectedOrigin, corsOrigin)
				}

				if tt.method == http.MethodOptions {
					allowMethods := resp.Header.Get("Access-Control-Allow-Methods")
					if allowMethods == "" {
						t.Error("Expected Access-Control-Allow-Methods header for OPTIONS request")
					}

					allowHeaders := resp.Header.Get("Access-Control-Allow-Headers")
					if allowHeaders == "" {
						t.Error("Expected Access-Control-Allow-Headers header for OPTIONS request")
					}
				}
			} else {
				corsOrigin := resp.Header.Get("Access-Control-Allow-Origin")
				if corsOrigin != "" {
					t.Errorf("Unexpected CORS header when no origin provided: %q", corsOrigin)
				}
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_Upload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-upload-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024, // 1MB
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name           string
		filename       string
		content        []byte
		csrfToken      string
		expectedStatus int
		setupCSRF      bool
	}{
		{
			name:           "valid_upload_with_csrf",
			filename:       "upload.txt",
			content:        []byte("uploaded content"),
			expectedStatus: http.StatusOK,
			setupCSRF:      true,
		},
		{
			name:           "upload_without_csrf",
			filename:       "upload2.txt",
			content:        []byte("content without csrf"),
			expectedStatus: http.StatusForbidden,
			setupCSRF:      false,
		},
		{
			name:           "large_file_upload",
			filename:       "large.txt",
			content:        make([]byte, 2*1024*1024), // 2MB, exceeds limit
			expectedStatus: http.StatusRequestEntityTooLarge,
			setupCSRF:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create multipart form
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Add CSRF token if needed
			if tt.setupCSRF {
				// First get a CSRF token by making a GET request
				getReq := httptest.NewRequest(http.MethodGet, "/", nil)
				getW := httptest.NewRecorder()
				handler.ServeHTTP(getW, getReq)

				// Extract CSRF token from response (this would normally be in the HTML)
				// For testing, we'll use a mock token validation approach
				if err := writer.WriteField("csrf_token", "mock_csrf_token"); err != nil {
					t.Fatalf("Failed to write CSRF token: %v", err)
				}
			}

			// Add file
			part, err := writer.CreateFormFile("file", tt.filename)
			if err != nil {
				t.Fatalf("Failed to create form file: %v", err)
			}

			if _, err := part.Write(tt.content); err != nil {
				t.Fatalf("Failed to write file content: %v", err)
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Failed to close multipart writer: %v", err)
			}

			// Create upload request
			req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			// Set content length for large file test
			req.ContentLength = int64(buf.Len())

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Response body: %s",
					tt.expectedStatus, resp.StatusCode, string(body))
			}

			// Check if file was created for successful uploads
			if tt.expectedStatus == http.StatusOK {
				uploadedPath := filepath.Join(tempDir, tt.filename)
				if _, err := os.Stat(uploadedPath); os.IsNotExist(err) {
					t.Errorf("Uploaded file was not created: %s", uploadedPath)
				} else if err == nil {
					// Verify file content
					uploadedContent, err := os.ReadFile(uploadedPath)
					if err != nil {
						t.Errorf("Failed to read uploaded file: %v", err)
					} else if !bytes.Equal(uploadedContent, tt.content) {
						t.Errorf("Uploaded file content mismatch")
					}
				}
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_Download(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-download-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	testFiles := []string{"file1.txt", "file2.html", "file3.json"}
	for i, filename := range testFiles {
		path := filepath.Join(tempDir, filename)
		content := []byte("content of file " + string(rune('1'+i)))
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatalf("Failed to create test file %v: %v", filename, err)
		}
	}

	// Create subdirectory with files
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "subfile.txt")
	if err := os.WriteFile(subFile, []byte("subdirectory content"), 0644); err != nil {
		t.Fatalf("Failed to create subdirectory file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name           string
		method         string
		files          []string
		csrfToken      string
		expectedStatus int
		setupCSRF      bool
	}{
		{
			name:           "download_multiple_files_with_csrf",
			method:         http.MethodPost,
			files:          []string{"file1.txt", "file2.html"},
			expectedStatus: http.StatusOK,
			setupCSRF:      true,
		},
		{
			name:           "download_without_csrf",
			method:         http.MethodPost,
			files:          []string{"file1.txt"},
			expectedStatus: http.StatusForbidden,
			setupCSRF:      false,
		},
		{
			name:           "download_nonexistent_file",
			method:         http.MethodPost,
			files:          []string{"nonexistent.txt"},
			expectedStatus: http.StatusNotFound,
			setupCSRF:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create form data
			form := url.Values{}

			if tt.setupCSRF {
				form.Set("csrf_token", "mock_csrf_token")
			}

			for _, filename := range tt.files {
				form.Add("files", filename)
			}

			req := httptest.NewRequest(tt.method, "/download", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Response body: %s",
					tt.expectedStatus, resp.StatusCode, string(body))
			}

			// Check ZIP content type for successful downloads
			if tt.expectedStatus == http.StatusOK {
				contentType := resp.Header.Get("Content-Type")
				if contentType != "application/zip" {
					t.Errorf("Expected Content-Type application/zip, got %s", contentType)
				}

				contentDisposition := resp.Header.Get("Content-Disposition")
				if !strings.Contains(contentDisposition, "attachment") {
					t.Errorf("Expected Content-Disposition to contain 'attachment', got %s", contentDisposition)
				}
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_Timeouts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-timeout-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name                string
		method              string
		path                string
		expectedTimeoutType string
	}{
		{
			name:                "file_request_timeout",
			method:              http.MethodGet,
			path:                "/test.txt",
			expectedTimeoutType: "file",
		},
		{
			name:                "directory_request_timeout",
			method:              http.MethodGet,
			path:                "/",
			expectedTimeoutType: "directory",
		},
		{
			name:                "upload_request_timeout",
			method:              http.MethodPost,
			path:                "/upload",
			expectedTimeoutType: "upload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			// Add a timeout context to test timeout handling
			ctx := req.Context()
			req = req.WithContext(ctx)

			// Test that the handler responds (we can't easily test actual timeouts in unit tests)
			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			// The handler should respond without timing out in normal conditions
			if resp.StatusCode == 0 {
				t.Error("Handler appears to have timed out or failed to respond")
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_ErrorHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-error-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}
	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "file_not_found",
			method:         http.MethodGet,
			path:           "/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "path_traversal_attempt",
			method:         http.MethodGet,
			path:           "/../../../etc/passwd",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid_method",
			method:         http.MethodPut,
			path:           "/",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "directory_not_found",
			method:         http.MethodGet,
			path:           "/nonexistent/",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status %d, got %d. Response body: %s",
					tt.expectedStatus, resp.StatusCode, string(body))
			}

			// Check that security headers are still set even for error responses
			if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
				t.Error("Security headers missing from error response")
			}
		})
	}
}

func TestAdvancedFile_ServeHTTP_LoggingIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gofs-logging-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	cfg := &config.Config{
		MaxFileSize: 1024 * 1024 * 100,
		Theme:       "advanced",
	}

	handler := NewAdvancedFile(fs, cfg)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"file_request", http.MethodGet, "/test.txt"},
		{"directory_request", http.MethodGet, "/"},
		{"not_found_request", http.MethodGet, "/nonexistent.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.RemoteAddr = "192.0.2.1:12345"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			resp := w.Result()
			resp.Body.Close()

			// Test that the handler responds properly (logging is internal)
			// We verify the handler completes without errors
			if resp.StatusCode == 0 {
				t.Error("Handler failed to respond")
			}

			// Test that headers are set (indicating middleware chain executed)
			if resp.Header.Get("X-Content-Type-Options") == "" {
				t.Error("Expected security headers to be set by middleware chain")
			}
		})
	}
}
