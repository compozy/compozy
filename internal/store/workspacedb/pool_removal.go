package workspacedb

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// WorkspaceRemovalPreparation owns a sealed database removal until commit or rollback.
type WorkspaceRemovalPreparation struct {
	pool        *Pool
	workspaceID string
	path        string
	db          *DB

	mu             sync.Mutex
	staged         bool
	removalStarted bool
	committed      bool
	rolledBack     bool
	detachedClosed bool
}

// RemoveWorkspace closes one workspace database and removes its SQLite files.
func (p *Pool) RemoveWorkspace(ctx context.Context, workspaceID string) error {
	if p == nil {
		return nil
	}
	preparation, err := p.PrepareWorkspaceRemoval(ctx, workspaceID)
	if err != nil {
		return err
	}
	if err := preparation.BeforeDelete(ctx); err != nil {
		return errors.Join(err, preparation.Rollback(context.WithoutCancel(ctx)))
	}
	return preparation.Commit(ctx)
}

// PrepareWorkspaceRemoval captures the database identity through the registered root resolver.
func (p *Pool) PrepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
) (*WorkspaceRemovalPreparation, error) {
	return p.prepareWorkspaceRemoval(ctx, workspaceID, "")
}

// PrepareWorkspaceRemovalAt captures a database identity from a durable workspace snapshot.
func (p *Pool) PrepareWorkspaceRemovalAt(
	ctx context.Context,
	workspaceID string,
	rootDir string,
) (*WorkspaceRemovalPreparation, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, errors.New("store: workspace root directory is required for removal")
	}
	return p.prepareWorkspaceRemoval(ctx, workspaceID, rootDir)
}

func (p *Pool) prepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
	durableRoot string,
) (*WorkspaceRemovalPreparation, error) {
	if p == nil {
		return nil, errors.New("store: workspace database pool is required")
	}
	workspaceID, err := normalizeWorkspaceID(workspaceID)
	if err != nil {
		return nil, err
	}
	if err := p.waitForWorkspaceOpening(ctx, workspaceID); err != nil {
		return nil, err
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errWorkspacePoolClosed
	}
	if _, removing := p.removing[workspaceID]; removing {
		p.mu.Unlock()
		return nil, workspaceRemovalPendingError(workspaceID)
	}
	db := p.entries[workspaceID]
	p.mu.Unlock()

	path := ""
	if db != nil {
		path = db.Path()
	} else {
		rootDir := durableRoot
		if rootDir == "" {
			resolved, resolveErr := p.resolveRoot(ctx, workspaceID)
			if resolveErr != nil {
				return nil, fmt.Errorf(
					"store: resolve workspace database root %q for removal: %w",
					workspaceID,
					resolveErr,
				)
			}
			rootDir = strings.TrimSpace(resolved.RootDir)
		}
		if rootDir == "" {
			return nil, fmt.Errorf("store: resolved workspace database root %q is incomplete", workspaceID)
		}
		path = filepath.Join(rootDir, compozyconfig.DirName, store.GlobalDatabaseName)
	}
	return &WorkspaceRemovalPreparation{pool: p, workspaceID: workspaceID, path: path}, nil
}

// BeforeDelete seals this workspace against new database admission.
func (p *WorkspaceRemovalPreparation) BeforeDelete(context.Context) error {
	if p == nil || p.pool == nil {
		return errors.New("store: workspace database removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return errors.New("store: workspace database removal preparation was rolled back")
	}
	if p.staged {
		return nil
	}
	p.pool.mu.Lock()
	defer p.pool.mu.Unlock()
	if p.pool.closed {
		return errWorkspacePoolClosed
	}
	if _, removing := p.pool.removing[p.workspaceID]; removing {
		return workspaceRemovalPendingError(p.workspaceID)
	}
	p.pool.removing[p.workspaceID] = p
	p.staged = true
	return nil
}

// Commit closes the detached handle and removes its SQLite files. It is retry-safe.
func (p *WorkspaceRemovalPreparation) Commit(ctx context.Context) error {
	if p == nil || p.pool == nil {
		return errors.New("store: workspace database removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed {
		return nil
	}
	if p.rolledBack {
		return errors.New("store: workspace database removal preparation was rolled back")
	}
	if !p.staged {
		return errors.New("store: workspace database removal was not staged before commit")
	}
	if !p.removalStarted {
		p.pool.mu.Lock()
		if p.pool.closed {
			p.pool.mu.Unlock()
			return errWorkspacePoolClosed
		}
		p.db = p.pool.entries[p.workspaceID]
		delete(p.pool.entries, p.workspaceID)
		p.pool.mu.Unlock()
		p.removalStarted = true
	}
	if err := p.closeDatabase(ctx); err != nil {
		return err
	}
	if err := removeSQLiteFiles(p.path); err != nil {
		return err
	}
	p.pool.mu.Lock()
	delete(p.pool.removing, p.workspaceID)
	p.pool.mu.Unlock()
	p.committed = true
	return nil
}

func (p *WorkspaceRemovalPreparation) closeDatabase(ctx context.Context) error {
	if p.db == nil || p.detachedClosed {
		return nil
	}
	if err := p.db.Close(ctx); err != nil {
		return err
	}
	p.detachedClosed = true
	return nil
}

func (p *WorkspaceRemovalPreparation) closeDetached(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeDatabase(ctx)
}

// Rollback releases the admission seal without touching the database or files.
func (p *WorkspaceRemovalPreparation) Rollback(context.Context) error {
	if p == nil || p.pool == nil {
		return errors.New("store: workspace database removal preparation is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.committed || p.rolledBack {
		return nil
	}
	if p.removalStarted {
		return errors.New("store: workspace database removal cannot roll back after commit started")
	}
	p.pool.mu.Lock()
	delete(p.pool.removing, p.workspaceID)
	p.pool.mu.Unlock()
	p.rolledBack = true
	return nil
}
