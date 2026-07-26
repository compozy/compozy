package globaldb

import (
	"github.com/compozy/agh/internal/store"
	globalschema "github.com/compozy/agh/internal/store/globaldb/schema"
)

const globalMigrationVersionTable = "goose_db_version_global"

// MigrationStream returns the embedded global database migration stream.
func MigrationStream() store.MigrationStream {
	return store.MigrationStream{
		Name:         "global",
		FS:           globalschema.Files,
		Dir:          "migrations",
		VersionTable: globalMigrationVersionTable,
		LegacyTables: []string{"schema_migrations", "memory_schema_migrations"},
		Bootstrap: &store.MigrationBootstrap{
			FS:           globalschema.Files,
			MigrationDir: "migrations",
			SchemaSource: "definitions",
			SeedSource:   "bootstrap.sql",
		},
	}
}
