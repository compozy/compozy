//go:build mage

package main

import (
	"path/filepath"
	"testing"
)

func TestGolangCILintCacheDir(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve explicit isolation", func(t *testing.T) {
		t.Parallel()

		want := filepath.Join(t.TempDir(), "isolated-lint-cache")
		got, err := golangCILintCacheDir("  "+want+"  ", "ignored-project-root")
		if err != nil {
			t.Fatalf("golangCILintCacheDir() error = %v", err)
		}
		if got != want {
			t.Fatalf("golangCILintCacheDir() = %q, want %q", got, want)
		}
	})

	t.Run("Should isolate the implicit cache by project root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		first, err := golangCILintCacheDir("", root)
		if err != nil {
			t.Fatalf("golangCILintCacheDir() first error = %v", err)
		}
		repeated, err := golangCILintCacheDir("", root)
		if err != nil {
			t.Fatalf("golangCILintCacheDir() repeated error = %v", err)
		}
		other, err := golangCILintCacheDir("", t.TempDir())
		if err != nil {
			t.Fatalf("golangCILintCacheDir() other error = %v", err)
		}
		if first != repeated {
			t.Fatalf("golangCILintCacheDir() = %q, want stable %q", repeated, first)
		}
		if first == other {
			t.Fatalf("golangCILintCacheDir() reused %q across project roots", first)
		}
	})
}
