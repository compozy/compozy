package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	"modernc.org/sqlite"
)

// SQLiteReadInitializer configures connection-local attachments and views. The
// database files remain mode=ro; query_only is enabled after creating temporary views.
type SQLiteReadInitializer func(context.Context, driver.ExecerContext) error

type sqliteReadConnector struct {
	driver       *sqlite.Driver
	dsn          string
	initializers []SQLiteReadInitializer
}

func (c *sqliteReadConnector) Driver() driver.Driver { return c.driver }

func (c *sqliteReadConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	conn, err := c.driver.Open(c.dsn)
	if err != nil {
		return nil, err
	}
	exec, ok := conn.(driver.ExecerContext)
	if !ok {
		return nil, errors.Join(errors.New("store: SQLite read connection lacks context execution"), conn.Close())
	}
	if _, err := exec.ExecContext(ctx, "PRAGMA busy_timeout = 5000", nil); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	for _, initialize := range c.initializers {
		if err := initialize(ctx, exec); err != nil {
			return nil, errors.Join(err, conn.Close())
		}
	}
	if _, err := exec.ExecContext(ctx, "PRAGMA query_only = ON", nil); err != nil {
		return nil, errors.Join(err, conn.Close())
	}
	return conn, nil
}

func openInitializedSQLiteReadPool(
	ctx context.Context,
	path string,
	initializers []SQLiteReadInitializer,
) (*sql.DB, error) {
	if err := sqliteUnicodeSearchRegistrationErr; err != nil {
		return nil, err
	}
	db := sql.OpenDB(&sqliteReadConnector{
		driver: &sqlite.Driver{}, dsn: sqliteReadOnlyDSN(path), initializers: initializers,
	})
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("store: initialize read pool: %w", err), db.Close())
	}
	return db, nil
}
