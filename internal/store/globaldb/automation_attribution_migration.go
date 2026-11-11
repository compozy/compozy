package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
)

func applyGlobalMigrations(ctx context.Context, db *sql.DB, path string, progress MigrationProgressFunc) error {
	status, err := store.Status(ctx, db, MigrationStream())
	if err != nil {
		return fmt.Errorf("globaldb: inspect migration status: %w", err)
	}
	if err := applyTelemetryRelocation(ctx, db, path, status.Version, progress); err != nil {
		return err
	}
	if err := repairUsageCosts(ctx, db, path); err != nil {
		return err
	}
	if status.Version >= 149 {
		return nil
	}
	var derived, unknown int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(CASE WHEN attribution = 'derived' THEN 1 END), COUNT(CASE WHEN attribution = 'unknown' THEN 1 END) FROM automation_runs`).
		Scan(&derived, &unknown); err != nil {
		return fmt.Errorf("globaldb: count automation run attribution backfill: %w", err)
	}
	slog.InfoContext(ctx, "automation run attribution backfill", "derived", derived, "unknown", unknown)
	return nil
}
