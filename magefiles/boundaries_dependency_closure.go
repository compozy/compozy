//go:build mage

package main

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
)

type dependencyClosureRule struct {
	root              string
	allowedPrefixes   []string
	forbiddenExact    []string
	forbiddenPrefixes []string
}

type dependencyClosureViolation struct {
	root       string
	dependency string
}

var subprocessDependencyClosureRules = []dependencyClosureRule{
	{
		root: "./internal/subprocess",
		forbiddenPrefixes: []string{
			compozyModulePath + "internal/store",
			compozyModulePath + "internal/resources",
			compozyModulePath + "internal/task",
			compozyModulePath + "internal/session",
			compozyModulePath + "internal/automation",
			compozyModulePath + "internal/observe",
			compozyModulePath + "internal/extension",
			compozyModulePath + "internal/daemon",
		},
	},
}

func listPackageDependencies(ctx context.Context, root string) ([]string, error) {
	command := exec.CommandContext(ctx, "go", "list", "-deps", root)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go list -deps %s: %w: %s", root, err, strings.TrimSpace(string(output)))
	}
	dependencies := strings.Fields(string(output))
	slices.Sort(dependencies)
	return dependencies, nil
}
