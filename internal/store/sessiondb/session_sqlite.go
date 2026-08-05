package sessiondb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/store"
)

const (
	sessionVacuumMinBytes = 4 << 20
	sessionVacuumMinRatio = 4
)

func openSessionSQLiteForExistingOwnerWithVacuum(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
	vacuumFn sessionVacuumFunc,
) (*sql.DB, error) {
	db, err := openGuardedSessionSQLite(ctx, owner, path, false)
	if err != nil {
		return nil, err
	}
	if err := initializeSessionSQLite(ctx, db, MigrationStream(), vacuumFn, path); err != nil {
		primaryErr := fmt.Errorf("store: initialize owned session sqlite database %q: %w", path, err)
		if closeErr := db.Close(); closeErr != nil {
			primaryErr = errors.Join(
				primaryErr,
				fmt.Errorf("store: close rejected owned session sqlite database: %w", closeErr),
			)
		}
		return nil, primaryErr
	}
	return db, nil
}

type sessionSQLiteSerializer interface {
	Serialize() ([]byte, error)
}

func buildSessionSQLiteCandidate(
	ctx context.Context,
	owner store.SessionDBOwner,
	vacuumFn sessionVacuumFunc,
) (contents []byte, retErr error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("store: open in-memory session sqlite candidate: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("store: close in-memory session sqlite candidate: %w", closeErr))
		}
	}()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("store: ping in-memory session sqlite candidate: %w", err)
	}
	if err := initializeSessionSQLite(
		ctx,
		db,
		migrationStreamForOwner(owner),
		vacuumFn,
		"in-memory candidate",
	); err != nil {
		return nil, fmt.Errorf("store: initialize in-memory session sqlite candidate: %w", err)
	}
	connection, err := db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: acquire in-memory session sqlite candidate connection: %w", err)
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			retErr = errors.Join(
				retErr,
				fmt.Errorf("store: close in-memory session sqlite candidate connection: %w", closeErr),
			)
		}
	}()
	if err := connection.Raw(func(driverConnection any) error {
		serializer, ok := driverConnection.(sessionSQLiteSerializer)
		if !ok {
			return errors.New("store: sqlite driver connection does not support serialization")
		}
		serialized, err := serializer.Serialize()
		if err != nil {
			return fmt.Errorf("store: serialize in-memory session sqlite candidate: %w", err)
		}
		contents = serialized
		return nil
	}); err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return nil, errors.New("store: serialized session sqlite candidate is empty")
	}
	return contents, nil
}

type sessionVacuumFunc func(context.Context, *sql.DB) error

func initializeSessionSQLite(
	ctx context.Context,
	db *sql.DB,
	stream store.MigrationStream,
	vacuumFn sessionVacuumFunc,
	path string,
) error {
	if err := store.Apply(ctx, db, stream); err != nil {
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
