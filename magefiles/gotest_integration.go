//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// Integration retains race and full pointer checks. Split the oversized SQLite
// and daemon suites so all tagged tests run within each package's timeout.
func goIntegrationTestInvocations(ctx context.Context) ([]goUnitTestInvocation, error) {
	shard, enabled, err := parseGoTestShard(os.Getenv(goTestShardIndexEnvVar), os.Getenv(goTestShardTotalEnvVar))
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-tags=integration", goAllPackagesPattern)
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(nil))
	packages, err := goListPackagePaths(cmd)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return isolateGlobalDBIntegrationTests(packages), nil
	}
	census, err := loadGoTestCensus(goTestCensusPath)
	if err != nil {
		return nil, err
	}
	if census != nil {
		census = census.Integration
	}
	split := map[string][]string{}
	for _, packagePath := range []string{
		goSplitTestPackage,
		compozyModulePath + "internal/daemon",
		compozyModulePath + "internal/store",
	} {
		if !slices.Contains(packages, packagePath) {
			return nil, fmt.Errorf("integration split package %q was not listed", packagePath)
		}
		tests, err := listGoTopLevelTests(ctx, packagePath, "-tags=integration")
		if err != nil {
			return nil, err
		}
		if len(tests) == 0 {
			return nil, fmt.Errorf("integration package %q listed no tests", packagePath)
		}
		split[packagePath] = tests
	}
	invocations := partitionIntegrationInvocations(packages, split, census, shard)
	fmt.Printf("Go integration shard %d/%d selected %d invocations\n", shard.index+1, shard.total, len(invocations))
	return invocations, nil
}

func isolateGlobalDBIntegrationTests(packages []string) []goUnitTestInvocation {
	regular := slices.DeleteFunc(slices.Clone(packages), func(path string) bool { return path == goSplitTestPackage })
	var invocations []goUnitTestInvocation
	if len(regular) > 0 {
		invocations = append(invocations, goUnitTestInvocation{packages: regular})
	}
	if slices.Contains(packages, goSplitTestPackage) {
		invocations = append(invocations, goUnitTestInvocation{packages: []string{goSplitTestPackage}})
	}
	return invocations
}

func goIntegrationTestSafetyArgs(packages []string) []string {
	if slices.Contains(packages, goSplitTestPackage) {
		return goTestSafetyArgs(true, false)
	}
	return []string{"-race", "-parallel=4"}
}

// All integration work enters one LPT partition; later packages cannot restart
// the shard load accounting. Qualified test keys also disambiguate equal names.
func partitionIntegrationInvocations(
	packages []string,
	split map[string][]string,
	census *goTestCensus,
	shard goTestShard,
) []goUnitTestInvocation {
	var items []goShardItem
	for _, packagePath := range packages {
		if tests, ok := split[packagePath]; ok {
			for _, test := range tests {
				key := packagePath + "#" + test
				items = append(items, goShardItem{name: key, weight: census.splitTestWeight(key), splitTest: true})
			}
		} else {
			items = append(items, goShardItem{name: packagePath, weight: census.packageWeight(packagePath)})
		}
	}
	grouped := map[string][]string{}
	var regular []string
	for _, item := range partitionGoShardItems(items, shard.total)[shard.index] {
		if item.splitTest {
			pkg, test, _ := strings.Cut(item.name, "#")
			grouped[pkg] = append(grouped[pkg], test)
		} else {
			regular = append(regular, item.name)
		}
	}
	var result []goUnitTestInvocation
	slices.Sort(regular)
	if len(regular) > 0 {
		result = append(result, goUnitTestInvocation{packages: regular})
	}
	var names []string
	for name := range grouped {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		slices.Sort(grouped[name])
		result = append(result, goUnitTestInvocation{packages: []string{name}, tests: grouped[name]})
	}
	return result
}
