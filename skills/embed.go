package skills

import "embed"

// FS holds every file under the skills/ directory (SKILL.md and references/).
// The go:embed directive intentionally uses explicit patterns to exclude
// non-skill artifacts such as autoresearch result directories.
//
//go:embed */SKILL.md */references/*
var FS embed.FS
