package resources

import (
	"github.com/compozy/agh/internal/store"
	globalschema "github.com/compozy/agh/internal/store/globaldb/schema"
)

func globalTestMigrationStream() store.MigrationStream {
	return store.MigrationStream{
		Name:         "global",
		FS:           globalschema.Files,
		Dir:          "migrations",
		VersionTable: "goose_db_version_global",
		LegacyTables: []string{"schema_migrations", "memory_schema_migrations"},
		Bootstrap: &store.MigrationBootstrap{
			FS:           globalschema.Files,
			MigrationDir: "migrations",
			SchemaSource: "definitions",
			SeedSource:   "bootstrap.sql",
		},
	}
}
