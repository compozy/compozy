package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

// defaultReadStatementCacheLimit bounds the distinct dynamic read shapes kept prepared.
// Catalog builders emit placeholders only, so the shape count stays small.
const defaultReadStatementCacheLimit = 64

// defaultStaticReadStatementCacheLimit bounds sqlc's static read statements. It stays well above
// the about 500 generated single- and multi-row queries, of which the read pool issues a subset,
// and expanded slice lists are never cached, so every static read stays prepared once used. A
// statement is prepared only on the connections that run it. TestStaticReadStatementReuse fails
// when the generated query count outgrows the bound.
const defaultStaticReadStatementCacheLimit = 1024

// readStatementCache reuses prepared read-pool statements for structural dynamic SQL.
// database/sql re-prepares a cached statement lazily on each pooled connection, so
// repeated catalog reads skip SQLite's parser after the first use per connection.
type readStatementCache struct {
	db    *sql.DB
	limit int
	// use is held for reading while a cached statement starts a query and for writing while
	// close releases the statements, so no read starts on a statement close already released.
	use sync.RWMutex
	// closing turns new reads to the unprepared path while close waits for running ones.
	closing atomic.Bool
	mu      sync.Mutex
	closed  bool
	stmts   map[string]*sql.Stmt
}

func newReadStatementCache(db *sql.DB, limit int) *readStatementCache {
	return &readStatementCache{db: db, limit: limit, stmts: make(map[string]*sql.Stmt)}
}

// query runs a read through a cached statement; shapes beyond the bound and reads
// after close fall back to an unprepared query so callers see the pool's own errors.
func (c *readStatementCache) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	var rows *sql.Rows
	var queryErr error
	ran, err := c.run(ctx, query, func(stmt *sql.Stmt) { rows, queryErr = stmt.QueryContext(ctx, args...) })
	if err != nil {
		return nil, err
	}
	if !ran {
		return c.db.QueryContext(ctx, query, args...)
	}
	return rows, queryErr
}

// run starts fn on the cached statement for query and reports whether it did. It reports false
// without calling fn when the read must run unprepared: beyond the bound or after close.
func (c *readStatementCache) run(ctx context.Context, query string, fn func(*sql.Stmt)) (bool, error) {
	stmt, err := c.statement(ctx, query)
	if err != nil || stmt == nil || c.closing.Load() {
		return false, err
	}
	c.use.RLock()
	defer c.use.RUnlock()
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return false, nil
	}
	fn(stmt)
	return true, nil
}

func (c *readStatementCache) statement(ctx context.Context, query string) (*sql.Stmt, error) {
	c.mu.Lock()
	if stmt, ok := c.stmts[query]; ok || c.closed || len(c.stmts) >= c.limit {
		c.mu.Unlock()
		return stmt, nil
	}
	c.mu.Unlock()

	// Prepare outside the lock so one slow parse does not serialize unrelated shapes.
	prepared, err := c.db.PrepareContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("store: prepare cached read statement: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if stmt, ok := c.stmts[query]; ok {
		// The losing duplicate is never used, so its close error cannot affect this read.
		_ = prepared.Close()
		return stmt, nil
	}
	if c.closed || len(c.stmts) >= c.limit {
		// A nil statement sends the caller to the unprepared fallback.
		if err := prepared.Close(); err != nil {
			return nil, fmt.Errorf("store: release uncached read statement: %w", err)
		}
		return nil, nil
	}
	c.stmts[query] = prepared
	return prepared, nil
}

func (c *readStatementCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.stmts)
}

// closeReadPool closes the pool before its cached statements. Closing the pool fails reads
// still queued for a connection, and those hold the statement close lock, so releasing the
// statements afterwards never waits for a connection to free up during shutdown. A read
// already running keeps its statement until it ends, so ctx bounds that wait.
func closeReadPool(ctx context.Context, db *sql.DB, caches ...*readStatementCache) error {
	err := db.Close()
	released := make(chan error, 1)
	go func() {
		var closeErr error
		for _, cache := range caches {
			if cache != nil {
				closeErr = errors.Join(closeErr, cache.close())
			}
		}
		released <- closeErr
	}()
	var closeErr error
	select {
	case closeErr = <-released:
	case <-ctx.Done():
		select {
		case closeErr = <-released:
		default:
			// The release goroutine finishes once the running reads end.
			closeErr = ctx.Err()
		}
	}
	if closeErr != nil {
		err = errors.Join(err, fmt.Errorf("store: close cached read statements: %w", closeErr))
	}
	return err
}

// close releases every cached statement; later reads use the unprepared fallback.
func (c *readStatementCache) close() error {
	c.closing.Store(true)
	c.use.Lock()
	defer c.use.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	var err error
	for query, stmt := range c.stmts {
		if closeErr := stmt.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close cached read statement: %w", closeErr))
		}
		delete(c.stmts, query)
	}
	return err
}

// statementReader is the sqlc DBTX for static reads on the read pool. Each query text runs
// through one cached statement, so supervision passes and other repeated reads skip SQLite's
// parser after the first use per connection. A read that cannot be cached, or whose
// preparation fails, runs unprepared on the pool, which reports a deterministic failure such
// as invalid SQL exactly as an uncached read would.
type statementReader struct {
	db    *sql.DB
	cache *readStatementCache
}

func (r statementReader) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

func (r statementReader) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return r.db.PrepareContext(ctx, query)
}

func (r statementReader) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	var rows *sql.Rows
	var err error
	if r.run(ctx, query, func(stmt *sql.Stmt) { rows, err = stmt.QueryContext(ctx, args...) }) {
		return rows, err
	}
	return r.db.QueryContext(ctx, query, args...)
}

func (r statementReader) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	var row *sql.Row
	if r.run(ctx, query, func(stmt *sql.Stmt) { row = stmt.QueryRowContext(ctx, args...) }) {
		return row
	}
	return r.db.QueryRowContext(ctx, query, args...)
}

// run reports false when the read must run unprepared. sqlc expands a slice argument into one
// placeholder per element, so each length would claim its own cache slot; those texts stay
// unprepared. A preparation error recurs on the unprepared query, which reports it.
func (r statementReader) run(ctx context.Context, query string, fn func(*sql.Stmt)) bool {
	if r.cache == nil || strings.Contains(query, "?,?") {
		return false
	}
	ran, err := r.cache.run(ctx, query, fn)
	return err == nil && ran
}
