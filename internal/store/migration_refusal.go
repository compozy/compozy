package store

import (
	"context"
	"database/sql"
	"fmt"
)

const migrationFamilyRecoveryInstruction = "stop AGH; preserve or move the complete AGH_HOME or workspace .agh " +
	"directory containing this database, including every sibling database file; then start AGH with a separately " +
	"selected fresh AGH_HOME or workspace"

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
				"install a newer AGH binary or %s",
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
