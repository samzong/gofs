package fileutil

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func SafePath(path string) string {
	if path == "" {
		return ""
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return ""
	}

	// Normalize Unicode to prevent encoding-based bypasses
	path = norm.NFC.String(path)

	// Check for various forms of parent directory references
	dangerousPatterns := []string{
		"..",
		"..\\",
		"../",
		"..;",
		"%2e%2e",
		"%252e%252e",
		"0x2e0x2e",
	}

	lowerPath := strings.ToLower(path)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerPath, pattern) {
			return ""
		}
	}

	// Check for control characters
	for _, r := range path {
		if unicode.IsControl(r) && r != '\t' {
			return ""
		}
	}

	// Remove leading path separators
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "\\")

	if path == "" {
		return ""
	}

	// Clean the path
	path = filepath.Clean(path)

	// Additional validation after cleaning
	if strings.Contains(path, "..") ||
		strings.HasPrefix(path, "/") ||
		strings.HasPrefix(path, "\\") ||
		filepath.IsAbs(path) {
		return ""
	}

	// Check for Windows drive letters and UNC paths
	if len(path) >= 2 && path[1] == ':' {
		return ""
	}

	// Check for UNC paths
	if strings.HasPrefix(path, "\\\\") || strings.HasPrefix(path, "//") {
		return ""
	}

	// Final check: ensure the path doesn't escape after normalization
	if !isWithinBounds(path) {
		return ""
	}

	return path
}

// isWithinBounds performs a final check to ensure the path stays within bounds
func isWithinBounds(path string) bool {
	// Split the path and check each component
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})

	depth := 0
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			depth--
			if depth < 0 {
				return false // Escaping the root
			}
		} else {
			depth++
		}
	}

	return true
}

func IsHidden(name string) bool {
	if name == "" {
		return false
	}

	if strings.HasPrefix(name, ".") {
		return true
	}

	systemFiles := []string{
		"Thumbs.db",
		"Desktop.ini",
		".DS_Store",
		"$RECYCLE.BIN",
	}

	for _, sysFile := range systemFiles {
		if strings.EqualFold(name, sysFile) {
			return true
		}
	}

	return false
}

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
