package global

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil/storeseed/internal/seedbase"
)

type Seed = seedbase.Seed

// New creates a closed seed containing the current global schema.
func New(ctx context.Context) (*Seed, error) {
	return seedbase.New(ctx, Initialize)
}

// Initialize creates and closes the current global schema at path.
func Initialize(ctx context.Context, path string) error {
	database, err := globaldb.OpenGlobalDB(ctx, path)
	if err != nil {
		return fmt.Errorf("open global schema: %w", err)
	}
	if err := database.Close(ctx); err != nil {
		return fmt.Errorf("close global schema: %w", err)
	}
	return nil
}
