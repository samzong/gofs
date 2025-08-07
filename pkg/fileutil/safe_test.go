package fileutil

import (
	"testing"
)

func TestSafePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "simple filename",
			input:    "test.txt",
			expected: "test.txt",
		},
		{
			name:     "path with slash prefix",
			input:    "/test.txt",
			expected: "test.txt",
		},
		{
			name:     "normal directory path",
			input:    "dir/file.txt",
			expected: "dir/file.txt",
		},
		{
			name:     "path with parent directory",
			input:    "../test.txt",
			expected: "",
		},
		{
			name:     "path with multiple parent directories",
			input:    "../../etc/passwd",
			expected: "",
		},
		{
			name:     "complex path with parent directory",
			input:    "dir/../../../etc/passwd",
			expected: "",
		},
		{
			name:     "normal nested path",
			input:    "dir/subdir/file.txt",
			expected: "dir/subdir/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafePath(tt.input)
			if result != tt.expected {
				t.Errorf("SafePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsHidden(t *testing.T) {
	testCases := []struct {
		name     string
		filename string
		expected bool
	}{
		{"empty name", "", false},
		{"normal file", "test.txt", false},
		{"dot file", ".hidden", true},
		{"dot directory", ".git", true},
		{"current directory", ".", true},
		{"parent directory", "..", true},
		{"DS_Store", ".DS_Store", true},
		{"Thumbs.db", "Thumbs.db", true},
		{"Desktop.ini", "Desktop.ini", true},
		{"normal system file", "system32", false},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHidden(tt.filename)
			if result != tt.expected {
				t.Errorf("IsHidden(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		size     int64
	}{
		{
			name:     "zero bytes",
			size:     0,
			expected: "0 B",
		},
		{
			name:     "bytes",
			size:     500,
			expected: "500.0 B",
		},
		{
			name:     "kilobytes",
			size:     1536, // 1.5 KB
			expected: "1.5 KB",
		},
		{
			name:     "megabytes",
			size:     1572864, // 1.5 MB
			expected: "1.5 MB",
		},
		{
			name:     "gigabytes",
			size:     1610612736, // 1.5 GB
			expected: "1.5 GB",
		},
		{
			name:     "exactly 1 KB",
			size:     1024,
			expected: "1.0 KB",
		},
		{
			name:     "exactly 1 MB",
			size:     1048576,
			expected: "1.0 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSize(tt.size)
			if result != tt.expected {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.size, result, tt.expected)
			}
		})
	}
}
