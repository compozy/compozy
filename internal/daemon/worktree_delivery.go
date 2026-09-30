package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/session"
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
