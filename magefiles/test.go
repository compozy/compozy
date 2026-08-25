//go:build mage

package main

import (
	"context"
	"os"
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
	baseArgs := []string{"--format", "pkgname", "--"}
	baseArgs = append(baseArgs, goUnitTestSafetyArgs(
		os.Getenv(goTestFullCheckptrEnvVar) == "1",
		os.Getenv(goTestUncachedEnvVar) == "1",
	)...)
	baseArgs = append(baseArgs,
		"-p", goUnitTestPackageLimit(),
		"-parallel="+strconv.Itoa(goUnitTestParallelism),
		"-timeout", goUnitTestTimeout,
	)
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
	runSDK, err := shouldRunSDKGoTests(
		os.Getenv(goTestShardIndexEnvVar),
		os.Getenv(goTestShardTotalEnvVar),
	)
	if err != nil {
		return err
	}
	if !runSDK {
		return nil
	}
	return runRaceEnabledCommandInDir(
		ctx,
		"sdk/go",
		nil,
		"go",
		goSDKTestArgs(os.Getenv(goTestUncachedEnvVar) == "1")...,
	)
}

func shouldRunSDKGoTests(shardIndex, shardTotal string) (bool, error) {
	_, sharded, err := parseGoTestShard(shardIndex, shardTotal)
	if err != nil {
		return false, err
	}
	return !sharded, nil
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
