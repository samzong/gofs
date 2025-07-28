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

// StylesCSS contains the CSS styles for the directory listing page.
//
//go:embed styles.css
var StylesCSS string

// DirectoryTemplate is the pre-compiled template for directory listings.
// Pre-compiling templates improves performance by avoiding parsing on each request.
var DirectoryTemplate = template.Must(template.New("directory").Parse(DirectoryHTML))
