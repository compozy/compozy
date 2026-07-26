package clientstate

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type workspacePurgePreparation struct {
	mu        sync.Mutex
	store     *stateStore
	workspace WorkspaceID
	resolved  bool
}

var _ WorkspacePurgePreparation = (*workspacePurgePreparation)(nil)

func (p *workspacePurgePreparation) Commit(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resolved {
		return nil
	}
	if err := p.store.commitPurge(ctx, p.workspace); err != nil {
		return err
	}
	p.resolved = true
	return nil
}

func (p *workspacePurgePreparation) Rollback(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.resolved {
		return nil
	}
	if err := p.store.rollbackPurge(ctx, p.workspace); err != nil {
		return err
	}
	p.resolved = true
	return nil
}

func (e *Engine) recoverWorkspacePurges(ctx context.Context) error {
	workspaces, err := e.store.stagedPurges(ctx)
	if err != nil {
		return err
	}
	for _, workspaceID := range workspaces {
		_, resolveErr := e.resolver.ResolveWorkspace(ctx, workspaceID)
		switch {
		case resolveErr == nil:
			if err := e.store.rollbackPurge(ctx, workspaceID); err != nil {
				return fmt.Errorf("clientstate: recover staged purge for workspace %q: %w", workspaceID, err)
			}
		case errors.Is(resolveErr, ErrWorkspaceNotFound):
			if err := e.store.commitPurge(ctx, workspaceID); err != nil {
				return fmt.Errorf("clientstate: discard staged purge for workspace %q: %w", workspaceID, err)
			}
		default:
			return fmt.Errorf("clientstate: resolve staged purge workspace %q: %w", workspaceID, resolveErr)
		}
	}
	return nil
}
