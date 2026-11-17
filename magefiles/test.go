//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/e2elane"
)

// Test runs unit tests only (no integration tag).
func Test() error {
	ctx := context.Background()
	invocations, err := goUnitTestInvocations(ctx)
	if err != nil {
		return err
	}
	jsonFileDir, err := ensureGoTestJSONFileDir()
	if err != nil {
		return err
	}
	testArgs := goTestSafetyArgs(
		os.Getenv(goTestFullCheckptrEnvVar) == "1",
		os.Getenv(goTestUncachedEnvVar) == "1",
	)
	testArgs = append(testArgs,
		"-p", goUnitTestPackageLimit(),
		"-timeout", goUnitTestTimeout,
	)
	for index, invocation := range invocations {
		args := gotestsumInvocationArgs(jsonFileDir, index)
		args = append(args, testArgs...)
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
	return TestSDKGo()
}

// TestSDKGo runs the separate SDK module with the same bounded retry policy.
func TestSDKGo() error {
	jsonDir, err := ensureGoTestJSONFileDir()
	if err != nil {
		return err
	}
	jsonPath := ""
	if jsonDir != "" {
		jsonPath = filepath.Join(jsonDir, "gotest-sdk.json")
	}
	args := append(gotestsumJSONArgs(jsonPath), goSDKTestArgs(os.Getenv(goTestUncachedEnvVar) == "1")[1:]...)
	return runGotestsumInDir(context.Background(), "sdk/go", nil, args...)
}

func ensureGoTestJSONFileDir() (string, error) {
	dir := strings.TrimSpace(os.Getenv(goTestJSONFileDirEnvVar))
	if dir == "" {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create gotestsum JSON dir %q: %w", dir, err)
	}
	return dir, nil
}

func gotestsumRunnerArgs() []string {
	args := []string{"--format", "pkgname"}
	if os.Getenv("CI") != "" {
		args = append(args, "--rerun-fails=1", "--rerun-fails-max-failures=5")
	}
	return args
}

func gotestsumInvocationArgs(jsonFileDir string, invocationIndex int) []string {
	if jsonFileDir == "" {
		return gotestsumJSONArgs("")
	}
	stamp := strings.TrimSpace(os.Getenv(goTestShardIndexEnvVar))
	if stamp == "" {
		stamp = "all"
	}
	return gotestsumJSONArgs(filepath.Join(jsonFileDir, fmt.Sprintf("gotest-shard%s-%d.json", stamp, invocationIndex)))
}

func gotestsumJSONArgs(path string) []string {
	args := gotestsumRunnerArgs()
	if path != "" {
		args = append(args, "--jsonfile", path)
	}
	return append(args, "--")
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

// TestIntegration runs integration-tagged tests outside the dedicated E2E lanes.
func TestIntegration() error {
	preparedAt := time.Now()
	ctx := context.Background()
	invocations, err := goIntegrationTestInvocations(ctx)
	if err != nil {
		return err
	}
	jsonFileDir, err := ensureGoTestJSONFileDir()
	if err != nil {
		return err
	}
	fmt.Printf("Integration preparation (package and source discovery): %s\n", time.Since(preparedAt))
	packageLimit := goIntegrationPackageLimit
	if strings.TrimSpace(os.Getenv(goTestPackageLimitEnvVar)) != "" {
		packageLimit = goUnitTestPackageLimit()
	}
	for index, invocation := range invocations {
		args := gotestsumInvocationArgs(jsonFileDir, index)
		args = append(args, goIntegrationTestSafetyArgs()...)
		args = append(args, "-p", packageLimit,
			"-timeout", goIntegrationTestTimeout, "-tags", "integration")
		if len(invocation.tests) > 0 {
			args = append(args, "-run", exactGoTestRunPattern(invocation.tests))
		}
		args = append(args, invocation.packages...)
		startedAt := time.Now()
		err := runGotestsum(ctx, nil, args...)
		fmt.Printf(
			"Integration invocation %d packages=%v wall=%s (remaining compilation, setup and execution)\n",
			index,
			invocation.packages,
			time.Since(startedAt),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func runIntegrationSuite(ctx context.Context, suite e2elane.GoSuite, env map[string]string) error {
	artifactDir, err := os.MkdirTemp("", "compozy-e2e-artifacts-")
	if err != nil {
		return fmt.Errorf("create e2e artifact directory: %w", err)
	}
	// Keep evidence after the test process exits; CI uploads this existing E2E prefix.
	fmt.Printf("E2E artifacts: %s\n", artifactDir)
	jsonDir, err := ensureGoTestJSONFileDir()
	if err != nil {
		return err
	}
	// Each runtime suite has its own receipt; never overwrite an earlier suite.
	jsonPath := ""
	if jsonDir != "" {
		jsonPath = filepath.Join(jsonDir, "gotest-"+filepath.Base(artifactDir)+".json")
	}
	args := gotestsumJSONArgs(jsonPath)
	args = append(args,
		"-race",
		"-p",
		goIntegrationPackageLimit,
		"-parallel=4",
		"-timeout",
		goE2ETestTimeout,
		"-count=1",
		"-artifacts",
		"-outputdir",
		artifactDir,
		"-tags",
		"integration",
	)
	if strings.TrimSpace(suite.Run) != "" {
		args = append(args, "-run", suite.Run)
	}
	args = append(args, suite.Packages...)
	return runGotestsum(ctx, env, args...)
}

func runGotestsum(ctx context.Context, env map[string]string, args ...string) error {
	return runGotestsumInDir(ctx, ".", env, args...)
}

func runGotestsumInDir(ctx context.Context, dir string, env map[string]string, args ...string) error {
	if hasPinnedTool("gotestsum", gotestsumVersion, "--version") {
		if dir == "." {
			return runRaceEnabledCommand(ctx, env, "gotestsum", args...)
		}
		return runRaceEnabledCommandInDir(ctx, dir, env, "gotestsum", args...)
	}
	goArgs := append([]string{"run", "gotest.tools/gotestsum@" + gotestsumVersion}, args...)
	if dir == "." {
		return runRaceEnabledGoCommand(ctx, env, goArgs...)
	}
	return runRaceEnabledCommandInDir(ctx, dir, env, "go", goArgs...)
}
