package workspacedb

import (
	"github.com/compozy/agh/internal/store"
	workspaceschema "github.com/compozy/agh/internal/store/workspacedb/schema"
)

const workspaceMigrationVersionTable = "goose_db_version_workspace"

// MigrationStream returns the embedded workspace database migration stream.
func MigrationStream() store.MigrationStream {
	return store.MigrationStream{
		Name:         "workspace",
		FS:           workspaceschema.Files,
		Dir:          "migrations",
		VersionTable: workspaceMigrationVersionTable,
		LegacyTables: []string{"schema_migrations"},
		Bootstrap: &store.MigrationBootstrap{
			FS:           workspaceschema.Files,
			MigrationDir: "migrations",
			SchemaSource: "schema.sql",
		},
	}
}
