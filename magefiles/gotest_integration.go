//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
)

// Integration retains race and full pointer checks. Split the oversized SQLite
// and daemon suites so all tagged tests run within each package's timeout.
func goIntegrationTestInvocations(ctx context.Context) ([]goUnitTestInvocation, error) {
	shard, enabled, err := parseGoTestShard(os.Getenv(goTestShardIndexEnvVar), os.Getenv(goTestShardTotalEnvVar))
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []goUnitTestInvocation{{packages: []string{goAllPackagesPattern}}}, nil
	}
	cmd := exec.CommandContext(ctx, "go", "list", "-tags=integration", goAllPackagesPattern)
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(nil))
	packages, err := goListPackagePaths(cmd)
	if err != nil {
		return nil, err
	}
	census, err := loadGoTestCensus(goTestCensusPath)
	if err != nil {
		return nil, err
	}
	isolated := []string{
		"github.com/compozy/compozy/internal/daemon",
		"github.com/compozy/compozy/internal/store",
	}
	regular := slices.DeleteFunc(
		slices.Clone(packages),
		func(path string) bool { return slices.Contains(isolated, path) },
	)
	splitTests, err := listGoTopLevelTests(ctx, goSplitTestPackage, "-tags=integration")
	if err != nil {
		return nil, err
	}
	invocations, err := shardGoUnitTestInvocations(regular, splitTests, census, shard)
	if err != nil {
		return nil, err
	}
	for _, packagePath := range isolated {
		if !slices.Contains(packages, packagePath) {
			return nil, fmt.Errorf("integration split package %q was not listed", packagePath)
		}
		tests, err := listGoTopLevelTests(ctx, packagePath, "-tags=integration")
		if err != nil {
			return nil, err
		}
		selected, err := shardGoPackageTestInvocations([]string{packagePath}, packagePath, tests, census, shard)
		if err != nil {
			return nil, err
		}
		invocations = append(invocations, selected...)
	}
	fmt.Printf("Go integration shard %d/%d selected %d invocations\n", shard.index+1, shard.total, len(invocations))
	return invocations, nil
}
