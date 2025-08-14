package constants

import (
	"testing"
	"time"
)

func TestDefaultConstants(t *testing.T) {
	// Test network defaults
	if DefaultHost != "127.0.0.1" {
		t.Errorf("Expected DefaultHost '127.0.0.1', got %q", DefaultHost)
	}

	if DefaultPort != 8000 {
		t.Errorf("Expected DefaultPort 8000, got %d", DefaultPort)
	}

	// Test file size limits
	expectedMaxFileSize := 100 << 20 // 100MB in bytes
	if DefaultMaxFileSize != expectedMaxFileSize {
		t.Errorf("Expected DefaultMaxFileSize %d bytes (100MB), got %d", expectedMaxFileSize, DefaultMaxFileSize)
	}

	if MaxUploadSize != expectedMaxFileSize {
		t.Errorf("Expected MaxUploadSize %d bytes (100MB), got %d", expectedMaxFileSize, MaxUploadSize)
	}

	// Test that MaxUploadSize and DefaultMaxFileSize are consistent
	if MaxUploadSize != DefaultMaxFileSize {
		t.Errorf("MaxUploadSize (%d) and DefaultMaxFileSize (%d) should be equal", MaxUploadSize, DefaultMaxFileSize)
	}
}

func TestTimeoutConstants(t *testing.T) {
	testCases := []struct {
		name     string
		actual   time.Duration
		expected time.Duration
	}{
		{"DefaultRequestTimeout", DefaultRequestTimeout, 30 * time.Second},
		{"ServerReadTimeout", ServerReadTimeout, 30 * time.Second},
		{"ServerWriteTimeout", ServerWriteTimeout, 30 * time.Second},
		{"ServerIdleTimeout", ServerIdleTimeout, 2 * time.Minute},
		{"UploadTimeout", UploadTimeout, 5 * time.Minute},
		{"FileServeTimeout", FileServeTimeout, 30 * time.Second},
		{"DirectoryTimeout", DirectoryTimeout, 10 * time.Second},
		{"TemplateTimeout", TemplateTimeout, 5 * time.Second},
		{"ShutdownTimeout", ShutdownTimeout, 5 * time.Second},
		{"HealthCheckTimeout", HealthCheckTimeout, 5 * time.Second},
		{"CSRFTokenExpiry", CSRFTokenExpiry, 1 * time.Hour},
		{"CSRFCleanupInterval", CSRFCleanupInterval, 5 * time.Minute},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.actual != tc.expected {
				t.Errorf("Expected %s to be %v, got %v", tc.name, tc.expected, tc.actual)
			}
		})
	}
}

func TestCacheAndBufferConstants(t *testing.T) {
	if StaticAssetCacheMaxAge != 3600 {
		t.Errorf("Expected StaticAssetCacheMaxAge 3600 seconds (1 hour), got %d", StaticAssetCacheMaxAge)
	}

	if DefaultPathBufferSize != 256 {
		t.Errorf("Expected DefaultPathBufferSize 256 bytes, got %d", DefaultPathBufferSize)
	}
}

func TestSecurityConstants(t *testing.T) {
	// Test bcrypt cost
	if BcryptCost != 12 {
		t.Errorf("Expected BcryptCost 12, got %d", BcryptCost)
	}

	// Validate bcrypt cost is reasonable (between 10 and 15)
	if BcryptCost < 10 || BcryptCost > 15 {
		t.Errorf("BcryptCost (%d) should be between 10 and 15 for good security/performance balance", BcryptCost)
	}
}

func TestTimeoutHierarchy(t *testing.T) {
	// Test that timeout values make sense relative to each other

	// Upload timeout should be longer than other operations
	if UploadTimeout <= DefaultRequestTimeout {
		t.Errorf("UploadTimeout (%v) should be longer than DefaultRequestTimeout (%v)",
			UploadTimeout, DefaultRequestTimeout)
	}

	if UploadTimeout <= FileServeTimeout {
		t.Errorf("UploadTimeout (%v) should be longer than FileServeTimeout (%v)",
			UploadTimeout, FileServeTimeout)
	}

	// Directory operations should be faster than file operations
	if DirectoryTimeout > FileServeTimeout {
		t.Errorf("DirectoryTimeout (%v) should not exceed FileServeTimeout (%v)",
			DirectoryTimeout, FileServeTimeout)
	}

	// Template operations should be very fast
	if TemplateTimeout > DirectoryTimeout {
		t.Errorf("TemplateTimeout (%v) should not exceed DirectoryTimeout (%v)",
			TemplateTimeout, DirectoryTimeout)
	}

	// Server timeouts should accommodate request timeouts
	if ServerReadTimeout < DefaultRequestTimeout {
		t.Errorf("ServerReadTimeout (%v) should be at least DefaultRequestTimeout (%v)",
			ServerReadTimeout, DefaultRequestTimeout)
	}

	if ServerWriteTimeout < DefaultRequestTimeout {
		t.Errorf("ServerWriteTimeout (%v) should be at least DefaultRequestTimeout (%v)",
			ServerWriteTimeout, DefaultRequestTimeout)
	}
}

