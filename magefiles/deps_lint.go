//go:build mage

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/magefile/mage/sh"
)

const (
	goLintScopesEnvVar         = "COMPOZY_GO_LINT_SCOPES"
	goLintConcurrencyEnvVar    = "COMPOZY_GO_LINT_CONCURRENCY"
	golangciLintMaxConcurrency = 8
)

func Deps() error {
	return sh.RunV("go", "mod", "tidy")
}

// DepsCheck verifies that the module graph is tidy and its downloaded content is intact.
func DepsCheck() error {
	if err := sh.RunV("go", "mod", "tidy", "-diff"); err != nil {
		return fmt.Errorf("check tidy module graph: %w", err)
	}
	if err := sh.RunV("go", "mod", "verify"); err != nil {
		return fmt.Errorf("verify module graph: %w", err)
	}
	return nil
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
	if err := SourceSize(); err != nil {
		return err
	}
	if err := GoLint(); err != nil {
		return err
	}
	return BunLint()
}

// GoLint runs the complete Go-only static analysis gate.
func GoLint() error {
	if err := SourcePolicy(); err != nil {
		return err
	}
	return goLint()
}

func goLint() error {
	return runGolangCILint()
}

func runGolangCILint() error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve golangci-lint project root: %w", err)
	}
	cacheDir, err := golangCILintCacheDir(os.Getenv("GOLANGCI_LINT_CACHE"), projectRoot)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return fmt.Errorf("create golangci-lint cache directory: %w", err)
	}
	env := map[string]string{"GOLANGCI_LINT_CACHE": cacheDir}

	args := []string{
		"run",
		"--allow-parallel-runners",
		"--timeout",
		golangciLintTimeout,
		"--concurrency",
		golangciLintConcurrency(),
	}
	args = append(args, golangciLintScopes(os.Getenv(goLintScopesEnvVar))...)
	if hasPinnedTool("golangci-lint", golangciLintVersion, "version") {
		return sh.RunWithV(env, "golangci-lint", args...)
	}
	goRunArgs := append(
		[]string{"run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + golangciLintVersion},
		args...,
	)
	return sh.RunWithV(env, "go", goRunArgs...)
}

// golangciLintScopes returns the package patterns to lint: explicit
// space-separated scopes, or the whole module when unset.
func golangciLintScopes(raw string) []string {
	scopes := strings.Fields(raw)
	if len(scopes) == 0 {
		return []string{"./..."}
	}
	return scopes
}

func golangciLintConcurrency() string {
	if raw := strings.TrimSpace(os.Getenv(goLintConcurrencyEnvVar)); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return strconv.Itoa(value)
		}
		fmt.Printf("Warning: ignoring invalid %s=%q; using derived cap\n", goLintConcurrencyEnvVar, raw)
	}
	return strconv.Itoa(golangciLintConcurrencyFor(runtime.GOMAXPROCS(0)))
}

// golangciLintConcurrencyFor keeps small machines at full width and caps larger
// hosts: a full-width run pins every core, and past the performance cores Apple
// Silicon runs slower, not faster.
func golangciLintConcurrencyFor(effectiveCPU int) int {
	if effectiveCPU < 1 {
		return 1
	}
	if effectiveCPU <= golangciLintMaxConcurrency {
		return effectiveCPU
	}
	return golangciLintMaxConcurrency
}

func golangCILintCacheDir(explicit, projectRoot string) (string, error) {
	if cacheDir := strings.TrimSpace(explicit); cacheDir != "" {
		return cacheDir, nil
	}
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache directory: %w", err)
	}
	rootDigest := sha256.Sum256([]byte(filepath.Clean(projectRoot)))
	return filepath.Join(
		userCacheDir,
		"compozy-dev",
		"golangci-lint",
		strings.TrimPrefix(golangciLintVersion, "v"),
		fmt.Sprintf("%x", rootDigest[:8]),
	), nil
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
