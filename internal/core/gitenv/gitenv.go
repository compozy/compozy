// Package gitenv centralizes process environment sanitization for internal git
// commands that explicitly select their repository with -C or Cmd.Dir.
package gitenv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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

// Command constructs a git subprocess pinned to dir with inherited repository
// variables removed from the environment.
func Command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = strings.TrimSpace(dir)
	cmd.Env = SanitizedEnv()
	return cmd
}

// Run executes a sanitized git subprocess and returns trimmed stdout.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := Command(ctx, dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
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
