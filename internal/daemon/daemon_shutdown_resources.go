package daemon

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// shutdownPersistentResources preserves replacement discovery after singleton ownership ends.
func (d *Daemon) shutdownPersistentResources(
	ctx context.Context,
	targets *shutdownTargets,
	terminal bool,
	errs *[]error,
) {
	// Caller expiry retains remaining resources; an earlier attempt may have released the singleton.
	if targets.sessionShutdownIncomplete {
		settleDatabaseAfterIncompleteShutdown(ctx, targets, terminal, errs)
		return
	}
	if targets.viewPatches != nil {
		targets.viewPatches.CloseViewPatches()
	}
	if targets.windowManagers != nil {
		appendWrappedError(errs, "daemon: close window managers", targets.windowManagers.Close())
	}
	if targets.windowManagerStore != nil {
		appendWrappedError(errs, "daemon: close window-manager store", targets.windowManagerStore.Close())
	}
	if targets.marketplace != nil {
		appendWrappedError(errs, "daemon: shutdown marketplace", targets.marketplace.Shutdown(ctx))
	}
	if targets.lock != nil && !targets.lock.released.Load() {
		if err := RemoveInfo(targets.infoPath); err != nil {
			*errs = append(*errs, err)
		}
	}
	if targets.registry != nil {
		appendWrappedError(errs, "daemon: close global database", targets.registry.Close(ctx))
	}
	if targets.lock != nil {
		if err := targets.lock.Release(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if targets.closeLogger != nil {
		appendWrappedError(errs, "daemon: close logger", targets.closeLogger())
	}
}

// walCheckpointSource exposes the open database of a registry that owns a write-ahead log.
type walCheckpointSource interface {
	DB() *sql.DB
}

// settleDatabaseAfterIncompleteShutdown checkpoints the global database although the session manager
// did not finish. A terminal shutdown is about to end the process, so it runs the checkpointed close
// and keeps only the singleton for the pending cleanup. A retryable shutdown keeps the database open
// for the caller that joins that cleanup later and checkpoints the write-ahead log in place.
func settleDatabaseAfterIncompleteShutdown(
	ctx context.Context,
	targets *shutdownTargets,
	terminal bool,
	errs *[]error,
) {
	if targets.registry == nil {
		return
	}
	if terminal {
		appendWrappedError(errs, "daemon: close global database", targets.registry.Close(ctx))
		return
	}
	if source, ok := targets.registry.(walCheckpointSource); ok {
		appendWrappedError(errs, "daemon: checkpoint global database", store.CheckpointPassive(ctx, source.DB()))
	}
}

func appendWrappedError(errs *[]error, prefix string, err error) {
	if errs == nil || err == nil {
		return
	}
	*errs = append(*errs, fmt.Errorf("%s: %w", prefix, err))
}
