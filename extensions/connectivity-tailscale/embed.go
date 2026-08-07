package connectivitytailscale

import (
	"embed"
	"io/fs"
)

//go:embed extension.json
var bundledFS embed.FS

// FS returns the embedded Tailscale connectivity extension payload.
func FS() fs.FS {
	return bundledFS
}
