package journal

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/compozy/compozy/internal/store/workspacedb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

// workspaceRemovalPreparation keeps the journal database identity available
// after the resolver removes the workspace registration. It does not remove
// lanes or files until Commit, so Rollback leaves historical data readable.
type workspaceRemovalPreparation struct {
	service     *Service
	database    *workspacedb.WorkspaceRemovalPreparation
	workspaceID string

	mu           sync.Mutex
	beforeDelete bool
	committed    bool
	rolledBack   bool
}

// PrepareWorkspaceRemoval captures the database path before registration
// deletion makes the resolver unable to resolve it again.
func (s *Service) PrepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
) (workspacepkg.UnregisterPreparation, error) {
	if s == nil {
		return nil, errors.New("terminal journal: service is required")
	}
	databasePreparation, err := s.databases.PrepareWorkspaceRemoval(ctx, workspaceID)
	return s.newWorkspaceRemovalPreparation(workspaceID, databasePreparation, err)
}

// PrepareWorkspaceRemovalAt reconstructs removal ownership from a durable workspace snapshot.
func (s *Service) PrepareWorkspaceRemovalAt(
	ctx context.Context,
	workspaceID string,
	rootDir string,
) (workspacepkg.UnregisterPreparation, error) {
	if s == nil {
		return nil, errors.New("terminal journal: service is required")
	}
	databasePreparation, err := s.databases.PrepareWorkspaceRemovalAt(ctx, workspaceID, rootDir)
	return s.newWorkspaceRemovalPreparation(workspaceID, databasePreparation, err)
}

func (s *Service) newWorkspaceRemovalPreparation(
	workspaceID string,
	databasePreparation *workspacedb.WorkspaceRemovalPreparation,
	err error,
) (workspacepkg.UnregisterPreparation, error) {
	if err != nil {
		return nil, fmt.Errorf("terminal journal: prepare workspace database removal: %w", err)
	}
	return &workspaceRemovalPreparation{
		service: s, database: databasePreparation, workspaceID: workspaceID,
	}, nil
}

func (p *workspaceRemovalPreparation) BeforeDelete(ctx context.Context) error {
	if p == nil || p.service == nil || p.database == nil {
		return errors.New("terminal journal: workspace removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return errors.New("terminal journal: workspace removal preparation was rolled back")
	}
	if p.beforeDelete {
		return nil
	}
	// Database preparation only records the lifecycle stage. Lanes remain open
	// until terminal processes are closed in the terminal domain's Commit.
	if err := p.database.BeforeDelete(ctx); err != nil {
		return fmt.Errorf("terminal journal: stage workspace database removal: %w", err)
	}
	p.beforeDelete = true
	return nil
}

func (p *workspaceRemovalPreparation) Commit(ctx context.Context) error {
	if p == nil || p.service == nil || p.database == nil {
		return errors.New("terminal journal: workspace removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return errors.New("terminal journal: workspace removal preparation was rolled back")
	}
	if !p.beforeDelete {
		return errors.New("terminal journal: workspace removal was not staged before commit")
	}
	if err := p.service.closeLanes(ctx, func(lane *terminalLane) bool {
		return lane.info.WS == p.workspaceID
	}); err != nil {
		return fmt.Errorf("terminal journal: close workspace lanes: %w", err)
	}
	p.service.artifactMu.Lock()
	artifactErr := p.service.removeWorkspaceFiles(p.workspaceID)
	p.service.artifactMu.Unlock()
	databaseErr := p.database.Commit(ctx)
	if err := errors.Join(artifactErr, databaseErr); err != nil {
		return err
	}
	p.service.removeWorkspaceLiveTails(p.workspaceID)
	p.committed = true
	return nil
}

func (p *workspaceRemovalPreparation) Rollback(ctx context.Context) error {
	if p == nil || p.database == nil {
		return errors.New("terminal journal: workspace removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return nil
	}
	if err := p.database.Rollback(ctx); err != nil {
		return fmt.Errorf("terminal journal: rollback workspace database removal: %w", err)
	}
	p.rolledBack = true
	return nil
}

var _ workspacepkg.UnregisterPreparation = (*workspaceRemovalPreparation)(nil)
var _ terminalpkg.Journal = (*Service)(nil)
