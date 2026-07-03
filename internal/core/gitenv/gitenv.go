// Package gitenv centralizes process environment sanitization for internal git
// commands that explicitly select their repository with -C or Cmd.Dir.
package gitenv

import (
	"os"
	"strings"
)

// SanitizedEnv returns the current process environment without repository-local
// Git variables. Transport and credential variables such as GIT_SSH_COMMAND are
// intentionally preserved.
func SanitizedEnv() []string {
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if IsRepositoryEnvName(name) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// IsRepositoryEnvName reports whether name points Git at an inherited repository,
// index, object store, or namespace that can override an explicit worktree path.
func IsRepositoryEnvName(name string) bool {
	switch strings.TrimSpace(name) {
	case "GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_NAMESPACE",
		"GIT_PREFIX":
		return true
	default:
		return false
	}
}
