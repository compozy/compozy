package globaldb

import (
	"errors"
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

		if err := globalDB.RegisterSession(ctx, new(SessionInfo{
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
		})); err != nil {
			t.Fatalf("RegisterSession(root) error = %v", err)
		}
		if err := globalDB.RegisterSession(ctx, new(SessionInfo{
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
				Kind:             store.LineageKindSpawn,
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
					Tools:          []string{"compozy__task_update", "compozy__skill_view"},
					Skills:         []string{"go"},
					MCPServers:     []string{"memory"},
					WorkspacePaths: []string{"/repo"},
				},
			},
			State:     "active",
			CreatedAt: now.Add(time.Minute),
			UpdatedAt: now.Add(time.Minute),
		})); err != nil {
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
			LineageKind:     store.LineageKindSpawn,
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
			lineage.Kind != store.LineageKindSpawn ||
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
		// Migration owns its own budget; verification starts after it completes.
		ctx = globalMigrationTestContext(t)
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
		if err := upgraded.RegisterSession(ctx, &(child)); err != nil {
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
		ctx = globalMigrationTestContext(t)
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

func TestGlobalDBUpdateSessionLineages(t *testing.T) {
	t.Parallel()

	register := func(t *testing.T, globalDB *GlobalDB, workspaceID string, id string, lineage *store.SessionLineage) {
		t.Helper()
		now := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
		if err := globalDB.RegisterSession(testutil.Context(t), new(SessionInfo{
			ProfileID:     store.DefaultProfileID,
			ID:            id,
			AgentName:     "coder",
			Provider:      "claude",
			RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID:   workspaceID,
			SessionType:   "user",
			State:         "stopped",
			Lineage:       lineage,
			CreatedAt:     now,
			UpdatedAt:     now,
		})); err != nil {
			t.Fatalf("RegisterSession(%s) error = %v", id, err)
		}
	}
	lineageOf := func(t *testing.T, globalDB *GlobalDB, id string) store.SessionLineage {
		t.Helper()
		rows, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			ID:        id,
		})
		if err != nil || len(rows) != 1 || rows[0].Lineage == nil {
			t.Fatalf("ListSessions(%s) = %#v, %v", id, rows, err)
		}
		return *rows[0].Lineage
	}
	setup := func(t *testing.T) (*GlobalDB, string) {
		t.Helper()
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t, globalDB, "lineage-update", filepath.Join(t.TempDir(), "workspace-lineage-update"),
		)
		register(t, globalDB, workspaceID, "sess-parent", nil)
		register(t, globalDB, workspaceID, "sess-source", nil)
		register(t, globalDB, workspaceID, "sess-child", &store.SessionLineage{
			ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
			Kind: store.LineageKindProvenance,
		})
		return globalDB, workspaceID
	}

	t.Run("Should rewrite hierarchy columns for every session in the batch", func(t *testing.T) {
		t.Parallel()

		globalDB, workspaceID := setup(t)
		updatedAt := time.Date(2026, 10, 22, 10, 0, 0, 0, time.UTC)
		err := globalDB.UpdateSessionLineages(testutil.Context(t), workspaceID, []store.SessionLineageUpdate{
			{SessionID: "sess-source", UpdatedAt: updatedAt, Lineage: store.SessionLineage{
				ParentSessionID: "sess-parent", RootSessionID: "sess-parent", SpawnDepth: 1,
				Kind: store.LineageKindProvenance, NotifyCreator: true,
			}},
			{SessionID: "sess-child", UpdatedAt: updatedAt, Lineage: store.SessionLineage{
				ParentSessionID: "sess-source", RootSessionID: "sess-parent", SpawnDepth: 2,
				Kind: store.LineageKindProvenance,
			}},
		})
		if err != nil {
			t.Fatalf("UpdateSessionLineages() error = %v", err)
		}
		source := lineageOf(t, globalDB, "sess-source")
		if source.ParentSessionID != "sess-parent" || source.RootSessionID != "sess-parent" ||
			source.SpawnDepth != 1 || source.Kind != store.LineageKindProvenance {
			t.Fatalf("source lineage = %#v, want adopted by sess-parent", source)
		}
		child := lineageOf(t, globalDB, "sess-child")
		if child.ParentSessionID != "sess-source" || child.RootSessionID != "sess-parent" || child.SpawnDepth != 2 {
			t.Fatalf("child lineage = %#v, want re-rooted at sess-parent depth 2", child)
		}
	})

	t.Run("Should leave every row unchanged when one session is missing or invalid", func(t *testing.T) {
		t.Parallel()

		globalDB, workspaceID := setup(t)
		adopt := store.SessionLineageUpdate{SessionID: "sess-source", Lineage: store.SessionLineage{
			ParentSessionID: "sess-parent", RootSessionID: "sess-parent", SpawnDepth: 1,
			Kind: store.LineageKindProvenance,
		}}
		err := globalDB.UpdateSessionLineages(testutil.Context(t), workspaceID, []store.SessionLineageUpdate{
			adopt,
			{SessionID: "sess-missing", Lineage: store.SessionLineage{RootSessionID: "sess-missing"}},
		})
		if !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("UpdateSessionLineages(missing) error = %v, want %v", err, store.ErrSessionNotFound)
		}
		err = globalDB.UpdateSessionLineages(testutil.Context(t), "ws-foreign", []store.SessionLineageUpdate{adopt})
		if !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("UpdateSessionLineages(foreign workspace) error = %v, want %v", err, store.ErrSessionNotFound)
		}
		err = globalDB.UpdateSessionLineages(testutil.Context(t), workspaceID, []store.SessionLineageUpdate{
			{SessionID: "sess-source", Lineage: store.SessionLineage{ParentSessionID: "sess-parent"}},
		})
		if err == nil {
			t.Fatal("UpdateSessionLineages(invalid lineage) error = nil, want validation failure")
		}
		if source := lineageOf(t, globalDB, "sess-source"); source.ParentSessionID != "" {
			t.Fatalf("source lineage after failed batches = %#v, want unchanged root", source)
		}
	})
}

