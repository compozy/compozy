package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

var errSQLiteCheckpointBusy = errors.New("store: sqlite wal checkpoint is busy")

// PassiveCheckpointer owns WAL maintenance and joins it before final shutdown.
type PassiveCheckpointer struct {
	cancel  context.CancelFunc
	done    chan struct{}
	err     error
	restart *walRestartGuard
}

// OpenPassiveCheckpointer starts periodic PASSIVE checkpoints on an existing
// database. PASSIVE never waits for readers or the WAL writer; unfinished work
// is retried on the next tick instead of extending a foreground COMMIT.
func OpenPassiveCheckpointer(ctx context.Context, path string) (*PassiveCheckpointer, error) {
	return openPassiveCheckpointer(ctx, path, false)
}

// OpenRestartCheckpointer also assigns WAL-header synchronization to maintenance.
func OpenRestartCheckpointer(ctx context.Context, path string) (*PassiveCheckpointer, error) {
	return openPassiveCheckpointer(ctx, path, true)
}

func openPassiveCheckpointer(ctx context.Context, path string, restart bool) (*PassiveCheckpointer, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("store: checkpoint database path is required")
	}
	db, err := openSQLiteHandle(ctx, cleanPath, sqliteCheckpointDSN(cleanPath))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	workerCtx, cancel := context.WithCancel(context.Background())
	worker := &PassiveCheckpointer{cancel: cancel, done: make(chan struct{})}
	if restart {
		worker.restart, err = openWALRestartGuard(ctx, workerCtx, db, cleanPath)
		if err != nil {
			cancel()
			return nil, errors.Join(err, db.Close())
		}
	}
	go worker.run(workerCtx, db)
	return worker, nil
}

func (p *PassiveCheckpointer) run(ctx context.Context, db *sql.DB) {
	defer func() {
		p.err = errors.Join(p.restart.close(), db.Close())
		close(p.done)
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var optimizedAt time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := p.checkpoint(ctx, db); err != nil && ctx.Err() == nil && !IsSQLiteBusy(err) {
				slog.Warn("sqlite passive checkpoint failed", "error", err)
			}
			if now.Sub(optimizedAt) >= time.Hour {
				if err := optimizeSQLite(ctx, db); err != nil {
					if ctx.Err() == nil && !IsSQLiteBusy(err) && !errors.Is(err, context.DeadlineExceeded) {
						slog.Warn("sqlite statistics maintenance failed", "error", err)
					}
				} else {
					optimizedAt = now
				}
			}
		}
	}
}

func optimizeSQLite(ctx context.Context, db *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	// Maintenance has no query history; consider all tables while retaining SQLite's analysis limit.
	if _, err := db.ExecContext(ctx, "PRAGMA optimize = 0x10012"); err != nil {
		return fmt.Errorf("store: optimize sqlite statistics: %w", err)
	}
	return nil
}

func (p *PassiveCheckpointer) checkpoint(ctx context.Context, db *sql.DB) error {
	if p.restart != nil {
		return p.restart.checkpoint(ctx, db)
	}
	return CheckpointPassive(ctx, db)
}

// Close cancels maintenance and waits until it exits or ctx ends.
func (p *PassiveCheckpointer) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.cancel()
	select {
	case <-p.done:
		return p.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CloseCheckpointedDatabase retains database ownership until maintenance has exited.
func CloseCheckpointedDatabase(ctx context.Context, db *sql.DB, worker *PassiveCheckpointer) error {
	done := make(chan error, 1)
	go func() {
		maintenanceErr := worker.Close(context.WithoutCancel(ctx))
		checkpointErr := CheckpointPassive(ctx, db)
		done <- errors.Join(maintenanceErr, checkpointErr, db.Close())
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Checkpoint truncates the WAL for an open SQLite database.
func Checkpoint(ctx context.Context, db *sql.DB) (retErr error) {
	if db == nil {
		return nil
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire sqlite checkpoint connection: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			retErr = errors.Join(retErr, fmt.Errorf("store: close sqlite checkpoint connection: %w", err))
		}
	}()
	return CheckpointTruncateConnection(ctx, conn)
}

// CheckpointTruncateConnection truncates the WAL on an already acquired connection, waiting for
// readers until ctx ends. A deadline bounds how long a live owner lets readers delay it.
func CheckpointTruncateConnection(ctx context.Context, conn *sql.Conn) error {
	if conn == nil {
		return nil
	}
	if err := executeSQLiteWithBusyWait(ctx, conn, func() error {
		var busy, frames, checkpointed int
		if err := conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").
			Scan(&busy, &frames, &checkpointed); err != nil {
			return err
		}
		if busy != 0 {
			return fmt.Errorf("%w: checkpointed %d of %d frames", errSQLiteCheckpointBusy, checkpointed, frames)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("store: checkpoint sqlite wal: %w", err)
	}
	return nil
}

// CheckpointPassive runs a non-truncating SQLite WAL checkpoint.
func CheckpointPassive(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	return passiveCheckpoint(ctx, db)
}

// CheckpointPassiveConnection runs a checkpoint on an already acquired connection.
// Owners can coordinate physical file mutation without opening a connection under their lease.
func CheckpointPassiveConnection(ctx context.Context, connection *sql.Conn) error {
	if connection == nil {
		return nil
	}
	return passiveCheckpoint(ctx, connection)
}

func passiveCheckpoint(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}) error {
	if _, err := executor.ExecContext(ctx, "PRAGMA wal_checkpoint(PASSIVE)"); err != nil {
		return fmt.Errorf("store: passive checkpoint sqlite wal: %w", err)
	}
	return nil
}
