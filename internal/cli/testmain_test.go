package cli

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/agh/internal/testutil/storeseed"
)

var cliTestStoreSeed *storeseed.Seed

func TestMain(m *testing.M) {
	os.Exit(runCLITests(m))
}

func runCLITests(m *testing.M) (code int) {
	seed, err := storeseed.NewGlobal(context.Background())
	if err != nil {
		reportCLITestMainError("create store seed: %v", err)
		return 1
	}
	defer func() {
		if err := seed.Close(); err != nil {
			reportCLITestMainError("close store seed: %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	cliTestStoreSeed = seed
	return m.Run()
}

func reportCLITestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "cli tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