func TestGlobalDBSessionDerivationReceipts(t *testing.T) {
	t.Parallel()

	t.Run("Should register a derived child with its receipt atomically and tombstone it on delete", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), GlobalDatabaseName))
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := globalDB.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("Close() error = %v", closeErr)
			}
		})
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"derive-workspace",
			filepath.Join(t.TempDir(), "workspace-derive"),
		)
		now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
		derivedInfo := func(id string) SessionInfo {
			return SessionInfo{
				ProfileID: store.DefaultProfileID, ID: id, AgentName: "claude-code", Provider: "claude",
				RuntimeStatus: store.SessionRuntimeUnbound, WorkspaceID: workspaceID, SessionType: "user",
				Lineage: &store.SessionLineage{
					ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
					Kind: store.LineageKindContinue, OriginAgentName: "codex",
				},
				State: "active", CreatedAt: now, UpdatedAt: now,
			}
		}
		identity := store.SessionCreationIdentity{
			CreationProfileRef: "profile:sha256:derive",
			PolicySpecDigest:   "policy:sha256:derive",
			CreationDigest:     "creation:sha256:derive",
		}
		receiptFor := func(childID string) store.SessionDerivationReceipt {
			return store.SessionDerivationReceipt{
				WorkspaceID: workspaceID, ProfileID: store.DefaultProfileID, IdempotencyKey: "idem-derive",
				RequestFingerprint: "sha256:request", SourceSessionID: "sess-source", ChildSessionID: childID,
				Kind: store.LineageKindContinue,
				Outcome: store.SessionDerivationOutcome{
					Seed: store.SessionDerivationSeedReplay, ThroughTurnID: "turn-1", ReplayMessageCount: 4,
					ReplayBytes: 512, SourceEpoch: 1, SourceGeneration: 2, SourceMaxSequence: 9,
					FirstPrompt: store.SessionDerivationFirstPromptStaged,
				},
				CreatedAt: now,
			}
		}
		if err := globalDB.RegisterSession(ctx, new(SessionInfo{
			ProfileID: store.DefaultProfileID, ID: "sess-source", AgentName: "codex", Provider: "codex",
			RuntimeStatus: store.SessionRuntimeUnbound, WorkspaceID: workspaceID, SessionType: "user",
			State: "active", CreatedAt: now, UpdatedAt: now,
		})); err != nil {
			t.Fatalf("RegisterSession(source) error = %v", err)
		}

		if err := globalDB.RegisterDerivedSession(
			ctx, new(derivedInfo("sess-child")), identity, receiptFor("sess-child"),
		); err != nil {
			t.Fatalf("RegisterDerivedSession(first) error = %v", err)
		}
		receipt, found, err := globalDB.SessionDerivationReceipt(
			ctx,
			workspaceID,
			"idem-derive",
			store.DefaultProfileID,
		)
		if err != nil || !found {
			t.Fatalf("SessionDerivationReceipt() = found %v, error %v; want recorded receipt", found, err)
		}
		if receipt.ChildSessionID != "sess-child" || receipt.Kind != store.LineageKindContinue ||
			receipt.Outcome != receiptFor("sess-child").Outcome || receipt.ChildDeletedAt != nil {
			t.Fatalf("SessionDerivationReceipt() = %#v, want the recorded continue outcome", receipt)
		}
		gotIdentity, err := globalDB.GetSessionCreationIdentity(ctx, "sess-child")
		if err != nil || gotIdentity != identity {
			t.Fatalf("GetSessionCreationIdentity(child) = %#v, %v; want committed identity", gotIdentity, err)
		}

		err = globalDB.RegisterDerivedSession(ctx, new(derivedInfo("sess-racer")), identity, receiptFor("sess-racer"))
		if !errors.Is(err, store.ErrSessionDerivationExists) {
			t.Fatalf("RegisterDerivedSession(duplicate key) error = %v, want ErrSessionDerivationExists", err)
		}
		racers, err := globalDB.ListSessions(ctx, SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, ID: "sess-racer",
		})
		if err != nil || len(racers) != 0 {
			t.Fatalf("ListSessions(racer) = %#v, %v; want no catalog row for the losing registration", racers, err)
		}

		if err := globalDB.DeleteSession(ctx, "sess-child"); err != nil {
			t.Fatalf("DeleteSession(child) error = %v", err)
		}
		tombstoned, found, err := globalDB.SessionDerivationReceipt(
			ctx,
			workspaceID,
			"idem-derive",
			store.DefaultProfileID,
		)
		if err != nil || !found || tombstoned.ChildDeletedAt == nil ||
			tombstoned.Outcome != receiptFor("sess-child").Outcome {
			t.Fatalf("SessionDerivationReceipt(after child delete) = %#v, found %v, error %v", tombstoned, found, err)
		}
		if _, found, err := globalDB.SessionDerivationReceipt(
			ctx,
			workspaceID,
			"idem-missing",
			store.DefaultProfileID,
		); err != nil ||
			found {
			t.Fatalf("SessionDerivationReceipt(missing) = found %v, error %v; want not found", found, err)
		}
	})
}

