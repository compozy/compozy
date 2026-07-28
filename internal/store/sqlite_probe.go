package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// probeExistingSQLiteDatabase forces SQLite to parse an existing database in
// read-only immutable mode before any writable handle can perform journal
// housekeeping. It intentionally ignores WAL contents: the probe only proves
// that the durable main database image is structurally readable.
func probeExistingSQLiteDatabase(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("store: stat sqlite database %q before open: %w", path, err)
	}

	db, err := sql.Open(sqliteDriverName, sqliteImmutableDSN(path))
	if err != nil {
		return fmt.Errorf("store: open immutable sqlite probe %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	var schemaVersion int64
	if err := db.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&schemaVersion); err != nil {
		primaryErr := fmt.Errorf("store: probe sqlite database %q: %w", path, err)
		return closeSQLiteAfterError(db, primaryErr, fmt.Sprintf("immutable database probe %q", path))
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("store: close immutable sqlite probe %q: %w", path, err)
	}
	return nil
}
