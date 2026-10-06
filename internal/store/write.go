package store

import (
	"context"
	cryptorand "crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	defaultWriteOperation         = "sqlite_write"
	defaultWriteMaxAttempts       = 15
	defaultWriteMinRetryDelay     = 20 * time.Millisecond
	defaultWriteMaxRetryDelay     = 150 * time.Millisecond
	defaultWriteRollbackTimeout   = 5 * time.Second
	sqlitePrimaryResultCodeMask   = 0xff
	sqliteBeginImmediateStatement = "BEGIN IMMEDIATE"
	sqliteCommitStatement         = "COMMIT"
	sqliteRollbackStatement       = "ROLLBACK"
)

var errWriteReentry = errors.New("store: nested write transaction on the same database is not supported")

type writeContextKey struct{}

type writeContextOwner struct {
	db     *sql.DB
	parent *writeContextOwner
	active atomic.Bool
}

// WriteTx is the single-connection transaction handle passed to ExecuteWrite callbacks.
type WriteTx struct {
	conn *sql.Conn
}

// ExecContext executes a statement inside the active write transaction.
func (tx *WriteTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if tx == nil || tx.conn == nil {
		return nil, errors.New("store: write transaction is closed")
	}
	return tx.conn.ExecContext(ctx, query, args...)
}

// PrepareContext prepares a statement inside the active write transaction.
func (tx *WriteTx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if tx == nil || tx.conn == nil {
		return nil, errors.New("store: write transaction is closed")
	}
	return tx.conn.PrepareContext(ctx, query)
}

// QueryContext executes a query inside the active write transaction.
func (tx *WriteTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if tx == nil || tx.conn == nil {
		return nil, errors.New("store: write transaction is closed")
	}
	return tx.conn.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a single-row query inside the active write transaction.
func (tx *WriteTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return tx.conn.QueryRowContext(ctx, query, args...)
}

// ExecuteWrite runs bounded immediate writes; callbacks propagate ctx and reuse WriteTx for the same database.
func ExecuteWrite(ctx context.Context, db *sql.DB, fn func(context.Context, *WriteTx) error) error {
	cfg := defaultExecuteWriteConfig()
	cfg.operation = writeCallerOperation()
	return executeWrite(ctx, db, cfg, fn)
}

type executeWriteConfig struct {
	operation     string
	maxAttempts   int
	minRetryDelay time.Duration
	maxRetryDelay time.Duration
	jitter        func(time.Duration, time.Duration) time.Duration
}

func defaultExecuteWriteConfig() executeWriteConfig {
	return executeWriteConfig{
		maxAttempts:   defaultWriteMaxAttempts,
		minRetryDelay: defaultWriteMinRetryDelay,
		maxRetryDelay: defaultWriteMaxRetryDelay,
		jitter:        randomWriteRetryDelay,
	}
}

func executeWrite(
	ctx context.Context,
	db *sql.DB,
	cfg executeWriteConfig,
	fn func(context.Context, *WriteTx) error,
) error {
	if ctx == nil {
		return errors.New("store: execute write context is required")
	}
	if db == nil {
		return errors.New("store: execute write database is required")
	}
	if fn == nil {
		return errors.New("store: execute write callback is required")
	}
	parent, ok := ctx.Value(writeContextKey{}).(*writeContextOwner)
	if !ok {
		parent = nil
	}
	for owner := parent; owner != nil; owner = owner.parent {
		if owner.db == db && owner.active.Load() {
			return errWriteReentry
		}
	}
	cfg = normalizeExecuteWriteConfig(cfg)

	admission, unregister := registerWriteAdmission(db)
	defer unregister()
	started := time.Now()
	releaseAdmission, err := admission.acquire(ctx)
	if err != nil {
		return writeContentionError(db, cfg.operation, 0, started, err)
	}
	defer releaseAdmission()
	owner := &writeContextOwner{db: db, parent: parent}
	owner.active.Store(true)
	defer owner.active.Store(false)
	ctx = context.WithValue(ctx, writeContextKey{}, owner)
	var lastErr error
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		err := executeWriteAttempt(ctx, db, cfg.operation, fn)
		if err == nil {
			return nil
		}
		lastErr = err
		if !IsSQLiteBusy(err) {
			return err
		}
		if attempt == cfg.maxAttempts {
			return writeContentionError(db, cfg.operation, attempt, started, err)
		}
		if waitErr := waitForWriteRetry(ctx, cfg.jitter(cfg.minRetryDelay, cfg.maxRetryDelay)); waitErr != nil {
			return writeContentionError(db, cfg.operation, attempt, started, errors.Join(err, waitErr))
		}
	}

	return lastErr
}

