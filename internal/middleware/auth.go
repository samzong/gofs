package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type authCache struct {
	validUntil time.Time
}

// BasicAuth implements HTTP Basic Authentication with timing attack protection.
// Uses bcrypt for password hashing and includes request-scoped caching.
type BasicAuth struct {
	realm        string
	username     string
	passwordHash []byte
	cacheMu      sync.RWMutex
	cache        map[string]*authCache
	cacheTTL     time.Duration
}

func NewBasicAuth(realm, username, password string) (*BasicAuth, error) {
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}
	if realm == "" {
		realm = "gofs"
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	return &BasicAuth{
		realm:        realm,
		username:     username,
		passwordHash: passwordHash,
		cache:        make(map[string]*authCache),
		cacheTTL:     5 * time.Minute,
	}, nil
}

func NewBasicAuthFromCredentials(credentials string) (*BasicAuth, error) {
	if credentials == "" {
		return nil, errors.New("credentials cannot be empty")
	}

	colonIndex := strings.IndexByte(credentials, ':')
	if colonIndex == -1 {
		return nil, errors.New("invalid credentials format: expected 'user:password'")
	}

	username := credentials[:colonIndex]
	password := credentials[colonIndex+1:]

	if username == "" {
		return nil, errors.New("username cannot be empty")
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}

	return NewBasicAuth("gofs", username, password)
}

// Middleware returns an HTTP middleware function that enforces Basic Authentication.
// It returns 401 Unauthorized for missing or invalid credentials.
// Health check endpoints (/healthz, /readyz) are excluded from authentication.
func (ba *BasicAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for health check endpoints
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		// Extract Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			ba.requireAuth(w)
			return
		}

		// Check if it's Basic auth
		if !strings.HasPrefix(auth, "Basic ") {
			ba.requireAuth(w)
			return
		}

		// Check cache first to avoid expensive bcrypt on every request
		encoded := auth[6:] // Remove "Basic " prefix

		// Try to get from cache
		ba.cacheMu.RLock()
		cached, found := ba.cache[encoded]
		ba.cacheMu.RUnlock()

		if found && time.Now().Before(cached.validUntil) {
			// Cache hit and still valid
			next.ServeHTTP(w, r)
			return
		}

		// Cache miss or expired, perform full authentication
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			ba.requireAuth(w)
			return
		}

		// Split username:password
		credentials := string(decoded)
		colonIndex := strings.IndexByte(credentials, ':')
		if colonIndex == -1 {
			ba.requireAuth(w)
			return
		}

		providedUsername := credentials[:colonIndex]
		providedPassword := credentials[colonIndex+1:]

		// Constant-time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(providedUsername), []byte(ba.username))

		// Use bcrypt for secure password verification (inherently timing-safe)
		passwordErr := bcrypt.CompareHashAndPassword(ba.passwordHash, []byte(providedPassword))
		passwordMatch := 0
		if passwordErr == nil {
			passwordMatch = 1
		}

		if usernameMatch == 1 && passwordMatch == 1 {
			// Authentication successful, update cache
			ba.cacheMu.Lock()
			ba.cache[encoded] = &authCache{
				validUntil: time.Now().Add(ba.cacheTTL),
			}
			// Clean up expired entries while we have the lock
			ba.cleanupCacheLocked()
			ba.cacheMu.Unlock()

			// Continue to next handler
			next.ServeHTTP(w, r)
			return
		}

		// Authentication failed
		ba.requireAuth(w)
	})
}

// cleanupCacheLocked removes expired entries from the cache.
// Must be called with cacheMu write lock held.
func (ba *BasicAuth) cleanupCacheLocked() {
	now := time.Now()
	for key, entry := range ba.cache {
		if now.After(entry.validUntil) {
			delete(ba.cache, key)
		}
	}
}

// requireAuth sends a 401 Unauthorized response with WWW-Authenticate header.
func (ba *BasicAuth) requireAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+ba.realm+`", charset="UTF-8"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	// In this context, the error from w.Write is intentionally ignored.
	// The primary purpose is to send a 401 Unauthorized status, and the response body is secondary.
	// If writing the body fails, the client will still receive the 401, which is the critical part of the response.
	_, _ = w.Write([]byte("401 Unauthorized\n"))
}
