package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/samzong/gofs/internal/config"
)

func TestSingleReadonlyMountRejectsWrites(t *testing.T) {
	root := t.TempDir()
	cfg, err := config.New(0, "127.0.0.1", "", "advanced", false, []string{"/:" + root + ":ro"})
	if err != nil {
		t.Fatal(err)
	}
	h := createFileHandler(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, err := writer.CreateFormFile("file", "uploaded.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, "hello"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	for _, operation := range []struct {
		name        string
		endpoint    string
		contentType string
		body        io.Reader
		path        string
	}{
		{"upload", "/api/upload", writer.FormDataContentType(), &upload, "uploaded.txt"},
		{"folder", "/api/folder", "application/json", strings.NewReader(`{"path":"created"}`), "created"},
	} {
		t.Run(operation.name, func(t *testing.T) {
			csrf := httptest.NewRecorder()
			h.ServeHTTP(csrf, httptest.NewRequest(http.MethodGet, "/api/csrf", nil))
			if csrf.Code != http.StatusOK {
				t.Fatalf("CSRF request returned %d", csrf.Code)
			}
			var response struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(csrf.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, operation.endpoint, operation.body)
			req.Header.Set("Content-Type", operation.contentType)
			req.Header.Set("X-CSRF-Token", response.Token)
			result := httptest.NewRecorder()
			h.ServeHTTP(result, req)
			if result.Code < http.StatusBadRequest {
				t.Errorf("read-only %s accepted: HTTP %d, %s", operation.name, result.Code, result.Body.String())
			}
			if _, err := os.Stat(filepath.Join(root, operation.path)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("read-only %s changed the filesystem: stat error = %v", operation.name, err)
			}
		})
	}
}
