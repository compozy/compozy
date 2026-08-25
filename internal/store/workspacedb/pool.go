package workspacedb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// RootResolver resolves a canonical workspace id to its filesystem root.
type RootResolver func(ctx context.Context, workspaceID string) (string, error)

// Pool owns lazy, reusable database handles for daemon-known workspaces.
type Pool struct {
	resolveRoot RootResolver

	mu      sync.Mutex
	entries map[string]*DB
	closed  bool
}

// NewPool constructs an empty per-workspace database pool.
func NewPool(resolveRoot RootResolver) (*Pool, error) {
	if resolveRoot == nil {
		return nil, errors.New("store: workspace database root resolver is required")
	}
	return &Pool{resolveRoot: resolveRoot, entries: make(map[string]*DB)}, nil
}

// Open returns the shared database for workspaceID, opening it on first use.
func (p *Pool) Open(ctx context.Context, workspaceID string) (*DB, error) {
	if p == nil {
		return nil, errors.New("store: workspace database pool is required")
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("store: workspace id is required")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, errors.New("store: workspace database pool is closed")
	}
	if existing := p.entries[workspaceID]; existing != nil {
		return existing, nil
	}
	root, err := p.resolveRoot(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("store: resolve workspace database root %q: %w", workspaceID, err)
	}
	db, err := OpenWorkspace(ctx, root)
	if err != nil {
		return nil, err
	}
	if db.WorkspaceID() != workspaceID {
		closeErr := db.Close(context.WithoutCancel(ctx))
		return nil, errors.Join(
			fmt.Errorf("store: workspace database identity %q does not match requested %q", db.WorkspaceID(), workspaceID),
			closeErr,
		)
	}
	p.entries[workspaceID] = db
	return db, nil
}

// CloseWorkspace closes and forgets one workspace handle.
func (p *Pool) CloseWorkspace(ctx context.Context, workspaceID string) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	db := p.entries[strings.TrimSpace(workspaceID)]
	delete(p.entries, strings.TrimSpace(workspaceID))
	p.mu.Unlock()
	if db == nil {
		return nil
	}
	return db.Close(ctx)
}

// RemoveWorkspace closes one workspace database and removes its SQLite files.
func (p *Pool) RemoveWorkspace(ctx context.Context, workspaceID string) error {
	if p == nil {
		return nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	p.mu.Lock()
	db := p.entries[workspaceID]
	delete(p.entries, workspaceID)
	p.mu.Unlock()
	if db != nil {
		path := db.Path()
		closeErr := db.Close(ctx)
		removeErr := removeSQLiteFiles(path)
		return errors.Join(closeErr, removeErr)
	}
	root, err := p.resolveRoot(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("store: resolve workspace database root %q for removal: %w", workspaceID, err)
	}
	path := filepath.Join(root, compozyconfig.DirName, store.GlobalDatabaseName)
	return removeSQLiteFiles(path)
}

// Close closes every open workspace database exactly once.
func (p *Pool) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	entries := p.entries
	p.entries = make(map[string]*DB)
	p.mu.Unlock()
	errs := make([]error, 0, len(entries))
	for workspaceID, db := range entries {
		if err := db.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("store: close workspace database %q: %w", workspaceID, err))
		}
	}
	return errors.Join(errs...)
}

func removeSQLiteFiles(path string) error {
	var errs []error
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(candidate); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("store: remove workspace database file %q: %w", candidate, err))
		}
	}
	return errors.Join(errs...)
}
