package internal

import (
	"context"
	"net/http"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	err := &APIError{
		Code:    "test_error",
		Message: "This is a test error",
		Status:  http.StatusBadRequest,
	}

	if err.Error() != "This is a test error" {
		t.Errorf("Expected error message 'This is a test error', got %q", err.Error())
	}
}

func TestAPIError_WithStatus(t *testing.T) {
	err := &APIError{
		Code:    "test_error",
		Message: "Test message",
	}

	// Test chaining
	result := err.WithStatus(http.StatusNotFound)

	// Should return the same instance
	if result != err {
		t.Error("WithStatus should return the same instance for chaining")
	}

	// Should set the status
	if err.Status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, err.Status)
	}
}

func TestAPIError_WithDetails(t *testing.T) {
	err := &APIError{
		Code:    "test_error",
		Message: "Test message",
	}

	details := map[string]string{"field": "value"}

	// Test chaining
	result := err.WithDetails(details)

	// Should return the same instance
	if result != err {
		t.Error("WithDetails should return the same instance for chaining")
	}

	// Should set the details
	if err.Details == nil {
		t.Error("Expected details to be set")
	}
}

func TestAPIError_Chaining(t *testing.T) {
	details := map[string]any{
		"field":  "test",
		"reason": "validation failed",
	}

	err := &APIError{
		Code:    "validation_error",
		Message: "Validation failed",
	}
	err = err.WithStatus(http.StatusBadRequest).WithDetails(details)

	// Test that chaining worked
	if err.Status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, err.Status)
	}

	if err.Details == nil {
		t.Error("Expected details to be set correctly after chaining")
	}

	if err.Code != "validation_error" {
		t.Errorf("Expected code 'validation_error', got %q", err.Code)
	}

	if err.Message != "Validation failed" {
		t.Errorf("Expected message 'Validation failed', got %q", err.Message)
	}
}

func TestAPIError_JSONSerialization(t *testing.T) {
	err := &APIError{
		Code:    "not_found",
		Message: "Resource not found",
		Status:  http.StatusNotFound, // Should not be serialized due to json:"-"
		Details: map[string]string{"resource": "user"},
	}

	// Test that the struct can be used for JSON serialization
	// (We're not testing actual JSON here, just the struct fields)

	if err.Code != "not_found" {
		t.Errorf("Expected code 'not_found', got %q", err.Code)
	}

	if err.Message != "Resource not found" {
		t.Errorf("Expected message 'Resource not found', got %q", err.Message)
	}

	if err.Details == nil {
		t.Error("Expected details to be set")
	}

	detailsMap, ok := err.Details.(map[string]string)
	if !ok {
		t.Error("Expected details to be a map[string]string")
	} else if detailsMap["resource"] != "user" {
		t.Errorf("Expected details['resource'] to be 'user', got %q", detailsMap["resource"])
	}
}

func TestAPIError_ZeroValue(t *testing.T) {
	var err APIError

	if err.Code != "" {
		t.Errorf("Expected zero value Code to be empty, got %q", err.Code)
	}

	if err.Message != "" {
		t.Errorf("Expected zero value Message to be empty, got %q", err.Message)
	}

	if err.Status != 0 {
		t.Errorf("Expected zero value Status to be 0, got %d", err.Status)
	}

	if err.Details != nil {
		t.Error("Expected zero value Details to be nil")
	}

	// Zero value should return empty string for Error()
	if err.Error() != "" {
		t.Errorf("Expected zero value Error() to return empty string, got %q", err.Error())
	}
}

func TestMountInfo_Structure(t *testing.T) {
	info := MountInfo{
		Path:     "/data",
		Name:     "Data Files",
		Readonly: true,
	}

	if info.Path != "/data" {
		t.Errorf("Expected Path '/data', got %q", info.Path)
	}

	if info.Name != "Data Files" {
		t.Errorf("Expected Name 'Data Files', got %q", info.Name)
	}

	if !info.Readonly {
		t.Error("Expected Readonly to be true")
	}
}

func TestMountInfo_ZeroValue(t *testing.T) {
	var info MountInfo

	if info.Path != "" {
		t.Errorf("Expected zero value Path to be empty, got %q", info.Path)
	}

	if info.Name != "" {
		t.Errorf("Expected zero value Name to be empty, got %q", info.Name)
	}

	if info.Readonly {
		t.Error("Expected zero value Readonly to be false")
	}
}

func TestWithMountInfo(t *testing.T) {
	ctx := context.Background()

	// Test adding mount info to context
	newCtx := WithMountInfo(ctx, "/data", "Data Files", true)

	if newCtx == ctx {
		t.Error("WithMountInfo should return a new context, not the same one")
	}

	// Test retrieving mount info from context
	value := newCtx.Value(mountInfoKey)
	if value == nil {
		t.Error("Expected mount info to be present in context")
		return
	}

	info, ok := value.(MountInfo)
	if !ok {
		t.Error("Expected context value to be MountInfo type")
		return
	}

	if info.Path != "/data" {
		t.Errorf("Expected Path '/data', got %q", info.Path)
	}

	if info.Name != "Data Files" {
		t.Errorf("Expected Name 'Data Files', got %q", info.Name)
	}

	if !info.Readonly {
		t.Error("Expected Readonly to be true")
	}
}

