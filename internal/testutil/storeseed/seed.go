package storeseed

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/memory"
	globalseed "github.com/compozy/compozy/internal/testutil/storeseed/global"
	"github.com/compozy/compozy/internal/testutil/storeseed/internal/seedbase"
)

type Seed = seedbase.Seed

// NewGlobal creates a closed seed containing the current global schema.
func NewGlobal(ctx context.Context) (*Seed, error) {
	return globalseed.New(ctx)
}

// NewMemory creates a closed seed containing the current memory schema.
func NewMemory(ctx context.Context) (*Seed, error) {
	return seedbase.New(ctx, initializeMemory)
}

// NewCombined creates a closed seed containing the current global and memory schemas.
func NewCombined(ctx context.Context) (*Seed, error) {
	return seedbase.New(ctx, func(ctx context.Context, path string) error {
		if err := globalseed.Initialize(ctx, path); err != nil {
			return err
		}
		return initializeMemory(ctx, path)
	})
}

func initializeMemory(ctx context.Context, path string) error {
	memoryStore := memory.NewStore(
		filepath.Join(filepath.Dir(path), "memory"),
		memory.WithCatalogDatabasePath(path),
	)
	if err := memoryStore.OpenCatalog(ctx); err != nil {
		return fmt.Errorf("open memory schema: %w", err)
	}
	if err := memoryStore.CloseCatalog(ctx); err != nil {
		return fmt.Errorf("close memory schema: %w", err)
	}
	return nil
}
