package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/agh/internal/store"
)

const (
	sessionVacuumMinBytes          = 4 << 20
	sessionVacuumMinRatio          = 4
	sessionWALAutoCheckpointPragma = "wal_autocheckpoint(0)"
)

func openSessionSQLite(ctx context.Context, path string) (*sql.DB, error) {
	return openSessionSQLiteWithVacuum(ctx, path, vacuumSessionSQLite)
}

type sessionVacuumFunc func(context.Context, *sql.DB) error

func openSessionSQLiteWithVacuum(
	ctx context.Context,
	path string,
	vacuumFn sessionVacuumFunc,
) (*sql.DB, error) {
	return store.OpenSQLiteDatabaseWithPragmas(
		ctx,
		path,
		[]string{sessionWALAutoCheckpointPragma},
		func(ctx context.Context, db *sql.DB) error {
			if err := store.Apply(ctx, db, MigrationStream()); err != nil {
				return err
			}
			if err := initializeTranscriptProjectionState(ctx, db); err != nil {
				return err
			}
			if vacuumFn == nil {
				return nil
			}
			if err := vacuumFn(ctx, db); err != nil {
				slog.Default().WarnContext(
					ctx,
					"store: skip session sqlite vacuum after non-fatal failure",
					"path",
					path,
					"error",
					err,
				)
			}
			return nil
		},
	)
}

type sqlitePageStats struct {
	pageCount     int64
	pageSize      int64
	freelistCount int64
}

func vacuumSessionSQLite(ctx context.Context, db *sql.DB) error {
	stats, err := loadSQLitePageStats(ctx, db)
	if err != nil {
		return err
	}
	if !shouldVacuumSessionSQLite(stats) {
		return nil
	}
	// dynamic-sql: VACUUM is SQLite pool maintenance outside sqlc's schema query model and cannot run in a transaction.
	if _, err := db.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("store: vacuum session sqlite database: %w", err)
	}
	return nil
}

func loadSQLitePageStats(ctx context.Context, db *sql.DB) (sqlitePageStats, error) {
	if ctx == nil {
		return sqlitePageStats{}, errors.New("store: sqlite page stats context is required")
	}
	if db == nil {
		return sqlitePageStats{}, errors.New("store: sqlite page stats database is required")
	}

	var stats sqlitePageStats
	// dynamic-sql: SQLite page-stat PRAGMAs are connection maintenance outside sqlc's schema query model.
	if err := db.QueryRowContext(ctx, "PRAGMA page_count").Scan(&stats.pageCount); err != nil {
		return sqlitePageStats{}, fmt.Errorf("store: query sqlite page_count: %w", err)
	}
	// dynamic-sql: SQLite page-stat PRAGMAs are connection maintenance outside sqlc's schema query model.
	if err := db.QueryRowContext(ctx, "PRAGMA page_size").Scan(&stats.pageSize); err != nil {
		return sqlitePageStats{}, fmt.Errorf("store: query sqlite page_size: %w", err)
	}
	// dynamic-sql: SQLite page-stat PRAGMAs are connection maintenance outside sqlc's schema query model.
	if err := db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&stats.freelistCount); err != nil {
		return sqlitePageStats{}, fmt.Errorf("store: query sqlite freelist_count: %w", err)
	}
	return stats, nil
}

func shouldVacuumSessionSQLite(stats sqlitePageStats) bool {
	if stats.pageCount <= 0 || stats.pageSize <= 0 || stats.freelistCount <= 0 {
		return false
	}
	freeBytes := stats.freelistCount * stats.pageSize
	if freeBytes < sessionVacuumMinBytes {
		return false
	}
	return stats.freelistCount*sessionVacuumMinRatio >= stats.pageCount
}
