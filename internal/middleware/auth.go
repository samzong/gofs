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

func (ba *BasicAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			ba.requireAuth(w)
			return
		}

		if !strings.HasPrefix(auth, "Basic ") {
			ba.requireAuth(w)
			return
		}

		encoded := auth[6:]

		ba.cacheMu.RLock()
		cached, found := ba.cache[encoded]
		ba.cacheMu.RUnlock()

		if found && time.Now().Before(cached.validUntil) {
			next.ServeHTTP(w, r)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			ba.requireAuth(w)
			return
		}

		credentials := string(decoded)
		colonIndex := strings.IndexByte(credentials, ':')
		if colonIndex == -1 {
			ba.requireAuth(w)
			return
		}

		providedUsername := credentials[:colonIndex]
		providedPassword := credentials[colonIndex+1:]

		usernameMatch := subtle.ConstantTimeCompare([]byte(providedUsername), []byte(ba.username))

		passwordErr := bcrypt.CompareHashAndPassword(ba.passwordHash, []byte(providedPassword))
		passwordMatch := 0
		if passwordErr == nil {
			passwordMatch = 1
		}

		if usernameMatch == 1 && passwordMatch == 1 {
			ba.cacheMu.Lock()
			ba.cache[encoded] = &authCache{
				validUntil: time.Now().Add(ba.cacheTTL),
			}
			ba.cleanupCacheLocked()
			ba.cacheMu.Unlock()

			next.ServeHTTP(w, r)
			return
		}

		ba.requireAuth(w)
	})
}

func (ba *BasicAuth) cleanupCacheLocked() {
	now := time.Now()
	for key, entry := range ba.cache {
		if now.After(entry.validUntil) {
			delete(ba.cache, key)
		}
	}
}

func (ba *BasicAuth) requireAuth(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="`+ba.realm+`", charset="UTF-8"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte("401 Unauthorized\n"))
}
