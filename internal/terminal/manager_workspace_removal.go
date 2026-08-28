package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// workspaceRemovalPreparation stages terminal authority and journal storage
// for the resolver's workspace deletion transaction. Terminal processes stay
// available until Commit succeeds, so a failed workspace-row deletion can be
// rolled back without losing terminal state.
type workspaceRemovalPreparation struct {
	manager     *Service
	journal     workspacepkg.UnregisterPreparation
	workspaceID string

	mu           sync.Mutex
	beforeDelete bool
	committed    bool
	rolledBack   bool
}

// PrepareWorkspaceRemoval captures journal identity while the workspace
// registration still exists. It does not close a terminal or delete data.
func (m *Service) PrepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
) (workspacepkg.UnregisterPreparation, error) {
	return m.prepareWorkspaceRemoval(ctx, workspaceID, "")
}

// PrepareWorkspaceDeletion rebuilds removal ownership from the durable deletion snapshot.
func (m *Service) PrepareWorkspaceDeletion(
	ctx context.Context,
	workspace workspacepkg.Workspace,
) (workspacepkg.UnregisterPreparation, error) {
	return m.prepareWorkspaceRemoval(ctx, workspace.ID, workspace.RootDir)
}

func (m *Service) prepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
	rootDir string,
) (workspacepkg.UnregisterPreparation, error) {
	if err := requestContextError(ctx, "prepare workspace removal"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("terminal: workspace id is required")
	}
	var journalPreparation workspacepkg.UnregisterPreparation
	var err error
	if strings.TrimSpace(rootDir) == "" {
		journalPreparation, err = m.journal.PrepareWorkspaceRemoval(ctx, workspaceID)
	} else {
		journalPreparation, err = m.journal.PrepareWorkspaceRemovalAt(ctx, workspaceID, rootDir)
	}
	if err != nil {
		return nil, fmt.Errorf("terminal: prepare journal workspace removal: %w", err)
	}
	if journalPreparation == nil {
		return nil, errors.New("terminal: journal workspace removal preparation is required")
	}
	return &workspaceRemovalPreparation{
		manager: m, journal: journalPreparation, workspaceID: workspaceID,
	}, nil
}

func (p *workspaceRemovalPreparation) BeforeDelete(ctx context.Context) error {
	if p == nil || p.manager == nil || p.journal == nil {
		return errors.New("terminal: workspace removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return errors.New("terminal: workspace removal preparation was rolled back")
	}
	if p.beforeDelete {
		return nil
	}
	if err := p.manager.sealWorkspace(ctx, p.workspaceID); err != nil {
		return err
	}
	if err := p.manager.waitWorkspaceProducers(ctx, p.workspaceID); err != nil {
		p.manager.unsealWorkspace(p.workspaceID)
		return err
	}
	if err := p.journal.BeforeDelete(ctx); err != nil {
		p.manager.unsealWorkspace(p.workspaceID)
		return fmt.Errorf("terminal: stage journal workspace removal: %w", err)
	}
	p.beforeDelete = true
	return nil
}

func (p *workspaceRemovalPreparation) Commit(ctx context.Context) error {
	if p == nil || p.manager == nil || p.journal == nil {
		return errors.New("terminal: workspace removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return errors.New("terminal: workspace removal preparation was rolled back")
	}
	if !p.beforeDelete {
		return errors.New("terminal: workspace removal was not staged before commit")
	}
	if err := p.manager.archiveTerminals(ctx, "workspace_deleted", "workspace-lifecycle", func(key terminalKey) bool {
		return key.workspaceID == p.workspaceID
	}); err != nil {
		return fmt.Errorf("terminal: archive workspace terminals: %w", err)
	}
	if err := p.journal.Commit(ctx); err != nil {
		return fmt.Errorf("terminal: commit journal workspace removal: %w", err)
	}
	p.committed = true
	return nil
}

func (p *workspaceRemovalPreparation) Rollback(ctx context.Context) error {
	if p == nil || p.manager == nil || p.journal == nil {
		return errors.New("terminal: workspace removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return nil
	}
	if err := p.journal.Rollback(ctx); err != nil {
		return fmt.Errorf("terminal: rollback journal workspace removal: %w", err)
	}
	p.manager.unsealWorkspace(p.workspaceID)
	p.rolledBack = true
	return nil
}
