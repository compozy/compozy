package globaldb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
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
			ProfileID:     store.DefaultProfileID,
			ID:            "sess-root",
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			SessionType:   "user",
			State:         "active",
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			t.Fatalf("RegisterSession(root) error = %v", err)
		}
		if err := globalDB.RegisterSession(ctx, SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            "sess-child",
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			SessionType:   "spawned",
			Lineage: &store.SessionLineage{
				ParentSessionID:  "sess-root",
				RootSessionID:    "sess-root",
				SpawnDepth:       1,
				SpawnRole:        "worker",
				TTLExpiresAt:     &ttl,
				AutoStopOnParent: true,
				NotifyCreator:    true,
				SpawnBudget: store.SessionSpawnBudget{
					MaxChildren:           2,
					MaxDepth:              1,
					TTLSeconds:            int64(ttl.Sub(now).Seconds()),
					MaxActivePerWorkspace: 3,
				},
				PermissionPolicy: store.SessionPermissionPolicy{
					Tools:           []string{"compozy__task_update", "compozy__skill_view"},
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
			ReadScope:       store.ReadScope{ProfileID: store.DefaultProfileID},
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
			!lineage.AutoStopOnParent ||
			!lineage.NotifyCreator {
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
			got[0] != "compozy__skill_view" ||
			got[1] != "compozy__task_update" {
			t.Fatalf("policy tools = %#v, want stable policy atoms", got)
		}

		roots, err := reopened.ListSessions(ctx, SessionListQuery{
			ReadScope:     store.ReadScope{ProfileID: store.DefaultProfileID},
			SessionType:   "user",
			RootSessionID: "sess-root",
		})
		if err != nil {
			t.Fatalf("ListSessions(root filter) error = %v", err)
		}
		if len(roots) != 1 || roots[0].Lineage == nil || roots[0].Lineage.ParentSessionID != "" ||
			roots[0].Lineage.RootSessionID != "sess-root" || roots[0].Lineage.SpawnDepth != 0 {
			t.Fatalf("root sessions = %#v", roots)
		}
	})

	t.Run("Should default existing children to notify creator and preserve opt out after reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00066_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00066 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		now := store.FormatTimestamp(time.Date(2026, 8, 16, 9, 0, 0, 0, time.UTC))
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
			id, root_dir, add_dirs, name, created_at, updated_at
		) VALUES ('ws-notify-migration', ?, '[]', 'notify-migration', ?, ?)`,
			t.TempDir(), now, now); err != nil {
			t.Fatalf("seed workspace before 00066 error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO sessions (
			id, name, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES ('sess-notify-root', 'Root', 'creator', 'ws-notify-migration', 'user', 'active', ?, ?)`,
			now, now); err != nil {
			t.Fatalf("seed root before 00066 error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO sessions (
			id, name, agent_name, workspace_id, session_type, state,
			parent_session_id, root_session_id, spawn_depth, spawn_role, created_at, updated_at
		) VALUES ('sess-notify-child', 'Child', 'worker', 'ws-notify-migration', 'spawned', 'active',
			'sess-notify-root', 'sess-notify-root', 1, 'worker', ?, ?)`, now, now); err != nil {
			t.Fatalf("seed child before 00066 error = %v", err)
		}
		columns, err := tableColumns(ctx, prefixDB, "sessions")
		if err != nil {
			t.Fatalf("tableColumns(sessions before 00066) error = %v", err)
		}
		if _, exists := columns["notify_creator"]; exists {
			t.Fatal("sessions.notify_creator exists before 00066")
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00066 error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00066 error = %v", err)
		}
		children, err := upgraded.ListSessions(ctx, SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			ID:        "sess-notify-child",
		})
		if err != nil {
			t.Fatalf("ListSessions(after migration) error = %v", err)
		}
		if len(children) != 1 || children[0].Lineage == nil || !children[0].Lineage.NotifyCreator {
			t.Fatalf("migrated child lineage = %#v, want notify_creator=true", children)
		}
		child := children[0]
		child.Lineage.NotifyCreator = false
		child.UpdatedAt = time.Date(2026, 8, 16, 9, 1, 0, 0, time.UTC)
		if err := upgraded.RegisterSession(ctx, child); err != nil {
			t.Fatalf("RegisterSession(opt out) error = %v", err)
		}
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(after migration) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded database error = %v", err)
		}

		reopened, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("reopen migrated database error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened database error = %v", closeErr)
			}
		})
		children, err = reopened.ListSessions(ctx, SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			ID:        "sess-notify-child",
		})
		if err != nil {
			t.Fatalf("ListSessions(after reopen) error = %v", err)
		}
		if len(children) != 1 || children[0].Lineage == nil || children[0].Lineage.NotifyCreator {
			t.Fatalf("reopened child lineage = %#v, want notify_creator=false", children)
		}
	})
}
