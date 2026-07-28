package globaldb

import "testing"

func TestOpenGlobalDBCreatesResourceTablesAndIndexes(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)

	assertTablesPresent(t, globalDB.db, "resource_records", "resource_source_state")
	assertTableColumns(t, globalDB.db, "resource_records", []string{
		"kind",
		"id",
		"version",
		"scope_kind",
		"scope_id",
		"owner_kind",
		"owner_id",
		"source_kind",
		"source_id",
		"spec_json",
		"created_at",
		"updated_at",
	})
	assertTableColumns(t, globalDB.db, "resource_source_state", []string{
		"source_kind",
		"source_id",
		"session_nonce",
		"last_snapshot_version",
		"updated_at",
	})
	assertIndexesPresent(
		t,
		globalDB.db,
		"resource_records",
		"idx_resource_kind",
		"idx_resource_scope",
		"idx_resource_owner",
		"idx_resource_source",
	)
}
