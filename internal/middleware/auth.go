// Package middleware provides HTTP middleware components for the gofs file server.
package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// BasicAuth implements HTTP Basic Authentication middleware following RFC 7617.
// It provides secure password hashing and constant-time comparison to prevent timing attacks.
type BasicAuth struct {
	realm        string
	username     string
	passwordHash []byte
}

// NewBasicAuth creates a new Basic Authentication middleware with the specified credentials.
// The realm parameter defines the authentication realm shown to users.
// The password is securely hashed using bcrypt.
func NewBasicAuth(realm, username, password string) (*BasicAuth, error) {
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}
	if password == "" {
		return nil, errors.New("password cannot be empty")
	}
	if realm == "" {
		realm = "gofs" // default realm
	}

	// Hash the password with bcrypt (cost 12 for security)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	return &BasicAuth{
		realm:        realm,
		username:     username,
		passwordHash: passwordHash,
	}, nil
}

// NewBasicAuthFromCredentials parses credentials in "user:password" format and creates BasicAuth middleware.
// This is the primary method for command-line usage with --auth/-a flag.
func NewBasicAuthFromCredentials(credentials string) (*BasicAuth, error) {
	if credentials == "" {
		return nil, errors.New("credentials cannot be empty")
	}

	// Parse credentials in "user:password" format
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
func (ba *BasicAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Decode the base64 encoded credentials
		encoded := auth[6:] // Remove "Basic " prefix
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
			// Authentication successful, continue to next handler
			next.ServeHTTP(w, r)
			return
		}

		// Authentication failed
		ba.requireAuth(w)
	})
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
