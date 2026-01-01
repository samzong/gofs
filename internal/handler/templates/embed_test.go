package templates

import (
	"strings"
	"testing"
)

func TestGetThemeCSS(t *testing.T) {
	testCases := []struct {
		name        string
		theme       string
		expectedCSS string
		description string
	}{
		{
			name:        "default_theme",
			theme:       "default",
			expectedCSS: DefaultCSS,
			description: "Should return default CSS for 'default' theme",
		},
		{
			name:        "empty_theme",
			theme:       "",
			expectedCSS: DefaultCSS,
			description: "Should return default CSS for empty theme",
		},
		{
			name:        "advanced_theme",
			theme:       "advanced",
			expectedCSS: AdvancedCSS,
			description: "Should return advanced CSS for 'advanced' theme",
		},
		{
			name:        "unknown_theme",
			theme:       "unknown",
			expectedCSS: DefaultCSS,
			description: "Should return default CSS for unknown theme",
		},
		{
			name:        "case_sensitive_theme",
			theme:       "Default",
			expectedCSS: DefaultCSS,
			description: "Should return default CSS for case-mismatched theme",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetThemeCSS(tc.theme)
			if result != tc.expectedCSS {
				t.Errorf("GetThemeCSS(%q): expected to return the correct CSS content", tc.theme)
			}

			// Verify CSS content is not empty
			if result == "" {
				t.Errorf("GetThemeCSS(%q): returned empty CSS content", tc.theme)
			}
		})
	}
}

func TestEmbeddedContent_NotEmpty(t *testing.T) {
	testCases := []struct {
		name    string
		content string
		desc    string
	}{
		{"DirectoryHTML", DirectoryHTML, "Directory HTML template"},
		{"StylesCSS", StylesCSS, "Base styles CSS"},

		{"DefaultCSS", DefaultCSS, "Default theme CSS"},
		{"AdvancedCSS", AdvancedCSS, "Advanced theme CSS"},
		{"AdvancedJS", AdvancedJS, "Advanced theme JavaScript"},
		{"AdvancedHTML", AdvancedHTML, "Advanced theme HTML"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.content == "" {
				t.Errorf("%s should not be empty", tc.desc)
			}

			if len(tc.content) < 10 {
				t.Errorf("%s should have meaningful content (got %d characters)", tc.desc, len(tc.content))
			}
		})
	}
}

func TestEmbeddedHTML_Structure(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		contains []string
		desc     string
	}{
		{
			name:    "DirectoryHTML",
			content: DirectoryHTML,
			contains: []string{
				"<html", "</html>",
				"<head", "</head>",
				"<body", "</body>",
			},
			desc: "Directory HTML should have basic HTML structure",
		},
		{
			name:    "AdvancedHTML",
			content: AdvancedHTML,
			contains: []string{
				"<html", "</html>",
				"<head", "</head>",
				"<body", "</body>",
			},
			desc: "Advanced HTML should have basic HTML structure",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := strings.ToLower(tc.content)

			for _, expected := range tc.contains {
				if !strings.Contains(content, strings.ToLower(expected)) {
					t.Errorf("%s should contain %q", tc.desc, expected)
				}
			}
		})
	}
}

func TestEmbeddedCSS_Structure(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		contains []string
		desc     string
	}{
		{
			name:    "StylesCSS",
			content: StylesCSS,
			contains: []string{
				"{", "}",
			},
			desc: "Base styles should contain CSS syntax",
		},

		{
			name:    "DefaultCSS",
			content: DefaultCSS,
			contains: []string{
				"{", "}",
			},
			desc: "Default CSS should contain CSS syntax",
		},
		{
			name:    "AdvancedCSS",
			content: AdvancedCSS,
			contains: []string{
				"{", "}",
			},
			desc: "Advanced CSS should contain CSS syntax",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, expected := range tc.contains {
				if !strings.Contains(tc.content, expected) {
					t.Errorf("%s should contain %q", tc.desc, expected)
				}
			}
		})
	}
}

func TestEmbeddedJS_Structure(t *testing.T) {
	// Test JavaScript content structure
	if AdvancedJS == "" {
		t.Error("Advanced JavaScript should not be empty")
		return
	}

	// Basic JavaScript syntax checks
	jsContent := AdvancedJS

	// Should contain some basic JavaScript patterns
	jsPatterns := []string{
		"function", // Should have functions
		"(", ")",   // Should have parentheses
		"{", "}", // Should have braces
	}

	for _, pattern := range jsPatterns {
		if !strings.Contains(jsContent, pattern) {
			t.Errorf("Advanced JavaScript should contain %q", pattern)
		}
	}
}

func TestDirectoryTemplate_Valid(t *testing.T) {
	if DirectoryTemplate == nil {
		t.Fatal("DirectoryTemplate should not be nil")
	}

	// Test template name
	if DirectoryTemplate.Name() != "directory" {
		t.Errorf("DirectoryTemplate should have name 'directory', got %q", DirectoryTemplate.Name())
	}
}

func TestAdvancedTemplate_Valid(t *testing.T) {
	if AdvancedTemplate == nil {
		t.Fatal("AdvancedTemplate should not be nil")
	}

	// Test template name
	if AdvancedTemplate.Name() != "advanced" {
		t.Errorf("AdvancedTemplate should have name 'advanced', got %q", AdvancedTemplate.Name())
	}
}

func TestTemplate_Execution(t *testing.T) {
	testCases := []struct {
		name     string
		template interface {
			Name() string
		}
		data map[string]any
	}{
		{
			name:     "DirectoryTemplate",
			template: DirectoryTemplate,
			data: map[string]any{
				"Path":        "/test",
				"Parent":      true,
				"Files":       []any{},
				"FileCount":   0,
				"Breadcrumbs": []any{},
				"CSS":         "/* test css */",
			},
		},
		{
			name:     "AdvancedTemplate",
			template: AdvancedTemplate,
			data: map[string]any{
				"Path":        "/test",
				"Parent":      true,
				"Files":       []any{},
				"FileCount":   0,
				"Breadcrumbs": []any{},
				"CSS":         "/* test css */",
				"JS":          "// test js",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf strings.Builder

			// Execute template - should not panic
			err := DirectoryTemplate.Execute(&buf, tc.data)
			if tc.name == "AdvancedTemplate" {
				err = AdvancedTemplate.Execute(&buf, tc.data)
			}

			if err != nil {
				t.Errorf("Template execution failed: %v", err)
			}

			// Should produce some output
			if buf.Len() == 0 {
				t.Errorf("Template should produce output")
			}
		})
	}
}

func TestAllThemes_Coverage(t *testing.T) {
	// Test that all themes defined in GetThemeCSS actually have content
	themes := []string{"default", "advanced"}

	for _, theme := range themes {
		t.Run("theme_"+theme, func(t *testing.T) {
			css := GetThemeCSS(theme)
			if css == "" {
				t.Errorf("Theme %q should have CSS content", theme)
			}
		})
	}
}

// Benchmark template retrieval performance
func BenchmarkGetThemeCSS_Default(b *testing.B) {
	for range b.N {
		GetThemeCSS("default")
	}
}

func BenchmarkGetThemeCSS_Advanced(b *testing.B) {
	for range b.N {
		GetThemeCSS("advanced")
	}
}

func BenchmarkGetThemeCSS_Unknown(b *testing.B) {
	for range b.N {
		GetThemeCSS("nonexistent")
	}
}
