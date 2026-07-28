//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMain chdirs to the module root so mage helpers that use repo-relative
// paths (packages/, web/, openapi/, …) behave the same under
// `go test -tags mage ./magefiles` as under `mage` (`-w .`).
func TestMain(m *testing.M) {
	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "magefiles tests: %v\n", err)
		os.Exit(1)
	}
	if err := os.Chdir(root); err != nil {
		fmt.Fprintf(os.Stderr, "magefiles tests: chdir %s: %v\n", root, err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		goMod := filepath.Join(dir, "go.mod")
		if info, statErr := os.Stat(goMod); statErr == nil && !info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
