package daemon

import (
	"context"
	"fmt"
)

func (d *Daemon) shutdownPersistentResources(ctx context.Context, targets shutdownTargets, errs *[]error) {
	if targets.windowManager != nil {
		appendWrappedError(errs, "daemon: close window manager", targets.windowManager.Close())
	}
	if targets.windowManagerStore != nil {
		appendWrappedError(errs, "daemon: close window-manager store", targets.windowManagerStore.Close())
	}
	if targets.marketplace != nil {
		appendWrappedError(errs, "daemon: shutdown marketplace", targets.marketplace.Shutdown(ctx))
	}
	if err := RemoveInfo(targets.infoPath); err != nil {
		*errs = append(*errs, err)
	}
	if targets.memoryStore != nil {
		appendWrappedError(errs, "daemon: close memory catalog database", targets.memoryStore.CloseCatalog(ctx))
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

func appendWrappedError(errs *[]error, prefix string, err error) {
	if errs == nil || err == nil {
		return
	}
	*errs = append(*errs, fmt.Errorf("%s: %w", prefix, err))
}
