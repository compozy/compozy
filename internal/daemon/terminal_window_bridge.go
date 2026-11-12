package daemon

import (
	"context"
	"errors"
	"log/slog"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/compozy/compozy/internal/windowmanager"
)

// Agent terminals stay in the catalog until the operator explicitly opens a window.
func attachTerminalWindowBridge(
	ctx context.Context,
	terminals *terminalpkg.Service,
	windows windowManagerProvider,
	logger *slog.Logger,
) *ownedWorkerGroup {
	if terminals == nil || windows == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	worker := newOwnedWorkerGroup(cancelWorker)
	terminals.Observe(func(ctx context.Context, event terminalpkg.Event) {
		retire := event.Kind == terminalpkg.EventKindDeleted ||
			(event.Kind == terminalpkg.EventKindClosed && event.Origin.Kind == terminalpkg.ActorKindAgent)
		if !retire {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := retireTerminalWindow(cleanupCtx, windows, event); err != nil {
			logger.Warn("daemon: retire terminal window", "terminal_id", event.TerminalID,
				"workspace_id", event.WorkspaceID, "profile_id", event.ProfileID, "error", err)
			if complete, ok := worker.Begin(); ok {
				go func() {
					defer complete()
					retryTerminalWindowRetirement(workerCtx, windows, event, logger)
				}()
			}
		}
	})
	return worker
}

func retireTerminalWindow(ctx context.Context, windows windowManagerProvider, event terminalpkg.Event) error {
	manager, err := windows.For(event.ProfileID)
	if errors.Is(err, errWindowManagerProfileDeleted) {
		return nil
	}
	if err != nil {
		return err
	}
	ids := map[string]bool{string(event.TerminalID): true}
	// Terminal windows can be opened on the project, All projects, or User desktop.
	for _, partition := range []windowmanager.WorkspaceID{
		windowmanager.WorkspaceID(event.WorkspaceID),
		windowmanager.GlobalDesktopWorkspaceID,
		windowmanager.UserDesktopWorkspaceID,
	} {
		if err := manager.RetireTerminals(
			ctx,
			partition,
			ids,
		); err != nil &&
			!errors.Is(err, windowmanager.ErrWorkspaceNotFound) {
			return err
		}
	}
	return nil
}

// Terminal processes never survive daemon startup, so every stored terminal binding is stale at boot.
func (r *windowManagerRegistry) reconcileBootTerminals(ctx context.Context) error {
	return r.reconcilePartitions(ctx, func(manager *windowmanager.Manager, partition windowmanager.WorkspaceID) error {
		return manager.RetireTerminals(ctx, partition, nil)
	})
}

func retryTerminalWindowRetirement(
	ctx context.Context,
	windows windowManagerProvider,
	event terminalpkg.Event,
	logger *slog.Logger,
) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			attemptCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := retireTerminalWindow(attemptCtx, windows, event)
			cancel()
			if err == nil {
				return
			}
			logger.Warn("daemon: retry terminal window retirement", "terminal_id", event.TerminalID,
				"workspace_id", event.WorkspaceID, "profile_id", event.ProfileID, "error", err)
		}
	}
}
