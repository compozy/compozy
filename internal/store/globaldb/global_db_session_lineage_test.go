package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBSessionLineagePersistsAfterReopenAndFilters(t *testing.T) {
	t.Parallel()

	t.Run("Should persist lineage after reopen and filter by parent root and role", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		globalDB, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(initial) error = %v", err)
		}
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"lineage-workspace",
			filepath.Join(t.TempDir(), "workspace-lineage"),
		)
		now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
		ttl := now.Add(90 * time.Minute)

		if err := globalDB.RegisterSession(ctx, SessionInfo{
			ID:          "sess-root",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: workspaceID,
			SessionType: "user",
			State:       "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("RegisterSession(root) error = %v", err)
		}
		if err := globalDB.RegisterSession(ctx, SessionInfo{
			ID:          "sess-child",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: workspaceID,
			SessionType: "spawned",
			Lineage: &store.SessionLineage{
				ParentSessionID:  "sess-root",
				RootSessionID:    "sess-root",
				SpawnDepth:       1,
				SpawnRole:        "worker",
				TTLExpiresAt:     &ttl,
				AutoStopOnParent: true,
				SpawnBudget: store.SessionSpawnBudget{
					MaxChildren:           2,
					MaxDepth:              1,
					TTLSeconds:            int64(ttl.Sub(now).Seconds()),
					MaxActivePerWorkspace: 3,
				},
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:           []string{"agh__task_update", "agh__skill_view"},
					Skills:          []string{"go"},
					MCPServers:      []string{"memory"},
					WorkspacePaths:  []string{"/repo"},
					NetworkChannels: []string{"coord"},
					SandboxProfiles: []string{"local"},
				},
			},
			State:     "active",
			CreatedAt: now.Add(time.Minute),
			UpdatedAt: now.Add(time.Minute),
		}); err != nil {
			t.Fatalf("RegisterSession(child) error = %v", err)
		}
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(initial) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("Close(reopened) error = %v", closeErr)
			}
		})

		spawned, err := reopened.ListSessions(ctx, SessionListQuery{
			SessionType:     "spawned",
			RootSessionID:   "sess-root",
			ParentSessionID: "sess-root",
			SpawnRole:       "worker",
		})
		if err != nil {
			t.Fatalf("ListSessions(spawned filters) error = %v", err)
		}
		if got, want := len(spawned), 1; got != want {
			t.Fatalf("len(spawned) = %d, want %d", got, want)
		}
		lineage := spawned[0].Lineage
		if lineage == nil {
			t.Fatal("spawned[0].Lineage = nil, want metadata")
		}
		if lineage.ParentSessionID != "sess-root" ||
			lineage.RootSessionID != "sess-root" ||
			lineage.SpawnDepth != 1 ||
			lineage.SpawnRole != "worker" ||
			!lineage.AutoStopOnParent {
			t.Fatalf("lineage = %#v", lineage)
		}
		if lineage.TTLExpiresAt == nil || !lineage.TTLExpiresAt.Equal(ttl) {
			t.Fatalf("TTLExpiresAt = %#v, want %s", lineage.TTLExpiresAt, ttl)
		}
		if lineage.SpawnBudget.MaxChildren != 2 ||
			lineage.SpawnBudget.MaxDepth != 1 ||
			lineage.SpawnBudget.MaxActivePerWorkspace != 3 {
			t.Fatalf("spawn budget = %#v", lineage.SpawnBudget)
		}
		if got := lineage.PermissionPolicy.Tools; len(got) != 2 ||
			got[0] != "agh__skill_view" ||
			got[1] != "agh__task_update" {
			t.Fatalf("policy tools = %#v, want stable policy atoms", got)
		}

		roots, err := reopened.ListSessions(ctx, SessionListQuery{SessionType: "user", RootSessionID: "sess-root"})
		if err != nil {
			t.Fatalf("ListSessions(root filter) error = %v", err)
		}
		if len(roots) != 1 || roots[0].Lineage == nil || roots[0].Lineage.ParentSessionID != "" ||
			roots[0].Lineage.RootSessionID != "sess-root" || roots[0].Lineage.SpawnDepth != 0 {
			t.Fatalf("root sessions = %#v", roots)
		}
	})
}
