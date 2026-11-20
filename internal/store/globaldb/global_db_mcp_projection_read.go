package globaldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const defaultMCPProjectionTimeout = 100 * time.Millisecond

var restoreMCPProjectionBusyTimeout = fmt.Sprintf("PRAGMA busy_timeout = %d", store.DefaultSQLiteBusyTimeoutMS)

type mcpProjectionReaderKey struct{}

type mcpProjectionReader struct {
	pool    *sql.DB
	queries *sqlcgen.Queries
}

// WithMCPProjectionTimeout returns a repository with a custom observation budget.
func (r *InstanceRepo) WithMCPProjectionTimeout(timeout time.Duration) *InstanceRepo {
	repo := *r
	repo.mcpProjectionTimeout = timeout
	return &repo
}

func (r *InstanceRepo) MCPProjectionObservationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, r.mcpProjectionTimeout)
}

// MCPProjectionObservation uses one connection with lock waits disabled for bounded policy reads.
func (r *InstanceRepo) MCPProjectionObservation(ctx context.Context) (context.Context, func(), error) {
	if err := r.checkReady(ctx, "observe MCP projection"); err != nil {
		return ctx, nil, err
	}
	conn, release, err := mcpProjectionConnection(ctx, r.readDB)
	if err != nil {
		return ctx, nil, err
	}
	reader := mcpProjectionReader{pool: r.readDB, queries: sqlcgen.New(conn)}
	return context.WithValue(ctx, mcpProjectionReaderKey{}, reader), release, nil
}

func (r *repoBase) mcpProjectionQueries(ctx context.Context) *sqlcgen.Queries {
	if reader, ok := ctx.Value(mcpProjectionReaderKey{}).(mcpProjectionReader); ok && reader.pool == r.readDB {
		return reader.queries
	}
	return r.readQueries
}

func mcpProjectionConnection(ctx context.Context, pool *sql.DB) (*sql.Conn, func(), error) {
	conn, err := pool.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("store: acquire MCP observation connection: %w", err)
	}
	release := func() {
		restore, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultMCPProjectionTimeout)
		defer cancel()
		if _, err := conn.ExecContext(restore, restoreMCPProjectionBusyTimeout); err != nil {
			discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
			if discardErr != nil && !errors.Is(discardErr, driver.ErrBadConn) &&
				!errors.Is(discardErr, sql.ErrConnDone) {
				slog.Warn("store: discard MCP observation connection", "error", discardErr)
			}
		}
		if err := conn.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			slog.Warn("store: close MCP observation connection", "error", err)
		}
	}
	// SQLite's native busy handler can outlive a Go deadline. Cache observations
	// fail fast on a database lock and defer to the established uncached path.
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		release()
		return nil, nil, fmt.Errorf("store: disable MCP observation lock wait: %w", err)
	}
	return conn, release, nil
}
