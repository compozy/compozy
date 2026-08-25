package daemon

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/windowmanager"
)

type windowManagerSessionReconciler struct {
	registry *windowManagerRegistry
	ready    atomic.Bool
}

var _ session.WindowReconciler = (*windowManagerSessionReconciler)(nil)

var errWindowManagerSessionReconcilerNotReady = errors.New(
	"daemon: window-manager session reconciler is not ready",
)

func windowManagerSessionReconcilerDependency(
	registry *windowManagerRegistry,
) session.WindowReconciler {
	if registry == nil {
		return nil
	}
	return &windowManagerSessionReconciler{registry: registry}
}

func (r *windowManagerSessionReconciler) SetReady() {
	if r != nil {
		r.ready.Store(true)
	}
}

func (r *windowManagerSessionReconciler) ReconcileDeletedSession(
	ctx context.Context,
	profileID string,
	workspaceID string,
	sessionID string,
) error {
	if r == nil || r.registry == nil {
		return errors.New("daemon: window-manager session reconciler is unavailable")
	}
	if !r.ready.Load() {
		return errWindowManagerSessionReconcilerNotReady
	}
	manager, err := r.registry.For(profileID)
	if err != nil {
		if errors.Is(err, errWindowManagerProfileDeleted) {
			return nil
		}
		return fmt.Errorf("resolve window manager for profile %q: %w", profileID, err)
	}
	if err := manager.ReconcileDeletedSession(ctx, windowmanager.WorkspaceID(workspaceID), sessionID); err != nil {
		if errors.Is(err, windowmanager.ErrWorkspaceNotFound) {
			return nil
		}
		return fmt.Errorf(
			"reconcile deleted session %q in workspace %q: %w",
			sessionID,
			workspaceID,
			err,
		)
	}
	return nil
}
