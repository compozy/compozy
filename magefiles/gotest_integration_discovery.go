//go:build mage

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/e2elane"
)

func listIntegrationTests(ctx context.Context, group string) (map[string][]string, error) {
	unit, err := listGoTestPackageFiles(ctx, false)
	if err != nil {
		return nil, err
	}
	tagged, err := listGoTestPackageFiles(ctx, true)
	if err != nil {
		return nil, err
	}
	plan, err := e2elane.PlanForLane(e2elane.LaneNightly)
	if err != nil {
		return nil, err
	}
	result := make(map[string][]string)
	for pkg, files := range tagged {
		if group != "" && integrationPackageGroup(pkg) != group {
			continue
		}
		tests, err := integrationTestsInFiles(files, unit[pkg], plan.GoSuites)
		if err != nil {
			return nil, err
		}
		if len(tests) > 0 {
			result[pkg] = tests
		}
	}
	return result, nil
}

func listGoTestPackageFiles(ctx context.Context, integration bool) (map[string]goTestPackageFiles, error) {
	args := []string{"list", "-race", "-json"}
	if integration {
		args = append(args, "-tags=integration")
	}
	args = append(args, goAllPackagesPattern)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(nil))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list Go test sources (integration=%t): %w: %s", integration, err, goCommandStderr(err))
	}
	decoder := json.NewDecoder(bytes.NewReader(output))
	result := make(map[string]goTestPackageFiles)
	for {
		var files goTestPackageFiles
		if err := decoder.Decode(&files); err != nil {
			if errors.Is(err, io.EOF) {
				return result, nil
			}
			return nil, fmt.Errorf("decode Go test sources: %w", err)
		}
		result[files.ImportPath] = files
	}
}

func goCommandStderr(err error) string {
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return strings.TrimSpace(string(exitErr.Stderr))
	}
	return ""
}

func integrationTestsInFiles(tagged, unit goTestPackageFiles, e2e []e2elane.GoSuite) ([]string, error) {
	unitFiles := append(slices.Clone(unit.TestGoFiles), unit.XTestGoFiles...)
	taggedFiles := append(slices.Clone(tagged.TestGoFiles), tagged.XTestGoFiles...)
	var tests []string
	for _, name := range taggedFiles {
		if slices.Contains(unitFiles, name) {
			continue
		}
		names, err := topLevelTestsInFile(filepath.Join(tagged.Dir, name))
		if err != nil {
			return nil, err
		}
		for _, test := range names {
			owned, err := e2eOwnsGoTest(e2e, tagged.ImportPath, test)
			if err != nil {
				return nil, err
			}
			if !owned {
				tests = append(tests, test)
			}
		}
	}
	slices.Sort(tests)
	return slices.Compact(tests), nil
}

func e2eOwnsGoTest(suites []e2elane.GoSuite, pkg, test string) (bool, error) {
	for _, suite := range suites {
		for _, path := range suite.Packages {
			if pkg != compozyModulePath+strings.TrimPrefix(path, "./") {
				continue
			}
			matches, err := regexp.MatchString(suite.Run, test)
			if err != nil {
				return false, fmt.Errorf("parse E2E run pattern %q: %w", suite.Run, err)
			}
			if matches {
				return true, nil
			}
		}
	}
	return false, nil
}
