//go:build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/compozy/agh/internal/e2elane"
)

// TestE2ERuntime runs the PR-required daemon/runtime E2E lane without sweeping every integration package.
func TestE2ERuntime() error {
	return runE2ELane(e2elane.LaneRuntime)
}

// TestE2EWeb runs the daemon-served Playwright E2E lane for shipped browser workflows.
func TestE2EWeb() error {
	return runE2ELane(e2elane.LaneWeb)
}

// TestE2E runs the default PR-required runtime and browser E2E lanes.
func TestE2E() error {
	return runE2ELane(e2elane.LaneCombined)
}

// TestE2ENightly runs the combined E2E lane plus credentialed nightly coverage.
func TestE2ENightly() error {
	return runE2ELane(e2elane.LaneNightly)
}

func ensureWebAssets() error {
	return ensureWebAssetsWith(CodegenCheck, WebBuild)
}

func ensureWebAssetsWith(codegenCheck, webBuild func() error) error {
	if err := codegenCheck(); err != nil {
		return fmt.Errorf("check generated web contracts: %w", err)
	}
	if err := webBuild(); err != nil {
		return fmt.Errorf("build current web assets: %w", err)
	}
	return nil
}

func runE2ELane(lane e2elane.Lane) (runErr error) {
	releaseLock := acquireVerifyLock()
	defer releaseLock()
	ctx := context.Background()

	plan, err := e2elane.PlanForLane(lane)
	if err != nil {
		return err
	}

	if shouldEnsureWebBundle(plan) {
		if err := ensureWebAssets(); err != nil {
			return err
		}
	}

	laneEnv, err := prepareE2ELaneEnv()
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := laneEnv.Cleanup(); cleanupErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("cleanup e2e lane environment: %w", cleanupErr))
		}
	}()

	for _, suite := range plan.GoSuites {
		if err := runIntegrationSuite(ctx, suite, laneEnv.Values); err != nil {
			return err
		}
	}

	for _, suite := range plan.ScriptSuites {
		if err := runCommandInDirWithEnv(ctx, suite.Dir, laneEnv.Values, "bun", "run", suite.Script); err != nil {
			return err
		}
	}

	return nil
}

func shouldEnsureWebBundle(plan e2elane.Plan) bool {
	return len(plan.GoSuites) > 0 || plan.RequiresDaemonServedBrowser
}

type e2eLaneEnv struct {
	Values  map[string]string
	cleanup func() error
}

func (env e2eLaneEnv) Cleanup() error {
	if env.cleanup == nil {
		return nil
	}
	return env.cleanup()
}

func prepareE2ELaneEnv() (e2eLaneEnv, error) {
	var cleanups []func() error
	daemonPath, cleanup, err := resolveOrBuildLaneBinary(daemonBinaryEnvVar, func(outputPath string) error {
		return runCommandInDir(
			context.Background(),
			".",
			"go",
			"build",
			"-ldflags",
			buildLDFlags(),
			"-o",
			outputPath,
			"./cmd/agh",
		)
	}, cliBinary)
	if err != nil {
		return e2eLaneEnv{}, err
	}
	cleanups = append(cleanups, cleanup)

	driverPath, cleanup, err := resolveOrBuildLaneBinary(driverBinaryEnvVar, func(outputPath string) error {
		return runCommandInDir(
			context.Background(),
			".",
			"go",
			"build",
			"-o",
			outputPath,
			"./internal/testutil/acpmock/cmd/acpmock-driver",
		)
	}, "acpmock-driver")
	if err != nil {
		return e2eLaneEnv{}, errors.Join(err, runCleanups(cleanups))
	}
	cleanups = append(cleanups, cleanup)

	values := map[string]string{
		daemonBinaryEnvVar: daemonPath,
		driverBinaryEnvVar: driverPath,
	}
	if _, err := os.Stat(webDistIndex); err == nil {
		absWebDistDir, absErr := filepath.Abs(webDistDir)
		if absErr != nil {
			return e2eLaneEnv{}, errors.Join(
				fmt.Errorf("resolve %s for e2e lane: %w", webDistDir, absErr),
				runCleanups(cleanups),
			)
		}
		values[webDistDirEnvVar] = absWebDistDir
	} else if !errors.Is(err, os.ErrNotExist) {
		return e2eLaneEnv{}, errors.Join(err, runCleanups(cleanups))
	}

	return e2eLaneEnv{
		Values: values,
		cleanup: func() error {
			return runCleanups(cleanups)
		},
	}, nil
}

func resolveOrBuildLaneBinary(
	envVar string,
	build func(string) error,
	name string,
) (string, func() error, error) {
	if override := strings.TrimSpace(os.Getenv(envVar)); override != "" {
		overridePath, err := resolveLaneBinaryOverride(envVar, override)
		if err != nil {
			return "", nil, err
		}
		return overridePath, noopCleanup, nil
	}

	buildDir, err := os.MkdirTemp("", "agh-e2e-lane-")
	if err != nil {
		return "", nil, err
	}
	outputPath := filepath.Join(buildDir, laneBinaryName(name))
	if err := build(outputPath); err != nil {
		return "", nil, errors.Join(
			fmt.Errorf("build %s e2e lane binary: %w", name, err),
			cleanupLaneBuildDir(buildDir),
		)
	}
	return outputPath, func() error {
		return cleanupLaneBuildDir(buildDir)
	}, nil
}

func resolveLaneBinaryOverride(envVar, override string) (string, error) {
	overridePath := override
	if !filepath.IsAbs(overridePath) {
		absPath, err := filepath.Abs(overridePath)
		if err != nil {
			return "", fmt.Errorf("resolve %s override %q: %w", envVar, override, err)
		}
		overridePath = absPath
	}

	info, err := os.Stat(overridePath)
	if err != nil {
		return "", fmt.Errorf("%s points to %q: %w", envVar, overridePath, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s points to directory %q: %w", envVar, overridePath, errLaneBinaryOverrideDirectory)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf(
			"%s points to non-executable file %q: %w",
			envVar,
			overridePath,
			errLaneBinaryOverrideNotExecutable,
		)
	}
	return overridePath, nil
}

func noopCleanup() error {
	return nil
}

func runCleanups(cleanups []func() error) error {
	var joined error
	for idx := len(cleanups) - 1; idx >= 0; idx-- {
		if err := cleanups[idx](); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func cleanupLaneBuildDir(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove e2e lane build dir %q: %w", path, err)
	}
	return nil
}

func laneBinaryName(name string) string {
	if strings.EqualFold(filepath.Ext(name), ".exe") {
		return name
	}
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
