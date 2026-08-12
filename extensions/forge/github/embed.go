package github

import (
	"embed"
	"io/fs"
)

//go:embed extension.json
var bundledFS embed.FS

// FS returns the embedded GitHub forge extension payload.
func FS() fs.FS {
	return bundledFS
}
