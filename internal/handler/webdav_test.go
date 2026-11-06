package handler

import (
	"encoding/xml"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/samzong/gofs/internal"
	"github.com/samzong/gofs/internal/config"
	"github.com/samzong/gofs/internal/filesystem"
)

func TestNewWebDAV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()

	webdavHandler := NewWebDAV(fs, cfg, logger)

	if webdavHandler == nil {
		t.Fatal("NewWebDAV returned nil")
	}

	if webdavHandler.config != cfg {
		t.Error("Config not set correctly")
	}

	if webdavHandler.logger != logger {
		t.Error("Logger not set correctly")
	}

	if webdavHandler.prefix != "/dav" {
		t.Errorf("Expected prefix '/dav', got %q", webdavHandler.prefix)
	}

	if webdavHandler.handler == nil {
		t.Error("WebDAV handler not initialized")
	}
}

func TestWebDAV_BlockWriteOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	writeOperations := []string{
		"PUT", "DELETE", "MKCOL", "COPY", "MOVE", "PROPPATCH", "LOCK", "UNLOCK",
	}

	for _, method := range writeOperations {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/dav/test.txt", nil)
			w := httptest.NewRecorder()

			webdavHandler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status %d for %s, got %d",
					http.StatusMethodNotAllowed, method, w.Code)
			}

			if !strings.Contains(w.Body.String(), "Method Not Allowed - Read Only") {
				t.Errorf("Expected 'Method Not Allowed - Read Only' in response body, got %q",
					w.Body.String())
			}
		})
	}
}

func TestWebDAV_OPTIONS(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	req := httptest.NewRequest("OPTIONS", "/dav/", nil)
	w := httptest.NewRecorder()

	webdavHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check WebDAV headers
	if w.Header().Get("DAV") != "1, 2" {
		t.Errorf("Expected DAV header '1, 2', got %q", w.Header().Get("DAV"))
	}

	if w.Header().Get("MS-Author-Via") != "DAV" {
		t.Errorf("Expected MS-Author-Via header 'DAV', got %q", w.Header().Get("MS-Author-Via"))
	}

	// Check allowed methods
	allowHeader := w.Header().Get("Allow")
	expectedMethods := []string{"OPTIONS", "GET", "HEAD", "PROPFIND"}
	for _, method := range expectedMethods {
		if !strings.Contains(allowHeader, method) {
			t.Errorf("Expected Allow header to contain %s, got %q", method, allowHeader)
		}
	}

	publicHeader := w.Header().Get("Public")
	for _, method := range expectedMethods {
		if !strings.Contains(publicHeader, method) {
			t.Errorf("Expected Public header to contain %s, got %q", method, publicHeader)
		}
	}
}

