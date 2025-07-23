// Package fileutil provides MIME type detection and file type utilities.
package fileutil

import (
	"mime"
	"path/filepath"
	"strings"
)

// DetectMimeType determines the MIME type of a file based on its extension.
// It first tries the standard library's mime package, then falls back to custom mappings.
func DetectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	// Custom MIME type mappings for common file types not covered by stdlib
	customTypes := map[string]string{
		".md":   "text/markdown",
		".yaml": "application/x-yaml",
		".yml":  "application/x-yaml",
		".log":  "text/plain",
		".conf": "text/plain",
		".cfg":  "text/plain",
		".ini":  "text/plain",
	}

	if mimeType, exists := customTypes[ext]; exists {
		return mimeType
	}

	// Default to binary stream for unknown types
	return "application/octet-stream"
}

// IsTextFile determines whether a file should be treated as text based on its MIME type.
func IsTextFile(filename string) bool {
	mimeType := DetectMimeType(filename)
	return strings.HasPrefix(mimeType, "text/") ||
		strings.Contains(mimeType, "json") ||
		strings.Contains(mimeType, "xml") ||
		strings.Contains(mimeType, "yaml")
}

// IsImageFile determines whether a file is an image based on its MIME type.
func IsImageFile(filename string) bool {
	mimeType := DetectMimeType(filename)
	return strings.HasPrefix(mimeType, "image/")
}

// IsVideoFile determines whether a file is a video based on its MIME type.
func IsVideoFile(filename string) bool {
	mimeType := DetectMimeType(filename)
	return strings.HasPrefix(mimeType, "video/")
}

// IsAudioFile determines whether a file is audio based on its MIME type.
func IsAudioFile(filename string) bool {
	mimeType := DetectMimeType(filename)
	return strings.HasPrefix(mimeType, "audio/")
}
