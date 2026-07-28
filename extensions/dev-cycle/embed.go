package devcycle

import (
	"embed"
	"io/fs"
)

//go:embed extension.json loops agents skills
var bundledFS embed.FS

// FS returns the embedded dev-cycle extension payload.
func FS() fs.FS {
	return bundledFS
}
