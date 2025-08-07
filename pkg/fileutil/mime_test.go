package fileutil

import (
	"testing"
)

func TestDetectMimeType(t *testing.T) {
	t.Run("HTML and CSS files", testHTMLAndCSS)
	t.Run("JavaScript and JSON files", testJavaScriptAndJSON)
	t.Run("Image files", testImageFiles)
	t.Run("Document files", testDocumentFiles)
	t.Run("Configuration files", testConfigFiles)
	t.Run("Edge cases", testEdgeCases)
}

func testHTMLAndCSS(t *testing.T) {
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

func testJavaScriptAndJSON(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "JavaScript file",
			filename: "script.js",
			expected: "text/javascript; charset=utf-8",
		},
		{
			name:     "JSON file",
			filename: "data.json",
			expected: "application/json; charset=utf-8",
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

func testImageFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
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

func testDocumentFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
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
			expected: "text/markdown; charset=utf-8",
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

func testConfigFiles(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "YAML file",
			filename: "config.yaml",
			expected: "application/x-yaml; charset=utf-8",
		},
		{
			name:     "YML file",
			filename: "docker-compose.yml",
			expected: "application/x-yaml; charset=utf-8",
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

func testEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
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
	testCases := []struct {
		name     string
		filename string
		expected bool
	}{
		{"HTML file", "index.html", true},
		{"JavaScript file", "app.js", true},
		{"JSON file", "data.json", true},
		{"Markdown file", "README.md", true},
		{"YAML file", "config.yaml", true},
		{"XML file", "config.xml", true},
		{"Plain text", "notes.txt", true},
		{"Binary file", "app.exe", false},
		{"Image file", "photo.jpg", false},
		{"PDF file", "doc.pdf", false},
	}

	runBoolTests(t, testCases, IsTextFile)
}

func runBoolTests(t *testing.T, tests []struct {
	name     string
	filename string
	expected bool
}, fn func(string) bool) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fn(tt.filename)
			if result != tt.expected {
				t.Errorf("function(%q) = %v, want %v", tt.filename, result, tt.expected)
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
