package globaldb

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

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

func TestResourceRecordRemovedKindsCleanupMigration(t *testing.T) {
	// Keep this full-history fixture serial: migration helpers share a process-wide lock.
	t.Run("Should delete removed resource rows and preserve homonyms across reopen", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00040_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open v39 resource fixture error = %v", err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if prefixClosed {
				return
			}
			if closeErr := prefixDB.Close(); closeErr != nil {
				t.Errorf("close v39 resource fixture cleanup error = %v", closeErr)
			}
		})
		ctx := globalMigrationTestContext(t)
		rows := []struct {
			kind      string
			id        string
			ownerKind string
			ownerID   string
			specJSON  string
		}{
			{
				kind: "bundle", id: "removed-definition", ownerKind: "extension", ownerID: "kit",
				specJSON: `{"name":"starter"}`,
			},
			{
				kind: "bundle.activation", id: "removed-activation", ownerKind: "extension", ownerID: "kit",
				specJSON: `{"name":"starter"}`,
			},
			{
				kind:      "agent",
				id:        "removed-agent",
				ownerKind: "bundle.activation",
				ownerID:   "removed-activation",
				specJSON:  `{"name":"coder"}`,
			},
			{
				kind:      "soul",
				id:        "removed-soul",
				ownerKind: "bundle.activation",
				ownerID:   "removed-activation",
				specJSON:  `{"agent_name":"coder"}`,
			},
			{
				kind:      "automation.job",
				id:        "removed-job",
				ownerKind: "bundle.activation",
				ownerID:   "removed-activation",
				specJSON:  `{"name":"daily"}`,
			},
			{
				kind:      "skill",
				id:        "support-bundle",
				ownerKind: "extension",
				ownerID:   "support-kit",
				specJSON:  `{"name":"support-bundle","source":"bundled","context_bundle":{"name":"diagnostics"}}`,
			},
		}
		for _, row := range rows {
			if _, err := prefixDB.ExecContext(ctx, `INSERT INTO resource_records (
				kind, id, version, scope_kind, scope_id, owner_kind, owner_id,
				source_kind, source_id, spec_json, created_at, updated_at
			) VALUES (?, ?, 1, 'global', NULL, ?, ?, 'daemon', 'migration-fixture', ?, ?, ?)`,
				row.kind,
				row.id,
				row.ownerKind,
				row.ownerID,
				row.specJSON,
				"2026-08-02T12:00:00Z",
				"2026-08-02T12:00:00Z",
			); err != nil {
				t.Fatalf("seed resource %s/%s error = %v", row.kind, row.id, err)
			}
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close v39 resource fixture error = %v", err)
		}
		prefixClosed = true

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade v39 resource fixture error = %v", err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded resource fixture cleanup error = %v", closeErr)
			}
		})
		assertRemovedResourceRows(t, upgraded.db)
		assertPreservedResourceHomonym(t, upgraded.db)
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("read upgraded migration status error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded resource fixture error = %v", err)
		}
		upgradedClosed = true

		reopened, err := OpenGlobalDB(globalMigrationTestContext(t), path)
		if err != nil {
			t.Fatalf("reopen upgraded resource fixture error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("close reopened resource fixture error = %v", err)
			}
		})
		assertRemovedResourceRows(t, reopened.db)
		assertPreservedResourceHomonym(t, reopened.db)
		reopenedStatus, err := store.Status(globalMigrationTestContext(t), reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("read reopened migration status error = %v", err)
		}
		assertCompleteMigrationStream(t, reopenedStatus, MigrationStream())
	})
}

func assertRemovedResourceRows(t *testing.T, db *sql.DB) {
	t.Helper()

	var count int
	if err := db.QueryRowContext(testutil.Context(t), `SELECT COUNT(*) FROM resource_records
		WHERE kind IN ('bundle', 'bundle.activation') OR owner_kind = 'bundle.activation'`).Scan(&count); err != nil {
		t.Fatalf("count removed resource rows error = %v", err)
	}
	if count != 0 {
		t.Fatalf("removed resource row count = %d, want 0", count)
	}
}

func assertPreservedResourceHomonym(t *testing.T, db *sql.DB) {
	t.Helper()

	const wantSpec = `{"name":"support-bundle","source":"bundled","context_bundle":{"name":"diagnostics"}}`
	var kind, ownerKind, ownerID, specJSON string
	err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT kind, owner_kind, owner_id, spec_json FROM resource_records WHERE kind = 'skill' AND id = 'support-bundle'`,
	).Scan(&kind, &ownerKind, &ownerID, &specJSON)
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatal("preserved homonym resource is missing")
	}
	if err != nil {
		t.Fatalf("read preserved homonym resource error = %v", err)
	}
	if kind != "skill" || ownerKind != "extension" || ownerID != "support-kit" || specJSON != wantSpec {
		t.Fatalf(
			"preserved homonym resource = kind:%q owner:%q/%q spec:%q, want byte-identical survivor",
			kind,
			ownerKind,
			ownerID,
			specJSON,
		)
	}
}
