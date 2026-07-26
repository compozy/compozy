package jsbin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestArgv(t *testing.T) {
	t.Parallel()

	writeShim := func(t *testing.T, root string, tool string, mode os.FileMode) string {
		t.Helper()
		binDir := filepath.Join(root, "node_modules", ".bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			t.Fatalf("create bin dir: %v", err)
		}
		shim := filepath.Join(binDir, tool)
		if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
			t.Fatalf("write shim: %v", err)
		}
		return shim
	}

	t.Run("Should resolve the workspace-local shim when present and executable", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		shim := writeShim(t, root, "oxfmt", 0o755)
		got := Argv(root, "oxfmt", "--check", "file.ts")
		want := []string{shim, "--check", "file.ts"}
		if len(got) != len(want) {
			t.Fatalf("Argv() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Argv()[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("Should fall back to bunx when the shim is missing", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		got := Argv(root, "openapi-typescript", "spec.json")
		want := []string{"bunx", "openapi-typescript", "spec.json"}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Argv() = %v, want %v", got, want)
			}
		}
	})

	t.Run("Should fall back to bunx when the shim is not executable", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("executable bit is not meaningful on windows")
		}
		root := t.TempDir()
		writeShim(t, root, "oxfmt", 0o644)
		got := Argv(root, "oxfmt")
		if got[0] != "bunx" || got[1] != "oxfmt" {
			t.Fatalf("Argv() = %v, want bunx fallback", got)
		}
	})

	t.Run("Should fall back to bunx when the shim path is a directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "node_modules", ".bin", "oxfmt"), 0o755); err != nil {
			t.Fatalf("create dir shim: %v", err)
		}
		got := Argv(root, "oxfmt")
		if got[0] != "bunx" {
			t.Fatalf("Argv() = %v, want bunx fallback", got)
		}
	})
}
