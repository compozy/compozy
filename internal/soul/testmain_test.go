package soul_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/agh/internal/testutil/storeseed"
)

var soulTestStoreSeed *storeseed.Seed

func TestMain(m *testing.M) {
	os.Exit(runSoulTests(m))
}

func runSoulTests(m *testing.M) (code int) {
	seed, err := storeseed.NewGlobal(context.Background())
	if err != nil {
		reportSoulTestMainError("create store seed: %v", err)
		return 1
	}
	defer func() {
		if err := seed.Close(); err != nil {
			reportSoulTestMainError("close store seed: %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	soulTestStoreSeed = seed
	return m.Run()
}

func reportSoulTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "soul tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
