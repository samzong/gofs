// Package config provides configuration management for the gofs HTTP file server.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the server configuration.
type Config struct {
	Host           string
	Dir            string
	Port           int
	MaxFileSize    int64
	RequestTimeout int
	EnableSecurity bool
}

// New creates a new configuration with the provided values.
// Zero values are replaced with sensible defaults.
func New(port int, host, dir string) (*Config, error) {
	cfg := &Config{
		Port: port,
		Host: host,
		Dir:  dir,
	}

	cfg.setDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// setDefaults applies default values for zero-value fields.
func (c *Config) setDefaults() {
	if c.Port == 0 {
		c.Port = 8000
	}
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Dir == "" {
		c.Dir = "."
	}
	if c.MaxFileSize == 0 {
		c.MaxFileSize = 100 << 20 // 100MB default
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30 // 30 seconds default
	}
	// EnableSecurity defaults to false for backward compatibility
}

// validate checks that the configuration is valid.
func (c *Config) validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}

	absDir, err := filepath.Abs(c.Dir)
	if err != nil {
		return fmt.Errorf("invalid directory path %q: %w", c.Dir, err)
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("directory %q does not exist: %w", absDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", absDir)
	}

	c.Dir = absDir // Use absolute path
	return nil
}

// Address returns the network address to listen on.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
