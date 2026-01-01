package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gofs-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testCases := []struct {
		name        string
		port        int
		host        string
		dir         string
		theme       string
		showHidden  bool
		expectError bool
	}{
		{
			name:        "valid_config_with_defaults",
			port:        0,  // should default to 8000
			host:        "", // should default to "0.0.0.0"
			dir:         tmpDir,
			theme:       "", // should default to "default"
			showHidden:  false,
			expectError: false,
		},
		{
			name:        "valid_config_explicit_values",
			port:        9000,
			host:        "127.0.0.1",
			dir:         tmpDir,
			theme:       "advanced",
			showHidden:  true,
			expectError: false,
		},
		{
			name:        "invalid_port_too_low",
			port:        -1,
			host:        "127.0.0.1",
			dir:         tmpDir,
			theme:       "default",
			showHidden:  false,
			expectError: true,
		},
		{
			name:        "invalid_port_too_high",
			port:        70000,
			host:        "127.0.0.1",
			dir:         tmpDir,
			theme:       "default",
			showHidden:  false,
			expectError: true,
		},
		{
			name:        "invalid_theme_fallback_to_default",
			port:        8000,
			host:        "127.0.0.1",
			dir:         tmpDir,
			theme:       "invalid-theme",
			showHidden:  false,
			expectError: false, // Now falls back to default instead of error
		},
		{
			name:        "nonexistent_directory",
			port:        8000,
			host:        "127.0.0.1",
			dir:         "/nonexistent/directory",
			theme:       "default",
			showHidden:  false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := New(tc.port, tc.host, tc.dir, tc.theme, tc.showHidden, nil)
			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check defaults are applied
			expectedPort := tc.port
			if expectedPort == 0 {
				expectedPort = 8000
			}
			if cfg.Port != expectedPort {
				t.Errorf("expected port %d, got %d", expectedPort, cfg.Port)
			}

			expectedHost := tc.host
			if expectedHost == "" {
				expectedHost = "0.0.0.0"
			}
			if cfg.Host != expectedHost {
				t.Errorf("expected host %q, got %q", expectedHost, cfg.Host)
			}

			if !filepath.IsAbs(cfg.Dir) {
				t.Errorf("expected absolute path, got %q", cfg.Dir)
			}

			expectedTheme := tc.theme
			if expectedTheme == "" || !validThemes[expectedTheme] {
				expectedTheme = "default" // Invalid themes fall back to default
			}
			if cfg.Theme != expectedTheme {
				t.Errorf("expected theme %q, got %q", expectedTheme, cfg.Theme)
			}

			if cfg.ShowHidden != tc.showHidden {
				t.Errorf("expected showHidden %v, got %v", tc.showHidden, cfg.ShowHidden)
			}
		})
	}
}

func TestConfig_setDefaults(t *testing.T) {
	c := &Config{}
	c.setDefaults()

	if c.Port != 8000 {
		t.Errorf("expected default port 8000, got %d", c.Port)
	}
	if c.Host != "0.0.0.0" {
		t.Errorf("expected default host '0.0.0.0', got %q", c.Host)
	}
	if c.Dir != "." {
		t.Errorf("expected default dir '.', got %q", c.Dir)
	}
	if c.MaxFileSize != 100<<20 {
		t.Errorf("expected default max file size 104857600, got %d", c.MaxFileSize)
	}
	if c.RequestTimeout != 30 {
		t.Errorf("expected default request timeout 30, got %d", c.RequestTimeout)
	}
	if c.Theme != "default" {
		t.Errorf("expected default theme 'default', got %q", c.Theme)
	}
}

func TestConfig_validate(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "gofs-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp(tmpDir, "testfile")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	testCases := []struct {
		name        string
		config      *Config
		expectError bool
	}{
		{
			name: "valid_config",
			config: &Config{
				Port:  8000,
				Host:  "localhost",
				Dir:   tmpDir,
				Theme: "default",
			},
			expectError: false,
		},
		{
			name: "invalid_port_too_low",
			config: &Config{
				Port:  -1,
				Host:  "localhost",
				Dir:   tmpDir,
				Theme: "default",
			},
			expectError: true,
		},
		{
			name: "invalid_port_too_high",
			config: &Config{
				Port:  70000,
				Host:  "localhost",
				Dir:   tmpDir,
				Theme: "default",
			},
			expectError: true,
		},
		{
			name: "invalid_theme_fallback",
			config: &Config{
				Port:  8000,
				Host:  "localhost",
				Dir:   tmpDir,
				Theme: "invalid",
			},
			expectError: false, // Falls back to default
		},
		{
			name: "nonexistent_directory",
			config: &Config{
				Port:  8000,
				Host:  "localhost",
				Dir:   "/nonexistent/directory",
				Theme: "default",
			},
			expectError: true,
		},
		{
			name: "file_instead_of_directory",
			config: &Config{
				Port:  8000,
				Host:  "localhost",
				Dir:   tmpFile.Name(),
				Theme: "default",
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.validate()
			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConfig_Address(t *testing.T) {
	testCases := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "localhost_8000",
			host:     "127.0.0.1",
			port:     8000,
			expected: "127.0.0.1:8000",
		},
		{
			name:     "all_interfaces_9000",
			host:     "0.0.0.0",
			port:     9000,
			expected: "0.0.0.0:9000",
		},
		{
			name:     "custom_host_port",
			host:     "192.168.1.100",
			port:     3000,
			expected: "192.168.1.100:3000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Host: tc.host,
				Port: tc.port,
			}
			if addr := cfg.Address(); addr != tc.expected {
				t.Errorf("expected address %q, got %q", tc.expected, addr)
			}
		})
	}
}

func TestConfig_BackwardCompatibility(t *testing.T) {
	// Test that the simplified config maintains backward compatibility
	cfg := &Config{
		Host:           "127.0.0.1",
		Dir:            ".",
		Port:           8000,
		MaxFileSize:    100 << 20,
		RequestTimeout: 30,
		EnableSecurity: false,
		Theme:          "default",
		ShowHidden:     false,
	}

	// Verify that the config can be created and validated
	cfg.setDefaults()
	err := cfg.validate()
	if err != nil {
		t.Errorf("backward compatibility test failed: %v", err)
	}

	// Verify expected values
	if cfg.Address() != "127.0.0.1:8000" {
		t.Errorf("expected address '127.0.0.1:8000', got %q", cfg.Address())
	}
}
