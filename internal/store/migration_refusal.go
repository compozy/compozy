package store

import (
	"context"
	"database/sql"
	"fmt"
)

const migrationFamilyRecoveryInstruction = "stop CompozyOS; preserve or move the complete COMPOZY_HOME or " +
	"workspace .compozy directory containing this database, including every sibling database file; " +
	"then start CompozyOS with a separately " +
	"selected fresh COMPOZY_HOME or workspace"

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
