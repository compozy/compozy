package sessiondb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	sessionschema "github.com/compozy/compozy/internal/store/sessiondb/schema"
)

const sessionMigrationVersionTable = "goose_db_version_session"

// MigrationStream returns the embedded session database migration stream.
func MigrationStream() store.MigrationStream {
	return store.MigrationStream{
		Name:         "session",
		FS:           sessionschema.Files,
		Dir:          "migrations",
		VersionTable: sessionMigrationVersionTable,
		LegacyTables: []string{"schema_migrations"},
		Bootstrap: &store.MigrationBootstrap{
			FS:           sessionschema.Files,
			MigrationDir: "migrations",
			SchemaSource: "schema.sql",
			Initialize:   initializeSessionDatabaseIdentity,
		},
	}
}

func initializeSessionDatabaseIdentity(ctx context.Context, tx *sql.Tx) error {
	// dynamic-sql: the immutable physical identity is instance-specific
	// bootstrap state and cannot be part of the declarative schema.
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO session_db_identity (singleton, database_id) VALUES (1, lower(hex(randomblob(16))))`,
	); err != nil {
		return fmt.Errorf("store: stamp session database identity: %w", err)
	}
	return nil
}
