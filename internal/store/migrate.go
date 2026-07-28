package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/pressly/goose/v3"
)

var (
	// ErrLegacyDatabase reports a database created by the removed migration runner.
	ErrLegacyDatabase = errors.New("store: database was created by the removed pre-goose migration runner")
	// ErrSchemaBehind reports a database older than the running binary.
	ErrSchemaBehind = errors.New("store: database schema is behind this binary")
	// ErrSchemaAhead reports a database newer than the running binary.
	ErrSchemaAhead = errors.New("store: database schema is ahead of this binary")
	// ErrMigrationConnectionCapacity reports a pool too small for a non-transactional migration.
	ErrMigrationConnectionCapacity = errors.New("store: migration database connection capacity is insufficient")
)

// MigrationStream describes one independently versioned embedded schema.
type MigrationStream struct {
	Name         string
	FS           fs.FS
	Dir          string
	VersionTable string
	LegacyTables []string
	Bootstrap    *MigrationBootstrap
}

// StreamStatus describes the applied state of one migration stream.
type StreamStatus struct {
	Stream       string
	Version      int64
	AppliedCount int
	SumDigest    string
}

type migrationInspection struct {
	path      string
	directory migrationDirectory
	status    StreamStatus
}

// Apply validates and applies all pending SQL migrations for stream.
func Apply(ctx context.Context, db *sql.DB, stream MigrationStream) error {
	inspection, err := inspectMigrationStream(ctx, db, stream)
	if err != nil {
		return err
	}
	startedAt := time.Now()
	bootstrapped, err := tryBootstrapMigrationStream(ctx, db, stream, inspection)
	if err != nil {
		return err
	}
	if bootstrapped {
		return logAppliedMigrationStream(ctx, db, stream, inspection.directory.sumDigest, startedAt)
	}
	plan, err := prepareMigrationPlan(inspection.directory)
	if err != nil {
		return fmt.Errorf("store: prepare migration stream %q: %w", stream.Name, err)
	}
	if plan.requiresIndependentConnection() && db.Stats().MaxOpenConnections == 1 {
		return fmt.Errorf(
			"%w: stream %q contains a non-transactional migration and requires at least two open connections",
			ErrMigrationConnectionCapacity,
			stream.Name,
		)
	}

	provider, err := goose.NewProvider(
		goose.DialectSQLite3,
		db,
		plan.gooseFS,
		goose.WithTableName(stream.VersionTable),
		goose.WithDisableGlobalRegistry(true),
		goose.WithGoMigrations(plan.gooseMigrations()...),
	)
	if err != nil {
		return fmt.Errorf("store: create migration provider for stream %q: %w", stream.Name, err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("store: apply migration stream %q: %w", stream.Name, err)
	}
	return logAppliedMigrationStream(ctx, db, stream, inspection.directory.sumDigest, startedAt)
}

func logAppliedMigrationStream(
	ctx context.Context,
	db *sql.DB,
	stream MigrationStream,
	sumDigest string,
	startedAt time.Time,
) error {
	status, err := readStreamStatus(ctx, db, stream, sumDigest)
	if err != nil {
		return err
	}
	migrationLogger(ctx).InfoContext(ctx, "store.migrations.applied",
		"stream", status.Stream,
		"version", status.Version,
		"applied_count", status.AppliedCount,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	)
	return nil
}

// Status returns stream state without creating or mutating a version table.
func Status(ctx context.Context, db *sql.DB, stream MigrationStream) (StreamStatus, error) {
	inspection, err := inspectMigrationStream(ctx, db, stream)
	if err != nil {
		return StreamStatus{}, err
	}
	return inspection.status, nil
}

// RequireCurrent validates that a migration stream is exactly at the embedded
// head without creating tables or applying migrations.
func RequireCurrent(ctx context.Context, db *sql.DB, stream MigrationStream) error {
	inspection, err := inspectMigrationStream(ctx, db, stream)
	if err != nil {
		return err
	}
	if inspection.status.Version == inspection.directory.maxVersion &&
		inspection.status.AppliedCount == inspection.directory.migrationCount {
		return nil
	}
	return fmt.Errorf(
		"%w: %s stream %s is at version %d with %d applied migrations; "+
			"binary head is version %d with %d migrations; open the database through its writable migration owner first",
		ErrSchemaBehind,
		inspection.path,
		stream.Name,
		inspection.status.Version,
		inspection.status.AppliedCount,
		inspection.directory.maxVersion,
		inspection.directory.migrationCount,
	)
}

func inspectMigrationStream(
	ctx context.Context,
	db *sql.DB,
	stream MigrationStream,
) (migrationInspection, error) {
	if err := validateMigrationInputs(ctx, db, stream); err != nil {
		return migrationInspection{}, err
	}
	path, err := migrationDatabasePath(ctx, db)
	if err != nil {
		return migrationInspection{}, err
	}
	inspection := migrationInspection{path: path}
	if err := refuseLegacyDatabase(ctx, db, stream, path); err != nil {
		migrationLogger(ctx).ErrorContext(ctx, "store.migrations.refused",
			"stream", stream.Name,
			"reason", "legacy_database",
			"path", path,
		)
		return inspection, err
	}
	directory, err := loadMigrationDirectory(stream)
	if err != nil {
		migrationLogger(ctx).ErrorContext(ctx, "store.migrations.refused",
			"stream", stream.Name,
			"reason", "sum_mismatch",
			"path", path,
		)
		return inspection, err
	}
	inspection.directory = directory
	status, err := readStreamStatus(ctx, db, stream, directory.sumDigest)
	if err != nil {
		return inspection, err
	}
	inspection.status = status
	if err := refuseAheadDatabase(path, stream, status, directory.maxVersion); err != nil {
		return inspection, err
	}
	return inspection, nil
}

func validateMigrationInputs(ctx context.Context, db *sql.DB, stream MigrationStream) error {
	if ctx == nil {
		return errors.New("store: migration context is required")
	}
	if db == nil {
		return errors.New("store: migration database is required")
	}
	if stream.Name == "" {
		return errors.New("store: migration stream name is required")
	}
	if stream.FS == nil {
		return fmt.Errorf("store: migration stream %q filesystem is required", stream.Name)
	}
	if stream.Dir == "" {
		return fmt.Errorf("store: migration stream %q directory is required", stream.Name)
	}
	if _, err := NormalizeSQLiteIdentifier(stream.VersionTable); err != nil {
		return fmt.Errorf("store: migration stream %q version table: %w", stream.Name, err)
	}
	for _, table := range stream.LegacyTables {
		if _, err := NormalizeSQLiteIdentifier(table); err != nil {
			return fmt.Errorf("store: migration stream %q legacy table: %w", stream.Name, err)
		}
	}
	return nil
}
