//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Census weights drive the duration-aware (LPT) shard partition. The committed
// JSON file is the only partition input besides the package list, so every
// shard job of a commit derives the identical partition. Refresh it from
// gotestsum JSON files with `mage testCensusUpdate <dir>`.

const (
	goTestCensusPath          = "magefiles/gotest_census.json"
	goTestCensusMinimumWeight = 0.1
)

type goTestCensus struct {
	DefaultPackageSeconds   float64            `json:"defaultPackageSeconds"`
	DefaultSplitTestSeconds float64            `json:"defaultSplitTestSeconds"`
	Packages                map[string]float64 `json:"packages"`
	SplitTests              map[string]float64 `json:"splitTests"`
}

func loadGoTestCensus(path string) (*goTestCensus, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read go test census %q: %w", path, err)
	}
	census := &goTestCensus{}
	if err := json.Unmarshal(data, census); err != nil {
		return nil, fmt.Errorf("decode go test census %q: %w", path, err)
	}
	if census.DefaultPackageSeconds <= 0 || census.DefaultSplitTestSeconds <= 0 {
		return nil, fmt.Errorf(
			"go test census %q: defaultPackageSeconds and defaultSplitTestSeconds must be positive",
			path,
		)
	}
	return census, nil
}

// A nil census keeps sharding functional with uniform weights, which balances
// shards by item count only.
func (c *goTestCensus) packageWeight(packagePath string) float64 {
	if c == nil {
		return 1
	}
	if seconds, ok := c.Packages[packagePath]; ok {
		return max(seconds, goTestCensusMinimumWeight)
	}
	return max(c.DefaultPackageSeconds, goTestCensusMinimumWeight)
}

func (c *goTestCensus) splitTestWeight(testName string) float64 {
	if c == nil {
		return 1
	}
	if seconds, ok := c.SplitTests[testName]; ok {
		return max(seconds, goTestCensusMinimumWeight)
	}
	return max(c.DefaultSplitTestSeconds, goTestCensusMinimumWeight)
}

func (c *goTestCensus) missingPackageCount(packagePaths []string) int {
	if c == nil {
		return 0
	}
	missing := 0
	for _, packagePath := range packagePaths {
		if packagePath == goSplitTestPackage {
			continue
		}
		if _, ok := c.Packages[packagePath]; !ok {
			missing++
		}
	}
	return missing
}

type goShardItem struct {
	name      string
	weight    float64
	splitTest bool
}

// partitionGoShardItems packs items into total bins with the LPT greedy rule:
// heaviest first, always into the lightest bin. Ordering ties break by name and
// load ties by bin index, so the partition is a pure function of its inputs.
func partitionGoShardItems(items []goShardItem, total int) [][]goShardItem {
	ordered := slices.Clone(items)
	slices.SortStableFunc(ordered, func(a, b goShardItem) int {
		if a.weight != b.weight {
			if a.weight > b.weight {
				return -1
			}
			return 1
		}
		return strings.Compare(a.name, b.name)
	})
	bins := make([][]goShardItem, total)
	loads := make([]float64, total)
	for _, item := range ordered {
		lightest := 0
		for index := 1; index < total; index++ {
			if loads[index] < loads[lightest] {
				lightest = index
			}
		}
		bins[lightest] = append(bins[lightest], item)
		loads[lightest] += max(item.weight, goTestCensusMinimumWeight)
	}
	return bins
}