func TestWithMountInfo_EmptyValues(t *testing.T) {
	ctx := context.Background()

	// Test with empty values
	newCtx := WithMountInfo(ctx, "", "", false)

	value := newCtx.Value(mountInfoKey)
	if value == nil {
		t.Error("Expected mount info to be present in context even with empty values")
		return
	}

	info, ok := value.(MountInfo)
	if !ok {
		t.Error("Expected context value to be MountInfo type")
		return
	}

	if info.Path != "" {
		t.Errorf("Expected empty Path, got %q", info.Path)
	}

	if info.Name != "" {
		t.Errorf("Expected empty Name, got %q", info.Name)
	}

	if info.Readonly {
		t.Error("Expected Readonly to be false")
	}
}

func TestContextKey_Uniqueness(t *testing.T) {
	ctx := context.Background()

	// Add mount info
	ctx1 := WithMountInfo(ctx, "/data1", "Data1", true)

	// Add different mount info to same context
	ctx2 := WithMountInfo(ctx, "/data2", "Data2", false)

	// Both contexts should have their respective values
	value1 := ctx1.Value(mountInfoKey)
	value2 := ctx2.Value(mountInfoKey)

	if value1 == nil || value2 == nil {
		t.Error("Both contexts should have mount info values")
		return
	}

	info1, ok1 := value1.(MountInfo)
	info2, ok2 := value2.(MountInfo)

	if !ok1 || !ok2 {
		t.Error("Both context values should be MountInfo type")
		return
	}

	// They should have different values
	if info1.Path == info2.Path {
		t.Error("Different contexts should have different mount info paths")
	}

	if info1.Name == info2.Name {
		t.Error("Different contexts should have different mount info names")
	}

	if info1.Readonly == info2.Readonly {
		t.Error("Different contexts should have different mount info readonly values")
	}
}

func TestMountInfoKey_String(t *testing.T) {
	// Test that the context key has the expected string value
	key := mountInfoKey
	if string(key) != "mount_info" {
		t.Errorf("Expected context key string to be 'mount_info', got %q", string(key))
	}
}

func TestContextKey_TypeSafety(t *testing.T) {
	ctx := context.Background()

	// Test that using a different key type doesn't retrieve the value
	wrongKey := "mount_info" // string instead of contextKey
	ctx = WithMountInfo(ctx, "/test", "Test", true)

	// Should not be able to retrieve with wrong key type
	value := ctx.Value(wrongKey)
	if value != nil {
		t.Error("Should not be able to retrieve mount info with wrong key type")
	}

	// Should be able to retrieve with correct key type
	correctValue := ctx.Value(mountInfoKey)
	if correctValue == nil {
		t.Error("Should be able to retrieve mount info with correct key type")
	}
}

// Test interfaces are correctly defined (compilation test)
func TestFileSystemInterface(t *testing.T) {
	// This test ensures the interface is properly defined
	// If this compiles, the interface definition is correct

	var fs FileSystem
	_ = fs // Use the variable to avoid unused variable error
	// Zero value of interface is nil - test passes by compilation
}

func TestFileInfoInterface(t *testing.T) {
	// This test ensures the interface is properly defined
	// If this compiles, the interface definition is correct

	var fi FileInfo
	_ = fi // Use the variable to avoid unused variable error
	// Zero value of interface is nil - test passes by compilation
}

// Test that APIError implements error interface
func TestAPIError_ImplementsError(t *testing.T) {
	var err error = &APIError{
		Message: "test error",
	}

	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got %q", err.Error())
	}
}

// Benchmark tests for performance
func BenchmarkAPIError_Error(b *testing.B) {
	err := &APIError{
		Code:    "benchmark_error",
		Message: "This is a benchmark error message",
		Status:  http.StatusInternalServerError,
	}

	b.ResetTimer()
	for range b.N {
		_ = err.Error()
	}
}

func BenchmarkAPIError_WithStatus(b *testing.B) {
	err := &APIError{
		Code:    "benchmark_error",
		Message: "Test message",
	}

	b.ResetTimer()
	for range b.N {
		_ = err.WithStatus(http.StatusBadRequest)
	}
}

func BenchmarkAPIError_WithDetails(b *testing.B) {
	err := &APIError{
		Code:    "benchmark_error",
		Message: "Test message",
	}
	details := map[string]string{"key": "value"}

	b.ResetTimer()
	for range b.N {
		_ = err.WithDetails(details)
	}
}

func BenchmarkWithMountInfo(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	for range b.N {
		_ = WithMountInfo(ctx, "/data", "Data", true)
	}
}

func BenchmarkMountInfo_ContextRetrieval(b *testing.B) {
	ctx := WithMountInfo(context.Background(), "/data", "Data", true)

	b.ResetTimer()
	for range b.N {
		_ = ctx.Value(mountInfoKey)
	}
}
