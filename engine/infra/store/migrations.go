package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sync"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

var (
	migrationOnce sync.Once
	migrationErr  error
)

// ResetMigrationsForTest resets the migration singleton for testing.
// This function should only be used in test code.
func ResetMigrationsForTest() {
	migrationOnce = sync.Once{}
	migrationErr = nil
}

// runEmbeddedMigrations runs database migrations from embedded SQL files
// in a thread-safe manner using PostgreSQL advisory locks.
func runEmbeddedMigrations(ctx context.Context, db *sql.DB) error {
	migrationOnce.Do(func() {
		// Acquire PostgreSQL advisory lock for multi-instance safety
		const lockID = 4242

		_, err := db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockID)
		if err != nil {
			migrationErr = fmt.Errorf("failed to acquire migration lock: %w", err)
			return
		}
		defer func() {
			if _, unlockErr := db.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID); unlockErr != nil {
				log := logger.FromContext(ctx)
				log.Error("failed to release migration lock", "error", unlockErr)
			}
		}()

		// Set the base filesystem for embedded migrations
		goose.SetBaseFS(migrationsFS)

		// Configure Goose
		if err := goose.SetDialect("postgres"); err != nil {
			migrationErr = fmt.Errorf("failed to set dialect: %w", err)
			return
		}

		// Run migrations from embedded filesystem
		if err := goose.Up(db, "migrations"); err != nil {
			migrationErr = fmt.Errorf("migration failed: %w", err)
			return
		}
	})

	return migrationErr
}
