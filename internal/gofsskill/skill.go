package gofsskill

import "embed"

const Root = "gofs"

//go:embed gofs
var FS embed.FS
