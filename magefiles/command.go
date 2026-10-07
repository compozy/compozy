//go:build mage

package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"os/exec"
)

func runCommandInDir(ctx context.Context, dir string, name string, args ...string) error {
	return runCommandInDirWithEnv(ctx, dir, nil, name, args...)
}

func runCommandInDirWithEnv(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = mergeCommandEnv(env)
	return cmd.Run()
}

func mergeCommandEnv(overrides map[string]string) []string {
	return mergeEnvOverrides(os.Environ(), overrides)
}

func runRaceEnabledGoCommand(ctx context.Context, env map[string]string, args ...string) error {
	return runRaceEnabledCommand(ctx, env, "go", args...)
}

func runRaceEnabledCommand(
	ctx context.Context,
	env map[string]string,
	name string,
	args ...string,
) error {
	return runRaceEnabledCommandInDir(ctx, ".", env, name, args...)
}

func runRaceEnabledCommandInDir(
	ctx context.Context,
	dir string,
	env map[string]string,
	name string,
	args ...string,
) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = raceEnabledCommandEnv(env)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("race-enabled %s command %v: %w", name, args, err)
	}
	return nil
}

func raceEnabledCommandEnv(overrides map[string]string) []string {
	return hermeticGoTestEnv(withRaceEnabledEnv(overrides))
}

func withRaceEnabledEnv(overrides map[string]string) map[string]string {
	env := make(map[string]string, len(overrides)+1)
	maps.Copy(env, overrides)
	env["CGO_ENABLED"] = "1"
	return env
}
