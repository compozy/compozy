package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"sync"
)

// readStatementCacheLimit bounds the distinct dynamic read shapes kept prepared.
// Catalog builders emit placeholders only, so the shape count stays small.
const readStatementCacheLimit = 64

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
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if stmt, ok := c.stmts[query]; ok {
		return stmt, prepared.Close()
	}
	if c.closed || len(c.stmts) >= c.limit {
		// A nil statement sends the caller to the unprepared fallback.
		return nil, prepared.Close()
	}
	c.stmts[query] = prepared
	return prepared, nil
}

func (c *readStatementCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.stmts)
}

// close releases every cached statement; it must run before the read pool closes.
func (c *readStatementCache) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	var err error
	for query, stmt := range c.stmts {
		err = errors.Join(err, stmt.Close())
		delete(c.stmts, query)
	}
	return err
}
