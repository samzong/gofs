package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var validThemes = map[string]bool{
	"default":  true,
	"classic":  true,
	"advanced": true,
}

// DirMount represents a directory mount configuration
type DirMount struct {
	Path     string // URL path prefix (e.g., "/config")
	Dir      string // Local directory path
	Readonly bool   // Whether the mount is read-only
	Name     string // Display name for UI
}

type Config struct {
	Host           string
	Dir            string     // Legacy single directory support
	Dirs           []DirMount // Multi-directory support
	Port           int
	MaxFileSize    int64
	RequestTimeout int
	EnableSecurity bool
	Theme          string
	ShowHidden     bool
	EnableWebDAV   bool
}

func New(port int, host, dir, theme string, showHidden bool, dirs []string) (*Config, error) {
	cfg := &Config{
		Port:       port,
		Host:       host,
		Dir:        dir,
		Theme:      theme,
		ShowHidden: showHidden,
	}

	// Parse directory configuration
	if err := cfg.parseDirConfig(dirs); err != nil {
		return nil, err
	}

	cfg.setDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// parseDirConfig handles directory mount configuration
func (c *Config) parseDirConfig(dirs []string) error {
	// Use provided dirs, or fall back to legacy single dir
	dirsToParse := dirs
	if len(dirs) == 0 && c.Dir != "" {
		dirsToParse = []string{c.Dir}
	}

	// Parse each directory specification
	for _, dirStr := range dirsToParse {
		mount, err := ParseDir(dirStr)
		if err != nil {
			return fmt.Errorf("parsing directory %q: %w", dirStr, err)
		}
		c.Dirs = append(c.Dirs, mount)
	}

	return ValidateDirs(c.Dirs)
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

// ParseDir parses a directory configuration string
// Format: [path:]dir[:ro][:name] or just dir for legacy compatibility
func ParseDir(dirStr string) (DirMount, error) {
	parts := strings.Split(dirStr, ":")

	// Legacy: single directory path
	if len(parts) == 1 {
		return DirMount{Path: "/", Dir: dirStr, Name: "Files"}, nil
	}

	if len(parts) < 2 {
		return DirMount{}, fmt.Errorf("invalid format: %s (expected path:dir[:options])", dirStr)
	}

	mount := DirMount{Path: parts[0], Dir: parts[1]}

	// Parse optional flags: "ro" for readonly, anything else for name
	for _, part := range parts[2:] {
		switch part {
		case "ro":
			mount.Readonly = true
		case "":
			// Skip empty parts
		default:
			if mount.Name == "" {
				mount.Name = part
			}
		}
	}

	// Generate default name from path
	if mount.Name == "" {
		if name := strings.Trim(mount.Path, "/"); name != "" {
			mount.Name = name
		} else {
			mount.Name = "Files"
		}
	}

	return mount, nil
}

// ValidateDirs checks for path conflicts and validates directory mounts
func ValidateDirs(dirs []DirMount) error {
	paths := make(map[string]string)
	for _, d := range dirs {
		if d.Path == "" {
			return errors.New("empty path in directory mount")
		}
		if d.Dir == "" {
			return fmt.Errorf("empty directory in mount for path %s", d.Path)
		}

		// Ensure path starts with /
		if !strings.HasPrefix(d.Path, "/") {
			return fmt.Errorf("path must start with /: %s", d.Path)
		}

		// Check for conflicts
		if existing, ok := paths[d.Path]; ok {
			return fmt.Errorf("path conflict: %s maps to both %s and %s", d.Path, existing, d.Dir)
		}
		paths[d.Path] = d.Dir
	}
	return nil
}
