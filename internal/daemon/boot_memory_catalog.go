package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/memory"
)

// bootMemoryCatalog keeps operator access independent of automatic memory processing.
func (d *Daemon) bootMemoryCatalog(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state.memoryStore != nil {
		state.memoryCatalogStore = state.memoryStore
		if err := state.memoryStore.OpenCatalog(ctx); err != nil {
			return fmt.Errorf("daemon: open memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
		}
		cleanup.add(func(ctx context.Context) error {
			return state.memoryStore.CloseCatalog(ctx)
		})
		return nil
	}

	catalogStore := memory.NewStore(
		firstNonEmptyString(state.cfg.Memory.GlobalDir, d.homePaths.MemoryDir),
		memory.WithCatalogDatabasePath(d.homePaths.DatabaseFile),
		memory.WithFileLimits(state.cfg.Memory.File),
	)
	if err := catalogStore.OpenCatalog(ctx); err != nil {
		return fmt.Errorf("daemon: migrate disabled memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
	}
	state.memoryCatalogStore = catalogStore
	cleanup.add(catalogStore.CloseCatalog)
	return nil
}
