package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var validThemes = map[string]bool{
	"default":  true,
	"classic":  true,
	"advanced": true,
}

type Config struct {
	Host           string
	Dir            string
	Port           int
	MaxFileSize    int64
	RequestTimeout int
	EnableSecurity bool
	Theme          string
	ShowHidden     bool
	EnableWebDAV   bool
}

func New(port int, host, dir, theme string, showHidden bool) (*Config, error) {
	cfg := &Config{
		Port:       port,
		Host:       host,
		Dir:        dir,
		Theme:      theme,
		ShowHidden: showHidden,
	}

	cfg.setDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

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
	if c.Theme == "" {
		c.Theme = "default"
	}
}

func (c *Config) validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 0 and 65535, got %d", c.Port)
	}

	if !validThemes[c.Theme] {
		return fmt.Errorf("invalid theme %q: supported themes are 'default', 'classic', and 'advanced'", c.Theme)
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

	c.Dir = absDir

	return nil
}

func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
