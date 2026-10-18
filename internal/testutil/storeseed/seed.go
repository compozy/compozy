package storeseed

import (
	"context"

	globalseed "github.com/compozy/compozy/internal/testutil/storeseed/global"
	"github.com/compozy/compozy/internal/testutil/storeseed/internal/seedbase"
)

type Seed = seedbase.Seed

// NewGlobal creates a closed seed containing the current global schema.
func NewGlobal(ctx context.Context) (*Seed, error) {
	return globalseed.New(ctx)
}
