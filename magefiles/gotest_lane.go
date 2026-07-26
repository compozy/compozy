//go:build mage

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// Go test lane execution policy: package-parallelism cap plus ambient
// runtime-state scrubbing, so test lanes stay safe next to other worktrees and
// stale QA-lab shell exports cannot couple otherwise-isolated runs.

const (
	goTestPackageLimitEnvVar = "AGH_GO_TEST_P"
	goTestShardIndexEnvVar   = "AGH_GO_TEST_SHARD_INDEX"
	goTestShardTotalEnvVar   = "AGH_GO_TEST_SHARD_TOTAL"
	goUnitTestParallelism    = 4
)

type goTestShard struct {
	index int
	total int
}

// ambientRuntimeStateEnvVars are the runtime identity vars a QA lab or dev
// shell may export (bootstrap env block / worktree isolation envelope). Tests
// must never inherit them ambiently; lanes that need one pass it as an
// explicit override, which is applied after the scrub and therefore wins.
var ambientRuntimeStateEnvVars = []string{
	"AGH_HOME",
	"AGH_CONFIG_HOME",
	"AGH_HTTP_PORT",
	"AGH_UDS_PATH",
	"AGH_WEB_API_PROXY_TARGET",
	"AGH_WEB_DIST_DIR",
	"TMUX_BRIDGE_SOCKET",
	"PROVIDER_HOME",
	"PROVIDER_CODEX_HOME",
}

// goUnitTestPackageLimit resolves the unit lane's `go test -p` cap. An
// explicit AGH_GO_TEST_P wins; otherwise the package fan-out accounts for the
// per-package -parallel budget instead of multiplying it by the CPU count.
func goUnitTestPackageLimit() string {
	if raw := strings.TrimSpace(os.Getenv(goTestPackageLimitEnvVar)); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			return strconv.Itoa(value)
		}
		fmt.Printf("Warning: ignoring invalid %s=%q; using default cap\n", goTestPackageLimitEnvVar, raw)
	}
	return strconv.Itoa(goUnitTestPackageLimitFor(runtime.GOMAXPROCS(0), goUnitTestParallelism))
}

func goUnitTestPackageLimitFor(effectiveCPU, parallelism int) int {
	if effectiveCPU < 1 {
		effectiveCPU = 1
	}
	if parallelism < 1 {
		parallelism = 1
	}

	totalBudget := effectiveCPU / 2
	if totalBudget < parallelism {
		totalBudget = parallelism
	}
	limit := (totalBudget + parallelism - 1) / parallelism
	if limit < 1 {
		return 1
	}
	return limit
}

func goUnitTestPackages(ctx context.Context) ([]string, error) {
	shard, enabled, err := parseGoTestShard(
		os.Getenv(goTestShardIndexEnvVar),
		os.Getenv(goTestShardTotalEnvVar),
	)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []string{"./..."}, nil
	}

	cmd := exec.CommandContext(ctx, "go", "list", "./...")
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(nil))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list Go unit-test packages: %w: %s", err, strings.TrimSpace(string(output)))
	}
	packages := strings.Fields(string(output))
	if len(packages) == 0 {
		return nil, fmt.Errorf("list Go unit-test packages: go list returned no packages")
	}

	selected := shardGoTestPackages(packages, shard)
	if len(selected) == 0 {
		return nil, fmt.Errorf(
			"select Go unit-test shard %d/%d: no packages selected from %d packages",
			shard.index+1,
			shard.total,
			len(packages),
		)
	}
	fmt.Printf(
		"Go unit-test shard %d/%d selected %d of %d packages\n",
		shard.index+1,
		shard.total,
		len(selected),
		len(packages),
	)
	return selected, nil
}

func parseGoTestShard(indexRaw, totalRaw string) (goTestShard, bool, error) {
	indexRaw = strings.TrimSpace(indexRaw)
	totalRaw = strings.TrimSpace(totalRaw)
	if indexRaw == "" && totalRaw == "" {
		return goTestShard{}, false, nil
	}
	if indexRaw == "" || totalRaw == "" {
		return goTestShard{}, false, fmt.Errorf(
			"%s and %s must be set together",
			goTestShardIndexEnvVar,
			goTestShardTotalEnvVar,
		)
	}

	index, err := strconv.Atoi(indexRaw)
	if err != nil || index < 0 {
		return goTestShard{}, false, fmt.Errorf("%s must be a non-negative integer", goTestShardIndexEnvVar)
	}
	total, err := strconv.Atoi(totalRaw)
	if err != nil || total < 1 {
		return goTestShard{}, false, fmt.Errorf("%s must be a positive integer", goTestShardTotalEnvVar)
	}
	if index >= total {
		return goTestShard{}, false, fmt.Errorf(
			"%s=%d must be less than %s=%d",
			goTestShardIndexEnvVar,
			index,
			goTestShardTotalEnvVar,
			total,
		)
	}
	return goTestShard{index: index, total: total}, true, nil
}

func shardGoTestPackages(packages []string, shard goTestShard) []string {
	sorted := append([]string(nil), packages...)
	slices.Sort(sorted)
	selected := make([]string, 0, (len(sorted)+shard.total-1)/shard.total)
	for index, packagePath := range sorted {
		if index%shard.total == shard.index {
			selected = append(selected, packagePath)
		}
	}
	return selected
}

func hermeticGoTestEnv(overrides map[string]string) []string {
	return hermeticGoTestEnvFromBase(os.Environ(), overrides, os.Stdout)
}

func hermeticGoTestEnvFromBase(base []string, overrides map[string]string, log io.Writer) []string {
	scrubbed := make([]string, 0, len(base))
	dropped := make([]string, 0, 4)
	for _, entry := range base {
		name, _, ok := strings.Cut(entry, "=")
		if ok && slices.Contains(ambientRuntimeStateEnvVars, name) {
			dropped = append(dropped, name)
			continue
		}
		scrubbed = append(scrubbed, entry)
	}
	if len(dropped) > 0 && log != nil {
		fmt.Fprintf(
			log,
			"Note: scrubbed ambient runtime-state env from go test lane: %s\n",
			strings.Join(dropped, ", "),
		)
	}
	return mergeEnvOverrides(scrubbed, overrides)
}

func mergeEnvOverrides(base []string, overrides map[string]string) []string {
	env := append([]string(nil), base...)
	if len(overrides) == 0 {
		return env
	}
	for key, value := range overrides {
		prefix := key + "="
		replaced := false
		for idx, current := range env {
			if strings.HasPrefix(current, prefix) {
				env[idx] = prefix + value
				replaced = true
				break
			}
		}
		if !replaced {
			env = append(env, prefix+value)
		}
	}
	return env
}
