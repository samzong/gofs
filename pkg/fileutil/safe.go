// Package fileutil provides utilities for safe file operations and path handling.
package fileutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafePath sanitizes a file path to prevent directory traversal attacks.
// It cleans the path and removes any attempts to access parent directories.
func SafePath(path string) string {
	if path == "" {
		return ""
	}

	// Reject paths containing null bytes (security)
	if strings.Contains(path, "\x00") {
		return ""
	}

	// First check for any .. components before cleaning
	if strings.Contains(path, "..") {
		return ""
	}

	// Reject paths starting with / or \ to prevent absolute path access
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return ""
	}

	// Clean the path to resolve any . elements
	path = filepath.Clean("/" + path)

	// Remove leading slash to make it relative
	path = strings.TrimPrefix(path, "/")

	// Triple validation: check again after cleaning
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return ""
	}

	// Additional check for drive letters on Windows (C:, D:, etc.)
	if len(path) >= 2 && path[1] == ':' {
		return ""
	}

	return path
}

// IsHidden determines whether a file or directory should be considered hidden.
// It checks for Unix-style hidden files (starting with .) and common system files.
func IsHidden(name string) bool {
	if name == "" {
		return false
	}

	// Unix-style hidden files (starting with dot)
	if strings.HasPrefix(name, ".") {
		return true
	}

	// Common system files that should be hidden
	systemFiles := []string{
		"Thumbs.db",    // Windows thumbnail cache
		"Desktop.ini",  // Windows folder settings
		".DS_Store",    // macOS folder metadata
		"$RECYCLE.BIN", // Windows recycle bin
	}

	for _, sysFile := range systemFiles {
		if strings.EqualFold(name, sysFile) {
			return true
		}
	}

	return false
}

// FormatSize formats a file size in bytes into a human-readable string.
// It uses binary units (1024-based) and returns appropriate unit suffixes.
func FormatSize(size int64) string {
	if size == 0 {
		return "0 B"
	}

	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%.1f B", float64(size))
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp+1])
}
