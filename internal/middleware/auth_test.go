package middleware

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestNewBasicAuth(t *testing.T) {
	realm := "test-realm"
	username := "testuser"
	password := "testpass"

	auth, err := NewBasicAuth(realm, username, password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if auth.realm != realm {
		t.Errorf("expected realm %q, got %q", realm, auth.realm)
	}
	if auth.username != username {
		t.Errorf("expected username %q, got %q", username, auth.username)
	}
	// We can't directly compare password hash, but we can verify it was set
	if len(auth.passwordHash) == 0 {
		t.Error("expected password hash to be generated")
	}

	// Test password verification works
	err = bcrypt.CompareHashAndPassword(auth.passwordHash, []byte(password))
	if err != nil {
		t.Errorf("password hash verification failed: %v", err)
	}
}

func TestBasicAuthMiddleware_Success(t *testing.T) {
	// Setup
	realm := "test-realm"
	username := "admin"
	password := "secret"
	auth, err := NewBasicAuth(realm, username, password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a test handler that sets a header to verify it was called
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Handler-Called", "true")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Wrap with middleware
	handler := auth.Middleware(nextHandler)

	// Create request with valid credentials
	req := httptest.NewRequest("GET", "/", nil)
	credentials := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	req.Header.Set("Authorization", "Basic "+credentials)

	// Execute request
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Header().Get("X-Handler-Called") != "true" {
		t.Error("expected next handler to be called")
	}
	if body := rr.Body.String(); body != "success" {
		t.Errorf("expected body 'success', got %q", body)
	}
}

func TestBasicAuthMiddleware_NoAuthHeader(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request without Authorization header
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}

	// Verify WWW-Authenticate header
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	expected := `Basic realm="test-realm", charset="UTF-8"`
	if wwwAuth != expected {
		t.Errorf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}

	// Verify response body
	body := rr.Body.String()
	if body != "401 Unauthorized\n" {
		t.Errorf("expected body '401 Unauthorized\\n', got %q", body)
	}
}

func TestBasicAuthMiddleware_InvalidAuthScheme(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request with Bearer token instead of Basic
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_InvalidBase64(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request with invalid base64 encoding
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic invalid-base64!")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_MissingColon(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request with credentials missing colon separator
	credentials := base64.StdEncoding.EncodeToString([]byte("adminnocolon"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_WrongUsername(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request with wrong username
	credentials := base64.StdEncoding.EncodeToString([]byte("wronguser:secret"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_WrongPassword(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request with wrong password
	credentials := base64.StdEncoding.EncodeToString([]byte("admin:wrongpass"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_EmptyCredentials(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request with empty credentials
	credentials := base64.StdEncoding.EncodeToString([]byte(":"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_SpecialCharacters(t *testing.T) {
	// Setup with special characters in credentials
	auth, err := NewBasicAuth("test-realm", "user@domain.com", "p@$$w0rd!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	// Create request with special characters
	credentials := base64.StdEncoding.EncodeToString([]byte("user@domain.com:p@$$w0rd!"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify success
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestBasicAuthMiddleware_UnicodeCredentials(t *testing.T) {
	// Setup with Unicode characters in credentials
	auth, err := NewBasicAuth("测试域", "用户", "密码")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	// Create request with Unicode characters
	credentials := base64.StdEncoding.EncodeToString([]byte("用户:密码"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify success
	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestBasicAuthMiddleware_CaseSensitivity(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "Admin", "Secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Test with different case - should fail
	credentials := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify 401 response (case sensitive)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestBasicAuthMiddleware_RealmWithSpecialChars(t *testing.T) {
	// Setup with special characters in realm
	realm := `My "Special" Realm & More`
	auth, err := NewBasicAuth(realm, "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request without credentials to trigger 401
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify WWW-Authenticate header includes the realm correctly
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	expectedRealm := `Basic realm="` + realm + `", charset="UTF-8"`
	if wwwAuth != expectedRealm {
		t.Errorf("expected WWW-Authenticate %q, got %q", expectedRealm, wwwAuth)
	}
}

// Benchmark to verify constant-time comparison performance
func BenchmarkBasicAuthMiddleware_Success(b *testing.B) {
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	credentials := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)

	b.ResetTimer()
	for range b.N {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkBasicAuthMiddleware_Failure(b *testing.B) {
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		b.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	credentials := base64.StdEncoding.EncodeToString([]byte("admin:wrongpass"))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic "+credentials)

	b.ResetTimer()
	for range b.N {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

// Test to verify timing attack resistance
func TestBasicAuthMiddleware_TimingAttackResistance(t *testing.T) {
	auth, err := NewBasicAuth("test-realm", "admin", "verylongpasswordthatistotallysecret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	testCases := []struct {
		name        string
		credentials string
	}{
		{"wrong_short_password", "admin:a"},
		{"wrong_medium_password", "admin:mediumpass"},
		{"wrong_long_password", "admin:verylongpasswordthatistotallywrong"},
		{"wrong_username_short", "a:verylongpasswordthatistotallysecret"},
		{"wrong_username_long", "verylongusernamethatistotallywrong:verylongpasswordthatistotallysecret"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			credentials := base64.StdEncoding.EncodeToString([]byte(tc.credentials))
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Basic "+credentials)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// All should return 401 regardless of input length
			if rr.Code != http.StatusUnauthorized {
				t.Errorf("expected status %d, got %d for case %s", http.StatusUnauthorized, rr.Code, tc.name)
			}
		})
	}
}

func TestBasicAuthMiddleware_MultipleRequestsWithSameCredentials(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	credentials := base64.StdEncoding.EncodeToString([]byte("admin:secret"))

	// Send multiple requests with same credentials
	for i := range 5 {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Basic "+credentials)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("request %d: expected status %d, got %d", i+1, http.StatusOK, rr.Code)
		}
	}
}

func TestBasicAuthMiddleware_DifferentHTTPMethods(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	credentials := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/", nil)
			req.Header.Set("Authorization", "Basic "+credentials)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("method %s: expected status %d, got %d", method, http.StatusOK, rr.Code)
			}
		})
	}
}

func TestBasicAuthMiddleware_ContentTypeHeader(t *testing.T) {
	// Setup
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	// Create request without credentials
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify Content-Type header is set correctly
	contentType := rr.Header().Get("Content-Type")
	expected := "text/plain; charset=utf-8"
	if contentType != expected {
		t.Errorf("expected Content-Type %q, got %q", expected, contentType)
	}
}

// Test edge case with empty realm (gets default "gofs")
func TestBasicAuthMiddleware_EmptyRealm(t *testing.T) {
	auth, err := NewBasicAuth("", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("next handler should not be called")
	})
	handler := auth.Middleware(nextHandler)

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Verify WWW-Authenticate header with default realm "gofs"
	wwwAuth := rr.Header().Get("WWW-Authenticate")
	expected := `Basic realm="gofs", charset="UTF-8"`
	if wwwAuth != expected {
		t.Errorf("expected WWW-Authenticate %q, got %q", expected, wwwAuth)
	}
}

// Test case sensitivity of Authorization header name
func TestBasicAuthMiddleware_AuthorizationHeaderCase(t *testing.T) {
	auth, err := NewBasicAuth("test-realm", "admin", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := auth.Middleware(nextHandler)

	credentials := base64.StdEncoding.EncodeToString([]byte("admin:secret"))

	// Test different cases - HTTP headers are case-insensitive
	testCases := []string{"Authorization", "authorization", "AUTHORIZATION"}

	for _, headerCase := range testCases {
		t.Run(headerCase, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set(headerCase, "Basic "+credentials)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("header case %s: expected status %d, got %d", headerCase, http.StatusOK, rr.Code)
			}
		})
	}
}

// Test the new credential parsing functionality
func TestNewBasicAuthFromCredentials(t *testing.T) {
	testCases := []struct {
		name        string
		credentials string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid_credentials",
			credentials: "user:password",
			expectError: false,
		},
		{
			name:        "valid_credentials_with_special_chars",
			credentials: "admin@domain.com:p@ssw0rd!",
			expectError: false,
		},
		{
			name:        "empty_credentials",
			credentials: "",
			expectError: true,
			errorMsg:    "credentials cannot be empty",
		},
		{
			name:        "missing_colon",
			credentials: "userpassword",
			expectError: true,
			errorMsg:    "invalid credentials format: expected 'user:password'",
		},
		{
			name:        "empty_username",
			credentials: ":password",
			expectError: true,
			errorMsg:    "username cannot be empty",
		},
		{
			name:        "empty_password",
			credentials: "user:",
			expectError: true,
			errorMsg:    "password cannot be empty",
		},
		{
			name:        "multiple_colons",
			credentials: "user:pass:word",
			expectError: false, // Should work - only first colon is used as separator
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auth, err := NewBasicAuthFromCredentials(tc.credentials)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tc.errorMsg != "" && err.Error() != tc.errorMsg {
					t.Errorf("expected error message %q, got %q", tc.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if auth == nil {
				t.Error("expected auth to be non-nil")
				return
			}

			// Verify the auth object works
			if auth.realm != "gofs" {
				t.Errorf("expected default realm 'gofs', got %q", auth.realm)
			}
		})
	}
}

// Test error cases for NewBasicAuth
func TestNewBasicAuth_ErrorCases(t *testing.T) {
	testCases := []struct {
		name        string
		realm       string
		username    string
		password    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty_username",
			realm:       "test",
			username:    "",
			password:    "password",
			expectError: true,
			errorMsg:    "username cannot be empty",
		},
		{
			name:        "empty_password",
			realm:       "test",
			username:    "user",
			password:    "",
			expectError: true,
			errorMsg:    "password cannot be empty",
		},
		{
			name:        "empty_realm_gets_default",
			realm:       "",
			username:    "user",
			password:    "password",
			expectError: false,
		},
		{
			name:        "valid_all_fields",
			realm:       "test-realm",
			username:    "user",
			password:    "password",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auth, err := NewBasicAuth(tc.realm, tc.username, tc.password)

			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				if tc.errorMsg != "" && err.Error() != tc.errorMsg {
					t.Errorf("expected error message %q, got %q", tc.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if auth == nil {
				t.Error("expected auth to be non-nil")
				return
			}

			// Verify default realm behavior
			if tc.realm == "" && auth.realm != "gofs" {
				t.Errorf("expected default realm 'gofs', got %q", auth.realm)
			}
		})
	}
}

// Test bcrypt integration and security
func TestBasicAuth_PasswordSecurity(t *testing.T) {
	password := "test-password-123"
	auth, err := NewBasicAuth("test", "user", password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify password is hashed, not stored in plaintext
	if len(auth.passwordHash) == 0 {
		t.Error("password hash should be generated")
	}

	// Verify correct password validates
	err = bcrypt.CompareHashAndPassword(auth.passwordHash, []byte(password))
	if err != nil {
		t.Errorf("correct password should validate: %v", err)
	}

	// Verify incorrect password fails
	err = bcrypt.CompareHashAndPassword(auth.passwordHash, []byte("wrong-password"))
	if err == nil {
		t.Error("incorrect password should fail validation")
	}

	// Verify different instances generate different hashes (salt works)
	auth2, err := NewBasicAuth("test", "user", password)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(auth.passwordHash) == string(auth2.passwordHash) {
		t.Error("different instances should generate different password hashes (salt should be different)")
	}
}
