//go:build mage

package main

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var goLintModules = []string{
	"sdk/go",
	"sdk/examples/notes-commands",
	"sdk/examples/clarify-tool",
	"internal/extension/testdata/command-fixture-go",
	"internal/extension/testdata/palette-fixture-go",
}

type goLintTarget struct {
	dir    string
	scopes []string
	tags   string
}

// Repository-relative scopes select nested modules explicitly; ./... selects every target.
func golangciLintTargets(raw string) []goLintTarget {
	scopes := golangciLintScopes(raw)
	modules := append(slices.Clone(goLintModules), "magefiles")
	root := goLintTarget{dir: "."}
	var targets []goLintTarget
	for _, scope := range scopes {
		path := strings.TrimPrefix(scope, "./")
		nested := path == "sdk/..." || path == "sdk/examples/..." || path == "internal/extension/testdata/..."
		for _, module := range modules {
			if pathWithin(strings.TrimPrefix(scope, "./"), module) {
				nested = true
				break
			}
		}
		if !nested {
			root.scopes = append(root.scopes, scope)
		}
	}
	if len(root.scopes) > 0 {
		targets = append(targets, root)
	}
	for _, module := range modules {
		target := goLintTarget{dir: module}
		for _, scope := range scopes {
			path := strings.TrimPrefix(scope, "./")
			prefix := strings.TrimSuffix(path, "/...")
			switch {
			case path == "..." || (strings.HasSuffix(path, "/...") && pathWithin(module, prefix)):
				target.scopes = []string{"./..."}
			case path == module:
				target.scopes = append(target.scopes, ".")
			case strings.HasPrefix(path, module+"/"):
				target.scopes = append(target.scopes, "./"+strings.TrimPrefix(path, module+"/"))
			}
		}
		if slices.Contains(target.scopes, "./...") {
			target.scopes = []string{"./..."}
		}
		if len(target.scopes) == 0 {
			continue
		}
		if module == "magefiles" {
			target.tags = "mage"
		}
		targets = append(targets, target)
	}
	return targets
}

func runGolangciTarget(root string, env map[string]string, target goLintTarget, formatters bool) error {
	args := []string{"run", "--config", filepath.Join(root, golangciConfigPath),
		"--allow-parallel-runners", "--timeout", golangciLintTimeout,
		"--concurrency", golangciLintConcurrency()}
	if target.tags != "" {
		args = append(args, "--build-tags", target.tags)
	}
	if !formatters {
		linters, err := golangciEnabledLinters(filepath.Join(root, golangciConfigPath))
		if err != nil {
			return err
		}
		args = append(args, "--enable-only", strings.Join(linters, ","))
	}
	args = append(args, target.scopes...)
	targetEnv, cleanup, err := goLintModuleEnv(root, target.dir, env)
	if err != nil {
		return err
	}
	defer cleanup()
	fmt.Printf("go-lint module: %s (%s)\n", target.dir, strings.Join(target.scopes, " "))
	if err := runGolangciCommandInDir(filepath.Join(root, target.dir), targetEnv, args...); err != nil {
		return fmt.Errorf("lint %s: %w", target.dir, err)
	}
	return nil
}

func runGolangciCommandInDir(dir string, env map[string]string, args ...string) error {
	if hasPinnedTool("golangci-lint", golangciLintVersion, "version") {
		return runCommandInDirWithEnv(context.Background(), dir, env, "golangci-lint", args...)
	}
	goArgs := append(
		[]string{"run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + golangciLintVersion},
		args...)
	return runCommandInDirWithEnv(context.Background(), dir, env, "go", goArgs...)
}

// A disposable workspace resolves fixtures against the same local SDK used by authoring tests.
func goLintModuleEnv(root, dir string, env map[string]string) (map[string]string, func(), error) {
	noop := func() {}
	if !strings.HasPrefix(dir, "internal/extension/testdata/") {
		return env, noop, nil
	}
	temp, err := os.MkdirTemp("", "compozy-lint-module-")
	if err != nil {
		return nil, noop, err
	}
	cleanup := func() {
		if err := os.RemoveAll(temp); err != nil {
			fmt.Fprintf(os.Stderr, "remove lint workspace: %v\n", err)
		}
	}
	workspace := filepath.Join(temp, "go.work")
	data := fmt.Sprintf("go 1.26.4\n\nuse (\n%q\n%q\n)\n", filepath.Join(root, dir), filepath.Join(root, "sdk", "go"))
	if err := os.WriteFile(workspace, []byte(data), 0o600); err != nil {
		cleanup()
		return nil, noop, err
	}
	targetEnv := make(map[string]string, len(env)+1)
	maps.Copy(targetEnv, env)
	targetEnv["GOWORK"] = workspace
	return targetEnv, cleanup, nil
}
