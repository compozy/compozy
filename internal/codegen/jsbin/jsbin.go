// Package jsbin resolves workspace-pinned JS tool binaries with a bunx fallback.
package jsbin

import (
	"os"
	"path/filepath"
	"runtime"
)

const bunxFallback = "bunx"

// Argv returns the command argv for a JS CLI tool: the workspace-local
// node_modules/.bin shim when present and executable (the version pinned by the
// workspace lockfile), else a bunx invocation resolved from the global cache.
func Argv(workspaceRoot string, tool string, args ...string) []string {
	binPath := filepath.Join(workspaceRoot, "node_modules", ".bin", tool)
	if isExecutableFile(binPath) {
		return append([]string{binPath}, args...)
	}
	return append([]string{bunxFallback, tool}, args...)
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}
