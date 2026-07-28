//go:build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// BunLint runs the monorepo-wide lint script (oxfmt + oxlint over every workspace).
func BunLint() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", "lint")
}

// BunTypecheck runs the monorepo-wide typecheck pipeline (turbo run typecheck across every workspace).
func BunTypecheck() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", "typecheck")
}

// BunTest runs the monorepo-wide vitest projects suite from the repo root.
func BunTest() error {
	return runCommandInDir(context.Background(), ".", "bun", "run", "test")
}

func WebTypecheck() error {
	return runCommandInDir(context.Background(), "web", "bun", "run", "typecheck:raw")
}

func WebTest() error {
	return runCommandInDir(context.Background(), "web", "bun", "run", "test:raw")
}

func WebBuild() error {
	if err := runCommandInDir(
		context.Background(),
		".",
		"bunx",
		"turbo",
		"run",
		"build",
		"--filter=./web",
	); err != nil {
		return err
	}
	return ensureWebDist()
}

func ensureWebDist() error {
	if _, err := os.Stat(webDistIndex); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("web build output missing %s", webDistIndex)
		}
		return fmt.Errorf("stat web build output %s: %w", webDistIndex, err)
	}
	return nil
}
