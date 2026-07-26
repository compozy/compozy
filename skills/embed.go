package skills

import (
	"embed"
	"io/fs"
)

// embeddedSkills stores the bundled skills compiled into the AGH binary.
//
//go:embed agh
var embeddedSkills embed.FS

// FS returns the bundled skills filesystem compiled into the AGH binary.
func FS() fs.FS {
	return embeddedSkills
}