func normalizeExecuteWriteConfig(cfg executeWriteConfig) executeWriteConfig {
	defaults := defaultExecuteWriteConfig()
	if cfg.operation == "" {
		cfg.operation = defaultWriteOperation
	}
	if cfg.maxAttempts <= 0 {
		cfg.maxAttempts = defaults.maxAttempts
	}
	if cfg.minRetryDelay <= 0 {
		cfg.minRetryDelay = defaults.minRetryDelay
	}
	if cfg.maxRetryDelay < cfg.minRetryDelay {
		cfg.maxRetryDelay = cfg.minRetryDelay
	}
	if cfg.jitter == nil {
		cfg.jitter = defaults.jitter
	}
	return cfg
}

func executeWriteAttempt(
	ctx context.Context,
	db *sql.DB,
	operation string,
	fn func(context.Context, *WriteTx) error,
) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("store: acquire sqlite write connection: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close sqlite write connection: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()

	if _, err := conn.ExecContext(ctx, sqliteBeginImmediateStatement); err != nil {
		return fmt.Errorf("store: begin immediate sqlite write: %w", err)
	}
	releaseOwner := trackWriteOwner(db, operation)
	defer releaseOwner()
	active := true
	defer func() {
		if !active {
			return
		}
		if rollbackErr := rollbackWriteTx(context.WithoutCancel(ctx), conn); rollbackErr != nil {
			if err == nil {
				err = rollbackErr
				return
			}
			err = errors.Join(err, rollbackErr)
		}
	}()

	tx := &WriteTx{conn: conn}
	if err := fn(ctx, tx); err != nil {
		return fmt.Errorf("store: execute sqlite write callback: %w", err)
	}
	if err := validateMutationCommitFence(ctx); err != nil {
		return fmt.Errorf("store: mutation commit fence: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("store: commit sqlite write: %w", err)
	}
	// Once commit starts, cancellation must not disguise a durable write as retryable failure.
	if _, err := conn.ExecContext(context.WithoutCancel(ctx), sqliteCommitStatement); err != nil {
		return fmt.Errorf("store: commit sqlite write: %w", err)
	}
	active = false
	return nil
}

func rollbackWriteTx(ctx context.Context, conn *sql.Conn) error {
	rollbackCtx, cancel := context.WithTimeout(ctx, defaultWriteRollbackTimeout)
	defer cancel()
	if _, err := conn.ExecContext(rollbackCtx, sqliteRollbackStatement); err != nil {
		return fmt.Errorf("store: rollback sqlite write: %w", err)
	}
	return nil
}

func waitForWriteRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("store: wait for sqlite write retry: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func randomWriteRetryDelay(minDelay time.Duration, maxDelay time.Duration) time.Duration {
	if maxDelay <= minDelay {
		return minDelay
	}
	span := maxDelay - minDelay
	offset, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(span)+1))
	if err != nil {
		return minDelay
	}
	return minDelay + time.Duration(offset.Int64())
}

// IsSQLiteBusy reports whether err is a SQLite BUSY or LOCKED condition.
func IsSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok {
		return false
	}
	code := sqliteErr.Code() & sqlitePrimaryResultCodeMask
	return code == sqlite3.SQLITE_BUSY || code == sqlite3.SQLITE_LOCKED
}

// IsSQLiteIdentityConstraint reports whether err is a primary-key or unique-key collision.
func IsSQLiteIdentityConstraint(err error) bool {
	if err == nil {
		return false
	}
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok || sqliteErr == nil {
		return false
	}
	return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY ||
		sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