func TestWebDAV_PROPFIND(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"public.txt", "unknown"} {
		if err := os.WriteFile(root+"/"+name, []byte("hi\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(root+"/sub", 0755); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(8080, "localhost", root, "default", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewWebDAV(filesystem.NewLocal(root, false), cfg, slog.Default())
	for _, tc := range []struct {
		path  string
		depth string
	}{
		{"/dav/", "0"},
		{"/dav/", "1"},
		{"/dav/", "infinity"},
		{"/dav/unknown", "0"},
	} {
		t.Run(tc.path+tc.depth, func(t *testing.T) {
			req := httptest.NewRequest("PROPFIND", tc.path, nil)
			req.Header.Set("Depth", tc.depth)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusMultiStatus {
				t.Fatalf("PROPFIND status = %d, body = %q", w.Code, w.Body.String())
			}
			var result struct {
				XMLName xml.Name `xml:"DAV: multistatus"`
				Entries []struct {
					Href string `xml:"DAV: href"`
				} `xml:"DAV: response"`
			}
			decoder := xml.NewDecoder(w.Body)
			if err := decoder.Decode(&result); err != nil {
				t.Fatalf("Invalid multistatus: %v", err)
			}
			for {
				token, err := decoder.Token()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatalf("Invalid trailing XML: %v", err)
				}
				if text, ok := token.(xml.CharData); !ok || strings.TrimSpace(string(text)) != "" {
					t.Fatalf("Unexpected trailing XML: %v", token)
				}
			}
			var hrefs []string
			for _, entry := range result.Entries {
				hrefs = append(hrefs, entry.Href)
			}
			slices.Sort(hrefs)
			want := []string{tc.path}
			if tc.depth != "0" {
				want = append(want, "/dav/public.txt", "/dav/sub/", "/dav/unknown")
			}
			if !slices.Equal(hrefs, want) {
				t.Errorf("PROPFIND hrefs = %v, want %v", hrefs, want)
			}
			if tc.depth == "infinity" && req.Header.Get("Depth") != "1" {
				t.Error("Infinite depth was not limited to 1")
			}
		})
	}
}

func TestWebDAV_PROPFINDSkipsBadEntries(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir() + "/private.txt"
	if err := os.WriteFile(outside, []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"aaa.txt", "zzz.txt"} {
		if err := os.WriteFile(root+"/"+name, []byte("hi\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range map[string]string{"broken": root + "/missing", "outside": outside} {
		if err := os.Symlink(target, root+"/"+name); err != nil {
			t.Skipf("Cannot create symlink: %v", err)
		}
	}
	cfg, err := config.New(8080, "localhost", root, "default", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	h := NewWebDAV(filesystem.NewLocal(root, false), cfg, slog.Default())
	req := httptest.NewRequest("PROPFIND", "/dav/", nil)
	req.Header.Set("Depth", "1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMultiStatus {
		t.Fatalf("PROPFIND status = %d", w.Code)
	}
	var result struct {
		XMLName xml.Name `xml:"DAV: multistatus"`
		Hrefs   []string `xml:"response>href"`
	}
	decoder := xml.NewDecoder(strings.NewReader(strings.TrimSpace(w.Body.String())))
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if token, err := decoder.Token(); err != io.EOF {
		t.Fatalf("Trailing response data: token=%v, error=%v", token, err)
	}
	slices.Sort(result.Hrefs)
	want := []string{"/dav/", "/dav/aaa.txt", "/dav/zzz.txt"}
	if !slices.Equal(result.Hrefs, want) {
		t.Errorf("PROPFIND hrefs = %v, want %v", result.Hrefs, want)
	}
}

// setupWebDAVTestWithFile creates a temporary directory with a test file and returns
// the WebDAV handler and cleanup function
func setupWebDAVTestWithFile(t *testing.T) (*WebDAV, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Create test file
	testContent := "Hello, WebDAV world!"
	testFile := tempDir + "/test.txt"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		cleanup()
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		cleanup()
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	return webdavHandler, cleanup
}

func TestWebDAV_GET(t *testing.T) {
	h, cleanup := setupWebDAVTestWithFile(t)
	defer cleanup()
	for _, tc := range []struct {
		rangeHeader string
		status      int
		body        string
	}{
		{"", http.StatusOK, "Hello, WebDAV world!"},
		{"bytes=7-12", http.StatusPartialContent, "WebDAV"},
	} {
		t.Run(tc.rangeHeader, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/dav/test.txt", nil)
			req.Header.Set("Range", tc.rangeHeader)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status || w.Body.String() != tc.body {
				t.Errorf("GET = (%d, %q), want (%d, %q)", w.Code, w.Body.String(), tc.status, tc.body)
			}
			if tc.rangeHeader != "" && w.Header().Get("Content-Range") != "bytes 7-12/20" {
				t.Errorf("Content-Range = %q", w.Header().Get("Content-Range"))
			}
		})
	}
}

func TestWebDAV_HEAD(t *testing.T) {
	webdavHandler, cleanup := setupWebDAVTestWithFile(t)
	defer cleanup()

	req := httptest.NewRequest("HEAD", "/dav/test.txt", nil)
	w := httptest.NewRecorder()

	webdavHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Length") != "20" {
		t.Errorf("HEAD = (%d, %q, %q)", w.Code, w.Body.String(), w.Header().Get("Content-Length"))
	}

	// Check WebDAV headers are set
	if w.Header().Get("DAV") != "1, 2" {
		t.Errorf("Expected DAV header '1, 2', got %q", w.Header().Get("DAV"))
	}
}

func TestWebDAV_InvalidPrefix(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	invalidPaths := []string{
		"/invalid/test.txt",
		"/files/test.txt",
		"/test.txt",
		"/da/test.txt", // Missing 'v' at end
	}

	for _, path := range invalidPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()

			webdavHandler.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Errorf("Expected status %d for invalid path %s, got %d",
					http.StatusNotFound, path, w.Code)
			}
		})
	}
}

