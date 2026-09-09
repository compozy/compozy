package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/memory"
)

// bootMemoryCatalog applies the shared memory stream on every boot while
// retaining private checkpoint storage when session compaction needs it.
func (d *Daemon) bootMemoryCatalog(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	if state.memoryStore != nil {
		if err := state.memoryStore.OpenCatalog(ctx); err != nil {
			return fmt.Errorf("daemon: open memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
		}
		cleanup.add(func(ctx context.Context) error {
			return state.memoryStore.CloseCatalog(ctx)
		})
		return nil
	}

	migrationStore := memory.NewStore(
		firstNonEmptyString(state.cfg.Memory.GlobalDir, d.homePaths.MemoryDir),
		memory.WithCatalogDatabasePath(d.homePaths.DatabaseFile),
		memory.WithFileLimits(state.cfg.Memory.File),
	)
	if err := migrationStore.OpenCatalog(ctx); err != nil {
		return fmt.Errorf("daemon: migrate disabled memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
	}
	if state.cfg.Session.Compaction.Enabled {
		state.checkpointStore = migrationStore
		cleanup.add(migrationStore.CloseCatalog)
		return nil
	}
	if err := migrationStore.CloseCatalog(ctx); err != nil {
		return fmt.Errorf("daemon: close disabled memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
	}
	return nil
}
