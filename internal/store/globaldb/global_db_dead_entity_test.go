package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBDeadEntityStore(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate marks by workspace and refresh one entity in place", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "dead-entity-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "dead-entity-b", t.TempDir())
		firstMarkedAt := deadEntityTestTime()

		markDeadEntityForTest(t, globalDB, store.DeadEntity{
			DeadEntityKey: store.DeadEntityKey{
				WorkspaceID: workspaceA,
				Kind:        store.DeadEntityKindMCPSidecar,
				EntityID:    "github",
			},
			Reason:   "process exited permanently",
			MarkedAt: firstMarkedAt,
		})
		markDeadEntityForTest(t, globalDB, store.DeadEntity{
			DeadEntityKey: store.DeadEntityKey{
				WorkspaceID: workspaceB,
				Kind:        store.DeadEntityKindMCPSidecar,
				EntityID:    "github",
			},
			Reason:   "workspace-b failure",
			MarkedAt: firstMarkedAt.Add(time.Minute),
		})
		markDeadEntityForTest(t, globalDB, store.DeadEntity{
			DeadEntityKey: store.DeadEntityKey{
				WorkspaceID: workspaceA,
				Kind:        store.DeadEntityKindBridge,
				EntityID:    "telegram",
			},
			Reason:   "bridge credentials rejected",
			MarkedAt: firstMarkedAt.Add(2 * time.Minute),
		})

		refreshedAt := firstMarkedAt.Add(3 * time.Minute)
		markDeadEntityForTest(t, globalDB, store.DeadEntity{
			DeadEntityKey: store.DeadEntityKey{
				WorkspaceID: workspaceA,
				Kind:        store.DeadEntityKindMCPSidecar,
				EntityID:    "github",
			},
			Reason:   "configuration remains invalid",
			MarkedAt: refreshedAt,
		})

		got, found, err := globalDB.FindDeadEntity(
			ctx,
			workspaceA,
			store.DeadEntityKindMCPSidecar,
			"github",
		)
		if err != nil {
			t.Fatalf("FindDeadEntity() error = %v", err)
		}
		if !found {
			t.Fatal("FindDeadEntity() found = false, want true")
		}
		if got.Reason != "configuration remains invalid" || !got.MarkedAt.Equal(refreshedAt) {
			t.Fatalf("FindDeadEntity() = %#v, want refreshed reason and timestamp", got)
		}

		listedA, err := globalDB.ListDeadEntities(ctx, workspaceA)
		if err != nil {
			t.Fatalf("ListDeadEntities(workspace A) error = %v", err)
		}
		if len(listedA) != 2 {
			t.Fatalf("ListDeadEntities(workspace A) = %#v, want two rows", listedA)
		}
		if listedA[0].EntityID != "github" || listedA[1].EntityID != "telegram" {
			t.Fatalf("ListDeadEntities(workspace A) order = %#v, want newest first", listedA)
		}
		listedB, err := globalDB.ListDeadEntities(ctx, workspaceB)
		if err != nil {
			t.Fatalf("ListDeadEntities(workspace B) error = %v", err)
		}
		if len(listedB) != 1 || listedB[0].Reason != "workspace-b failure" {
			t.Fatalf("ListDeadEntities(workspace B) = %#v, want isolated row", listedB)
		}
	})

	t.Run("Should clear idempotently and cascade marks with their workspace", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "dead-entity-clear", t.TempDir())
		entity := store.DeadEntity{
			DeadEntityKey: store.DeadEntityKey{
				WorkspaceID: workspaceID,
				Kind:        store.DeadEntityKindExtension,
				EntityID:    "audit-extension",
			},
			Reason:   "extension startup failed",
			MarkedAt: deadEntityTestTime(),
		}
		markDeadEntityForTest(t, globalDB, entity)

		if err := globalDB.ClearDeadEntity(ctx, workspaceID, entity.Kind, entity.EntityID); err != nil {
			t.Fatalf("ClearDeadEntity() error = %v", err)
		}
		if err := globalDB.ClearDeadEntity(ctx, workspaceID, entity.Kind, entity.EntityID); err != nil {
			t.Fatalf("ClearDeadEntity(missing) error = %v", err)
		}
		_, found, err := globalDB.FindDeadEntity(ctx, workspaceID, entity.Kind, entity.EntityID)
		if err != nil || found {
			t.Fatalf("FindDeadEntity(after clear) found = %t, error = %v, want false/nil", found, err)
		}

		markDeadEntityForTest(t, globalDB, entity)
		if err := globalDB.DeleteWorkspace(ctx, workspaceID); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM dead_entities WHERE workspace_id = ?`,
			workspaceID,
		).Scan(&count); err != nil {
			t.Fatalf("query dead entity cascade count error = %v", err)
		}
		if count != 0 {
			t.Fatalf("dead entity cascade count = %d, want 0", count)
		}
	})
}

func TestGlobalDBDeadEntityMigration(t *testing.T) {
	t.Run("Should preserve workspace ownership through the dead-entity upgrade", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, previousDeadEntityMigrationStream(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v15 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: deadEntityTestTime}
		prefixGlobalDB.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(t, prefixGlobalDB, "dead-entity-upgrade", t.TempDir())
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v16 upgrade) error = %v", err)
		}
		ctx = testutil.Context(t)
		markDeadEntityForTest(t, globalDB, store.DeadEntity{
			DeadEntityKey: store.DeadEntityKey{
				WorkspaceID: workspaceID,
				Kind:        store.DeadEntityKindMCPSidecar,
				EntityID:    "upgrade-sidecar",
			},
			Reason:   "post-upgrade mark",
			MarkedAt: deadEntityTestTime(),
		})
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("GlobalDB.Close(upgrade) error = %v", err)
		}

		reopened, err := OpenGlobalDB(testutil.Context(t), path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		ctx = testutil.Context(t)
		t.Cleanup(func() {
			if err := reopened.Close(ctx); err != nil {
				t.Errorf("reopened.Close() error = %v", err)
			}
		})
		entity, found, err := reopened.FindDeadEntity(
			ctx,
			workspaceID,
			store.DeadEntityKindMCPSidecar,
			"upgrade-sidecar",
		)
		if err != nil {
			t.Fatalf("FindDeadEntity(reopen) error = %v", err)
		}
		if !found || entity.Reason != "post-upgrade mark" {
			t.Fatalf("FindDeadEntity(reopen) = %#v, %t, want durable mark", entity, found)
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(latest) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
}

func previousDeadEntityMigrationStream(t *testing.T) store.MigrationStream {
	t.Helper()

	return globalMigrationPrefix(t,
		"00001_baseline.sql",
		"00002_schema.sql",
		"00003_schema.sql",
		"00004_schema.sql",
		"00005_schema.sql",
		"00006_schema.sql",
		"00007_schema.sql",
		"00008_network_participation.sql",
		"00009_automation_network_participation.sql",
		"00010_network_participation_hardening.sql",
		"00011_schema.sql",
		"00012_schema.sql",
		"00013_network_subscription_hard_cut.sql",
		"00014_network_task_status_projections.sql",
		"00015_schema.sql",
	)
}

func markDeadEntityForTest(t *testing.T, globalDB *GlobalDB, entity store.DeadEntity) {
	t.Helper()

	if err := globalDB.MarkDeadEntity(testutil.Context(t), entity); err != nil {
		t.Fatalf("MarkDeadEntity(%q/%q/%q) error = %v", entity.WorkspaceID, entity.Kind, entity.EntityID, err)
	}
}

func deadEntityTestTime() time.Time {
	return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
}
