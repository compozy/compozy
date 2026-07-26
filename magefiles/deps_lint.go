//go:build mage

package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/magefile/mage/sh"
)

func Deps() error {
	return sh.RunV("go", "mod", "tidy")
}

func Fmt() error {
	files, err := goFiles(".")
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"-w"}, files...)
	return sh.RunV("gofmt", args...)
}

// FmtCheck verifies that every Go source file is gofmt-clean without mutating the worktree.
func FmtCheck() error {
	files, err := goFiles(".")
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	args := append([]string{"-l"}, files...)
	output, err := sh.Output("gofmt", args...)
	if err != nil {
		return fmt.Errorf("check gofmt: %w", err)
	}
	if unformatted := strings.TrimSpace(output); unformatted != "" {
		return fmt.Errorf("gofmt required for:\n%s", unformatted)
	}
	return nil
}

func Lint() error {
	if err := goLint(); err != nil {
		return err
	}
	return BunLint()
}

// GoLint runs the complete Go-only static analysis gate.
func GoLint() error {
	return goLint()
}

func goLint() error {
	return runGolangCILint()
}

func runGolangCILint() error {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("resolve user cache directory: %w", err)
	}
	cacheDir := filepath.Join(
		userCacheDir,
		"agh-dev",
		"golangci-lint",
		strings.TrimPrefix(golangciLintVersion, "v"),
	)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create golangci-lint cache directory: %w", err)
	}
	env := map[string]string{"GOLANGCI_LINT_CACHE": cacheDir}

	args := []string{
		"run",
		"--allow-parallel-runners",
		"--timeout",
		golangciLintTimeout,
		"./...",
	}
	if hasPinnedTool("golangci-lint", golangciLintVersion, "version") {
		return sh.RunWithV(env, "golangci-lint", args...)
	}
	goRunArgs := append(
		[]string{"run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + golangciLintVersion},
		args...,
	)
	return sh.RunWithV(env, "go", goRunArgs...)
}

func hasPinnedTool(name string, wantVersion string, versionArgs ...string) bool {
	path, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	output, err := exec.Command(path, versionArgs...).CombinedOutput()
	if err != nil {
		return false
	}
	version := strings.TrimPrefix(wantVersion, "v")
	return bytes.Contains(output, []byte("version "+version)) ||
		bytes.Contains(output, []byte("version v"+version))
}

func goFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (name == "vendor" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}
