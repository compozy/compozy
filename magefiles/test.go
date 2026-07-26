//go:build mage

package main

import (
	"context"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/e2elane"
)

// Test runs unit tests only (no integration tag).
func Test() error {
	ctx := context.Background()
	packages, err := goUnitTestPackages(ctx)
	if err != nil {
		return err
	}
	args := []string{
		"--format", "pkgname", "--", "-race", "-p", goUnitTestPackageLimit(),
		"-parallel=" + strconv.Itoa(goUnitTestParallelism),
		"-timeout", goUnitTestTimeout,
	}
	args = append(args, packages...)
	return runGotestsum(ctx, nil, args...)
}

// TestIntegration runs all tests including integration tests.
func TestIntegration() error {
	return runGotestsum(context.Background(), nil,
		"--format", "pkgname", "--", "-race", "-p", goIntegrationPackageLimit, "-parallel=4",
		"-timeout", goIntegrationTestTimeout, "-tags", "integration", "./...")
}

func runIntegrationSuite(ctx context.Context, suite e2elane.GoSuite, env map[string]string) error {
	args := []string{
		"--format",
		"pkgname",
		"--",
		"-race",
		"-p",
		goIntegrationPackageLimit,
		"-parallel=4",
		"-timeout",
		goIntegrationTestTimeout,
		"-count=1",
		"-tags",
		"integration",
	}
	if strings.TrimSpace(suite.Run) != "" {
		args = append(args, "-run", suite.Run)
	}
	args = append(args, suite.Packages...)
	return runGotestsum(ctx, env, args...)
}

func runGotestsum(ctx context.Context, env map[string]string, args ...string) error {
	if hasPinnedTool("gotestsum", gotestsumVersion, "--version") {
		return runRaceEnabledCommand(ctx, env, "gotestsum", args...)
	}
	goArgs := append([]string{"run", "gotest.tools/gotestsum@" + gotestsumVersion}, args...)
	return runRaceEnabledGoCommand(ctx, env, goArgs...)
}
