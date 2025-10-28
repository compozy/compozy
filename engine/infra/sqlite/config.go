package sqlite

import "time"

// Config captures SQLite store configuration derived from application settings.
type Config struct {
	// Path is the database location or ":memory:" for in-memory deployments.
	Path string

	// MaxOpenConns controls the pool size exposed by database/sql.
	MaxOpenConns int

	// MaxIdleConns limits idle connections retained in the pool.
	MaxIdleConns int

	// ConnMaxLifetime bounds connection reuse duration.
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime bounds idle connection retention.
	ConnMaxIdleTime time.Duration

	// BusyTimeout configures sqlite busy timeout via PRAGMA busy_timeout.
	BusyTimeout time.Duration
}
