package globaldb

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

// Canonical suite: workspace-predicated worktree storage and SQL constraints.
func TestGlobalDBWorktreeStore(t *testing.T) {
	t.Parallel()

	t.Run("Should retain missing worktrees until every bound session is archived", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		ws := registerWorkspaceForGlobalTests(t, db, "missing", t.TempDir())
		at := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{ID: "wt-missing", ProfileID: store.DefaultProfileID, WorkspaceID: ws,
			Name: "missing", Path: t.TempDir(), State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual,
			SetupState: worktreepkg.SetupNone, CreatedAt: at, UpdatedAt: at}
		if err := db.Worktrees.Insert(ctx, item); err != nil {
			t.Fatal(err)
		}
		info := store.SessionInfo{
			ID:            "session-missing",
			ProfileID:     store.DefaultProfileID,
			WorkspaceID:   ws,
			WorktreeID:    item.ID,
			AgentName:     "coder",
			Provider:      "claude",
			State:         "stopped",
			RuntimeStatus: store.SessionRuntimeUnbound,
			CreatedAt:     at,
			UpdatedAt:     at,
		}
		if err := db.RegisterSession(ctx, &(info)); err != nil {
			t.Fatal(err)
		}
		if err := db.Worktrees.SetState(ctx, ws, item.ID, worktreepkg.StateMissing, at); err != nil {
			t.Fatal(err)
		}
		if removed, err := db.Worktrees.RemoveMissing(ctx, ws, item.ID, at); err != nil || removed {
			t.Fatalf("non-archived removal = %v, %v", removed, err)
		}
		if _, err := db.Worktrees.db.ExecContext(
			ctx,
			"UPDATE sessions SET archived_at = ? WHERE id = ?",
			store.FormatTimestamp(at),
			info.ID,
		); err != nil {
			t.Fatal(err)
		}
		if removed, err := db.Worktrees.RemoveMissing(ctx, "foreign", item.ID, at); err != nil || removed {
			t.Fatalf("foreign removal = %v, %v", removed, err)
		}
		if removed, err := db.Worktrees.RemoveMissing(ctx, ws, item.ID, at); err != nil || !removed {
			t.Fatalf("archived removal = %v, %v", removed, err)
		}
		got, err := db.Worktrees.Get(ctx, ws, item.ID)
		if err != nil || got.State != worktreepkg.StateDismissed {
			t.Fatalf("retained history = %#v, %v", got, err)
		}
	})

	// Invariant: upgrades preserve every legacy row while copying only provable identities.
	// Owner: global SQLite migration; canonical worktree store suite.
	t.Run("Should migrate PR identities losslessly and retain the newest collision", func(t *testing.T) {
		t.Parallel()
		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prior, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00153_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		const timestamp = "2026-10-01T12:00:00Z"
		_, err = prior.ExecContext(
			ctx,
			`INSERT INTO workspaces(id,root_dir,add_dirs,name,created_at,updated_at) VALUES('ws-pr',?,'[]','PR',?,?)`,
			t.TempDir(),
			timestamp,
			timestamp,
		)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range []struct {
			id, url, at string
			number      int
		}{{"a", "https://GitHub.com/Acme/Repo/pull/7", timestamp, 7}, {"b", "https://github.com/acme/repo/pull/8", "2026-10-02T12:00:00Z", 8}, {"c", "invalid", timestamp, 9}} {
			_, err = prior.ExecContext(
				ctx,
				`INSERT INTO worktrees(id,profile_id,workspace_id,name,branch,path,state,origin,setup_state,created_at,updated_at) VALUES(?,?,'ws-pr',?,'topic',?,'ready','manual','none',?,?)`,
				row.id,
				store.DefaultProfileID,
				row.id,
				t.TempDir(),
				timestamp,
				timestamp,
			)
			if err != nil {
				t.Fatal(err)
			}
			_, err = prior.ExecContext(
				ctx,
				`INSERT INTO worktree_forge_status(worktree_id,provider,pr_number,pr_state,pr_url,merged,fetched_at) VALUES(?,'github',?,'merged',?,1,?)`,
				row.id,
				row.number,
				row.url,
				row.at,
			)
			if err != nil {
				t.Fatal(err)
			}
		}
		if err = prior.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(testutil.Context(t)); err != nil {
				t.Error(err)
			}
		})
		var count int
		if err = upgraded.db.QueryRowContext(ctx, `SELECT count(*) FROM worktree_forge_status_legacy`).
			Scan(&count); err != nil ||
			count != 3 {
			t.Fatalf("legacy count=%d %v", count, err)
		}
		key := worktreepkg.PullRequestKey{WorkspaceID: "ws-pr", RepoKey: "github.com/acme/repo", Branch: "topic"}
		rows, err := upgraded.Worktrees.ReadPullRequests(ctx, []worktreepkg.PullRequestKey{key})
		if err != nil {
			t.Fatal(err)
		}
		facts := rows[key].Facts
		if facts == nil || facts.PRNumber == nil || *facts.PRNumber != 8 || facts.Merged == nil || !*facts.Merged {
			t.Fatalf("migrated winner=%#v", facts)
		}
	})

	// Invariant: expanding exit actions preserves released receipts and admits durable delivery.
	// Owner: GlobalDB SQLite migration/persistence; canonical worktree store suite.
	t.Run("Should preserve exit receipts and persist managed delivery across migration reopen", func(t *testing.T) {
		t.Parallel()
		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prior, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00125_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		const workspaceID = "ws-delivery-migration"
		const worktreeID = "wt-delivery-migration"
		const timestamp = "2026-09-29T12:00:00Z"
		if _, err := prior.ExecContext(ctx, `INSERT INTO workspaces
			(id, root_dir, add_dirs, name, created_at, updated_at) VALUES (?, ?, '[]', ?, ?, ?)`,
			workspaceID, t.TempDir(), workspaceID, timestamp, timestamp); err != nil {
			t.Fatal(err)
		}
		if _, err := prior.ExecContext(ctx, `INSERT INTO worktrees
			(id, profile_id, workspace_id, name, path, state, origin, setup_state, created_at, updated_at)
			VALUES (?, ?, ?, 'delivery-migration', ?, 'ready', 'manual', 'none', ?, ?)`,
			worktreeID, store.DefaultProfileID, workspaceID, t.TempDir(), timestamp, timestamp); err != nil {
			t.Fatal(err)
		}
		legacyActions := []string{"commit", "commit_push", "push", "open_pr"}
		for _, action := range legacyActions {
			if _, err := prior.ExecContext(ctx, `INSERT INTO worktree_exit_ops
				(op_id, workspace_id, worktree_id, action, state, started_at, finished_at)
				VALUES (?, ?, ?, ?, 'completed', ?, ?)`,
				"old-"+action, workspaceID, worktreeID, action, timestamp, timestamp); err != nil {
				t.Fatal(err)
			}
		}
		if err := prior.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		upgradedClosed := false
		t.Cleanup(func() {
			if !upgradedClosed {
				if err := upgraded.Close(testutil.Context(t)); err != nil {
					t.Errorf("close upgraded database: %v", err)
				}
			}
		})
		for _, action := range legacyActions {
			var storedAction, state, started, finished string
			if err := upgraded.db.QueryRowContext(ctx, `SELECT action, state, started_at, finished_at
				FROM worktree_exit_ops WHERE op_id = ? AND workspace_id = ? AND worktree_id = ?`,
				"old-"+action, workspaceID, worktreeID).Scan(&storedAction, &state, &started, &finished); err != nil {
				t.Fatal(err)
			}
			if storedAction != action || state != "completed" || started != timestamp || finished != timestamp {
				t.Fatalf("legacy receipt changed: %s %s %s %s", storedAction, state, started, finished)
			}
		}
		now := time.Date(2026, 9, 29, 12, 1, 0, 0, time.UTC)
		operation := worktreepkg.ExitOperation{ID: "managed-delivery", WorkspaceID: workspaceID,
			WorktreeID: worktreeID, Action: string(worktreepkg.ExitActionDeliver), State: "running", StartedAt: now}
		if err := upgraded.Worktrees.InsertExitOperation(ctx, operation); err != nil {
			t.Fatal(err)
		}
		conflict := operation
		conflict.ID = "competing-exit"
		conflict.Action = "push"
		if err := upgraded.Worktrees.InsertExitOperation(
			ctx,
			conflict,
		); !errors.Is(
			err,
			worktreepkg.ErrOperationInProgress,
		) {
			t.Fatalf("active exit uniqueness lost: %v", err)
		}
		status, err := store.Status(ctx, upgraded.db, MigrationStream())
		if err != nil {
			t.Fatal(err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		if err := upgraded.Close(ctx); err != nil {
			t.Fatal(err)
		}
		upgradedClosed = true
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("close reopened database: %v", err)
			}
		})
		running, err := reopened.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 1 || running[0].ID != operation.ID ||
			running[0].Action != string(worktreepkg.ExitActionDeliver) || running[0].WorkspaceID != workspaceID ||
			running[0].WorktreeID != worktreeID || !running[0].StartedAt.Equal(now) {
			t.Fatalf("reopened delivery receipt=%#v error=%v", running, err)
		}
		if finished, err := reopened.Worktrees.FinishExitOperation(ctx, workspaceID, worktreeID,
			operation.ID, "completed", now.Add(time.Second)); err != nil || !finished {
			t.Fatalf("finish migrated delivery=%t error=%v", finished, err)
		}
	})

	t.Run("Should keep worktrees visible across profiles with immutable owner tags", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "worktree-profiles", t.TempDir())
		foreignProfileID := "bbbbbbbbbbbbbbbbbbbbbbbbbb"
		now := time.Date(2026, 8, 21, 16, 0, 0, 0, time.UTC)
		if _, err := globalDB.db.ExecContext(ctx, `
			INSERT INTO profiles (id, name, color, icon, state, created_at)
			VALUES (?, 'foreign-worktree-owner', '#5FBF85', 'circle', 'active', ?)`,
			foreignProfileID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert foreign profile error = %v", err)
		}
		for _, item := range []worktreepkg.Worktree{
			{
				ID: "wt_profile_default", ProfileID: store.DefaultProfileID, WorkspaceID: workspaceID,
				Name: "default-owned", Path: filepath.Join(t.TempDir(), "default-owned"),
				State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual,
				SetupState: worktreepkg.SetupNone, CreatedAt: now, UpdatedAt: now,
			},
			{
				ID: "wt_profile_foreign", ProfileID: foreignProfileID, WorkspaceID: workspaceID,
				Name: "foreign-owned", Path: filepath.Join(t.TempDir(), "foreign-owned"),
				State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual,
				SetupState: worktreepkg.SetupNone, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second),
			},
		} {
			if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
				t.Fatalf("Insert(%s) error = %v", item.ID, err)
			}
		}

		items, err := globalDB.Worktrees.List(ctx, workspaceID)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(items) != 2 || items[0].ProfileID != store.DefaultProfileID ||
			items[1].ProfileID != foreignProfileID {
			t.Fatalf("List() = %#v, want both owner-tagged worktrees", items)
		}
		foreign, err := globalDB.Worktrees.Get(ctx, workspaceID, "wt_profile_foreign")
		if err != nil {
			t.Fatalf("Get(foreign owner) error = %v", err)
		}
		if foreign.ProfileID != foreignProfileID {
			t.Fatalf("Get(foreign owner).ProfileID = %q, want %q", foreign.ProfileID, foreignProfileID)
		}
	})

	// Invariant: discovered removal must see a live owner in any workspace that
	// shares the repository, while terminal records release the path.
	t.Run("Should report live path ownership across workspaces only", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "owner-a", filepath.Join(t.TempDir(), "a"))
		now := time.Date(2026, 10, 21, 3, 0, 0, 0, time.UTC)
		livePath := filepath.Join(t.TempDir(), "live")
		removedPath := filepath.Join(t.TempDir(), "removed")
		for _, item := range []worktreepkg.Worktree{
			{ID: "wt_owner_live", Name: "live", Path: livePath, State: worktreepkg.StateReady},
			{ID: "wt_owner_removed", Name: "removed", Path: removedPath, State: worktreepkg.StateRemoved},
		} {
			item.ProfileID, item.WorkspaceID = store.DefaultProfileID, workspaceA
			item.Origin, item.SetupState = worktreepkg.OriginManual, worktreepkg.SetupNone
			item.CreatedAt, item.UpdatedAt = now, now
			if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
				t.Fatalf("Insert(%s) error = %v", item.ID, err)
			}
		}
		for path, want := range map[string]bool{
			livePath: true, removedPath: false, filepath.Join(t.TempDir(), "unknown"): false,
			// A sibling that differs only by a trailing space is a distinct checkout.
			livePath + " ": false,
		} {
			got, err := globalDB.Worktrees.PathOwnedByAnyWorkspace(ctx, path)
			if err != nil || got != want {
				t.Fatalf("PathOwnedByAnyWorkspace(%q) = %v, %v; want %v", path, got, err, want)
			}
		}
	})

	t.Run("Should isolate every repository read and side-table mutation by workspace", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "worktree-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "worktree-b", filepath.Join(t.TempDir(), "b"))
		now := time.Date(2026, 8, 12, 17, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ProfileID: store.DefaultProfileID,
			ID:        "wt_store_a", WorkspaceID: workspaceA, Name: "store-a", Branch: "feature/a",
			Path: filepath.Join(t.TempDir(), "store-a"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		if _, err := globalDB.Worktrees.Get(ctx, workspaceB, item.ID); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("Get(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if _, err := globalDB.Worktrees.GetByPath(
			ctx,
			workspaceB,
			item.Path,
		); !errors.Is(
			err,
			worktreepkg.ErrNotFound,
		) {
			t.Fatalf("GetByPath(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if rows, err := globalDB.Worktrees.List(ctx, workspaceB); err != nil || len(rows) != 0 {
			t.Fatalf("List(cross-workspace) = %#v, %v, want empty", rows, err)
		}
		if swapped, err := globalDB.Worktrees.CompareAndSwapState(
			ctx, workspaceB, item.ID, worktreepkg.StateReady, worktreepkg.StateRemoving, now,
		); err != nil || swapped {
			t.Fatalf("CompareAndSwapState(cross-workspace) = %v, %v, want false nil", swapped, err)
		}
		if err := globalDB.Worktrees.SetState(
			ctx, workspaceB, item.ID, worktreepkg.StateRemoving, now,
		); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("SetState(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.UpdatePending(
			ctx,
			workspaceB,
			item.ID,
			worktreepkg.PendingUpdate{Phase: worktreepkg.PhaseCheckout},
		); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("UpdatePending(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.CompleteCreate(
			ctx, workspaceB, item.ID, worktreepkg.SetupNone, "", now,
		); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("CompleteCreate(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.DeletePending(ctx, workspaceB, item.ID); !errors.Is(
			err,
			worktreepkg.ErrNotFound,
		) {
			t.Fatalf("DeletePending(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if err := globalDB.Worktrees.SaveStatus(ctx, workspaceB, item.ID, worktreepkg.Status{
			WorktreeID: item.ID,
		}); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("SaveStatus(cross-workspace) error = %v, want ErrNotFound", err)
		}
		if status, err := globalDB.Worktrees.GetStatus(ctx, workspaceB, item.ID); err != nil || status != nil {
			t.Fatalf("GetStatus(cross-workspace) = %#v, %v, want nil", status, err)
		}
		exitOperation := worktreepkg.ExitOperation{
			ID: "op-isolation", WorkspaceID: workspaceA, WorktreeID: item.ID,
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, exitOperation); err != nil {
			t.Fatalf("InsertExitOperation() error = %v", err)
		}
		if finished, err := globalDB.Worktrees.FinishExitOperation(
			ctx, workspaceB, item.ID, exitOperation.ID, "failed", now,
		); err != nil || finished {
			t.Fatalf("FinishExitOperation(cross-workspace) = %v, %v, want false nil", finished, err)
		}
		branch, dirty, aheadOfBase, refreshed := "feature/a", 2, 4, now
		hasUpstream := false
		if err := globalDB.Worktrees.SaveStatus(ctx, workspaceA, item.ID, worktreepkg.Status{
			WorktreeID: item.ID, Branch: &branch, DirtyFiles: &dirty, HasUpstream: &hasUpstream,
			AheadOfBase: &aheadOfBase, RefreshedAt: &refreshed,
		}); err != nil {
			t.Fatalf("SaveStatus() error = %v", err)
		}
		status, err := globalDB.Worktrees.GetStatus(ctx, workspaceA, item.ID)
		if err != nil || status == nil || status.DirtyFiles == nil || *status.DirtyFiles != 2 ||
			status.Ahead != nil || status.AheadOfBase == nil || *status.AheadOfBase != aheadOfBase {
			t.Fatalf("GetStatus() = %#v, %v, want dirty_files=2 and ahead_of_base=4", status, err)
		}
		provider, prState, merged, fetchedAt := "forge-test", "open", false, now.Add(time.Minute)
		// Invariant: canonical PR facts are isolated by workspace and repository.
		key := worktreepkg.PullRequestKey{WorkspaceID: workspaceA, RepoKey: "github.com/acme/repo", Branch: branch}
		facts := &worktreepkg.ForgeStatus{Provider: provider, PRState: &prState, Merged: &merged, FetchedAt: &fetchedAt}
		if err := globalDB.Worktrees.SavePullRequest(
			ctx,
			worktreepkg.PullRequestEntry{Key: key, Facts: facts, LookupState: "ok"},
		); err != nil {
			t.Fatal(err)
		}
		other := key
		other.WorkspaceID = workspaceB
		changedOrigin := key
		changedOrigin.RepoKey = "github.com/acme/other"
		rows, err := globalDB.Worktrees.ReadPullRequests(ctx, []worktreepkg.PullRequestKey{key, other, changedOrigin})
		if err != nil || len(rows) != 1 {
			t.Fatalf("isolated PRs = %#v, %v", rows, err)
		}
		forge := rows[key].Facts
		if forge == nil || forge.Provider != provider || forge.PRState == nil || *forge.PRState != prState ||
			forge.FetchedAt == nil ||
			!forge.FetchedAt.Equal(fetchedAt) {
			t.Fatalf("durable PR = %#v", forge)
		}
	})

	t.Run("Should persist lifecycle events with workspace and worktree correlation", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "event-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "event-b", filepath.Join(t.TempDir(), "b"))
		profileID := "cccccccccccccccccccccccccc"
		if _, err := globalDB.db.ExecContext(ctx, `
			INSERT INTO profiles (id, name, color, icon, state, created_at)
			VALUES (?, 'event-owner', '#5FBF85', 'circle', 'active', ?)`,
			profileID,
			store.FormatTimestamp(time.Date(2026, 8, 22, 18, 0, 0, 0, time.UTC)),
		); err != nil {
			t.Fatalf("insert event owner profile error = %v", err)
		}
		event := worktreepkg.LifecycleEvent{
			Name: worktreepkg.EventCreated, ProfileID: profileID,
			WorkspaceID: workspaceA, WorktreeID: "wt_event_a",
			Payload: json.RawMessage(`{"worktree_id":"wt_event_a"}`),
		}
		if err := globalDB.PublishWorktreeEvent(ctx, event); err != nil {
			t.Fatalf("PublishWorktreeEvent() error = %v", err)
		}
		summaries, err := globalDB.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
				WorkspaceID: workspaceA, WorktreeID: event.WorktreeID, Type: event.Name,
			},
		)
		if err != nil || len(summaries) != 1 {
			t.Fatalf("ListEventSummaries(owner) = %#v, %v, want one", summaries, err)
		}
		if summaries[0].WorkspaceID != workspaceA || summaries[0].WorktreeID != event.WorktreeID {
			t.Fatalf("event projections = %#v, want workspace/worktree correlation", summaries[0])
		}
		if summaries[0].ProfileID != profileID {
			t.Fatalf("event profile = %q, want %q", summaries[0].ProfileID, profileID)
		}
		crossWorkspace, err := globalDB.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
				WorkspaceID: workspaceB, WorktreeID: event.WorktreeID,
			},
		)
		if err != nil || len(crossWorkspace) != 0 {
			t.Fatalf("ListEventSummaries(cross-workspace) = %#v, %v, want empty", crossWorkspace, err)
		}
	})

	t.Run("Should commit an exit terminal state and replay event atomically", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "terminal-event", t.TempDir())
		now := time.Date(2026, 8, 12, 17, 30, 0, 0, time.UTC)
		profileID := "dddddddddddddddddddddddddd"
		if _, err := globalDB.db.ExecContext(ctx, `
			INSERT INTO profiles (id, name, color, icon, state, created_at)
			VALUES (?, 'terminal-event-owner', '#5FBF85', 'circle', 'active', ?)`,
			profileID,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert terminal event profile error = %v", err)
		}
		item := worktreepkg.Worktree{
			ProfileID: profileID,
			ID:        "wt_terminal_event", WorkspaceID: workspaceID, Name: "terminal-event",
			Branch: "feature/terminal", Path: filepath.Join(t.TempDir(), "terminal"),
			State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual,
			SetupState: worktreepkg.SetupNone, CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		operation := worktreepkg.ExitOperation{
			ID: "op-terminal", WorkspaceID: workspaceID, WorktreeID: item.ID,
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, operation); err != nil {
			t.Fatalf("InsertExitOperation() error = %v", err)
		}
		event := worktreepkg.LifecycleEvent{
			Name: worktreepkg.EventExitActionCompleted, ProfileID: profileID,
			WorkspaceID: workspaceID, WorktreeID: item.ID,
			Payload: json.RawMessage(`{"op_id":"op-terminal","state":"completed"}`),
		}
		if _, err := globalDB.FinishExitOperationWithEvent(
			ctx, workspaceID, item.ID, operation.ID, "invalid", now, event,
		); err == nil {
			t.Fatal("FinishExitOperationWithEvent(invalid state) error = nil")
		}
		running, err := globalDB.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 1 {
			t.Fatalf("running after rollback = %#v, %v, want original operation", running, err)
		}
		summaries, err := globalDB.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
				WorkspaceID: workspaceID, WorktreeID: item.ID, Type: event.Name,
			},
		)
		if err != nil || len(summaries) != 0 {
			t.Fatalf("events after rollback = %#v, %v, want none", summaries, err)
		}
		finished, err := globalDB.FinishExitOperationWithEvent(
			ctx, workspaceID, item.ID, operation.ID, "completed", now, event,
		)
		if err != nil || !finished {
			t.Fatalf("FinishExitOperationWithEvent() = %t, %v", finished, err)
		}
		running, err = globalDB.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 0 {
			t.Fatalf("running after commit = %#v, %v, want none", running, err)
		}
		summaries, err = globalDB.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
				WorkspaceID: workspaceID, WorktreeID: item.ID, Type: event.Name,
			},
		)
		if err != nil || len(summaries) != 1 {
			t.Fatalf("events after commit = %#v, %v, want one", summaries, err)
		}
		if summaries[0].ProfileID != profileID || summaries[0].ProfileName != "terminal-event-owner" {
			t.Fatalf("terminal event owner = %#v, want profile %q", summaries[0], profileID)
		}
	})

	t.Run("Should enforce profile run binding and one-active-exit constraints in SQLite", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, globalDB, "constraint-a", filepath.Join(t.TempDir(), "a"))
		workspaceB := registerWorkspaceForGlobalTests(t, globalDB, "constraint-b", filepath.Join(t.TempDir(), "b"))
		now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ProfileID: store.DefaultProfileID,
			ID:        "wt_constraints", WorkspaceID: workspaceB, Name: "constraints", Branch: "feature/c",
			Path: filepath.Join(t.TempDir(), "constraints"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}

		task := workspaceTaskRecordForTest("task-worktree-constraints", workspaceA)
		if err := globalDB.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-worktree-constraints", task.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE task_runs SET worktree_id = ? WHERE id = ?`, item.ID, run.ID,
		); err == nil {
			t.Fatal("cross-workspace task run binding succeeded, want composite FK rejection")
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE task_runs SET resolved_worktree_mode = 'none', resolved_worktree_ref = 'unexpected' WHERE id = ?`,
			run.ID,
		); err == nil {
			t.Fatal("invalid resolved mode/ref succeeded, want CHECK rejection")
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO task_execution_profiles (
			  task_id, worktree_mode, worktree_ref, created_at, updated_at
			) VALUES (?, 'none', 'unexpected', ?, ?)`,
			task.ID, store.FormatTimestamp(now), store.FormatTimestamp(now),
		); err == nil {
			t.Fatal("invalid profile mode/ref succeeded, want CHECK rejection")
		}

		first := worktreepkg.ExitOperation{
			ID: "op-one", WorkspaceID: workspaceB, WorktreeID: item.ID,
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, first); err != nil {
			t.Fatalf("InsertExitOperation(first) error = %v", err)
		}
		second := first
		second.ID = "op-two"
		if err := globalDB.Worktrees.InsertExitOperation(ctx, second); !errors.Is(
			err, worktreepkg.ErrOperationInProgress,
		) {
			t.Fatalf("InsertExitOperation(second) error = %v, want ErrOperationInProgress", err)
		}
	})

	t.Run("Should reject cross-workspace session bindings at the database boundary", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		globalDB := openTestGlobalDB(t)
		workspaceA := registerSessionForGlobalTests(t, globalDB, "session-worktree-constraints")
		workspaceB := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"session-worktree-b",
			filepath.Join(t.TempDir(), "b"),
		)
		now := time.Date(2026, 8, 12, 19, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ProfileID: store.DefaultProfileID,
			ID:        "wt_session_constraints", WorkspaceID: workspaceB, Name: "session-constraints",
			Path: filepath.Join(t.TempDir(), "session"), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		if workspaceA == workspaceB {
			t.Fatal("test fixture workspaces unexpectedly match")
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE sessions SET worktree_id = ? WHERE id = ?`, item.ID, "session-worktree-constraints",
		); err == nil {
			t.Fatal("cross-workspace session binding succeeded, want composite FK rejection")
		}
	})

	t.Run("Should make exit start and removal mutually exclusive at the SQLite fence", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t, globalDB, "exit-removal-fence", filepath.Join(t.TempDir(), "workspace"),
		)
		now := time.Date(2026, 8, 12, 19, 10, 0, 0, time.UTC)
		insertReady := func(id string) {
			t.Helper()
			if err := globalDB.Worktrees.Insert(ctx, worktreepkg.Worktree{
				ProfileID: store.DefaultProfileID,
				ID:        id, WorkspaceID: workspaceID, Name: id, Branch: "feature/" + id,
				Path: filepath.Join(t.TempDir(), id), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				t.Fatalf("Insert(%s) error = %v", id, err)
			}
		}

		// A ready checkout cannot enter the metadata-only missing-record removal path.
		insertReady("wt_remove_ready")
		if removed, err := globalDB.Worktrees.RemoveMissing(
			ctx,
			workspaceID,
			"wt_remove_ready",
			now,
		); err != nil ||
			removed {
			t.Fatalf("ready removal = %v, %v, want false nil", removed, err)
		}

		insertReady("wt_exit_first")
		exitOperation := worktreepkg.ExitOperation{
			ID: "op-exit-first", WorkspaceID: workspaceID, WorktreeID: "wt_exit_first",
			Action: "push", State: "running", StartedAt: now,
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, exitOperation); err != nil {
			t.Fatalf("InsertExitOperation(exit first) error = %v", err)
		}
		if swapped, err := globalDB.Worktrees.CompareAndSwapState(
			ctx, workspaceID, "wt_exit_first", worktreepkg.StateReady, worktreepkg.StateRemoving, now,
		); err != nil || swapped {
			t.Fatalf("CompareAndSwapState(after exit) = %t, %v, want false nil", swapped, err)
		}
		running, err := globalDB.Worktrees.ListRunningExitOperations(ctx)
		if err != nil || len(running) != 1 || running[0].ID != exitOperation.ID ||
			running[0].WorkspaceID != workspaceID || running[0].WorktreeID != "wt_exit_first" {
			t.Fatalf("ListRunningExitOperations() = %#v, %v", running, err)
		}

		if err := globalDB.Worktrees.SetState(
			ctx,
			workspaceID,
			"wt_exit_first",
			worktreepkg.StateMissing,
			now,
		); err != nil {
			t.Fatal(err)
		}
		if removed, err := globalDB.Worktrees.RemoveMissing(
			ctx,
			workspaceID,
			"wt_exit_first",
			now,
		); err != nil ||
			removed {
			t.Fatalf("missing removal during exit = %v, %v", removed, err)
		}
		if finished, err := globalDB.Worktrees.FinishExitOperation(
			ctx,
			workspaceID,
			"wt_exit_first",
			exitOperation.ID,
			"failed",
			now,
		); err != nil ||
			!finished {
			t.Fatalf("finish exit = %v, %v", finished, err)
		}
		if removed, err := globalDB.Worktrees.RemoveMissing(
			ctx,
			workspaceID,
			"wt_exit_first",
			now,
		); err != nil ||
			!removed {
			t.Fatalf("missing removal after exit = %v, %v", removed, err)
		}

		insertReady("wt_remove_first")
		if swapped, err := globalDB.Worktrees.CompareAndSwapState(
			ctx, workspaceID, "wt_remove_first", worktreepkg.StateReady, worktreepkg.StateRemoving, now,
		); err != nil || !swapped {
			t.Fatalf("CompareAndSwapState(removal first) = %t, %v, want true nil", swapped, err)
		}
		if err := globalDB.Worktrees.InsertExitOperation(ctx, worktreepkg.ExitOperation{
			ID: "op-remove-first", WorkspaceID: workspaceID, WorktreeID: "wt_remove_first",
			Action: "commit", State: "running", StartedAt: now,
		}); !errors.Is(err, worktreepkg.ErrNotReady) {
			t.Fatalf("InsertExitOperation(after removal) error = %v, want ErrNotReady", err)
		}
	})

	t.Run(
		"Should fence new session bindings after removal starts without blocking bound session updates",
		func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			globalDB := openTestGlobalDB(t)
			workspaceID := registerWorkspaceForGlobalTests(
				t,
				globalDB,
				"session-removal-fence",
				filepath.Join(t.TempDir(), "workspace"),
			)
			now := time.Date(2026, 8, 12, 19, 15, 0, 0, time.UTC)
			item := worktreepkg.Worktree{
				ProfileID: store.DefaultProfileID,
				ID:        "wt_session_removal_fence", WorkspaceID: workspaceID, Name: "session-removal-fence",
				Path: filepath.Join(t.TempDir(), "worktree"), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
				t.Fatalf("Insert() error = %v", err)
			}
			bound := store.SessionInfo{
				ID: "session-before-removal", ProfileID: store.DefaultProfileID,
				AgentName: "coder", Provider: "claude",
				RuntimeStatus: store.SessionRuntimeUnbound, WorkspaceID: workspaceID, WorktreeID: item.ID,
				State: "active", CreatedAt: now, UpdatedAt: now,
			}
			if err := globalDB.RegisterSession(ctx, &(bound)); err != nil {
				t.Fatalf("RegisterSession(bound) error = %v", err)
			}
			if err := globalDB.Worktrees.SetState(
				ctx, workspaceID, item.ID, worktreepkg.StateRemoving, now.Add(time.Minute),
			); err != nil {
				t.Fatalf("SetState(removing) error = %v", err)
			}

			bound.State = "stopped"
			bound.UpdatedAt = now.Add(2 * time.Minute)
			if err := globalDB.RegisterSession(ctx, &(bound)); err != nil {
				t.Fatalf("RegisterSession(existing bound update) error = %v", err)
			}
			newBinding := bound
			newBinding.ID = "session-after-removal"
			newBinding.State = "active"
			if err := globalDB.RegisterSession(ctx, &(newBinding)); !errors.Is(err, worktreepkg.ErrNotReady) {
				t.Fatalf("RegisterSession(new binding) error = %v, want %v", err, worktreepkg.ErrNotReady)
			}
			rows, err := globalDB.ListSessions(ctx, store.SessionListQuery{
				ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
				ID:        newBinding.ID,
			})
			if err != nil {
				t.Fatalf("ListSessions(new binding) error = %v", err)
			}
			if len(rows) != 0 {
				t.Fatalf("new binding rows = %#v, want none", rows)
			}
		},
	)

	// Invariant: explicit binding CAS preserves owner and rejects stale or unavailable destinations.
	// Owner: global SQLite session/worktree storage; canonical worktree store suite.
	t.Run("Should change existing session bindings only through guarded updates", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, db, "binding", t.TempDir())
		now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
		item := worktreepkg.Worktree{ID: "wt-binding", ProfileID: store.DefaultProfileID,
			WorkspaceID: workspaceID, Name: "binding", Path: t.TempDir(), State: worktreepkg.StateReady,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone, CreatedAt: now, UpdatedAt: now}
		if err := db.Worktrees.Insert(ctx, item); err != nil {
			t.Fatal(err)
		}
		original := store.SessionInfo{ID: "session-binding", ProfileID: store.DefaultProfileID,
			AgentName: "coder", Provider: "claude", RuntimeStatus: store.SessionRuntimeUnbound,
			WorkspaceID: workspaceID, State: "active", CreatedAt: now, UpdatedAt: now}
		if err := db.RegisterSession(ctx, &(original)); err != nil {
			t.Fatal(err)
		}
		update := store.SessionWorktreeBindingUpdate{SessionID: original.ID, WorkspaceID: workspaceID,
			ProfileID: store.DefaultProfileID, WorktreeID: item.ID, UpdatedAt: now.Add(time.Second)}
		if err := db.UpdateSessionWorktreeBinding(ctx, update); err != nil {
			t.Fatal(err)
		}
		if err := db.RegisterSession(ctx, &(original)); err == nil {
			t.Fatal("ordinary registration silently unbound session")
		}
		if err := db.UpdateSessionWorktreeBinding(ctx, update); !errors.Is(err, store.ErrSessionWorktreeConflict) {
			t.Fatalf("stale binding error = %v", err)
		}
		if err := db.RecoverSessionWorktreeBinding(ctx, update); err != nil {
			t.Fatal(err)
		}
		update.ExpectedWorktreeID, update.WorktreeID = item.ID, ""
		if err := db.UpdateSessionWorktreeBinding(ctx, update); err != nil {
			t.Fatal(err)
		}
		// Simulate a crash after durable acceptance and before the catalog write.
		meta := store.SessionMeta{ID: original.ID, ProfileID: original.ProfileID,
			WorkspaceID: workspaceID, AgentName: original.AgentName, Provider: original.Provider,
			State: "active", RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now,
			WorktreeTransition: &store.SessionWorktreeTransition{
				Previous: store.SessionExecutionLocationState{CWD: "/workspace"},
				Current:  store.SessionExecutionLocationState{WorktreeID: item.ID, CWD: item.Path},
				Pending:  true, NoticePending: true,
			},
		}
		meta.SetWorktreeID(item.ID)
		meta.SetCWD(item.Path)
		metaPath := store.SessionMetaFile(t.TempDir())
		if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
			t.Fatal(err)
		}
		if err := db.Worktrees.SetState(ctx, workspaceID, item.ID, worktreepkg.StateMissing, now); err != nil {
			t.Fatal(err)
		}
		update.ExpectedWorktreeID, update.WorktreeID = "", item.ID
		if err := db.UpdateSessionWorktreeBinding(ctx, update); !errors.Is(err, store.ErrSessionWorktreeConflict) {
			t.Fatalf("unavailable binding error = %v", err)
		}
		reloaded, err := store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecoverSessionWorktreeTransition(ctx, db, metaPath, &reloaded); err != nil {
			t.Fatal(err)
		}
		reloaded, err = store.ReadSessionMeta(metaPath)
		if err != nil {
			t.Fatal(err)
		}
		if reloaded.WorktreeTransition.Pending || !reloaded.WorktreeTransition.NoticePending ||
			reloaded.CWDValue() != item.Path || reloaded.WorktreeIDValue() != item.ID {
			t.Fatalf("recovered metadata = %#v", reloaded)
		}
		rows, err := db.ListSessions(
			ctx,
			store.SessionListQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, ID: original.ID},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0].WorktreeID != item.ID || rows[0].ID != original.ID {
			t.Fatalf("recovered rows = %#v", rows)
		}
	})

	t.Run("Should dismiss a tombstone without cascading session or task-run history", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		globalDB := openTestGlobalDB(t)
		sessionID := "session-worktree-history"
		workspaceID := registerSessionForGlobalTests(t, globalDB, sessionID)
		now := time.Date(2026, 8, 12, 19, 30, 0, 0, time.UTC)
		item := worktreepkg.Worktree{
			ProfileID: store.DefaultProfileID,
			ID:        "wt_history", WorkspaceID: workspaceID, Name: "history",
			Path: filepath.Join(t.TempDir(), "history"), State: worktreepkg.StateMissing,
			Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
		task := workspaceTaskRecordForTest("task-worktree-history", workspaceID)
		if err := globalDB.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		run := taskRunForTest("run-worktree-history", task.ID)
		if err := globalDB.CreateTaskRun(ctx, run); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE sessions SET worktree_id = ? WHERE id = ?`, item.ID, sessionID,
		); err != nil {
			t.Fatalf("bind session history: %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx, `UPDATE task_runs SET worktree_id = ? WHERE id = ?`, item.ID, run.ID,
		); err != nil {
			t.Fatalf("bind task-run history: %v", err)
		}
		service := worktreepkg.NewService(
			globalDB.Worktrees,
			nil,
			worktreepkg.WithClock(func() time.Time { return now }),
		)
		if err := service.Dismiss(ctx, workspaceID, item.ID); err != nil {
			t.Fatalf("Dismiss() error = %v", err)
		}
		var sessionWorktreeID string
		if err := globalDB.db.QueryRowContext(
			ctx, `SELECT worktree_id FROM sessions WHERE id = ?`, sessionID,
		).Scan(&sessionWorktreeID); err != nil {
			t.Fatalf("read session history: %v", err)
		}
		var runWorktreeID string
		if err := globalDB.db.QueryRowContext(
			ctx, `SELECT worktree_id FROM task_runs WHERE id = ?`, run.ID,
		).Scan(&runWorktreeID); err != nil {
			t.Fatalf("read task-run history: %v", err)
		}
		if sessionWorktreeID != item.ID || runWorktreeID != item.ID {
			t.Fatalf(
				"dismissed bindings = session %q run %q, want preserved %q",
				sessionWorktreeID, runWorktreeID, item.ID,
			)
		}

		t.Run("Should release the dismissed name for recreation", func(t *testing.T) {
			replacement := item
			replacement.ID = "wt_history_recreated"
			replacement.Path = filepath.Join(t.TempDir(), "history-recreated")
			replacement.State = worktreepkg.StateReady
			replacement.UpdatedAt = now.Add(time.Minute)
			if err := globalDB.Worktrees.Insert(ctx, replacement); err != nil {
				t.Fatalf("Insert(recreated name) error = %v", err)
			}
			resolved, err := globalDB.Worktrees.Get(ctx, workspaceID, item.Name)
			if err != nil || resolved == nil || resolved.ID != replacement.ID {
				t.Fatalf("Get(recreated name) = %#v, %v, want %q", resolved, err, replacement.ID)
			}
			tombstone, err := globalDB.Worktrees.Get(ctx, workspaceID, item.ID)
			if err != nil || tombstone == nil || tombstone.State != worktreepkg.StateDismissed {
				t.Fatalf("Get(dismissed id) = %#v, %v, want retained tombstone", tombstone, err)
			}
			listed, err := globalDB.Worktrees.List(ctx, workspaceID)
			if err != nil || len(listed) != 1 || listed[0].ID != replacement.ID {
				t.Fatalf("List(recreated name) = %#v, %v, want only replacement", listed, err)
			}
		})
	})
}

