package templates

import (
	"html/template"

	_ "embed"
)

//go:embed directory.html
var DirectoryHTML string

//go:embed styles.css
var StylesCSS string

//go:embed themes/classic.css
var ClassicCSS string

//go:embed themes/default.css
var DefaultCSS string

//go:embed themes/advanced.css
var AdvancedCSS string

//go:embed themes/advanced.js
var AdvancedJS string

//go:embed themes/advanced.html
var AdvancedHTML string

func GetThemeCSS(theme string) string {
	switch theme {
	case "classic":
		return ClassicCSS
	case "advanced":
		return AdvancedCSS
	case "default", "":
		return DefaultCSS
	default:
		return DefaultCSS
	}
}

var DirectoryTemplate = template.Must(template.New("directory").Parse(DirectoryHTML))
var AdvancedTemplate = template.Must(template.New("advanced").Parse(AdvancedHTML))