func TestCSRFConstants(t *testing.T) {
	// CSRF token expiry should be longer than cleanup interval
	if CSRFTokenExpiry <= CSRFCleanupInterval {
		t.Errorf("CSRFTokenExpiry (%v) should be longer than CSRFCleanupInterval (%v)",
			CSRFTokenExpiry, CSRFCleanupInterval)
	}

	// Cleanup should happen multiple times before tokens expire
	minCleanupRounds := 3
	if CSRFTokenExpiry < time.Duration(minCleanupRounds)*CSRFCleanupInterval {
		t.Errorf("CSRFTokenExpiry (%v) should allow at least %d cleanup rounds (interval: %v)",
			CSRFTokenExpiry, minCleanupRounds, CSRFCleanupInterval)
	}
}

func TestFileSizeLimits(t *testing.T) {
	// Test that file size constants are reasonable
	oneMB := 1 << 20
	hundredMB := 100 << 20

	if DefaultMaxFileSize != hundredMB {
		t.Errorf("Expected DefaultMaxFileSize to be 100MB (%d bytes), got %d", hundredMB, DefaultMaxFileSize)
	}

	// Test that the limit is not too small (at least 1MB)
	if DefaultMaxFileSize < oneMB {
		t.Errorf("DefaultMaxFileSize (%d) should be at least 1MB (%d bytes)", DefaultMaxFileSize, oneMB)
	}

	// Test that the limit is not unreasonably large (less than 1GB)
	oneGB := 1 << 30
	if DefaultMaxFileSize >= oneGB {
		t.Errorf("DefaultMaxFileSize (%d) should be less than 1GB (%d bytes) for reasonable memory usage",
			DefaultMaxFileSize, oneGB)
	}
}

func TestBufferSizeReasonableness(t *testing.T) {
	// Test that buffer size is reasonable
	minBufferSize := 64   // At least 64 bytes
	maxBufferSize := 4096 // At most 4KB for path buffers

	if DefaultPathBufferSize < minBufferSize {
		t.Errorf("DefaultPathBufferSize (%d) should be at least %d bytes", DefaultPathBufferSize, minBufferSize)
	}

	if DefaultPathBufferSize > maxBufferSize {
		t.Errorf("DefaultPathBufferSize (%d) should not exceed %d bytes", DefaultPathBufferSize, maxBufferSize)
	}

	// Should be a power of 2 for efficient memory allocation
	size := DefaultPathBufferSize
	if size&(size-1) != 0 {
		t.Errorf("DefaultPathBufferSize (%d) should be a power of 2 for efficient memory allocation", size)
	}
}

func TestCacheMaxAgeReasonableness(t *testing.T) {
	// Test that cache max age is reasonable
	minCacheAge := 300   // At least 5 minutes
	maxCacheAge := 86400 // At most 24 hours

	if StaticAssetCacheMaxAge < minCacheAge {
		t.Errorf("StaticAssetCacheMaxAge (%d) should be at least %d seconds", StaticAssetCacheMaxAge, minCacheAge)
	}

	if StaticAssetCacheMaxAge > maxCacheAge {
		t.Errorf("StaticAssetCacheMaxAge (%d) should not exceed %d seconds", StaticAssetCacheMaxAge, maxCacheAge)
	}

	// Current value should be exactly 1 hour
	if StaticAssetCacheMaxAge != 3600 {
		t.Errorf("StaticAssetCacheMaxAge (%d) should be 3600 seconds (1 hour)", StaticAssetCacheMaxAge)
	}
}

func TestTimeoutUnits(t *testing.T) {
	// Test that timeout constants have appropriate units (seconds or minutes)

	// These should be in seconds
	secondTimeouts := []time.Duration{
		DefaultRequestTimeout,
		ServerReadTimeout,
		ServerWriteTimeout,
		FileServeTimeout,
		DirectoryTimeout,
		TemplateTimeout,
		ShutdownTimeout,
		HealthCheckTimeout,
	}

	for _, timeout := range secondTimeouts {
		if timeout < time.Second || timeout > 60*time.Second {
			t.Errorf("Timeout %v should be between 1 second and 60 seconds", timeout)
		}
	}

	// These should be in minutes
	minuteTimeouts := []time.Duration{
		ServerIdleTimeout,
		UploadTimeout,
		CSRFCleanupInterval,
	}

	for _, timeout := range minuteTimeouts {
		if timeout < time.Minute || timeout > 30*time.Minute {
			t.Errorf("Timeout %v should be between 1 minute and 30 minutes", timeout)
		}
	}

	// CSRF token expiry should be in hours
	if CSRFTokenExpiry < time.Hour || CSRFTokenExpiry > 24*time.Hour {
		t.Errorf("CSRFTokenExpiry %v should be between 1 hour and 24 hours", CSRFTokenExpiry)
	}
}

// Benchmark constant access (should be extremely fast)
func BenchmarkConstantAccess(b *testing.B) {
	var result int
	for range b.N {
		result = DefaultPort
	}
	_ = result
}

func BenchmarkTimeConstantAccess(b *testing.B) {
	var result time.Duration
	for range b.N {
		result = DefaultRequestTimeout
	}
	_ = result
}

func BenchmarkSizeConstantAccess(b *testing.B) {
	var result int
	for range b.N {
		result = DefaultMaxFileSize
	}
	_ = result
}
