package observe

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/agh/internal/testutil/storeseed"
)

var observeTestGlobalSeed *storeseed.Seed

func TestMain(m *testing.M) {
	os.Exit(runObserveTests(m))
}

func runObserveTests(m *testing.M) (code int) {
	seed, err := storeseed.NewGlobal(context.Background())
	if err != nil {
		reportObserveTestMainError("create global seed: %v", err)
		return 1
	}
	defer func() {
		if err := seed.Close(); err != nil {
			reportObserveTestMainError("close global seed: %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	observeTestGlobalSeed = seed
	return m.Run()
}

func reportObserveTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "observe tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
