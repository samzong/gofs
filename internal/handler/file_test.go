package handler

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
