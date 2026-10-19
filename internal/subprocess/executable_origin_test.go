package subprocess

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveExecutablePermissionDistinction(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}

	t.Run("Should retain permission when only non-executable candidates exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "blocked")
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		_, err := ResolveExecutable("blocked", []string{"PATH=" + dir}, dir)
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("error = %v, want os.ErrPermission", err)
		}
		if errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("error = %v, must not collapse into not-found", err)
		}
	})

	t.Run("Should prefer later executable PATH candidate", func(t *testing.T) {
		t.Parallel()
		first := t.TempDir()
		second := t.TempDir()
		if err := os.WriteFile(filepath.Join(first, "tool"), []byte("x"), 0o644); err != nil {
			t.Fatalf("WriteFile(first) error = %v", err)
		}
		want := filepath.Join(second, "tool")
		if err := os.WriteFile(want, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("WriteFile(second) error = %v", err)
		}
		got, err := ResolveExecutable("tool", []string{"PATH=" + first + string(os.PathListSeparator) + second}, second)
		if err != nil {
			t.Fatalf("ResolveExecutable() error = %v", err)
		}
		if got != filepath.Clean(want) {
			t.Fatalf("executable = %q, want %q", got, want)
		}
	})
}
