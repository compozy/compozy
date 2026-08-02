package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrSessionDBOwnerMissing reports an events.db without immutable owner evidence.
	ErrSessionDBOwnerMissing = errors.New("store: session database owner is missing")
	// ErrSessionDBOwnerMismatch reports an events.db owned by another session or workspace.
	ErrSessionDBOwnerMismatch = errors.New("store: session database owner does not match")
	// ErrSessionDBFamilyChanged reports that events.db changed identity during an owned open.
	ErrSessionDBFamilyChanged = errors.New("store: session database family changed during owned open")
	// errSessionDBIdentityMissing reports a pre-identity SessionDB that still needs the append-only migration.
	errSessionDBIdentityMissing = errors.New("store: session database identity is missing")
)

func migrationStreamForOwner(owner store.SessionDBOwner) store.MigrationStream {
	stream := MigrationStream()
	bootstrap := *stream.Bootstrap
	initialize := bootstrap.Initialize
	bootstrap.Initialize = func(ctx context.Context, tx *sql.Tx) error {
		// dynamic-sql: the owner values are instance-specific bootstrap state,
		// so they cannot be part of the static sqlc schema query set.
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO session_db_owner (singleton, session_id, workspace_id) VALUES (1, ?, ?)`,
			owner.SessionID,
			owner.WorkspaceID,
		); err != nil {
			return fmt.Errorf("store: stamp session database owner: %w", err)
		}
		if initialize == nil {
			return errors.New("store: session database bootstrap identity initializer is required")
		}
		if err := initialize(ctx, tx); err != nil {
			return err
		}
		return nil
	}
	stream.Bootstrap = &bootstrap
	return stream
}

func verifySessionDBOwner(ctx context.Context, db *sql.DB, expected store.SessionDBOwner) error {
	if ctx == nil {
		return errors.New("store: verify session database owner context is required")
	}
	if db == nil {
		return errors.New("store: verify session database owner database is required")
	}

	var tableCount int
	// dynamic-sql: sqlite_master inspection guards the immutable database
	// identity before any migration or writable initialization may run.
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'session_db_owner'`,
	).Scan(&tableCount); err != nil {
		return fmt.Errorf("store: inspect session database owner table: %w", err)
	}
	if tableCount != 1 {
		return ErrSessionDBOwnerMissing
	}

	var actual store.SessionDBOwner
	// dynamic-sql: this identity guard intentionally runs before sqlc-backed
	// repositories because it controls whether the database may be opened.
	err := db.QueryRowContext(
		ctx,
		`SELECT session_id, workspace_id FROM session_db_owner WHERE singleton = 1`,
	).Scan(&actual.SessionID, &actual.WorkspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionDBOwnerMissing
	}
	if err != nil {
		return fmt.Errorf("store: read session database owner: %w", err)
	}
	if actual != expected {
		return fmt.Errorf(
			"%w: expected session %q workspace %q, found session %q workspace %q",
			ErrSessionDBOwnerMismatch,
			expected.SessionID,
			expected.WorkspaceID,
			actual.SessionID,
			actual.WorkspaceID,
		)
	}
	return nil
}
