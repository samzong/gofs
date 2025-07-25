package fileutil

import (
	"testing"
)

func TestDetectMimeType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "HTML file",
			filename: "index.html",
			expected: "text/html; charset=utf-8",
		},
		{
			name:     "CSS file",
			filename: "style.css",
			expected: "text/css; charset=utf-8",
		},
		{
			name:     "JavaScript file",
			filename: "script.js",
			expected: "text/javascript; charset=utf-8",
		},
		{
			name:     "JSON file",
			filename: "data.json",
			expected: "application/json",
		},
		{
			name:     "PNG image",
			filename: "image.png",
			expected: "image/png",
		},
		{
			name:     "JPEG image",
			filename: "photo.jpg",
			expected: "image/jpeg",
		},
		{
			name:     "PDF file",
			filename: "document.pdf",
			expected: "application/pdf",
		},
		{
			name:     "Text file",
			filename: "readme.txt",
			expected: "text/plain; charset=utf-8",
		},
		{
			name:     "Markdown file",
			filename: "README.md",
			expected: "text/markdown",
		},
		{
			name:     "YAML file",
			filename: "config.yaml",
			expected: "application/x-yaml",
		},
		{
			name:     "YML file",
			filename: "docker-compose.yml",
			expected: "application/x-yaml",
		},
		{
			name:     "Log file",
			filename: "server.log",
			expected: "text/plain; charset=utf-8",
		},
		{
			name:     "Config file",
			filename: "app.conf",
			expected: "text/plain; charset=utf-8",
		},
		{
			name:     "Unknown extension",
			filename: "file.unknown",
			expected: "application/octet-stream",
		},
		{
			name:     "No extension",
			filename: "Makefile",
			expected: "application/octet-stream",
		},
		{
			name:     "Case insensitive",
			filename: "IMAGE.PNG",
			expected: "image/png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectMimeType(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectMimeType(%q) = %q, want %q", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{
			name:     "HTML file",
			filename: "index.html",
			expected: true,
		},
		{
			name:     "JavaScript file",
			filename: "app.js",
			expected: true,
		},
		{
			name:     "JSON file",
			filename: "data.json",
			expected: true,
		},
		{
			name:     "Markdown file",
			filename: "README.md",
			expected: true,
		},
		{
			name:     "YAML file",
			filename: "config.yaml",
			expected: true,
		},
		{
			name:     "XML file",
			filename: "config.xml",
			expected: true,
		},
		{
			name:     "Plain text",
			filename: "notes.txt",
			expected: true,
		},
		{
			name:     "Binary file",
			filename: "app.exe",
			expected: false,
		},
		{
			name:     "Image file",
			filename: "photo.jpg",
			expected: false,
		},
		{
			name:     "PDF file",
			filename: "doc.pdf",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTextFile(tt.filename)
			if result != tt.expected {
				t.Errorf("IsTextFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{
			name:     "JPEG image",
			filename: "photo.jpg",
			expected: true,
		},
		{
			name:     "PNG image",
			filename: "logo.png",
			expected: true,
		},
		{
			name:     "GIF image",
			filename: "animation.gif",
			expected: true,
		},
		{
			name:     "SVG image",
			filename: "icon.svg",
			expected: true,
		},
		{
			name:     "Text file",
			filename: "readme.txt",
			expected: false,
		},
		{
			name:     "Video file",
			filename: "movie.mp4",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsImageFile(tt.filename)
			if result != tt.expected {
				t.Errorf("IsImageFile(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}
