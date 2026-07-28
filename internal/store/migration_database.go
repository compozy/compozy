package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func migrationDatabasePath(ctx context.Context, db *sql.DB) (path string, err error) {
	rows, err := db.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return "", fmt.Errorf("store: query migration database path: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close migration database path rows: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()
	for rows.Next() {
		var sequence int
		var name string
		var filename string
		if err := rows.Scan(&sequence, &name, &filename); err != nil {
			return "", fmt.Errorf("store: scan migration database path: %w", err)
		}
		if name == "main" {
			if filename == "" {
				return "in-memory database", nil
			}
			return filename, nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("store: iterate migration database path: %w", err)
	}
	return "unknown database", nil
}
