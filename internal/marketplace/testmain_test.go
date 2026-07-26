package marketplace

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/compozy/agh/internal/testutil/storeseed"
)

var marketplaceTestStoreSeed *storeseed.Seed

func TestMain(m *testing.M) {
	os.Exit(runMarketplaceTests(m))
}

func runMarketplaceTests(m *testing.M) (code int) {
	seed, err := storeseed.NewGlobal(context.Background())
	if err != nil {
		reportMarketplaceTestMainError("create store seed: %v", err)
		return 1
	}
	defer func() {
		if err := seed.Close(); err != nil {
			reportMarketplaceTestMainError("close store seed: %v", err)
			if code == 0 {
				code = 1
			}
		}
	}()

	marketplaceTestStoreSeed = seed
	return m.Run()
}

func reportMarketplaceTestMainError(format string, args ...any) {
	if _, err := fmt.Fprintf(os.Stderr, "marketplace tests: "+format+"\n", args...); err != nil {
		panic(err)
	}
}
