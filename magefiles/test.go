//go:build mage

package main

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/e2elane"
)

// Test runs unit tests only (no integration tag).
func Test() error {
	ctx := context.Background()
	invocations, err := goUnitTestInvocations(ctx)
	if err != nil {
		return err
	}
	baseArgs := []string{
		"--format", "pkgname", "--", "-race", "-p", goUnitTestPackageLimit(),
		"-parallel=" + strconv.Itoa(goUnitTestParallelism),
		"-timeout", goUnitTestTimeout,
	}
	for _, invocation := range invocations {
		args := append([]string(nil), baseArgs...)
		if len(invocation.tests) > 0 {
			args = append(args, "-run", exactGoTestRunPattern(invocation.tests))
		}
		args = append(args, invocation.packages...)
		if err := runGotestsum(ctx, nil, args...); err != nil {
			return err
		}
	}
	return runRaceEnabledCommandInDir(ctx, "sdk/go", nil, "go", "test", "-race", "-parallel=4", "./...")
}

func exactGoTestRunPattern(tests []string) string {
	quoted := make([]string, len(tests))
	for index, testName := range tests {
		quoted[index] = regexp.QuoteMeta(testName)
	}
	return "^(?:" + strings.Join(quoted, "|") + ")$"
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
