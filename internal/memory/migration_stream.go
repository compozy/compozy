package memory

import (
	memoryschema "github.com/compozy/agh/internal/memory/schema"
	storepkg "github.com/compozy/agh/internal/store"
)

const memoryMigrationVersionTable = "goose_db_version_memory"

// MigrationStream returns the embedded memory catalog migration stream.
func MigrationStream() storepkg.MigrationStream {
	return storepkg.MigrationStream{
		Name:         "memory",
		FS:           memoryschema.Files,
		Dir:          "migrations",
		VersionTable: memoryMigrationVersionTable,
		LegacyTables: []string{"schema_migrations", "memory_schema_migrations"},
		Bootstrap: &storepkg.MigrationBootstrap{
			FS:           memoryschema.Files,
			MigrationDir: "migrations",
			SchemaSource: "schema.sql",
		},
	}
}
