package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sync"

	"github.com/pressly/goose/v3"
	// Register modernc SQLite driver with database/sql.
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var gooseInitMu sync.Mutex

// ApplyMigrations executes all embedded SQLite migrations against the database.
func ApplyMigrations(ctx context.Context, dbPath string) error {
	cfg := &Config{Path: dbPath}
	dsn, _, err := buildDSN(cfg)
	if err != nil {
		return fmt.Errorf("sqlite: prepare migrations dsn: %w", err)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("sqlite: open database for migrations: %w", err)
	}
	defer db.Close()

	if err := applyBusyTimeout(ctx, db, cfg); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("sqlite: enable foreign keys: %w", err)
	}

	gooseInitMu.Lock()
	defer func() {
		goose.SetBaseFS(nil)
		gooseInitMu.Unlock()
	}()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("sqlite: set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("sqlite: apply migrations: %w", err)
	}
	return nil
}
