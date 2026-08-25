//go:build mage

package main

import (
	"context"
	"errors"
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
	goTestPackageLimitEnvVar = "COMPOZY_GO_TEST_P"
	goTestShardIndexEnvVar   = "COMPOZY_GO_TEST_SHARD_INDEX"
	goTestShardTotalEnvVar   = "COMPOZY_GO_TEST_SHARD_TOTAL"
	goTestFullCheckptrEnvVar = "COMPOZY_GO_FULL_CHECKPTR"
	goTestUncachedEnvVar     = "COMPOZY_GO_TEST_UNCACHED"
	goTestJSONFileDirEnvVar  = "COMPOZY_GO_TEST_JSONFILE_DIR"
	goSplitTestPackage       = "github.com/compozy/compozy/internal/store/globaldb"
	goAllPackagesPattern     = "./..."
	goUnitTestParallelism    = 4
	moderncCheckptrFlag      = "modernc.org/...=-d=checkptr=0"
)

type goTestShard struct {
	index int
	total int
}

type goUnitTestInvocation struct {
	packages []string
	tests    []string
}

func goUnitTestSafetyArgs(fullCheckptr, uncached bool) []string {
	args := []string{"-race"}
	if !fullCheckptr {
		args = append(args, "-gcflags="+moderncCheckptrFlag)
	}
	if uncached {
		args = append(args, "-count=1")
	}
	return args
}

func goSDKTestArgs(uncached bool) []string {
	args := []string{"test", "-race", "-parallel=4"}
	if uncached {
		args = append(args, "-count=1")
	}
	return append(args, "./...")
}

// ambientRuntimeStateEnvVars are the runtime identity vars a QA lab or dev
// shell may export (bootstrap env block / worktree isolation envelope). Tests
// must never inherit them ambiently; lanes that need one pass it as an
// explicit override, which is applied after the scrub and therefore wins.
var ambientRuntimeStateEnvVars = []string{
	"COMPOZY_HOME",
	"COMPOZY_CONFIG_HOME",
	"COMPOZY_HTTP_PORT",
	"COMPOZY_UDS_PATH",
	"COMPOZY_WEB_API_PROXY_TARGET",
	"COMPOZY_WEB_DIST_DIR",
	"TMUX_BRIDGE_SOCKET",
	"PROVIDER_HOME",
	"PROVIDER_CODEX_HOME",
}

// goUnitTestPackageLimit resolves the unit lane's `go test -p` cap. An
// explicit COMPOZY_GO_TEST_P wins; otherwise the package fan-out accounts for the
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

func goUnitTestInvocations(ctx context.Context) ([]goUnitTestInvocation, error) {
	shard, enabled, err := parseGoTestShard(
		os.Getenv(goTestShardIndexEnvVar),
		os.Getenv(goTestShardTotalEnvVar),
	)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return []goUnitTestInvocation{{packages: []string{goAllPackagesPattern}}}, nil
	}

	cmd := exec.CommandContext(ctx, "go", "list", goAllPackagesPattern)
	cmd.Env = hermeticGoTestEnv(withRaceEnabledEnv(nil))
	packages, err := goListPackagePaths(cmd)
	if err != nil {
		return nil, err
	}

	splitTests, err := listGoTopLevelTests(ctx, goSplitTestPackage)
	if err != nil {
		return nil, err
	}
	census, err := loadGoTestCensus(goTestCensusPath)
	if err != nil {
		return nil, err
	}
	if census == nil {
		fmt.Printf("Note: %s missing; shard partition balances by item count only\n", goTestCensusPath)
	} else if missing := census.missingPackageCount(packages); missing > 0 {
		fmt.Printf(
			"Note: %d packages missing from %s use the default weight; refresh via testCensusUpdate\n",
			missing,
			goTestCensusPath,
		)
	}
	invocations, err := shardGoUnitTestInvocations(packages, splitTests, census, shard)
	if err != nil {
		return nil, err
	}
	selectedPackageCount, selectedTestCount := 0, 0
	for _, invocation := range invocations {
		selectedPackageCount += len(invocation.packages)
		selectedTestCount += len(invocation.tests)
	}
	fmt.Printf(
		"Go unit-test shard %d/%d selected %d package invocations and %d of %d %s tests\n",
		shard.index+1,
		shard.total,
		selectedPackageCount,
		selectedTestCount,
		len(splitTests),
		goSplitTestPackage,
	)
	return invocations, nil
}

func goListPackagePaths(cmd *exec.Cmd) ([]string, error) {
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf(
				"list Go unit-test packages: %w: %s",
				err,
				strings.TrimSpace(string(exitErr.Stderr)),
			)
		}
		return nil, fmt.Errorf("list Go unit-test packages: %w", err)
	}
	packages := strings.Fields(string(output))
	if len(packages) == 0 {
		return nil, fmt.Errorf("list Go unit-test packages: go list returned no packages")
	}
	return packages, nil
}

// shardGoUnitTestInvocations partitions regular packages and split-package
// tests into one duration-weighted (LPT) bin per shard, so every item runs on
// exactly one shard. A shard may legitimately draw no split tests; its split
// invocation is then omitted.
func shardGoUnitTestInvocations(
	packages []string,
	splitTests []string,
	census *goTestCensus,
	shard goTestShard,
) ([]goUnitTestInvocation, error) {
	regularPackages := make([]string, 0, len(packages)-1)
	foundSplitPackage := false
	for _, packagePath := range packages {
		if packagePath == goSplitTestPackage {
			foundSplitPackage = true
			continue
		}
		regularPackages = append(regularPackages, packagePath)
	}
	if !foundSplitPackage {
		return nil, fmt.Errorf("split Go unit-test package %q was not listed", goSplitTestPackage)
	}
	if len(splitTests) == 0 {
		return nil, fmt.Errorf("split Go unit-test package %q listed no tests", goSplitTestPackage)
	}

	items := make([]goShardItem, 0, len(regularPackages)+len(splitTests))
	for _, packagePath := range regularPackages {
		items = append(items, goShardItem{name: packagePath, weight: census.packageWeight(packagePath)})
	}
	for _, testName := range splitTests {
		items = append(items, goShardItem{
			name:      testName,
			weight:    census.splitTestWeight(testName),
			splitTest: true,
		})
	}

	selectedPackages := make([]string, 0, len(regularPackages))
	selectedTests := make([]string, 0, len(splitTests))
	for _, item := range partitionGoShardItems(items, shard.total)[shard.index] {
		if item.splitTest {
			selectedTests = append(selectedTests, item.name)
			continue
		}
		selectedPackages = append(selectedPackages, item.name)
	}
	slices.Sort(selectedPackages)
	slices.Sort(selectedTests)

	invocations := make([]goUnitTestInvocation, 0, 2)
	if len(selectedPackages) > 0 {
		invocations = append(invocations, goUnitTestInvocation{packages: selectedPackages})
	}
	if len(selectedTests) > 0 {
		invocations = append(invocations, goUnitTestInvocation{
			packages: []string{goSplitTestPackage},
			tests:    selectedTests,
		})
	}
	if len(invocations) == 0 {
		return nil, fmt.Errorf(
			"select Go unit-test shard %d/%d: no work selected from %d packages and %d tests",
			shard.index+1,
			shard.total,
			len(regularPackages),
			len(splitTests),
		)
	}
	return invocations, nil
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
