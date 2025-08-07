package main

import (
	"os"
	"testing"
)

func TestGetEnvString(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		setEnv       bool
		envValue     string
		expected     string
	}{
		{
			name:         "returns default value when env var not set",
			key:          "TEST_KEY_NOT_SET",
			defaultValue: "default",
			setEnv:       false,
			expected:     "default",
		},
		{
			name:         "returns env value when set",
			key:          "TEST_KEY_SET",
			defaultValue: "default",
			setEnv:       true,
			envValue:     "env_value",
			expected:     "env_value",
		},
		{
			name:         "returns default when env var is empty",
			key:          "TEST_KEY_EMPTY",
			defaultValue: "default",
			setEnv:       true,
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			os.Unsetenv(tt.key)

			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnv[string]() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int
		setEnv       bool
		envValue     string
		expected     int
	}{
		{
			name:         "returns default value when env var not set",
			key:          "TEST_INT_NOT_SET",
			defaultValue: 8000,
			setEnv:       false,
			expected:     8000,
		},
		{
			name:         "returns parsed int when valid",
			key:          "TEST_INT_VALID",
			defaultValue: 8000,
			setEnv:       true,
			envValue:     "9000",
			expected:     9000,
		},
		{
			name:         "returns default when env var is invalid",
			key:          "TEST_INT_INVALID",
			defaultValue: 8000,
			setEnv:       true,
			envValue:     "invalid",
			expected:     8000,
		},
		{
			name:         "returns default when env var is empty",
			key:          "TEST_INT_EMPTY",
			defaultValue: 8000,
			setEnv:       true,
			envValue:     "",
			expected:     8000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			os.Unsetenv(tt.key)

			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnv[int]() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue bool
		setEnv       bool
		envValue     string
		expected     bool
	}{
		{
			name:         "returns default value when env var not set",
			key:          "TEST_BOOL_NOT_SET",
			defaultValue: false,
			setEnv:       false,
			expected:     false,
		},
		{
			name:         "returns true when env var is 'true'",
			key:          "TEST_BOOL_TRUE",
			defaultValue: false,
			setEnv:       true,
			envValue:     "true",
			expected:     true,
		},
		{
			name:         "returns false when env var is 'false'",
			key:          "TEST_BOOL_FALSE",
			defaultValue: true,
			setEnv:       true,
			envValue:     "false",
			expected:     false,
		},
		{
			name:         "returns true when env var is '1'",
			key:          "TEST_BOOL_ONE",
			defaultValue: false,
			setEnv:       true,
			envValue:     "1",
			expected:     true,
		},
		{
			name:         "returns false when env var is '0'",
			key:          "TEST_BOOL_ZERO",
			defaultValue: true,
			setEnv:       true,
			envValue:     "0",
			expected:     false,
		},
		{
			name:         "returns default when env var is invalid",
			key:          "TEST_BOOL_INVALID",
			defaultValue: true,
			setEnv:       true,
			envValue:     "invalid",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			os.Unsetenv(tt.key)

			if tt.setEnv {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnv[bool]() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	// Test default logger creation
	logger := setupLogger()
	if logger == nil {
		t.Error("setupLogger() returned nil")
	}

	// Test production environment
	os.Setenv("GOFS_ENV", "production")
	defer os.Unsetenv("GOFS_ENV")

	prodLogger := setupLogger()
	if prodLogger == nil {
		t.Error("setupLogger() returned nil in production mode")
	}

	// Test different log levels
	testLevels := []string{"debug", "warn", "error"}
	for _, level := range testLevels {
		t.Run("log_level_"+level, func(t *testing.T) {
			os.Setenv("GOFS_LOG_LEVEL", level)
			defer os.Unsetenv("GOFS_LOG_LEVEL")

			logger := setupLogger()
			if logger == nil {
				t.Errorf("setupLogger() returned nil for log level %s", level)
			}
		})
	}
}
