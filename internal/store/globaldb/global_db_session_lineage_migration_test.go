package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

// TestGlobalDBSessionLineageMigration owns IT-017: migration 00122 adds the lineage
// kind columns and the derivation receipt table, and backfills pre-feature rows with
// the same rule as store.UpgradeSessionLineageKind. Ahead-version refusal stays owned
// by internal/store/migrate_test.go:TestApplyMigrationStream.
func TestGlobalDBSessionLineageMigration(t *testing.T) {
	t.Parallel()

	t.Run("Should backfill lineage kinds and preserve them across reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00122_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00122 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		now := store.FormatTimestamp(time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES ('ws-lineage-migration', ?, '[]', 'lineage-migration', ?, ?)`,
			t.TempDir(), now, now); err != nil {
			t.Fatalf("seed workspace before 00122 error = %v", err)
		}
		seeds := []struct {
			id, sessionType, parent, root, role string
			depth                               int
		}{
			{id: "sess-root", sessionType: "user", root: "sess-root"},
			{id: "sess-provenance", sessionType: "user", parent: "sess-root", root: "sess-root", depth: 1},
			{id: "sess-spawned", sessionType: "spawned", parent: "sess-root", root: "sess-root", depth: 1},
			{id: "sess-coordinator", sessionType: "coordinator", root: "sess-coordinator", role: "coordinator"},
		}
		for _, seed := range seeds {
			if _, err := prefixDB.ExecContext(ctx, `INSERT INTO sessions (
				id, profile_id, agent_name, workspace_id, session_type, state,
				parent_session_id, root_session_id, spawn_depth, spawn_role, created_at, updated_at
			) VALUES (?, ?, 'coder', 'ws-lineage-migration', ?, 'stopped',
				NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?)`,
				seed.id, store.DefaultProfileID, seed.sessionType,
				seed.parent, seed.root, seed.depth, seed.role, now, now); err != nil {
				t.Fatalf("seed session %q before 00122 error = %v", seed.id, err)
			}
		}
		columns, err := tableColumns(ctx, prefixDB, "sessions")
		if err != nil {
			t.Fatalf("tableColumns(sessions before 00122) error = %v", err)
		}
		if _, exists := columns["lineage_kind"]; exists {
			t.Fatal("sessions.lineage_kind exists before 00122")
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00122 error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00122 error = %v", err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded database error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, upgraded.db, "session_derivations", []string{
			"workspace_id", "idempotency_key", "profile_id", "request_fingerprint", "source_session_id",
			"child_session_id", "kind", "outcome_json", "created_at", "child_deleted_at",
		})
		assertIndexesPresent(
			t,
			upgraded.db,
			"session_derivations",
			"idx_session_derivations_source",
			"idx_session_derivations_child",
		)
		want := map[string]store.LineageKind{
			"sess-root":        store.LineageKindRoot,
			"sess-provenance":  store.LineageKindProvenance,
			"sess-spawned":     store.LineageKindSpawn,
			"sess-coordinator": store.LineageKindSpawn,
		}
		assertMigratedLineageKinds(t, upgraded, want)
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(after migration) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded database before reopen error = %v", err)
		}
		upgradedClosed = true

		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("reopen migrated database error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened database error = %v", closeErr)
			}
		})
		assertMigratedLineageKinds(t, reopened, want)
		provenance, err := reopened.ListSessions(ctx, SessionListQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: "ws-lineage-migration",
			LineageKind: store.LineageKindProvenance,
		})
		if err != nil {
			t.Fatalf("ListSessions(lineage_kind=provenance) error = %v", err)
		}
		if len(provenance) != 1 || provenance[0].ID != "sess-provenance" {
			t.Fatalf("ListSessions(lineage_kind=provenance) = %#v, want sess-provenance only", provenance)
		}
	})
}

func assertMigratedLineageKinds(t *testing.T, db *GlobalDB, want map[string]store.LineageKind) {
	t.Helper()

	sessions, err := db.ListSessions(t.Context(), SessionListQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: "ws-lineage-migration",
	})
	if err != nil {
		t.Fatalf("ListSessions(after migration) error = %v", err)
	}
	if len(sessions) != len(want) {
		t.Fatalf("ListSessions(after migration) count = %d, want %d", len(sessions), len(want))
	}
	for _, info := range sessions {
		if info.Lineage == nil || info.Lineage.Kind != want[info.ID] {
			t.Fatalf("session %q lineage = %#v, want kind %q", info.ID, info.Lineage, want[info.ID])
		}
	}
}
