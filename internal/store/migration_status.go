package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func readStreamStatus(
	ctx context.Context,
	db *sql.DB,
	stream MigrationStream,
	sumDigest string,
) (status StreamStatus, err error) {
	status = StreamStatus{Stream: stream.Name, SumDigest: sumDigest}
	exists, err := migrationTableExists(ctx, db, stream.VersionTable)
	if err != nil {
		return StreamStatus{}, err
	}
	if !exists {
		return status, nil
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf(
		`SELECT version_id, is_applied FROM %s ORDER BY id ASC`,
		quoteIdentifier(stream.VersionTable),
	))
	if err != nil {
		return StreamStatus{}, fmt.Errorf("store: query migration stream %q status: %w", stream.Name, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("store: close migration stream %q status rows: %w", stream.Name, closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()
	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		var isApplied bool
		if err := rows.Scan(&version, &isApplied); err != nil {
			return StreamStatus{}, fmt.Errorf("store: scan migration stream %q status: %w", stream.Name, err)
		}
		if version > 0 {
			applied[version] = isApplied
		}
	}
	if err := rows.Err(); err != nil {
		return StreamStatus{}, fmt.Errorf("store: iterate migration stream %q status: %w", stream.Name, err)
	}
	for version, isApplied := range applied {
		if !isApplied {
			continue
		}
		status.AppliedCount++
		if version > status.Version {
			status.Version = version
		}
	}
	return status, nil
}

func migrationTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		table,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("store: query migration table %q: %w", table, err)
	}
	return count > 0, nil
}

func quoteIdentifier(name string) string {
	return `"` + name + `"`
}
