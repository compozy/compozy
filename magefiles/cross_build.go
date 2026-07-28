//go:build mage

package main

import (
	"fmt"

	"github.com/magefile/mage/sh"
)

var windowsSubprocessBoundaryPackages = []string{
	"./internal/procutil",
	"./internal/subprocess",
	"./internal/acp",
	"./internal/hooks",
}

// WindowsSubprocessBuild cross-compiles the process-lifecycle boundary packages.
func WindowsSubprocessBuild() error {
	env := map[string]string{
		"CGO_ENABLED": "0",
		"GOARCH":      "amd64",
		"GOOS":        "windows",
	}
	args := append([]string{"build"}, windowsSubprocessBoundaryPackages...)
	if err := sh.RunWithV(env, "go", args...); err != nil {
		return fmt.Errorf("cross-build Windows subprocess boundaries: %w", err)
	}
	return nil
}
