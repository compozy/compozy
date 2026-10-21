package daemon

import (
	"context"
	"errors"
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
	return r.registry.reconcilePartitions(ctx, func(manager *windowmanager.Manager, partition windowmanager.WorkspaceID) error {
		return manager.ReconcileDeletedSession(ctx, partition, sessionID)
	})
}
