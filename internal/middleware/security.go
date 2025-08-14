package middleware

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/samzong/gofs/pkg/fileutil"
)

// SecurityConfig defines security header configuration
type SecurityConfig struct {
	EnableSecurity        bool
	ContentSecurityPolicy string
}

// SecurityHeaders applies common security headers to responses
func SecurityHeaders(config SecurityConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setSecurityHeaders(w, config)
			next.ServeHTTP(w, r)
		})
	}
}

// setSecurityHeaders applies the standard security headers
func setSecurityHeaders(w http.ResponseWriter, config SecurityConfig) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	if config.EnableSecurity {
		csp := config.ContentSecurityPolicy
		if csp == "" {
			csp = "default-src 'self'"
		}
		w.Header().Set("Content-Security-Policy", csp)
	}
}

// WriteJSON writes a JSON response with proper content type and error handling
func WriteJSON[T any](w http.ResponseWriter, data T) error {
	b, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(b)
	return err
}

// WriteJSONError writes a JSON error response with the specified status code
func WriteJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := map[string]string{
		"error": message,
	}

	// Best effort to encode error - if this fails, the error is already written
	_ = json.NewEncoder(w).Encode(errorResponse)
}

// SafeRequestPath extracts and validates a safe path from an HTTP request path
// This consolidates the common pattern of fileutil.SafePath(strings.TrimPrefix(path, "/"))
func SafeRequestPath(path string) string {
	return fileutil.SafePath(strings.TrimPrefix(path, "/"))
}
