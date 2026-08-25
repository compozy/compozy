package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/storeseed"
)

const deadEntityMCPHelperEnv = "COMPOZY_TEST_DEAD_ENTITY_MCP_HELPER"

var daemonTestStoreSeed *storeseed.Seed

func TestMain(m *testing.M) {
	os.Exit(runDaemonTests(m))
}

func runDaemonTests(m *testing.M) (code int) {
	if isDaemonTestHelperProcess() {
		return m.Run()
	}
	seed, err := storeseed.NewCombined(context.Background())
	if err != nil {
		reportDaemonTestMainError("create store seed: %v", err)
		return 1
	}
	defer func() {
		if err := seed.Close(); err != nil {
			reportDaemonTestMainError("close store seed: %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	daemonTestStoreSeed = seed
	return m.Run()
}

func isDaemonTestHelperProcess() bool {
	for _, name := range []string{
		"COMPOZY_DAEMON_GOLEAK_HELPER",
		"COMPOZY_TEST_DAEMON_ENV_HELPER",
		"COMPOZY_TEST_DAEMON_EXTENSION_HELPER",
		"COMPOZY_TEST_DAEMON_SESSION_STOP_HELPER",
		deadEntityMCPHelperEnv,
		"COMPOZY_TEST_NIGHTLY_COMBINED_HELPER",
	} {
		if os.Getenv(name) == "1" {
			return true
		}
	}
	return false
}

func openDaemonTestGlobalDBAtPath(ctx context.Context, path string) (*globaldb.GlobalDB, error) {
	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat daemon test global database %q: %w", path, err)
		}
		if err := daemonTestStoreSeed.Clone(path); err != nil {
			return nil, fmt.Errorf("clone daemon test store seed to %q: %w", path, err)
		}
	}
	return globaldb.OpenGlobalDB(ctx, path)
}

func reportDaemonTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "daemon tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
