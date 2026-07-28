//go:build mage

package main

import (
	"context"
	"fmt"
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
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = "."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(env))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("race-enabled %s command %v: %w", name, args, err)
	}
	return nil
}

func withRaceEnabledEnv(overrides map[string]string) map[string]string {
	env := make(map[string]string, len(overrides)+1)
	for key, value := range overrides {
		env[key] = value
	}
	env["CGO_ENABLED"] = "1"
	return env
}
