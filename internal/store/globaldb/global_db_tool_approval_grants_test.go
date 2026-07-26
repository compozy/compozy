package globaldb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestGlobalDBToolApprovalGrants(t *testing.T) {
	t.Parallel()

	t.Run("Should upsert exact grants and touch the most-specific match", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "approval-upsert", t.TempDir())
		current := approvalGrantTestTime()
		globalDB.now = func() time.Time { return current }
		key := approvalGrantTestKey(workspaceID, "codex", approvalGrantDigest("exact"))

		created, err := globalDB.PutApprovalGrant(ctx, toolspkg.ApprovalGrant{
			ApprovalGrantKey: key,
			Decision:         toolspkg.ApprovalGrantAllow,
		})
		if err != nil {
			t.Fatalf("PutApprovalGrant() error = %v", err)
		}
		if created.ID == "" || !created.CreatedAt.Equal(current) || !created.LastUsedAt.Equal(current) {
			t.Fatalf("PutApprovalGrant() = %#v, want materialized identity and timestamps", created)
		}

		current = current.Add(time.Minute)
		found, ok, err := globalDB.LookupApprovalGrant(ctx, key)
		if err != nil {
			t.Fatalf("LookupApprovalGrant() error = %v", err)
		}
		if !ok || found.ID != created.ID || !found.LastUsedAt.Equal(current) {
			t.Fatalf("LookupApprovalGrant() = %#v, %t, want touched exact grant", found, ok)
		}

		current = current.Add(time.Minute)
		replaced, err := globalDB.PutApprovalGrant(ctx, toolspkg.ApprovalGrant{
			ApprovalGrantKey: key,
			Decision:         toolspkg.ApprovalGrantReject,
		})
		if err != nil {
			t.Fatalf("PutApprovalGrant(replace) error = %v", err)
		}
		if replaced.ID != created.ID || replaced.Decision != toolspkg.ApprovalGrantReject {
			t.Fatalf("PutApprovalGrant(replace) = %#v, want stable id and reject decision", replaced)
		}
		listed, err := globalDB.ListApprovalGrants(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ListApprovalGrants() error = %v", err)
		}
		if len(listed) != 1 || listed[0].ID != created.ID {
			t.Fatalf("ListApprovalGrants() = %#v, want one upserted row", listed)
		}
	})

	t.Run("Should resolve exact agent digest then agent digest and tool width", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "approval-specificity", t.TempDir())
		current := approvalGrantTestTime()
		globalDB.now = func() time.Time { return current }
		exactDigest := approvalGrantDigest("exact")
		otherDigest := approvalGrantDigest("other")
		grants := []toolspkg.ApprovalGrant{
			{
				ApprovalGrantKey: approvalGrantTestKey(workspaceID, "", ""),
				Decision:         toolspkg.ApprovalGrantReject,
			},
			{
				ApprovalGrantKey: approvalGrantTestKey(workspaceID, "", exactDigest),
				Decision:         toolspkg.ApprovalGrantAllow,
			},
			{
				ApprovalGrantKey: approvalGrantTestKey(workspaceID, "codex", ""),
				Decision:         toolspkg.ApprovalGrantReject,
			},
			{
				ApprovalGrantKey: approvalGrantTestKey(workspaceID, "codex", exactDigest),
				Decision:         toolspkg.ApprovalGrantAllow,
			},
		}
		for i, grant := range grants {
			current = current.Add(time.Second)
			if _, err := globalDB.PutApprovalGrant(ctx, grant); err != nil {
				t.Fatalf("PutApprovalGrant(%d) error = %v", i, err)
			}
		}

		assertApprovalGrantLookup(
			t,
			globalDB,
			approvalGrantTestKey(workspaceID, "codex", exactDigest),
			"codex",
			exactDigest,
		)
		assertApprovalGrantLookup(t, globalDB, approvalGrantTestKey(workspaceID, "codex", otherDigest), "codex", "")
		assertApprovalGrantLookup(
			t,
			globalDB,
			approvalGrantTestKey(workspaceID, "claude", exactDigest),
			"",
			exactDigest,
		)
		assertApprovalGrantLookup(t, globalDB, approvalGrantTestKey(workspaceID, "claude", otherDigest), "", "")
	})

	t.Run("Should isolate list and revoke by workspace and cascade on workspace deletion", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "approval-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "approval-b", t.TempDir())
		grantA := putApprovalGrantForTest(t, globalDB, toolspkg.ApprovalGrant{
			ApprovalGrantKey: approvalGrantTestKey(workspaceA, "codex", approvalGrantDigest("a")),
			Decision:         toolspkg.ApprovalGrantAllow,
		})
		putApprovalGrantForTest(t, globalDB, toolspkg.ApprovalGrant{
			ApprovalGrantKey: approvalGrantTestKey(workspaceB, "codex", approvalGrantDigest("b")),
			Decision:         toolspkg.ApprovalGrantReject,
		})

		listedA, err := globalDB.ListApprovalGrants(ctx, workspaceA)
		if err != nil {
			t.Fatalf("ListApprovalGrants(workspace A) error = %v", err)
		}
		listedB, err := globalDB.ListApprovalGrants(ctx, workspaceB)
		if err != nil {
			t.Fatalf("ListApprovalGrants(workspace B) error = %v", err)
		}
		if len(listedA) != 1 || len(listedB) != 1 || listedA[0].WorkspaceID == listedB[0].WorkspaceID {
			t.Fatalf("workspace lists = %#v / %#v, want isolated rows", listedA, listedB)
		}
		if err := globalDB.RevokeApprovalGrant(ctx, workspaceB, grantA.ID); !errors.Is(
			err,
			toolspkg.ErrApprovalGrantNotFound,
		) {
			t.Fatalf("RevokeApprovalGrant(cross-workspace) error = %v, want not found", err)
		}
		if err := globalDB.RevokeApprovalGrant(ctx, workspaceA, grantA.ID); err != nil {
			t.Fatalf("RevokeApprovalGrant() error = %v", err)
		}
		if err := globalDB.RevokeApprovalGrant(ctx, workspaceA, grantA.ID); !errors.Is(
			err,
			toolspkg.ErrApprovalGrantNotFound,
		) {
			t.Fatalf("RevokeApprovalGrant(missing) error = %v, want not found", err)
		}

		putApprovalGrantForTest(t, globalDB, toolspkg.ApprovalGrant{
			ApprovalGrantKey: approvalGrantTestKey(workspaceA, "codex", approvalGrantDigest("cascade")),
			Decision:         toolspkg.ApprovalGrantAllow,
		})
		if err := globalDB.DeleteWorkspace(ctx, workspaceA); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM tool_approval_grants WHERE workspace_id = ?`,
			workspaceA,
		).Scan(&count); err != nil {
			t.Fatalf("query approval grant cascade count error = %v", err)
		}
		if count != 0 {
			t.Fatalf("approval grant cascade count = %d, want 0", count)
		}
	})
}

func TestGlobalDBToolApprovalGrantMigration(t *testing.T) {
	t.Run("Should preserve durable grants through the head upgrade", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, previousApprovalGrantMigrationStream(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v16 prefix) error = %v", err)
		}
		ctx := testutil.Context(t)
		prefixGlobalDB := &GlobalDB{db: prefixDB, path: path, now: approvalGrantTestTime}
		prefixGlobalDB.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(t, prefixGlobalDB, "approval-upgrade", t.TempDir())
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(v17 upgrade) error = %v", err)
		}
		ctx = testutil.Context(t)
		created := putApprovalGrantForTest(t, globalDB, toolspkg.ApprovalGrant{
			ApprovalGrantKey: approvalGrantTestKey(workspaceID, "codex", approvalGrantDigest("upgrade")),
			Decision:         toolspkg.ApprovalGrantAllow,
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
		listed, err := reopened.ListApprovalGrants(ctx, workspaceID)
		if err != nil {
			t.Fatalf("ListApprovalGrants(reopen) error = %v", err)
		}
		if len(listed) != 1 || listed[0].ID != created.ID {
			t.Fatalf("ListApprovalGrants(reopen) = %#v, want durable grant", listed)
		}
		status, err := store.Status(ctx, reopened.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(latest) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
	})
}

func previousApprovalGrantMigrationStream(t *testing.T) store.MigrationStream {
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
		"00016_schema.sql",
	)
}

func putApprovalGrantForTest(
	t *testing.T,
	globalDB *GlobalDB,
	grant toolspkg.ApprovalGrant,
) toolspkg.ApprovalGrant {
	t.Helper()

	created, err := globalDB.PutApprovalGrant(testutil.Context(t), grant)
	if err != nil {
		t.Fatalf("PutApprovalGrant() error = %v", err)
	}
	return created
}

func assertApprovalGrantLookup(
	t *testing.T,
	globalDB *GlobalDB,
	key toolspkg.ApprovalGrantKey,
	wantAgent string,
	wantDigest string,
) {
	t.Helper()

	grant, found, err := globalDB.LookupApprovalGrant(testutil.Context(t), key)
	if err != nil {
		t.Fatalf("LookupApprovalGrant() error = %v", err)
	}
	if !found || grant.AgentName != wantAgent || grant.InputDigest != wantDigest {
		t.Fatalf("LookupApprovalGrant() = %#v, %t, want agent=%q digest=%q", grant, found, wantAgent, wantDigest)
	}
}

func approvalGrantTestKey(workspaceID, agentName, digest string) toolspkg.ApprovalGrantKey {
	return toolspkg.ApprovalGrantKey{
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		ToolID:      toolspkg.ToolIDWorkspaceList,
		InputDigest: digest,
	}
}

func approvalGrantDigest(seed string) string {
	return "sha256:" + seed
}

func approvalGrantTestTime() time.Time {
	return time.Date(2026, 7, 15, 18, 0, 0, 0, time.UTC)
}
