package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"
)

type daemonManagedDeliverySessions struct {
	sessions interface {
		Status(context.Context, string) (*session.Info, error)
		Stop(context.Context, string) error
	}
	registry worktreeRegistry
}

func (g daemonManagedDeliverySessions) AcquireDeliveryFence(
	ctx context.Context,
	workspaceID, worktreeID, sessionID string,
) (func(), error) {
	runtime, ok := g.sessions.(interface {
		AcquireWorktreeDeliveryFence(context.Context, string, string, string) (func(), error)
	})
	if !ok {
		return nil, worktree.ErrSessionActive
	}
	release, err := runtime.AcquireWorktreeDeliveryFence(ctx, workspaceID, worktreeID, sessionID)
	if err != nil {
		return nil, err
	}
	registry, ok := g.registry.(interface {
		HasOtherActiveWorktreeSession(context.Context, string, string, string) (bool, error)
	})
	if !ok {
		release()
		return nil, worktree.ErrSessionActive
	}
	other, err := registry.HasOtherActiveWorktreeSession(ctx, workspaceID, worktreeID, sessionID)
	if err != nil || other {
		release()
		if err != nil {
			return nil, err
		}
		return nil, worktree.ErrSessionActive
	}
	return release, nil
}

func (g daemonManagedDeliverySessions) StopDeliverySession(ctx context.Context, id string) error {
	info, err := g.sessions.Status(ctx, id)
	if err != nil {
		return err
	}
	if info.State == session.StateStopped {
		return nil
	}
	if err := g.sessions.Stop(ctx, id); err != nil {
		return fmt.Errorf("daemon: stop delivery caller: %w", err)
	}
	info, err = g.sessions.Status(ctx, id)
	if err != nil {
		return err
	}
	if info.State != session.StateStopped {
		return worktree.ErrSessionActive
	}
	return nil
}

// Inventory retries end once recovery succeeds so terminal journals are not polled forever.
func retryManagedDeliveryRecovery(
	ctx context.Context,
	recoverOperation func(context.Context) error,
	logger *slog.Logger,
) {
	delay := time.Second
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		err := recoverOperation(ctx)
		if !errors.Is(err, worktree.ErrDeliveryInventoryUnavailable) && !store.IsSQLiteBusy(err) {
			if err != nil && ctx.Err() == nil {
				logger.ErrorContext(ctx, "managed delivery deferred recovery failed", "error", err)
			}
			return
		}
		delay = min(2*delay, 30*time.Second)
		timer.Reset(delay)
	}
}

func stopManagedDeliveryRecovery(ctx context.Context, worker *ownedWorkerGroup) error {
	if worker == nil {
		return nil
	}
	select {
	case <-worker.Stop():
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: wait for managed delivery recovery: %w", ctx.Err())
	}
}
