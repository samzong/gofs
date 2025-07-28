// Package fileutil provides MIME type detection and file type utilities.
package fileutil

import (
	"mime"
	"path/filepath"
	"strings"
)

// DetectMimeType determines the MIME type of a file based on its extension.
// It uses a comprehensive mapping to ensure consistent results across platforms.
func DetectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))

	// Comprehensive MIME type mappings to ensure consistency across platforms
	mimeTypes := map[string]string{
		// Web files
		".html": "text/html; charset=utf-8",
		".htm":  "text/html; charset=utf-8",
		".css":  "text/css; charset=utf-8",
		".js":   "text/javascript; charset=utf-8",
		".json": "application/json",
		".xml":  "application/xml",

		// Images
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".svg":  "image/svg+xml",
		".webp": "image/webp",
		".ico":  "image/x-icon",

		// Documents
		".pdf": "application/pdf",
		".txt": "text/plain; charset=utf-8",
		".md":  "text/markdown",
		".rtf": "application/rtf",

		// Archives
		".zip": "application/zip",
		".tar": "application/x-tar",
		".gz":  "application/gzip",
		".rar": "application/x-rar-compressed",

		// Config files
		".yaml": "application/x-yaml",
		".yml":  "application/x-yaml",
		".toml": "application/toml",
		".ini":  "text/plain; charset=utf-8",
		".conf": "text/plain; charset=utf-8",
		".cfg":  "text/plain; charset=utf-8",
		".log":  "text/plain; charset=utf-8",

		// Programming languages
		".go":   "text/plain; charset=utf-8",
		".py":   "text/plain; charset=utf-8",
		".java": "text/plain; charset=utf-8",
		".c":    "text/plain; charset=utf-8",
		".cpp":  "text/plain; charset=utf-8",
		".h":    "text/plain; charset=utf-8",
		".sh":   "text/plain; charset=utf-8",

		// Audio
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".ogg":  "audio/ogg",
		".flac": "audio/flac",

		// Video
		".mp4":  "video/mp4",
		".avi":  "video/x-msvideo",
		".mov":  "video/quicktime",
		".webm": "video/webm",
	}

	if mimeType, exists := mimeTypes[ext]; exists {
		return mimeType
	}

	// Fallback to standard library for any types we might have missed
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
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
