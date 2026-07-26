// Package workspacedb owns per-workspace SQLite database lifecycle helpers.
package workspacedb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/compozy/agh/internal/store"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

// DB is an open per-workspace AGH database handle.
type DB struct {
	db            *sql.DB
	path          string
	workspaceRoot string
	identity      aghworkspace.Identity
	closed        atomic.Int32
}

// Options configures a per-workspace database open.
type Options struct {
	WorkspaceRoot string
}

// Open resolves the workspace identity and opens <workspace>/.agh/agh.db.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if ctx == nil {
		return nil, errors.New("store: open workspace database context is required")
	}
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil, errors.New("store: workspace root is required")
	}

	identity, err := aghworkspace.EnsureIdentity(ctx, workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("store: resolve workspace identity for %q: %w", workspaceRoot, err)
	}
	dbPath := filepath.Join(filepath.Dir(identity.Path), store.GlobalDatabaseName)
	db, err := store.OpenSQLiteDatabase(ctx, dbPath, func(ctx context.Context, db *sql.DB) error {
		return store.Apply(ctx, db, MigrationStream())
	})
	if err != nil {
		return nil, fmt.Errorf("store: open workspace database %q: %w", dbPath, err)
	}

	return &DB{
		db:            db,
		path:          dbPath,
		workspaceRoot: workspaceRoot,
		identity:      identity,
	}, nil
}

// OpenWorkspace opens a workspace database using its package-owned schema stream.
func OpenWorkspace(ctx context.Context, workspaceRoot string) (*DB, error) {
	return Open(ctx, Options{WorkspaceRoot: workspaceRoot})
}

// Path reports the database path.
func (d *DB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// WorkspaceID reports the resolved workspace identity.
func (d *DB) WorkspaceID() string {
	if d == nil {
		return ""
	}
	return d.identity.WorkspaceID
}

// WorkspaceRoot reports the workspace root used to open the database.
func (d *DB) WorkspaceRoot() string {
	if d == nil {
		return ""
	}
	return d.workspaceRoot
}

// DB exposes the underlying SQL handle for storage packages.
func (d *DB) DB() *sql.DB {
	if d == nil {
		return nil
	}
	return d.db
}

// Close checkpoints the WAL and closes the database.
func (d *DB) Close(ctx context.Context) error {
	if d == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close workspace database context is required")
	}
	if !d.closed.CompareAndSwap(0, 1) {
		return nil
	}

	checkpointErr := store.Checkpoint(ctx, d.db)
	closeErr := d.db.Close()
	return errors.Join(checkpointErr, closeErr)
}
