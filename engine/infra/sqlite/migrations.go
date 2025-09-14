package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ApplyMigrations runs embedded migrations using the sqlite dialect on the provided DB.
// Note: SQLite migrations are not concurrency-safe; run from a single process at startup.
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}
