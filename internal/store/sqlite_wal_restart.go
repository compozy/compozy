package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type walRestartGuard struct {
	readDB       *sql.DB
	snapshot     *sql.Tx
	lifetime     context.Context
	pinnedFrames int
	misses       int
	primed       bool
}

const walRestartFrames = 1000

// openWALRestartGuard keeps read marks independent of the database-open request's lifetime.
func openWALRestartGuard(ctx, lifetime context.Context, db *sql.DB, path string) (*walRestartGuard, error) {
	readDB, err := OpenSQLiteReadPool(ctx, path)
	if err != nil {
		return nil, err
	}
	readDB.SetMaxOpenConns(2)
	readDB.SetMaxIdleConns(2)
	guard := &walRestartGuard{readDB: readDB, lifetime: lifetime}
	if err := guard.checkpoint(ctx, db); err != nil && !IsSQLiteBusy(err) {
		return nil, errors.Join(err, guard.close())
	}
	return guard, nil
}

// checkpoint advances the read mark so ongoing writes cannot pin old WAL frames indefinitely.
func (g *walRestartGuard) checkpoint(ctx context.Context, db *sql.DB) error {
	frames, copied, err := walCheckpointProgress(ctx, db)
	if err != nil {
		return err
	}
	if copied < frames {
		if g.snapshot == nil || frames > g.pinnedFrames {
			if err := g.pin(ctx, frames); err != nil {
				return err
			}
		}
		if err := CheckpointPassive(ctx, db); err != nil {
			return err
		}
		_, copied, err = walCheckpointProgress(ctx, db)
		if err != nil {
			return err
		}
	}
	if g.primed && (frames < walRestartFrames || copied < frames) {
		return nil
	}
	return g.restart(ctx, db)
}

// walCheckpointProgress inspects the WAL without triggering checkpoint I/O.
func walCheckpointProgress(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (frames, copied int, err error) {
	var busy int
	err = db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(NOOP)").Scan(&busy, &frames, &copied)
	return frames, copied, err
}

// admit reserves an eventual writer slot so the retained reader cannot starve WAL reuse.
func (g *walRestartGuard) admit(ctx context.Context, admission *writeAdmission) (func(), error) {
	if g.misses >= 3 {
		return admission.acquireMaintenance(ctx)
	}
	admitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	return admission.acquire(admitCtx, true)
}

// restart excludes application writers until the WAL header is safe to append.
func (g *walRestartGuard) restart(ctx context.Context, db *sql.DB) (retErr error) {
	admission, unregister := registerWriteAdmission(db)
	defer unregister()
	release, err := g.admit(ctx, admission)
	if err != nil {
		if ctx.Err() == nil && errors.Is(err, context.DeadlineExceeded) {
			g.misses++
			return nil
		}
		return err
	}
	defer release()
	g.misses = 0
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire WAL restart connection: %w", err)
	}
	defer func() { retErr = errors.Join(retErr, conn.Close()) }()
	// Probe external contention before releasing the snapshot that protects writers.
	if err := beginSQLiteImmediate(ctx, conn); err != nil {
		return fmt.Errorf("store: begin WAL restart: %w", err)
	}
	if err := rollbackWriteTx(context.WithoutCancel(ctx), conn); err != nil {
		return discardSQLiteConnection(conn, err)
	}
	g.primed = false
	// Admission queues application writes before BEGIN while maintenance owns the sync.
	if err := g.releaseSnapshot(); err != nil {
		return err
	}
	if err := CheckpointPassiveConnection(ctx, conn); err != nil {
		return err
	}
	if err := beginWALRestart(ctx, conn); err != nil {
		return err
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
	frames, _, err := walCheckpointProgress(ctx, conn)
	if err != nil {
		return err
	}
	if err := g.pin(ctx, frames); err != nil {
		return err
	}
	g.primed = true
	return nil
}

// beginWALRestart retains admission across external-writer races after backfill.
func beginWALRestart(ctx context.Context, conn *sql.Conn) error {
	for {
		err := beginSQLiteImmediate(ctx, conn)
		if !IsSQLiteBusy(err) {
			return err
		}
		// Once backfilled, keep admission until priming is safe or the worker stops.
		if err := waitForWriteRetry(ctx, defaultWriteMinRetryDelay); err != nil {
			return err
		}
	}
}

// pin overlaps snapshots so advancing a read mark never exposes a backfilled WAL to writers.
func (g *walRestartGuard) pin(ctx context.Context, frames int) error {
	snapshot, err := g.readDB.BeginTx(g.lifetime, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("store: begin WAL restart snapshot: %w", err)
	}
	var version int
	if err := snapshot.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return errors.Join(fmt.Errorf("store: pin WAL restart snapshot: %w", err), snapshot.Rollback())
	}
	err = g.releaseSnapshot()
	g.snapshot, g.pinnedFrames = snapshot, frames
	return err
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
