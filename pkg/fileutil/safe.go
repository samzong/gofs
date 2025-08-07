package fileutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

func SafePath(path string) string {
	if path == "" {
		return ""
	}

	if strings.Contains(path, "\x00") {
		return ""
	}

	if strings.Contains(path, "..") {
		return ""
	}

	path = strings.TrimPrefix(path, "/")
	path = strings.TrimPrefix(path, "\\")

	if path == "" {
		return ""
	}

	path = filepath.Clean(path)

	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") {
		return ""
	}

	if len(path) >= 2 && path[1] == ':' {
		return ""
	}

	return path
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
