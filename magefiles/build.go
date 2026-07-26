//go:build mage

package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/magefile/mage/sh"
)

func Build() error {
	return runMageSteps(buildSteps())
}

func buildSteps() []mageStep {
	return []mageStep{
		{name: "CodegenCheck", run: CodegenCheck},
		{name: "buildGo", run: buildGo},
	}
}

func buildGo() error {
	ldflags := buildLDFlags()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	if err := sh.RunV("go", "build", "-ldflags", ldflags, "./..."); err != nil {
		return err
	}
	out := filepath.Join(binDir, cliBinary)
	return sh.RunV("go", "build", "-ldflags", ldflags, "-o", out, "./cmd/"+cliBinary)
}

// BuildGo compiles all Go packages and the AGH CLI without rerunning code generation checks.
func BuildGo() error {
	return buildGo()
}

func buildLDFlags() string {
	version, commit := buildGitMetadata(gitOutput)

	buildDate := time.Now().UTC().Format(time.RFC3339)

	return strings.Join([]string{
		"-X " + versionPackage + ".Version=" + version,
		"-X " + versionPackage + ".Commit=" + commit,
		"-X " + versionPackage + ".BuildDate=" + buildDate,
	}, " ")
}

func buildGitMetadata(readGit func(...string) (string, error)) (string, string) {
	version, err := readGit("describe", "--tags", "--always", "--dirty")
	if err != nil || strings.TrimSpace(version) == "" {
		version = "dev"
	}

	commit, err := readGit("rev-parse", "--short", "HEAD")
	if err != nil || strings.TrimSpace(commit) == "" {
		commit = "unknown"
	}
	return version, commit
}
