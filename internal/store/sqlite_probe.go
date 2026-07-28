package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// probeExistingSQLiteDatabase validates the required SQLite header before
// SQLite can touch a malformed database family, then parses valid candidates in
// read-only mode with normal locking and WAL visibility.
func probeExistingSQLiteDatabase(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("store: stat sqlite database %q before open: %w", path, err)
	}
	if err := validateSQLiteDatabaseHeader(path, info.Size()); err != nil {
		return err
	}

	db, err := sql.Open(sqliteDriverName, sqliteReadOnlyDSN(path))
	if err != nil {
		return fmt.Errorf("store: open read-only sqlite probe %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	var schemaVersion int64
	if err := db.QueryRowContext(ctx, "PRAGMA schema_version").Scan(&schemaVersion); err != nil {
		primaryErr := fmt.Errorf("store: probe sqlite database %q: %w", path, err)
		return closeSQLiteAfterError(db, primaryErr, fmt.Sprintf("read-only database probe %q", path))
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("store: close read-only sqlite probe %q: %w", path, err)
	}
	return nil
}
