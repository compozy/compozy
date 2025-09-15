package core

import (
	"path/filepath"
	"strings"
)

// IsTypeScript reports whether the given path has a .ts extension (case-insensitive).
func IsTypeScript(path string) bool {
	ext := filepath.Ext(path)
	return strings.EqualFold(ext, ".ts")
}
