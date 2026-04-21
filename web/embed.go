// Package webassets embeds the built daemon frontend bundle for HTTP serving.
package webassets

import "embed"

// DistFS embeds the built frontend assets under web/dist.
//
//go:embed all:dist
var DistFS embed.FS