func TestWebDAV_NonexistentFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	req := httptest.NewRequest("GET", "/dav/nonexistent.txt", nil)
	w := httptest.NewRecorder()

	webdavHandler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestWebDAV_DirectoryListing(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test directory structure
	subDir := tempDir + "/subdir"
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create files in both directories
	testFile1 := tempDir + "/file1.txt"
	testFile2 := subDir + "/file2.txt"
	if err := os.WriteFile(testFile1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	tests := []struct {
		name string
		path string
	}{
		{"root directory", "/dav/"},
		{"subdirectory", "/dav/subdir/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("PROPFIND", tt.path, nil)
			req.Header.Set("Depth", "1")
			w := httptest.NewRecorder()

			webdavHandler.ServeHTTP(w, req)

			// Should get success response for directory listing
			if w.Code >= 400 {
				t.Errorf("Expected success status for directory %s, got %d", tt.path, w.Code)
			}

			// Check WebDAV headers
			if w.Header().Get("DAV") != "1, 2" {
				t.Errorf("Expected DAV header '1, 2', got %q", w.Header().Get("DAV"))
			}
		})
	}
}

func TestWebDAV_WithNilLogger(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)

	// Create handler with nil logger (NewWebDAV should handle this)
	webdavHandler := NewWebDAV(fs, cfg, nil)

	if webdavHandler == nil {
		t.Fatal("NewWebDAV returned nil with nil logger")
	}

	// The nil logger is problematic in the current implementation
	// This test documents the current behavior - nil logger causes panic
	// In a production system, you would want to handle nil logger gracefully

	// Test with OPTIONS which doesn't trigger the problematic logger call
	req := httptest.NewRequest("OPTIONS", "/dav/", nil)
	w := httptest.NewRecorder()

	webdavHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestWebDAV_ConcurrentRequests(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "webdav_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test file
	testFile := tempDir + "/concurrent.txt"
	if err := os.WriteFile(testFile, []byte("concurrent access test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg, err := config.New(8080, "localhost", tempDir, "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	fs := filesystem.NewLocal(tempDir, false)
	logger := slog.Default()
	webdavHandler := NewWebDAV(fs, cfg, logger)

	const numConcurrent = 10
	done := make(chan bool, numConcurrent)

	// Test concurrent GET requests
	for i := range numConcurrent {
		go func(id int) {
			req := httptest.NewRequest("GET", "/dav/concurrent.txt", nil)
			w := httptest.NewRecorder()
			webdavHandler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Concurrent request %d: expected status 200, got %d", id, w.Code)
			}
			done <- true
		}(i)
	}

	// Wait for all concurrent requests to complete
	for range numConcurrent {
		<-done
	}
}

// Mock FileSystem for testing error scenarios
type mockFileSystem struct {
	statError    error
	openError    error
	readDirError error
}

func (m *mockFileSystem) Open(_ string) (io.ReadCloser, error) {
	if m.openError != nil {
		return nil, m.openError
	}
	return io.NopCloser(strings.NewReader("mock content")), nil
}

func (m *mockFileSystem) Stat(name string) (internal.FileInfo, error) {
	if m.statError != nil {
		return nil, m.statError
	}
	return &mockFileInfo{name: name, isDir: false}, nil
}

func (m *mockFileSystem) ReadDir(name string) ([]internal.FileInfo, error) {
	if m.readDirError != nil {
		return nil, m.readDirError
	}
	return []internal.FileInfo{
		&mockFileInfo{name: "file1.txt", isDir: false},
		&mockFileInfo{name: "subdir", isDir: true},
	}, nil
}

func (m *mockFileSystem) Create(_ string) (io.WriteCloser, error) {
	return nil, os.ErrPermission
}

func (m *mockFileSystem) Mkdir(_ string, _ os.FileMode) error {
	return os.ErrPermission
}

func (m *mockFileSystem) Remove(_ string) error {
	return os.ErrPermission
}

type mockFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (m *mockFileInfo) Name() string {
	return m.name
}

func (m *mockFileInfo) Size() int64 {
	return m.size
}

func (m *mockFileInfo) IsDir() bool {
	return m.isDir
}

func (m *mockFileInfo) ModTime() time.Time {
	return time.Now()
}

func TestWebDAV_ErrorHandling(t *testing.T) {
	cfg, err := config.New(8080, "localhost", ".", "default", false, nil)
	if err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	logger := slog.Default()

	tests := []struct {
		name           string
		fs             internal.FileSystem
		path           string
		method         string
		expectedStatus int
	}{
		{
			name:           "Stat error returns not found",
			fs:             &mockFileSystem{statError: os.ErrNotExist},
			path:           "/dav/nonexistent.txt",
			method:         "GET",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Open error returns not found",
			fs:             &mockFileSystem{openError: os.ErrNotExist},
			path:           "/dav/file.txt",
			method:         "GET",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			webdavHandler := NewWebDAV(tt.fs, cfg, logger)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			webdavHandler.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
