package globaldb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

const mcpProjectionReadTimeout = 100 * time.Millisecond

var restoreMCPProjectionBusyTimeout = fmt.Sprintf("PRAGMA busy_timeout = %d", store.DefaultSQLiteBusyTimeoutMS)

type mcpProjectionReaderKey struct{}

type mcpProjectionReader struct {
	pool    *sql.DB
	queries *sqlcgen.Queries
}

// MCPProjectionObservation bounds cache observation reads without changing normal request timeouts.
func (r *InstanceRepo) MCPProjectionObservation(ctx context.Context) (context.Context, func(), error) {
	if err := r.checkReady(ctx, "observe MCP projection"); err != nil {
		return ctx, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, mcpProjectionReadTimeout)
	conn, release, err := mcpProjectionConnection(ctx, r.readDB)
	if err != nil {
		cancel()
		return ctx, nil, err
	}
	reader := mcpProjectionReader{pool: r.readDB, queries: sqlcgen.New(conn)}
	return context.WithValue(ctx, mcpProjectionReaderKey{}, reader), func() {
		cancel()
		release()
	}, nil
}

func (r *repoBase) mcpProjectionQueries(ctx context.Context) *sqlcgen.Queries {
	if reader, ok := ctx.Value(mcpProjectionReaderKey{}).(mcpProjectionReader); ok && reader.pool == r.readDB {
		return reader.queries
	}
	return r.readQueries
}

// BeginMCPProjectionRead detects every committed change during a cold build.
// SQLite's data_version is meaningful only on the same read-only connection.
// One pinned reader leaves room in the read pool for concurrent builds.
func (r *InstanceRepo) BeginMCPProjectionRead(ctx context.Context) func() bool {
	if r.checkReady(ctx, "fence MCP projection") != nil ||
		r.mcpProjectionRead == nil || r.readDB.Stats().MaxOpenConnections == 1 || !r.mcpProjectionRead.TryLock() {
		return nil
	}
	begin, cancel := context.WithTimeout(ctx, mcpProjectionReadTimeout)
	conn, release, err := mcpProjectionConnection(begin, r.readDB)
	if err != nil {
		cancel()
		r.mcpProjectionRead.Unlock()
		return nil
	}
	version := readMCPDataVersion(begin, conn)
	cancel()
	if version == 0 {
		release()
		r.mcpProjectionRead.Unlock()
		return nil
	}
	return sync.OnceValue(func() bool {
		defer r.mcpProjectionRead.Unlock()
		defer release()
		check, cancel := context.WithTimeout(ctx, mcpProjectionReadTimeout)
		defer cancel()
		return version == readMCPDataVersion(check, conn)
	})
}

func mcpProjectionConnection(ctx context.Context, pool *sql.DB) (*sql.Conn, func(), error) {
	conn, err := pool.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	release := func() {
		restore, cancel := context.WithTimeout(context.WithoutCancel(ctx), mcpProjectionReadTimeout)
		defer cancel()
		if _, err := conn.ExecContext(restore, restoreMCPProjectionBusyTimeout); err != nil {
			discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
			if discardErr != nil && !errors.Is(discardErr, driver.ErrBadConn) &&
				!errors.Is(discardErr, sql.ErrConnDone) {
				slog.Warn("store: discard MCP observation connection", "error", discardErr)
			}
		}
		_ = conn.Close()
	}
	// SQLite's native busy handler can outlive a Go deadline. Cache observations
	// fail fast on a database lock and defer to the established uncached path.
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		release()
		return nil, nil, err
	}
	return conn, release, nil
}

func readMCPDataVersion(ctx context.Context, conn *sql.Conn) int64 {
	var version int64
	if err := conn.QueryRowContext(ctx, "PRAGMA data_version").Scan(&version); err != nil {
		return 0
	}
	return version
}
