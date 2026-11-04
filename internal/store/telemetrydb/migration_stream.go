package telemetrydb

import (
	"github.com/compozy/compozy/internal/store"
	telemetryschema "github.com/compozy/compozy/internal/store/telemetrydb/schema"
)

func MigrationStream() store.MigrationStream {
	return store.MigrationStream{
		Name: "telemetry", FS: telemetryschema.Files, Dir: "migrations",
		VersionTable: "goose_db_version_telemetry",
		LegacyTables: []string{"schema_migrations", "goose_db_version_global"},
		Bootstrap: &store.MigrationBootstrap{
			FS: telemetryschema.Files, MigrationDir: "migrations", SchemaSource: "schema.sql",
		},
	}
}
