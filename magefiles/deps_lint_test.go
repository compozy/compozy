//go:build mage

package main

import (
	"path/filepath"
	"testing"
)

func TestGolangCILintCacheDirShouldPreserveExplicitIsolation(t *testing.T) {
	t.Parallel()

	want := filepath.Join(t.TempDir(), "isolated-lint-cache")
	got, err := golangCILintCacheDir("  " + want + "  ")
	if err != nil {
		t.Fatalf("golangCILintCacheDir() error = %v", err)
	}
	if got != want {
		t.Fatalf("golangCILintCacheDir() = %q, want %q", got, want)
	}
}
