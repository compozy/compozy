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

	"github.com/compozy/compozy/internal/store"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

// DB is an open per-workspace Compozy database handle.
type DB struct {
	db            *sql.DB
	path          string
	workspaceRoot string
	identity      compozyworkspace.Identity
	closed        atomic.Int32
}

// Options configures a per-workspace database open.
type Options struct {
	HomeDir       string
	WorkspaceID   string
	WorkspaceRoot string
}

// Open resolves the workspace identity and opens <home>/workspaces/<registration-id>/compozy.db.
func Open(ctx context.Context, opts Options) (*DB, error) {
	if ctx == nil {
		return nil, errors.New("store: open workspace database context is required")
	}
	workspaceRoot := strings.TrimSpace(opts.WorkspaceRoot)
	if workspaceRoot == "" {
		return nil, errors.New("store: workspace root is required")
	}
	dbPath, err := workspaceDatabasePath(opts.HomeDir, opts.WorkspaceID)
	if err != nil {
		return nil, err
	}

	identity := compozyworkspace.Identity{WorkspaceID: opts.WorkspaceID}
	if !strings.HasPrefix(opts.WorkspaceID, "user-") || len(opts.WorkspaceID) <= len("user-") {
		identity, err = compozyworkspace.EnsureIdentity(ctx, workspaceRoot)
	}
	if err != nil {
		return nil, fmt.Errorf("store: resolve workspace identity for %q: %w", workspaceRoot, err)
	}
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

func workspaceDatabasePath(homeDir, workspaceID string) (string, error) {
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" || !filepath.IsAbs(homeDir) {
		return "", errors.New("store: absolute home directory is required")
	}
	workspaceID, err := normalizeWorkspaceID(workspaceID)
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "workspaces", workspaceID, store.GlobalDatabaseName), nil
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

	// Reader-pinned WAL frames must not block lifecycle close.
	checkpointErr := store.CheckpointPassive(ctx, d.db)
	closeErr := d.db.Close()
	return errors.Join(checkpointErr, closeErr)
}
