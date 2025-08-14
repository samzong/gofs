// Package themes provides the advanced theme for GoFS
package themes

import (
	_ "embed"
)

// Advanced theme embedded files
//
//go:embed advanced.html
var AdvancedHTML string

//go:embed advanced.css
var AdvancedCSS string

//go:embed advanced.js
var AdvancedJS string

// ThemeData represents the data structure for rendering the advanced theme
type ThemeData struct {
	Path        string
	Parent      bool
	Files       []FileItem
	FileCount   int
	Breadcrumbs []BreadcrumbItem
	CSS         string
	JS          string
}

// FileItem represents a file or directory in the listing
type FileItem struct {
	Name          string
	IsDir         bool
	Size          int64
	FormattedSize string
	FormattedTime string
}

// BreadcrumbItem represents a breadcrumb navigation item
type BreadcrumbItem struct {
	Name string
	Path string
}
