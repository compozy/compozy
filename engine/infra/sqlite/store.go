package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	// Import modernc SQLite driver for database/sql usage
	_ "modernc.org/sqlite"

	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/logger"
)

// Store is the concrete SQLite driver backed by database/sql.
// It intentionally does not leak *sql.DB outside higher layers.
type Store struct {
	db *sql.DB
}

// NewStore opens a SQLite database at path using modernc.org/sqlite with
// pragmatic defaults suitable for dev/standalone mode. When path is ":memory:",
// it uses a shared cache to allow multiple connections.
func NewStore(ctx context.Context, path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite: path is required")
	}
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	// Connection settings tuned for SQLite
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	// Health check
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}
	logger.FromContext(ctx).With("store_driver", "sqlite", "path", path).Info("Store initialized")
	return &Store{db: db}, nil
}

// buildDSN constructs a modernc.org/sqlite DSN with safe pragmas:
// - WAL journaling for durability in dev
// - foreign_keys=ON for FK enforcement
// - busy_timeout=5000ms to reduce transient lock errors
func buildDSN(path string) string {
	var base string
	if path == ":memory:" {
		base = "file::memory:?cache=shared"
	} else {
		base = "file:" + path
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	// Use _pragma to set pragmas at connection time
	return base + sep + "_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"
}

// Close closes the underlying DB.
func (s *Store) Close(ctx context.Context) error {
	if err := s.db.Close(); err != nil {
		return err
	}
	logger.FromContext(ctx).Info("SQLite store closed")
	return nil
}

// HealthCheck verifies the connection is alive.
func (s *Store) HealthCheck(ctx context.Context) error {
	hctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := s.db.PingContext(hctx); err != nil {
		return fmt.Errorf("sqlite: health check failed: %w", err)
	}
	return nil
}

// DB exposes the internal *sql.DB for driver-local usage (e.g., migrations).
// Higher layers should not depend on *sql.DB directly.
func (s *Store) DB() *sql.DB { return s.db }

// --- store.Store interface implementation ---

// Repositories is a driver-local accessor placeholder. It can expose
// strongly-typed constructors in a future task when repos are ported.
type Repositories struct{ tx *sql.Tx }

// WithTransaction executes fn within a single transaction.
func (s *Store) WithTransaction(ctx context.Context, fn func(store.Repositories) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	wrapped := Repositories{tx: tx}
	defer func() {
		if p := recover(); p != nil {
			if rerr := tx.Rollback(); rerr != nil {
				logger.FromContext(ctx).Warn(
					"SQLite rollback after panic failed",
					"error", rerr,
				)
			}
			panic(p)
		}
	}()
	if err := fn(wrapped); err != nil {
		if rerr := tx.Rollback(); rerr != nil {
			return fmt.Errorf("rollback tx: %v (original: %w)", rerr, err)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// ReadOnly returns repositories bound to no transaction for read-only ops.
func (s *Store) ReadOnly(ctx context.Context) store.Repositories {
	_ = ctx
	return Repositories{tx: nil}
}
