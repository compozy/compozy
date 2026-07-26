package skills

import (
	"embed"
	"io/fs"
)

// embeddedSkills stores the bundled skills compiled into the Compozy binary.
//
//go:embed compozy
var embeddedSkills embed.FS

// FS returns the bundled skills filesystem compiled into the Compozy binary.
func FS() fs.FS {
	return embeddedSkills
}
