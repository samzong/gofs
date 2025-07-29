// Package templates provides embedded HTML templates for the gofs file server.
package templates

import (
	"html/template"

	_ "embed"
)

// DirectoryHTML contains the HTML template for directory listings.
//
//go:embed directory.html
var DirectoryHTML string

// StylesCSS contains the default CSS styles for the directory listing page.
//
//go:embed styles.css
var StylesCSS string

// Theme CSS files - Enterprise optimized themes only
//
//go:embed themes/classic.css
var ClassicCSS string

//go:embed themes/default.css
var DefaultCSS string

// GetThemeCSS returns the CSS content for the specified theme.
// Only supports enterprise-grade 'classic' and 'default' themes.
// Falls back to the default theme if the theme is not recognized.
func GetThemeCSS(theme string) string {
	switch theme {
	case "classic":
		return ClassicCSS
	case "default", "":
		return DefaultCSS
	default:
		return DefaultCSS // fallback to default theme for enterprise stability
	}
}

// DirectoryTemplate is the pre-compiled template for directory listings.
// Pre-compiling templates improves performance by avoiding parsing on each request.
var DirectoryTemplate = template.Must(template.New("directory").Parse(DirectoryHTML))
