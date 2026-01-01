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
		fmt.Fprintf(os.Stderr, "Warning: invalid theme %q, falling back to 'default'. Supported themes: default, advanced\n", c.Theme)
		c.Theme = "default"
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

// ValidateDirs checks for path conflicts and validates directory mounts with security checks
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

		// Security validation: prevent directory traversal in mount paths
		if err := validateMountPath(d.Path); err != nil {
			return fmt.Errorf("invalid mount path %s: %w", d.Path, err)
		}

		// Validate local directory path
		if err := validateLocalDir(d.Dir); err != nil {
			return fmt.Errorf("invalid local directory %s: %w", d.Dir, err)
		}

		// Check for conflicts
		if existing, ok := paths[d.Path]; ok {
			return fmt.Errorf("path conflict: %s maps to both %s and %s", d.Path, existing, d.Dir)
		}
		paths[d.Path] = d.Dir
	}
	return nil
}

// validateMountPath ensures mount paths are safe and don't contain dangerous patterns
func validateMountPath(path string) error {
	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		return errors.New("path contains directory traversal sequence")
	}

	// Ensure normalized path matches original (after cleaning)
	if cleanPath != path && cleanPath+"/" != path {
		return errors.New("path contains unsafe characters or sequences")
	}

	// Check for null bytes and other control characters
	for _, r := range path {
		if r < 32 && r != 9 && r != 10 && r != 13 { // Allow tab, LF, CR
			return errors.New("path contains control characters")
		}
	}

	return nil
}

// validateLocalDir validates that the local directory exists and is accessible
func validateLocalDir(dir string) error {
	// Clean the directory path
	cleanDir := filepath.Clean(dir)

	// Convert to absolute path
	absDir, err := filepath.Abs(cleanDir)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}

	// Ensure it's actually a directory
	if !info.IsDir() {
		return errors.New("path is not a directory")
	}

	return nil
}
