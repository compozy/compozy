package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var errUnregisterCoordinatorClosed = errors.New("workspace: unregister coordinator is closed")

type unregisterCoordinator struct {
	preparerMu sync.RWMutex
	preparer   UnregisterPreparer

	stateMu sync.Mutex
	closed  bool
	gate    chan struct{}
	pending map[string]*unregisterFinalization
}

type unregisterFinalization struct {
	intent      DeletionIntent
	preparation UnregisterPreparation
	staged      bool
}

func newUnregisterCoordinator() *unregisterCoordinator {
	return &unregisterCoordinator{
		gate:    make(chan struct{}, 1),
		pending: make(map[string]*unregisterFinalization),
	}
}

func (c *unregisterCoordinator) acquire(ctx context.Context, allowClosed bool) error {
	if c == nil {
		return errors.New("workspace: unregister coordinator is required")
	}
	select {
	case c.gate <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("workspace: wait for unregister coordinator: %w", ctx.Err())
	}
	c.stateMu.Lock()
	closed := c.closed
	c.stateMu.Unlock()
	if closed && !allowClosed {
		<-c.gate
		return errUnregisterCoordinatorClosed
	}
	return nil
}

func (c *unregisterCoordinator) release() {
	<-c.gate
}

func (c *unregisterCoordinator) closeAdmission() {
	c.stateMu.Lock()
	c.closed = true
	c.stateMu.Unlock()
}

func (r *Resolver) unregisterWorkspace(ctx context.Context, workspaceID string) error {
	if err := r.unregister.acquire(ctx, false); err != nil {
		return err
	}
	defer r.unregister.release()

	if finalization := r.unregister.pending[workspaceID]; finalization != nil {
		return r.finishUnregister(ctx, finalization)
	}
	intent, err := r.store.GetWorkspaceDeletionIntent(ctx, workspaceID)
	if err == nil {
		finalization, prepareErr := r.recoverUnregister(ctx, intent)
		if prepareErr != nil {
			return prepareErr
		}
		return r.finishUnregister(ctx, finalization)
	}
	if !errors.Is(err, ErrWorkspaceDeletionIntentNotFound) {
		return fmt.Errorf("workspace: load unregister intent %q: %w", workspaceID, err)
	}

	workspace, err := r.store.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace: unregister %q: %w", workspaceID, err)
	}
	preparation, err := r.prepareUnregister(ctx, workspace)
	if err != nil {
		return fmt.Errorf("workspace: prepare unregister %q: %w", workspaceID, err)
	}
	if preparation != nil {
		if err := preparation.BeforeDelete(ctx); err != nil {
			return errors.Join(
				fmt.Errorf("workspace: stage unregister %q: %w", workspaceID, err),
				r.rollbackUnregisterPreparation(ctx, workspaceID, preparation),
			)
		}
	}
	finalization := &unregisterFinalization{
		intent: DeletionIntent{Workspace: cloneWorkspace(workspace)}, preparation: preparation, staged: true,
	}
	r.unregister.pending[workspaceID] = finalization
	if err := r.store.StageWorkspaceDeletion(ctx, workspaceID); err != nil {
		delete(r.unregister.pending, workspaceID)
		return errors.Join(
			fmt.Errorf("workspace: unregister %q: %w", workspaceID, err),
			r.rollbackUnregisterPreparation(ctx, workspaceID, preparation),
		)
	}
	r.Invalidate(workspaceID)
	return r.finishUnregister(ctx, finalization)
}

func (r *Resolver) recoverUnregister(
	ctx context.Context,
	intent DeletionIntent,
) (*unregisterFinalization, error) {
	workspaceID := strings.TrimSpace(intent.Workspace.ID)
	if workspaceID == "" {
		return nil, errors.New("workspace: deletion intent workspace id is required")
	}
	preparation, err := r.prepareUnregister(ctx, intent.Workspace)
	if err != nil {
		return nil, fmt.Errorf("workspace: restore unregister preparation %q: %w", workspaceID, err)
	}
	finalization := &unregisterFinalization{intent: intent, preparation: preparation}
	r.unregister.pending[workspaceID] = finalization
	return finalization, nil
}

func (r *Resolver) finishUnregister(ctx context.Context, finalization *unregisterFinalization) error {
	workspaceID := finalization.intent.Workspace.ID
	if !finalization.staged && finalization.preparation != nil {
		if err := finalization.preparation.BeforeDelete(ctx); err != nil {
			return fmt.Errorf("workspace: restore unregister stage %q: %w", workspaceID, err)
		}
		finalization.staged = true
	}
	if finalization.preparation != nil {
		if err := finalization.preparation.Commit(ctx); err != nil {
			return fmt.Errorf("workspace: commit unregister %q: %w", workspaceID, err)
		}
	}
	if err := r.store.CompleteWorkspaceDeletion(ctx, workspaceID); err != nil {
		return fmt.Errorf("workspace: complete unregister intent %q: %w", workspaceID, err)
	}
	delete(r.unregister.pending, workspaceID)
	if err := r.notifyChangeHook(ctx, "unregister", workspaceID); err != nil {
		r.logger.Error(
			"workspace: unregister committed before derived resource sync failed",
			"workspace_id", workspaceID,
			"error", err,
		)
	}
	return nil
}

func (r *Resolver) rollbackUnregisterPreparation(
	ctx context.Context,
	workspaceID string,
	preparation UnregisterPreparation,
) error {
	if preparation == nil {
		return nil
	}
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackDeleteTimeout)
	defer cancel()
	return wrapUnregisterRollbackError(workspaceID, preparation.Rollback(rollbackCtx))
}

// ResumeUnregisters completes durable workspace deletions before runtime traffic is accepted.
func (r *Resolver) ResumeUnregisters(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := r.unregister.acquire(ctx, false); err != nil {
		return err
	}
	defer r.unregister.release()
	return r.finishPersistedUnregisters(ctx)
}

// DrainUnregisters closes admission and retries every owned workspace finalization.
func (r *Resolver) DrainUnregisters(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	r.unregister.closeAdmission()
	if err := r.unregister.acquire(ctx, true); err != nil {
		return err
	}
	defer r.unregister.release()
	return r.finishPersistedUnregisters(ctx)
}

func (r *Resolver) finishPersistedUnregisters(ctx context.Context) error {
	intents, err := r.store.ListWorkspaceDeletionIntents(ctx)
	if err != nil {
		return fmt.Errorf("workspace: list unregister intents: %w", err)
	}
	var errs []error
	for _, intent := range intents {
		if err := checkContext(ctx); err != nil {
			errs = append(errs, err)
			break
		}
		workspaceID := intent.Workspace.ID
		finalization := r.unregister.pending[workspaceID]
		if finalization == nil {
			finalization, err = r.recoverUnregister(ctx, intent)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}
		if err := r.finishUnregister(ctx, finalization); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
