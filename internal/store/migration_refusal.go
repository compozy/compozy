package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

const migrationFamilyRecoveryInstruction = "stop CompozyOS; preserve or move the complete COMPOZY_HOME or " +
	"workspace .compozy directory containing this database, including every sibling database file; " +
	"then start CompozyOS with a separately " +
	"selected fresh COMPOZY_HOME or workspace"

// RefuseLegacyDatabaseAtPath checks legacy markers through a read-only handle
// before a writable SQLite connection can mutate the database header.
func RefuseLegacyDatabaseAtPath(ctx context.Context, path string, stream MigrationStream) (err error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: stat sqlite database %q before migration refusal: %w", path, err)
	}
	if err := validateSQLiteDatabaseHeader(path, info.Size()); err != nil {
		return err
	}
	db, err := sql.Open(sqliteDriverName, sqliteReadOnlyDSN(path))
	if err != nil {
		return fmt.Errorf("store: open read-only migration probe %q: %w", path, err)
	}
	defer func() {
		err = errors.Join(err, db.Close())
	}()
	resolvedPath, err := migrationDatabasePath(ctx, db)
	if err != nil {
		return err
	}
	if err := refuseLegacyDatabase(ctx, db, stream, resolvedPath); err != nil {
		migrationLogger(ctx).ErrorContext(ctx, "store.migrations.refused",
			"stream", stream.Name,
			"reason", "legacy_database",
			"path", resolvedPath,
		)
		return err
	}
	return nil
}

func refuseLegacyDatabase(ctx context.Context, db *sql.DB, stream MigrationStream, path string) error {
	for _, table := range stream.LegacyTables {
		exists, err := migrationTableExists(ctx, db, table)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf(
				"%w: %s contains legacy table %s; %s",
				ErrLegacyDatabase,
				path,
				table,
				migrationFamilyRecoveryInstruction,
			)
		}
	}
	return nil
}

func refuseAheadDatabase(
	path string,
	stream MigrationStream,
	status StreamStatus,
	maxEmbeddedVersion int64,
) error {
	if status.Version > maxEmbeddedVersion {
		return fmt.Errorf(
			"%w: %s stream %s is at version %d, binary head is %d; "+
				"install a newer CompozyOS binary or %s",
			ErrSchemaAhead,
			path,
			stream.Name,
			status.Version,
			maxEmbeddedVersion,
			migrationFamilyRecoveryInstruction,
		)
	}
	return nil
}
