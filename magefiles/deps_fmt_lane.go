//go:build mage

package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/magefile/mage/sh"
)

const (
	goLintFormattersEnvVar = "COMPOZY_GO_LINT_FORMATTERS"
	goFmtFilesEnvVar       = "COMPOZY_GO_FMT_FILES"
	gateBaseEnvVar         = "GATE_BASE"
)

// golangciFormattersInRun reports whether formatters stay inside `run` (the
// CI default) instead of the local file-scoped `fmt --diff` lane.
func golangciFormattersInRun() bool {
	return formattersMode(os.Getenv(goLintFormattersEnvVar), os.Getenv("CI"))
}

func formattersMode(explicit, ci string) bool {
	switch strings.TrimSpace(explicit) {
	case "run":
		return true
	case "split":
		return false
	case "":
	default:
		fmt.Printf("Warning: ignoring invalid %s=%q; use run or split\n", goLintFormattersEnvVar, explicit)
	}
	return strings.TrimSpace(ci) != ""
}

// goFmtLane checks gofmt/goimports/golines on the changed Go files only:
// formatter analyses are file-scoped and uncached, so package-wide runs cost
// minutes while the changed-file set costs seconds.
func goFmtLane(cacheDir string) error {
	files, err := fmtLaneFiles()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		fmt.Println("go-lint fmt lane: no changed Go files")
		return nil
	}
	fmt.Printf("go-lint fmt lane: checking %d changed Go file(s)\n", len(files))
	env := map[string]string{"GOLANGCI_LINT_CACHE": cacheDir}
	args := append([]string{"fmt", "--diff"}, files...)
	if err := runGolangciCommand(env, args...); err != nil {
		return fmt.Errorf("formatter drift in changed files; apply with `golangci-lint fmt <file>`: %w", err)
	}
	return nil
}

func fmtLaneFiles() ([]string, error) {
	if raw := os.Getenv(goFmtFilesEnvVar); strings.TrimSpace(raw) != "" {
		return filterFmtTargets(strings.Fields(raw), fileExists), nil
	}
	paths, err := changedRepoPaths()
	if err != nil {
		return nil, err
	}
	return filterFmtTargets(paths, fileExists), nil
}

// changedRepoPaths mirrors scripts/gate.sh changed_files: committed diff vs
// the merge base plus dirty and untracked paths.
func changedRepoPaths() ([]string, error) {
	base, err := fmtLaneBase()
	if err != nil {
		return nil, err
	}
	mergeBase, err := sh.Output("git", "merge-base", "HEAD", base)
	if err != nil {
		return nil, fmt.Errorf(
			"resolve merge base against %s (set %s or %s): %w",
			base, gateBaseEnvVar, goFmtFilesEnvVar, err,
		)
	}
	var paths []string
	for _, args := range [][]string{
		{"diff", "--name-only", strings.TrimSpace(mergeBase), "HEAD"},
		{"diff", "--name-only", "HEAD"},
		{"ls-files", "-o", "--exclude-standard"},
	} {
		out, err := sh.Output("git", args...)
		if err != nil {
			return nil, fmt.Errorf("list changed files (git %s): %w", strings.Join(args, " "), err)
		}
		paths = append(paths, strings.Split(out, "\n")...)
	}
	return paths, nil
}

func fmtLaneBase() (string, error) {
	if base := strings.TrimSpace(os.Getenv(gateBaseEnvVar)); base != "" {
		return base, nil
	}
	for _, candidate := range []string{"origin/main", "main"} {
		if _, err := sh.Output("git", "rev-parse", "--verify", "--quiet", candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no usable merge base ref; set %s or %s", gateBaseEnvVar, goFmtFilesEnvVar)
}

// filterFmtTargets keeps existing root-module Go files: sdk/ carries its own
// module and deleted paths would fail the formatter invocation.
func filterFmtTargets(paths []string, exists func(string) bool) []string {
	targets := make([]string, 0, len(paths))
	for _, path := range paths {
		trimmed := strings.TrimSpace(path)
		if trimmed == "" || !strings.HasSuffix(trimmed, ".go") {
			continue
		}
		if strings.HasPrefix(trimmed, "sdk/") {
			continue
		}
		if !exists(trimmed) {
			continue
		}
		targets = append(targets, trimmed)
	}
	slices.Sort(targets)
	return slices.Compact(targets)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
