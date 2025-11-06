package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_DefaultConfiguration(t *testing.T) {
	config := SecurityConfig{
		EnableSecurity: false,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify standard security headers are always set
	expectedHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for headerName, expectedValue := range expectedHeaders {
		actualValue := rr.Header().Get(headerName)
		if actualValue != expectedValue {
			t.Errorf("Expected %s header %q, got %q", headerName, expectedValue, actualValue)
		}
	}

	// Content-Security-Policy should NOT be set when EnableSecurity is false
	csp := rr.Header().Get("Content-Security-Policy")
	if csp != "" {
		t.Errorf("Expected no Content-Security-Policy header when EnableSecurity is false, got %q", csp)
	}

	// Verify response body and status are unchanged
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	if body := rr.Body.String(); body != "test response" {
		t.Errorf("Expected body 'test response', got %q", body)
	}
}

func TestSecurityHeaders_EnabledSecurity(t *testing.T) {
	config := SecurityConfig{
		EnableSecurity: true,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify all security headers are set including CSP
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'", // Default CSP when enabled
	}

	for headerName, expectedValue := range expectedHeaders {
		actualValue := rr.Header().Get(headerName)
		if actualValue != expectedValue {
			t.Errorf("Expected %s header %q, got %q", headerName, expectedValue, actualValue)
		}
	}
}

func TestSecurityHeaders_CustomCSP(t *testing.T) {
	customCSP := "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'"
	config := SecurityConfig{
		EnableSecurity:        true,
		ContentSecurityPolicy: customCSP,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify custom CSP is used
	csp := rr.Header().Get("Content-Security-Policy")
	if csp != customCSP {
		t.Errorf("Expected custom Content-Security-Policy %q, got %q", customCSP, csp)
	}
}

func TestSecurityHeaders_EmptyCSPWithSecurity(t *testing.T) {
	config := SecurityConfig{
		EnableSecurity:        true,
		ContentSecurityPolicy: "", // Empty CSP should get default
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify default CSP is used when custom is empty
	expectedCSP := "default-src 'self'"
	csp := rr.Header().Get("Content-Security-Policy")
	if csp != expectedCSP {
		t.Errorf("Expected default Content-Security-Policy %q, got %q", expectedCSP, csp)
	}
}

func TestSecurityHeaders_MultipleRequests(t *testing.T) {
	config := SecurityConfig{
		EnableSecurity: true,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	// Make multiple requests to ensure headers are consistently set
	for i := range 5 {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/test-%d", i), nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		// Verify headers are present in each request
		expectedHeaders := []string{
			"X-Content-Type-Options",
			"X-Frame-Options",
			"Referrer-Policy",
			"Content-Security-Policy",
		}

		for _, headerName := range expectedHeaders {
			if value := rr.Header().Get(headerName); value == "" {
				t.Errorf("Request %d: Expected %s header to be set, got empty", i+1, headerName)
			}
		}
	}
}

func TestSecurityHeaders_DifferentHTTPMethods(t *testing.T) {
	config := SecurityConfig{
		EnableSecurity: true,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Verify security headers are set regardless of HTTP method
			expectedHeaders := []string{
				"X-Content-Type-Options",
				"X-Frame-Options",
				"Referrer-Policy",
				"Content-Security-Policy",
			}

			for _, headerName := range expectedHeaders {
				if value := rr.Header().Get(headerName); value == "" {
					t.Errorf("Method %s: Expected %s header to be set, got empty", method, headerName)
				}
			}
		})
	}
}

func TestWriteJSON_Success(t *testing.T) {
	testData := map[string]any{
		"message": "success",
		"status":  200,
		"data": map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	rr := httptest.NewRecorder()
	err := WriteJSON(rr, testData)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Verify status code (should be 200 by default)
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Verify JSON content
	var result map[string]any
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal response JSON: %v", err)
	}

	if result["message"] != "success" {
		t.Errorf("Expected message 'success', got %v", result["message"])
	}

	if result["status"] != float64(200) { // JSON numbers are float64
		t.Errorf("Expected status 200, got %v", result["status"])
	}
}

func TestWriteJSON_WithStruct(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	testData := TestStruct{
		Name:  "John Doe",
		Age:   30,
		Email: "john@example.com",
	}

	rr := httptest.NewRecorder()
	err := WriteJSON(rr, testData)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify JSON structure
	var result TestStruct
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal response JSON: %v", err)
	}

	if result != testData {
		t.Errorf("Expected %+v, got %+v", testData, result)
	}
}

func TestWriteJSON_WithSlice(t *testing.T) {
	testData := []string{"apple", "banana", "cherry"}

	rr := httptest.NewRecorder()
	err := WriteJSON(rr, testData)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	var result []string
	err = json.Unmarshal(rr.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal response JSON: %v", err)
	}

	if len(result) != len(testData) {
		t.Errorf("Expected slice length %d, got %d", len(testData), len(result))
	}

	for i, v := range result {
		if v != testData[i] {
			t.Errorf("Expected item %d to be %q, got %q", i, testData[i], v)
		}
	}
}

func TestWriteJSON_InvalidData(t *testing.T) {
	// Use a channel which cannot be marshaled to JSON
	testData := make(chan int)

	rr := httptest.NewRecorder()
	err := WriteJSON(rr, testData)

	// Should return an error
	if err == nil {
		t.Error("Expected error when marshaling invalid data, got nil")
	}

	// Should set 500 status and error message
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	if !strings.Contains(body, "JSON encoding failed") {
		t.Errorf("Expected error message 'JSON encoding failed', got %q", body)
	}
}

func TestWriteJSONError_BasicError(t *testing.T) {
	message := "Resource not found"
	statusCode := http.StatusNotFound

	rr := httptest.NewRecorder()
	WriteJSONError(rr, message, statusCode)

	// Verify status code
	if rr.Code != statusCode {
		t.Errorf("Expected status %d, got %d", statusCode, rr.Code)
	}

	// Verify Content-Type header
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got %q", contentType)
	}

	// Verify JSON error structure
	var result map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal error JSON: %v", err)
	}

	if result["error"] != message {
		t.Errorf("Expected error message %q, got %q", message, result["error"])
	}
}

