package resources

import "slices"

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS resource_records (
		kind        TEXT NOT NULL,
		id          TEXT NOT NULL,
		version     INTEGER NOT NULL,
		scope_kind  TEXT NOT NULL CHECK (scope_kind IN ('global', 'workspace')),
		scope_id    TEXT,
		owner_kind  TEXT NOT NULL,
		owner_id    TEXT NOT NULL,
		source_kind TEXT NOT NULL,
		source_id   TEXT NOT NULL,
		spec_json   TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		PRIMARY KEY (kind, id),
		CHECK (
			(scope_kind = 'global' AND scope_id IS NULL) OR
			(scope_kind = 'workspace' AND scope_id IS NOT NULL)
		)
	);`,
	`CREATE INDEX IF NOT EXISTS idx_resource_kind ON resource_records(kind);`,
	`CREATE INDEX IF NOT EXISTS idx_resource_scope ON resource_records(scope_kind, scope_id, kind);`,
	`CREATE INDEX IF NOT EXISTS idx_resource_owner ON resource_records(owner_kind, owner_id, kind);`,
	`CREATE INDEX IF NOT EXISTS idx_resource_source ON resource_records(source_kind, source_id, kind);`,
	`CREATE TABLE IF NOT EXISTS resource_source_state (
		source_kind           TEXT NOT NULL,
		source_id             TEXT NOT NULL,
		session_nonce         TEXT NOT NULL,
		last_snapshot_version INTEGER NOT NULL,
		updated_at            TEXT NOT NULL,
		PRIMARY KEY (source_kind, source_id)
	);`,
}

// SchemaStatements returns the canonical SQLite schema bootstrap for the raw resource kernel.
func SchemaStatements() []string {
	return slices.Clone(schemaStatements)
}
