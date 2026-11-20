package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

// defaultReadStatementCacheLimit bounds the distinct dynamic read shapes kept prepared.
// Catalog builders emit placeholders only, so the shape count stays small.
const defaultReadStatementCacheLimit = 64

// readStatementCache reuses prepared read-pool statements for structural dynamic SQL.
// database/sql re-prepares a cached statement lazily on each pooled connection, so
// repeated catalog reads skip SQLite's parser after the first use per connection.
type readStatementCache struct {
	db     *sql.DB
	limit  int
	mu     sync.Mutex
	closed bool
	stmts  map[string]*sql.Stmt
}

func newReadStatementCache(db *sql.DB, limit int) *readStatementCache {
	return &readStatementCache{db: db, limit: limit, stmts: make(map[string]*sql.Stmt)}
}

// query runs a read through a cached statement; shapes beyond the bound and reads
// after close fall back to an unprepared query so callers see the pool's own errors.
func (c *readStatementCache) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	stmt, err := c.statement(ctx, query)
	if err != nil {
		return nil, err
	}
	if stmt == nil {
		return c.db.QueryContext(ctx, query, args...)
	}
	return stmt.QueryContext(ctx, args...)
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
// statements afterwards never waits for a connection to free up during shutdown.
func closeReadPool(db *sql.DB, cache *readStatementCache) error {
	err := db.Close()
	if cache == nil {
		return err
	}
	if closeErr := cache.close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("store: close cached read statements: %w", closeErr))
	}
	return err
}

// close releases every cached statement; later reads use the unprepared fallback.
func (c *readStatementCache) close() error {
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
