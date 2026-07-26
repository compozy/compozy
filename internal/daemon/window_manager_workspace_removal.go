package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/clientstate"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (r *windowManagerStoreWorkspaceResolver) prepareRemoval(
	workspace workspacepkg.Workspace,
	service *clientstate.Engine,
	manager *windowmanager.Manager,
	repository *windowManagerRepository,
) (workspacepkg.UnregisterPreparation, error) {
	if service == nil || manager == nil || repository == nil {
		return nil, errors.New("daemon: window-manager removal dependencies are required")
	}
	workspaceID := clientstate.WorkspaceID(workspace.ID)
	if workspaceID == "" {
		return nil, clientstate.ErrWorkspaceNotFound
	}
	r.mu.Lock()
	record := r.records[workspaceID]
	if record.deleting {
		r.mu.Unlock()
		return nil, clientstate.ErrWorkspaceNotFound
	}
	if record.generation == "" {
		record.generation = newWindowManagerWorkspaceGeneration()
	}
	record.deleting = true
	r.records[workspaceID] = record
	r.mu.Unlock()
	r.logger.Info(
		"window_manager.workspace.deletion_gate",
		"workspace_id", workspaceID,
		"state", "deleting",
	)
	return &windowManagerRemovalPreparation{
		resolver: r, service: service, manager: manager, repository: repository,
		workspace: workspaceID, generation: record.generation,
	}, nil
}

type windowManagerRemovalPreparation struct {
	resolver   *windowManagerStoreWorkspaceResolver
	service    *clientstate.Engine
	manager    *windowmanager.Manager
	repository *windowManagerRepository
	workspace  clientstate.WorkspaceID
	generation clientstate.WorkspaceGeneration
	purge      clientstate.WorkspacePurgePreparation
}

func (p *windowManagerRemovalPreparation) BeforeDelete(ctx context.Context) error {
	purge, err := p.service.PrepareWorkspacePurge(ctx, p.workspace)
	if err != nil {
		return fmt.Errorf("daemon: stage window-manager purge for workspace %q: %w", p.workspace, err)
	}
	p.purge = purge
	return nil
}

func (p *windowManagerRemovalPreparation) Commit(ctx context.Context) error {
	if p.purge == nil {
		return errors.New("daemon: window-manager purge was not staged")
	}
	if err := p.purge.Commit(ctx); err != nil {
		return fmt.Errorf("daemon: commit window-manager purge for workspace %q: %w", p.workspace, err)
	}
	p.resolver.mu.Lock()
	record := p.resolver.records[p.workspace]
	if record.generation == p.generation && record.deleting {
		delete(p.resolver.records, p.workspace)
	}
	p.resolver.mu.Unlock()
	workspaceID := windowmanager.WorkspaceID(p.workspace)
	p.manager.ForgetWorkspace(workspaceID)
	p.repository.forgetWorkspace(workspaceID)
	p.resolver.logger.Info(
		"window_manager.workspace.deletion_gate",
		"workspace_id", p.workspace,
		"state", "committed",
	)
	return nil
}

func (p *windowManagerRemovalPreparation) Rollback(ctx context.Context) error {
	if p.purge != nil {
		if err := p.purge.Rollback(ctx); err != nil {
			return fmt.Errorf("daemon: roll back window-manager purge for workspace %q: %w", p.workspace, err)
		}
	} else if err := ctx.Err(); err != nil {
		return err
	}
	p.resolver.mu.Lock()
	record := p.resolver.records[p.workspace]
	if record.generation == p.generation {
		record.deleting = false
		p.resolver.records[p.workspace] = record
	}
	p.resolver.mu.Unlock()
	p.resolver.logger.Info(
		"window_manager.workspace.deletion_gate",
		"workspace_id", p.workspace,
		"state", "rolled_back",
	)
	return nil
}