func TestGlobalDBUserDerivationReceipts(t *testing.T) {
	t.Run("Should isolate identical User idempotency keys by profile", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		profiles := []string{store.DefaultProfileID, insertLayoutTestProfile(ctx, t, db)}
		for _, profile := range profiles {
			childID := "child-" + profile
			now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
			info := SessionInfo{
				ID:            childID,
				ProfileID:     profile,
				AgentName:     "coder",
				Provider:      "codex",
				RuntimeStatus: store.SessionRuntimeUnbound,
				SessionType:   "user",
				State:         "active",
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			identity := store.SessionCreationIdentity{
				CreationProfileRef: "profile:sha256:derive",
				PolicySpecDigest:   "policy:sha256:derive",
				CreationDigest:     "creation:sha256:derive",
			}
			receipt := store.SessionDerivationReceipt{
				ProfileID:          profile,
				IdempotencyKey:     "same-key",
				RequestFingerprint: "sha256:request",
				SourceSessionID:    "source-" + profile,
				ChildSessionID:     childID,
				Kind:               store.LineageKindFork,
				CreatedAt:          now,
				Outcome: store.SessionDerivationOutcome{
					Seed:        store.SessionDerivationSeedReplay,
					FirstPrompt: store.SessionDerivationFirstPromptStaged,
				},
			}
			if err := db.RegisterDerivedSession(ctx, &(info), identity, receipt); err != nil {
				t.Fatal(err)
			}
		}
		for _, profile := range profiles {
			receipt, found, err := db.SessionDerivationReceipt(ctx, "", "same-key", profile)
			if err != nil || !found || receipt.ChildSessionID != "child-"+profile {
				t.Fatalf("receipt=%+v found=%t err=%v", receipt, found, err)
			}
		}
	})
}
