package postgres

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/pressly/goose/v3"

	// Register pgx stdlib driver for database/sql usage in migrations.
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS
var gooseMu sync.Mutex

// ApplyMigrations runs database migrations from the embedded SQL files
// using goose. It expects a DSN understood by database/sql with the
// pgx stdlib driver name ("pgx").
func ApplyMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()
	return runMigrations(ctx, db)
}

// ApplyMigrationsWithLock acquires a Postgres advisory lock before running
// migrations to prevent concurrent runners from racing during startup.
// The lock is held for the duration of the migration and released at the end
// or when the context is canceled. It uses a deterministic key derived from
// a constant string to avoid magic numbers.
func ApplyMigrationsWithLock(ctx context.Context, dsn string) error {
	const defaultLockTimeout = 45 * time.Second
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open db for migrations: %w", err)
	}
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire dedicated connection: %w", err)
	}
	defer conn.Close()
	log := logger.FromContext(ctx)
	lockCtx, cancel := context.WithTimeout(ctx, defaultLockTimeout)
	defer cancel()
	if _, err := conn.ExecContext(
		lockCtx,
		"select pg_advisory_lock(hashtext($1), hashtext($2))",
		"compozy",
		"migrations",
	); err != nil {
		return fmt.Errorf("acquire migration advisory lock: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(
			context.WithoutCancel(ctx),
			"select pg_advisory_unlock(hashtext($1), hashtext($2))",
			"compozy",
			"migrations",
		); err != nil {
			log.Warn("Failed to release migration advisory lock", "error", err)
		}
	}()
	return runMigrations(ctx, db)
}

// runMigrations applies migrations on the provided *sql.DB.
func runMigrations(_ context.Context, db *sql.DB) error {
	gooseMu.Lock()
	defer gooseMu.Unlock()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		goose.SetBaseFS(nil)
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		goose.SetBaseFS(nil)
		return fmt.Errorf("migrate up: %w", err)
	}
	goose.SetBaseFS(nil)
	return nil
}

// RunMigrationsForDB exposes migration execution on an existing *sql.DB.
func RunMigrationsForDB(ctx context.Context, db *sql.DB) error { return runMigrations(ctx, db) }
