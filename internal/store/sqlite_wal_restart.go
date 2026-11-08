package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type walRestartGuard struct {
	readDB   *sql.DB
	snapshot *sql.Tx
	lifetime context.Context
}

func openWALRestartGuard(ctx, lifetime context.Context, db *sql.DB, path string) (*walRestartGuard, error) {
	readDB, err := OpenSQLiteReadPool(ctx, path)
	if err != nil {
		return nil, err
	}
	readDB.SetMaxOpenConns(1)
	readDB.SetMaxIdleConns(1)
	guard := &walRestartGuard{readDB: readDB, lifetime: lifetime}
	if err := guard.checkpoint(ctx, db); err != nil {
		return nil, errors.Join(err, guard.close())
	}
	return guard, nil
}

func (g *walRestartGuard) checkpoint(ctx context.Context, db *sql.DB) (retErr error) {
	admission, unregister := registerWriteAdmission(db)
	defer unregister()
	admitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	release, err := admission.acquire(admitCtx, true)
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) && g.snapshot != nil {
			return CheckpointPassive(ctx, db)
		}
		return err
	}
	defer release()
	// Admission queues application writes before BEGIN while maintenance owns the sync.
	if err := g.releaseSnapshot(); err != nil {
		return err
	}
	if err := CheckpointPassive(ctx, db); err != nil {
		return err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire WAL restart connection: %w", err)
	}
	defer func() { retErr = errors.Join(retErr, conn.Close()) }()
	if _, err := conn.ExecContext(ctx, sqliteBeginImmediateStatement); err != nil {
		return fmt.Errorf("store: begin WAL restart: %w", err)
	}
	active := true
	defer func() {
		if active {
			if err := rollbackWriteTx(context.WithoutCancel(ctx), conn); err != nil {
				retErr = errors.Join(retErr, discardSQLiteConnection(conn, err))
			}
		}
	}()
	var version int
	if err := conn.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("store: read WAL restart marker: %w", err)
	}
	// Writing the unchanged header field primes the WAL without changing user state.
	if _, err := conn.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		return fmt.Errorf("store: prime WAL header: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), sqliteCommitStatement); err != nil {
		return fmt.Errorf("store: commit WAL restart: %w", err)
	}
	active = false
	// The new frame prevents another restart until a checkpoint; pin it before the next one.
	g.snapshot, err = g.readDB.BeginTx(g.lifetime, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("store: begin WAL restart snapshot: %w", err)
	}
	if err := g.snapshot.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return errors.Join(fmt.Errorf("store: pin WAL restart snapshot: %w", err), g.releaseSnapshot())
	}
	return nil
}

func (g *walRestartGuard) releaseSnapshot() error {
	if g.snapshot == nil {
		return nil
	}
	err := g.snapshot.Rollback()
	g.snapshot = nil
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		return fmt.Errorf("store: release WAL restart snapshot: %w", err)
	}
	return nil
}

func (g *walRestartGuard) close() error {
	if g == nil {
		return nil
	}
	return errors.Join(g.releaseSnapshot(), g.readDB.Close())
}