// Suite: worktree name-reservation migration.
// Invariant: v63 preserves dismissed rows and their side-table history while
// releasing only their names; every non-dismissed row still reserves its name.
// Owning layer: GlobalDB migration stream. Canonical suite: global_db_worktree_test.go.
func TestGlobalDBWorktreeNameReservationMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve dismissed tombstones and release names after migration", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00063_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open v62 GlobalDB error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		const workspaceID = "workspace-worktree-name-migration"
		const otherWorkspaceID = "workspace-worktree-name-migration-other"
		const dismissedID = "wt-name-migration-dismissed"
		const otherWorkspaceWorktreeID = "wt-name-migration-other"
		const reusedName = "reusable-name"
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
		id, root_dir, add_dirs, name, created_at, updated_at
	) VALUES (?, ?, '[]', ?, ?, ?)`, workspaceID, t.TempDir(), workspaceID,
			"2026-08-14T12:00:00Z", "2026-08-14T12:00:00Z"); err != nil {
			t.Fatalf("seed v62 workspace error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO workspaces (
		id, root_dir, add_dirs, name, created_at, updated_at
	) VALUES (?, ?, '[]', ?, ?, ?)`, otherWorkspaceID, t.TempDir(), otherWorkspaceID,
			"2026-08-14T12:00:00Z", "2026-08-14T12:00:00Z"); err != nil {
			t.Fatalf("seed v62 other workspace error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO worktrees (
		id, workspace_id, name, branch, path, state, origin, setup_state, created_at, updated_at
	) VALUES (?, ?, ?, 'feature/reusable-name', ?, 'dismissed', 'manual', 'none', ?, ?)`,
			dismissedID, workspaceID, reusedName, filepath.Join(t.TempDir(), reusedName),
			"2026-08-14T12:01:00Z", "2026-08-14T12:02:00Z"); err != nil {
			t.Fatalf("seed v62 dismissed worktree error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO worktree_status (
		worktree_id, dirty_files, read_error
	) VALUES (?, 3, '')`, dismissedID); err != nil {
			t.Fatalf("seed v62 worktree status error = %v", err)
		}
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO worktrees (
		id, workspace_id, name, branch, path, state, origin, setup_state, created_at, updated_at
	) VALUES (?, ?, ?, 'feature/reusable-name', ?, 'ready', 'manual', 'none', ?, ?)`,
			otherWorkspaceWorktreeID, otherWorkspaceID, reusedName,
			filepath.Join(t.TempDir(), "other-"+reusedName),
			"2026-08-14T12:01:00Z", "2026-08-14T12:02:00Z"); err != nil {
			t.Fatalf("seed v62 other workspace worktree error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close v62 GlobalDB error = %v", err)
		}

		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade GlobalDB through v63 error = %v", err)
		}
		// Upgrade owns its migration budget; verification starts a fresh phase.
		ctx = testutil.Context(t)
		upgradedClosed := false
		t.Cleanup(func() {
			if upgradedClosed {
				return
			}
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded GlobalDB cleanup error = %v", closeErr)
			}
		})
		tombstone, err := upgraded.Worktrees.Get(ctx, workspaceID, dismissedID)
		if err != nil || tombstone == nil || tombstone.State != worktreepkg.StateDismissed {
			t.Fatalf("Get(dismissed id after upgrade) = %#v, %v", tombstone, err)
		}
		status, err := upgraded.Worktrees.GetStatus(ctx, workspaceID, dismissedID)
		if err != nil || status == nil || status.DirtyFiles == nil || *status.DirtyFiles != 3 {
			t.Fatalf("GetStatus(dismissed id after upgrade) = %#v, %v", status, err)
		}
		now := time.Date(2026, 8, 14, 12, 3, 0, 0, time.UTC)
		replacement := worktreepkg.Worktree{
			ProfileID: store.DefaultProfileID,
			ID:        "wt-name-migration-live", WorkspaceID: workspaceID, Name: reusedName,
			Branch: "feature/reusable-name-v2", Path: filepath.Join(t.TempDir(), reusedName),
			State: worktreepkg.StateReady, Origin: worktreepkg.OriginManual,
			SetupState: worktreepkg.SetupNone, CreatedAt: now, UpdatedAt: now,
		}
		if err := upgraded.Worktrees.Insert(ctx, replacement); err != nil {
			t.Fatalf("Insert(reused dismissed name after upgrade) error = %v", err)
		}
		resolved, err := upgraded.Worktrees.Get(ctx, workspaceID, reusedName)
		if err != nil || resolved == nil || resolved.ID != replacement.ID {
			t.Fatalf("Get(reused name after upgrade) = %#v, %v, want %q", resolved, err, replacement.ID)
		}
		otherResolved, err := upgraded.Worktrees.Get(ctx, otherWorkspaceID, reusedName)
		if err != nil || otherResolved == nil || otherResolved.ID != otherWorkspaceWorktreeID {
			t.Fatalf(
				"Get(other workspace reused name) = %#v, %v, want %q",
				otherResolved,
				err,
				otherWorkspaceWorktreeID,
			)
		}
		collision := replacement
		collision.ID = "wt-name-migration-collision"
		collision.Path = filepath.Join(t.TempDir(), "collision")
		if err := upgraded.Worktrees.Insert(ctx, collision); err == nil || !isSQLiteUniqueConstraint(err) {
			t.Fatalf("Insert(live name collision) error = %v, want SQLite unique constraint", err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatalf("close upgraded GlobalDB error = %v", err)
		}
		upgradedClosed = true

		reopened, err := OpenGlobalDB(globalMigrationTestContext(t), path)
		if err != nil {
			t.Fatalf("reopen v63 GlobalDB error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close reopened GlobalDB error = %v", closeErr)
			}
		})
		resolved, err = reopened.Worktrees.Get(globalMigrationTestContext(t), workspaceID, reusedName)
		if err != nil || resolved == nil || resolved.ID != replacement.ID {
			t.Fatalf("Get(reused name after reopen) = %#v, %v, want %q", resolved, err, replacement.ID)
		}
	})
}

// Invariant: root snapshots survive reads and bound sessions never fall back after removal.
// Owner: global SQLite session branch query; canonical worktree persistence suite.
func TestSessionBranchSnapshots(t *testing.T) {
	t.Parallel()
	db := openTestGlobalDB(t)
	ctx := testutil.Context(t)
	workspace := registerWorkspaceForGlobalTests(t, db, "branches", t.TempDir())
	const timestamp = "2026-10-23T12:00:00Z"
	_, err := db.db.ExecContext(
		ctx,
		`INSERT INTO worktrees(id,profile_id,workspace_id,name,branch,path,state,origin,setup_state,created_at,updated_at) VALUES('wt-branch',?,?,'branch','managed',?,'ready','manual','none',?,?)`,
		store.DefaultProfileID,
		workspace,
		t.TempDir(),
		timestamp,
		timestamp,
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"root-branch", "bound-branch"} {
		_, err = db.db.ExecContext(
			ctx,
			`INSERT INTO sessions(id,profile_id,agent_name,provider,workspace_id,state,created_at,updated_at) VALUES(?,?,'general','codex',?,'stopped',?,?)`,
			id,
			store.DefaultProfileID,
			workspace,
			timestamp,
			timestamp,
		)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = db.SaveSessionBranch(ctx, workspace, "root-branch", "topic"); err != nil {
		t.Fatal(err)
	}
	_, err = db.db.ExecContext(
		ctx,
		`UPDATE sessions SET worktree_id='wt-branch',branch='must-not-fallback' WHERE id='bound-branch'`,
	)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"root-branch", "bound-branch"}
	got, err := db.SessionBranches(ctx, ids)
	if err != nil || got[ids[0]] != "topic" || got[ids[1]] != "managed" {
		t.Fatalf("branches=%#v %v", got, err)
	}
	// The status snapshot owns subsequent checkouts and detached HEAD for all readers.
	for _, detached := range []bool{false, true} {
		if _, err := db.db.ExecContext(ctx,
			`INSERT INTO worktree_status(worktree_id,branch,detached) VALUES('wt-branch','switched',?)
 ON CONFLICT(worktree_id) DO UPDATE SET branch=excluded.branch,detached=excluded.detached`, detached); err != nil {
			t.Fatal(err)
		}
		want := "switched"
		if detached {
			want = ""
		}
		got, err = db.SessionBranches(ctx, ids)
		if err != nil || got[ids[1]] != want {
			t.Fatalf("current branch=%#v %v", got, err)
		}
		query := store.SessionCatalogPageQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspace,
			Archive: store.SessionArchiveExclude, Sort: sessionCatalogSortRecent, Limit: 10,
			RepositoryKeys: map[string]string{workspace: "github.com/owner/repo"},
		}
		keys, err := db.SessionCatalogBranches(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, key := range keys {
			if key.Branch == "switched" {
				found = true
			}
		}
		if found == detached {
			t.Fatalf("detached=%t refresh keys=%#v", detached, keys)
		}
		if _, err := db.db.ExecContext(ctx,
			`INSERT OR REPLACE INTO forge_pull_requests(workspace_id,repo_key,branch,pr_state)
 VALUES (?,'github.com/owner/repo','switched','open')`, workspace); err != nil {
			t.Fatal(err)
		}
		query.Columns = []string{"in_review"}
		page, err := db.PageSessions(ctx, query)
		wantTotal := 1
		if detached {
			wantTotal = 0
		}
		if err != nil || page.Total != wantTotal {
			t.Fatalf("detached=%t page=%#v %v", detached, page, err)
		}
	}
	if err = db.SaveSessionBranch(ctx, workspace, "bound-branch", "wrong"); err != nil {
		t.Fatal(err)
	}
	_, err = db.db.ExecContext(ctx, `UPDATE worktrees SET state='removed' WHERE id='wt-branch'`)
	if err != nil {
		t.Fatal(err)
	}
	got, err = db.SessionBranches(ctx, ids)
	if err != nil || got[ids[0]] != "topic" || got[ids[1]] != "" {
		t.Fatalf("removed branches=%#v %v", got, err)
	}
}
