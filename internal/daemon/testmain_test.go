package daemon

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/agh/internal/testutil/storeseed"
)

const deadEntityMCPHelperEnv = "AGH_TEST_DEAD_ENTITY_MCP_HELPER"

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
		"AGH_DAEMON_GOLEAK_HELPER",
		"AGH_TEST_DAEMON_ENV_HELPER",
		"AGH_TEST_DAEMON_EXTENSION_HELPER",
		"AGH_TEST_DAEMON_SESSION_STOP_HELPER",
		deadEntityMCPHelperEnv,
		"AGH_TEST_NIGHTLY_COMBINED_HELPER",
	} {
		if os.Getenv(name) == "1" {
			return true
		}
	}
	return false
}

func reportDaemonTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "daemon tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
