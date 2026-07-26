package sessiondb

import (
	"github.com/compozy/agh/internal/store"
	sessionschema "github.com/compozy/agh/internal/store/sessiondb/schema"
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
		},
	}
}
