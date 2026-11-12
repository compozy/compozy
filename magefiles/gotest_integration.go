//go:build mage

package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

const goIntegrationGroupEnvVar = "COMPOZY_GO_INTEGRATION_GROUP"

// Integration selects only tests introduced by the integration build tag. Unit
// tests retain their race lane; the E2E plan owns runtime and credentialed tests.
func goIntegrationTestInvocations(ctx context.Context) ([]goUnitTestInvocation, error) {
	shard, enabled, err := parseGoTestShard(os.Getenv(goTestShardIndexEnvVar), os.Getenv(goTestShardTotalEnvVar))
	if err != nil {
		return nil, err
	}
	if !enabled {
		shard = goTestShard{total: 1}
	}
	group := os.Getenv(goIntegrationGroupEnvVar)
	if group != "" && group != "daemon" && group != "globaldb" && group != "other" {
		return nil, fmt.Errorf("unknown integration shard group %q", group)
	}
	tests, err := listIntegrationTests(ctx, group)
	if err != nil {
		return nil, err
	}
	census, err := loadGoTestCensus(goTestCensusPath)
	if err != nil {
		return nil, err
	}
	if census != nil {
		census = census.Integration
	}
	var packages []string
	for pkg := range tests {
		packages = append(packages, pkg)
	}
	invocations := partitionIntegrationInvocations(packages, tests, census, shard)
	fmt.Printf(
		"Go integration group %q shard %d/%d selected %d invocations\n",
		group, shard.index+1, shard.total, len(invocations),
	)
	return invocations, nil
}

func integrationPackageGroup(packagePath string) string {
	switch packagePath {
	case compozyModulePath + "internal/daemon":
		return "daemon"
	case goSplitTestPackage:
		return "globaldb"
	default:
		return "other"
	}
}

func goIntegrationTestSafetyArgs() []string {
	// The weekly audit retains full checkptr. Match the unit lane's modernc-only
	// exception here; race detection and checks in our own code stay enabled.
	return goTestSafetyArgs(false, false)
}

// Each group has its own LPT partition. Every invocation has an exact run list,
// including ordinary packages, so untagged tests cannot leak into this lane.
func partitionIntegrationInvocations(
	packages []string,
	tests map[string][]string,
	census *goTestCensus,
	shard goTestShard,
) []goUnitTestInvocation {
	var items []goShardItem
	for _, packagePath := range packages {
		for _, test := range tests[packagePath] {
			key := packagePath + "#" + test
			weight := census.splitTestWeight(key)
			if census != nil {
				if _, measured := census.SplitTests[key]; !measured {
					if seconds, measured := census.Packages[packagePath]; measured {
						weight = max(seconds/float64(len(tests[packagePath])), goTestCensusMinimumWeight)
					}
				}
			}
			items = append(items, goShardItem{name: key, weight: weight})
		}
	}
	grouped := map[string][]string{}
	for _, item := range partitionGoShardItems(items, shard.total)[shard.index] {
		pkg, test, _ := strings.Cut(item.name, "#")
		grouped[pkg] = append(grouped[pkg], test)
	}
	var names []string
	for name := range grouped {
		names = append(names, name)
	}
	slices.Sort(names)
	var result []goUnitTestInvocation
	for _, name := range names {
		slices.Sort(grouped[name])
		result = append(result, goUnitTestInvocation{packages: []string{name}, tests: grouped[name]})
	}
	return result
}
