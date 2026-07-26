package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/agh/internal/memory"
)

// bootMemoryCatalog applies the shared memory stream on every boot while
// retaining a long-lived catalog only when the memory runtime is enabled.
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
		d.homePaths.MemoryDir,
		memory.WithCatalogDatabasePath(d.homePaths.DatabaseFile),
	)
	if err := migrationStore.OpenCatalog(ctx); err != nil {
		return fmt.Errorf("daemon: migrate disabled memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
	}
	if err := migrationStore.CloseCatalog(ctx); err != nil {
		return fmt.Errorf("daemon: close disabled memory catalog database %q: %w", d.homePaths.DatabaseFile, err)
	}
	return nil
}