func TestWriteJSONError_DifferentStatusCodes(t *testing.T) {
	testCases := []struct {
		name       string
		message    string
		statusCode int
	}{
		{"bad_request", "Invalid input", http.StatusBadRequest},
		{"unauthorized", "Authentication required", http.StatusUnauthorized},
		{"forbidden", "Access denied", http.StatusForbidden},
		{"not_found", "Resource not found", http.StatusNotFound},
		{"internal_error", "Internal server error", http.StatusInternalServerError},
		{"service_unavailable", "Service temporarily unavailable", http.StatusServiceUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			WriteJSONError(rr, tc.message, tc.statusCode)

			if rr.Code != tc.statusCode {
				t.Errorf("Expected status %d, got %d", tc.statusCode, rr.Code)
			}

			var result map[string]string
			err := json.Unmarshal(rr.Body.Bytes(), &result)
			if err != nil {
				t.Errorf("Failed to unmarshal error JSON: %v", err)
			}

			if result["error"] != tc.message {
				t.Errorf("Expected error message %q, got %q", tc.message, result["error"])
			}
		})
	}
}

func TestWriteJSONError_EmptyMessage(t *testing.T) {
	message := ""
	statusCode := http.StatusBadRequest

	rr := httptest.NewRecorder()
	WriteJSONError(rr, message, statusCode)

	var result map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal error JSON: %v", err)
	}

	if result["error"] != message {
		t.Errorf("Expected empty error message, got %q", result["error"])
	}
}

func TestSafeRequestPath_ValidPaths(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty_path", "", ""},
		{"root_path", "/", ""},
		{"simple_file", "/file.txt", "file.txt"},
		{"nested_file", "/dir/file.txt", "dir/file.txt"},
		{"deep_nested", "/a/b/c/d/file.txt", "a/b/c/d/file.txt"},
		{"with_spaces", "/dir with spaces/file.txt", "dir with spaces/file.txt"},
		{"unicode_path", "/测试/文件.txt", "测试/文件.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SafeRequestPath(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestSafeRequestPath_DangerousPaths(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"path_traversal_unix", "/../../etc/passwd"},
		{"path_traversal_windows", "/..\\..\\windows\\system32\\config"},
		{"relative_traversal", "/../../../etc/passwd"},
		{"complex_traversal", "/dir/../../../etc/passwd"},
		{"encoded_traversal", "/%2e%2e/etc/passwd"},
		{"null_byte", "/file\x00.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SafeRequestPath(tc.input)
			// All dangerous paths should return empty string
			if result != "" {
				t.Errorf("Expected empty string for dangerous path %q, got %q", tc.input, result)
			}
		})
	}
}

func TestSafeRequestPath_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"multiple_slashes", "///file.txt", ""}, // Multiple slashes are treated as dangerous
		{"trailing_slash", "/dir/", "dir"},
		{"dot_path", "/./file.txt", "file.txt"},
		{"absolute_without_leading_slash", "file.txt", "file.txt"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SafeRequestPath(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// Test the interaction between security headers and JSON writing
func TestSecurityHeaders_WithJSONResponse(t *testing.T) {
	config := SecurityConfig{
		EnableSecurity: true,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		data := map[string]string{
			"message": "test response",
			"status":  "success",
		}
		_ = WriteJSON(w, data)
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Verify both security headers and JSON content-type are present
	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "strict-origin-when-cross-origin",
		"Content-Security-Policy": "default-src 'self'",
		"Content-Type":            "application/json",
	}

	for headerName, expectedValue := range expectedHeaders {
		actualValue := rr.Header().Get(headerName)
		if actualValue != expectedValue {
			t.Errorf("Expected %s header %q, got %q", headerName, expectedValue, actualValue)
		}
	}

	// Verify JSON response
	var result map[string]string
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Failed to unmarshal response JSON: %v", err)
	}

	if result["message"] != "test response" {
		t.Errorf("Expected message 'test response', got %q", result["message"])
	}
}

// Benchmark security header performance
func BenchmarkSecurityHeaders_Enabled(b *testing.B) {
	config := SecurityConfig{
		EnableSecurity: true,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for range b.N {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkSecurityHeaders_Disabled(b *testing.B) {
	config := SecurityConfig{
		EnableSecurity: false,
	}

	middleware := SecurityHeaders(config)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()
	for range b.N {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkWriteJSON(b *testing.B) {
	testData := map[string]any{
		"message": "benchmark test",
		"count":   b.N,
		"data": []string{
			"item1", "item2", "item3", "item4", "item5",
		},
	}

	b.ResetTimer()
	for range b.N {
		rr := httptest.NewRecorder()
		_ = WriteJSON(rr, testData)
	}
}

func BenchmarkSafeRequestPath(b *testing.B) {
	testPaths := []string{
		"/simple/file.txt",
		"/deep/nested/path/to/file.txt",
		"/../../etc/passwd",
		"/dir with spaces/file.txt",
		"/测试/文件.txt",
	}

	b.ResetTimer()
	for i := range b.N {
		path := testPaths[i%len(testPaths)]
		SafeRequestPath(path)
	}
}
