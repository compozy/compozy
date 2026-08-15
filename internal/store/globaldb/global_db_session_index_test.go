package globaldb

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	worktreepkg "github.com/compozy/compozy/internal/worktree"
)

const (
	sessionCatalogRecentIndex   = "idx_sessions_catalog_recent"
	sessionCatalogActivityIndex = "idx_sessions_catalog_activity"
)

func TestListSessionsWorkspaceStateIndex(t *testing.T) {
	t.Parallel()

	t.Run("Should use the covering recent catalog index for workspace and state filters", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		alphaWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-alpha",
			filepath.Join(t.TempDir(), "workspace-alpha"),
		)
		betaWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-beta",
			filepath.Join(t.TempDir(), "workspace-beta"),
		)
		baseAt := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
		for _, sessionInfo := range []store.SessionInfo{
			sessionInfoForWorkspaceStateIndexTest("sess-alpha-active", alphaWorkspaceID, globalDBSessionStateActive, baseAt),
			sessionInfoForWorkspaceStateIndexTest("sess-alpha-stopped", alphaWorkspaceID, globalDBSessionStateStopped, baseAt),
			sessionInfoForWorkspaceStateIndexTest("sess-beta-active", betaWorkspaceID, globalDBSessionStateActive, baseAt),
		} {
			if err := globalDB.RegisterSession(ctx, sessionInfo); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", sessionInfo.ID, err)
			}
		}

		plan := explainQueryPlan(
			t,
			globalDB.db,
			`SELECT id FROM sessions WHERE state = ? AND workspace_id = ? ORDER BY updated_at DESC, created_at DESC, id DESC`,
			globalDBSessionStateActive,
			alphaWorkspaceID,
		)
		if !strings.Contains(plan, sessionCatalogRecentIndex) {
			t.Fatalf("EXPLAIN QUERY PLAN detail = %q, want %s", plan, sessionCatalogRecentIndex)
		}
		assertIndexAbsent(t, globalDB.db, "idx_sessions_workspace_state")

		sessions, err := globalDB.ListSessions(ctx, store.SessionListQuery{
			State:       globalDBSessionStateActive,
			WorkspaceID: alphaWorkspaceID,
		})
		if err != nil {
			t.Fatalf("ListSessions(workspace/state) error = %v", err)
		}
		got := sessionIDsForWorkspaceStateIndexTest(sessions)
		want := []string{"sess-alpha-active"}
		if !slices.Equal(got, want) {
			t.Fatalf("ListSessions(workspace/state) ids = %#v, want %#v", got, want)
		}
	})
}

func TestPageSessionsVisibilityExclusion(t *testing.T) {
	t.Parallel()

	t.Run("Should keep archive state workspace scoped and outside lifecycle state", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-session-archive",
			filepath.Join(t.TempDir(), "workspace-session-archive"),
		)
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-session-archive-foreign",
			filepath.Join(t.TempDir(), "workspace-session-archive-foreign"),
		)
		baseAt := time.Date(2026, 8, 4, 20, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return baseAt.Add(time.Minute) }
		stopped := sessionInfoForWorkspaceStateIndexTest(
			"sess-archive",
			workspaceID,
			globalDBSessionStateStopped,
			baseAt,
		)
		active := sessionInfoForWorkspaceStateIndexTest(
			"sess-live",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		for _, info := range []store.SessionInfo{stopped, active} {
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
			}
		}

		archived, err := globalDB.SetSessionArchived(ctx, workspaceID, stopped.ID, true)
		if err != nil {
			t.Fatalf("SetSessionArchived() error = %v", err)
		}
		if archived.ArchivedAt == nil || archived.State != globalDBSessionStateStopped {
			t.Fatalf("SetSessionArchived() = %#v, want archived stopped session", archived)
		}
		idempotent, err := globalDB.SetSessionArchived(ctx, workspaceID, stopped.ID, true)
		if err != nil || idempotent.ArchivedAt == nil || !idempotent.ArchivedAt.Equal(*archived.ArchivedAt) {
			t.Fatalf("SetSessionArchived(idempotent) = %#v, %v, want stable timestamp", idempotent, err)
		}
		if _, err := globalDB.SetSessionArchived(ctx, foreignWorkspaceID, stopped.ID, false); !errors.Is(
			err,
			store.ErrSessionNotFound,
		) {
			t.Fatalf("SetSessionArchived(foreign workspace) error = %v, want ErrSessionNotFound", err)
		}
		if _, err := globalDB.SetSessionArchived(ctx, workspaceID, active.ID, true); !errors.Is(
			err,
			store.ErrSessionArchiveRequiresStopped,
		) {
			t.Fatalf("SetSessionArchived(active) error = %v, want ErrSessionArchiveRequiresStopped", err)
		}
		if err := globalDB.UpdateSessionState(ctx, store.SessionStateUpdate{
			ID: stopped.ID, State: globalDBSessionStateActive, UpdatedAt: baseAt.Add(2 * time.Minute),
		}); !errors.Is(err, store.ErrSessionArchived) {
			t.Fatalf("UpdateSessionState(archived) error = %v, want ErrSessionArchived", err)
		}

		for _, testCase := range []struct {
			name    string
			archive store.SessionArchiveFilter
			want    []string
		}{
			{name: "exclude", archive: store.SessionArchiveExclude, want: []string{active.ID}},
			{name: "only", archive: store.SessionArchiveOnly, want: []string{stopped.ID}},
			{name: "include", archive: store.SessionArchiveInclude, want: []string{stopped.ID, active.ID}},
		} {
			page, pageErr := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
				WorkspaceID: workspaceID,
				Archive:     testCase.archive,
				Sort:        sessionCatalogSortRecent,
				Limit:       10,
			})
			if pageErr != nil {
				t.Fatalf("PageSessions(%s) error = %v", testCase.name, pageErr)
			}
			got := sessionIDsForWorkspaceStateIndexTest(page.Sessions)
			if !slices.Equal(got, testCase.want) || page.Total != len(testCase.want) {
				t.Fatalf("PageSessions(%s) = %#v total=%d, want %#v", testCase.name, got, page.Total, testCase.want)
			}
		}

		restored, err := globalDB.SetSessionArchived(ctx, workspaceID, stopped.ID, false)
		if err != nil || restored.ArchivedAt != nil {
			t.Fatalf("SetSessionArchived(false) = %#v, %v, want restored session", restored, err)
		}
	})

	t.Run("Should preserve normal sessions with null spawn roles", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-visible-sessions",
			filepath.Join(t.TempDir(), "workspace-visible-sessions"),
		)
		baseAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		normal := sessionInfoForWorkspaceStateIndexTest(
			"sess-normal",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		normal.SessionType = "user"
		internal := sessionInfoForWorkspaceStateIndexTest(
			"sess-memory",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		internal.Lineage = &store.SessionLineage{SpawnRole: "memory-extractor"}
		system := sessionInfoForWorkspaceStateIndexTest(
			"sess-system",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		system.SessionType = "system"
		dream := sessionInfoForWorkspaceStateIndexTest(
			"sess-dream",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		dream.SessionType = "dream"
		overlaid := sessionInfoForWorkspaceStateIndexTest(
			"sess-active-overlay",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		for _, sessionInfo := range []store.SessionInfo{normal, internal, system, dream, overlaid} {
			if err := globalDB.RegisterSession(ctx, sessionInfo); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", sessionInfo.ID, err)
			}
		}

		page, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			WorkspaceID:         workspaceID,
			SessionType:         "user",
			Sort:                "recent",
			Limit:               10,
			ExcludeIDs:          []string{overlaid.ID},
			ExcludeSessionTypes: []string{"dream"},
			ExcludeSpawnRoles:   []string{"memory-extractor"},
		})
		if err != nil {
			t.Fatalf("PageSessions() error = %v", err)
		}
		got := sessionIDsForWorkspaceStateIndexTest(page.Sessions)
		want := []string{"sess-normal"}
		if !slices.Equal(got, want) {
			t.Fatalf("PageSessions() ids = %#v, want %#v", got, want)
		}
		if page.Total != 1 {
			t.Fatalf("PageSessions().Total = %d, want 1", page.Total)
		}
	})

	t.Run("Should filter sessions by the exact workspace and worktree pair", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-worktree-filter",
			filepath.Join(t.TempDir(), "workspace-worktree-filter"),
		)
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-worktree-filter-foreign",
			filepath.Join(t.TempDir(), "workspace-worktree-filter-foreign"),
		)
		baseAt := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
		worktrees := []worktreepkg.Worktree{
			{
				ID: "wt-target", WorkspaceID: workspaceID, Name: "target", Branch: "feature/target",
				Path: filepath.Join(t.TempDir(), "target"), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				CreatedAt: baseAt, UpdatedAt: baseAt,
			},
			{
				ID: "wt-other", WorkspaceID: workspaceID, Name: "other", Branch: "feature/other",
				Path: filepath.Join(t.TempDir(), "other"), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				CreatedAt: baseAt, UpdatedAt: baseAt,
			},
			{
				ID: "wt-foreign", WorkspaceID: foreignWorkspaceID, Name: "foreign", Branch: "feature/foreign",
				Path: filepath.Join(t.TempDir(), "foreign"), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				CreatedAt: baseAt, UpdatedAt: baseAt,
			},
		}
		for _, item := range worktrees {
			if err := globalDB.Worktrees.Insert(ctx, item); err != nil {
				t.Fatalf("Insert(worktree %q) error = %v", item.ID, err)
			}
		}

		nameBound := sessionInfoForWorkspaceStateIndexTest(
			"sess-bound-by-name",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(4*time.Minute),
		)
		nameBound.WorktreeID = "target"
		if err := globalDB.RegisterSession(ctx, nameBound); !errors.Is(err, worktreepkg.ErrNotFound) {
			t.Fatalf("RegisterSession(worktree name as ID) error = %v, want ErrNotFound", err)
		}

		bound := sessionInfoForWorkspaceStateIndexTest(
			"sess-bound-target",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(3*time.Minute),
		)
		bound.WorktreeID = "wt-target"
		other := sessionInfoForWorkspaceStateIndexTest(
			"sess-bound-other",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(2*time.Minute),
		)
		other.WorktreeID = "wt-other"
		root := sessionInfoForWorkspaceStateIndexTest(
			"sess-root-workspace",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(time.Minute),
		)
		foreign := sessionInfoForWorkspaceStateIndexTest(
			"sess-bound-foreign",
			foreignWorkspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		foreign.WorktreeID = "wt-foreign"
		for _, info := range []store.SessionInfo{bound, other, root, foreign} {
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
			}
		}

		page, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			WorkspaceID: workspaceID,
			WorktreeID:  "wt-target",
			Sort:        sessionCatalogSortRecent,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("PageSessions(worktree) error = %v", err)
		}
		want := []string{bound.ID}
		if got := sessionIDsForWorkspaceStateIndexTest(page.Sessions); !slices.Equal(got, want) || page.Total != 1 {
			t.Fatalf("PageSessions(worktree) ids = %#v total=%d, want %#v total=1", got, page.Total, want)
		}

		listed, err := globalDB.ListSessions(ctx, store.SessionListQuery{
			WorkspaceID: workspaceID,
			WorktreeID:  "wt-target",
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListSessions(worktree) error = %v", err)
		}
		if got := sessionIDsForWorkspaceStateIndexTest(listed); !slices.Equal(got, want) {
			t.Fatalf("ListSessions(worktree) ids = %#v, want %#v", got, want)
		}
	})

	t.Run("Should filter durable rows by parent and root session ids", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-lineage-filters",
			filepath.Join(t.TempDir(), "workspace-lineage-filters"),
		)
		baseAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		root := sessionInfoForWorkspaceStateIndexTest(
			"sess-prov-root",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		root.Lineage = &store.SessionLineage{RootSessionID: "sess-prov-root"}
		child := sessionInfoForWorkspaceStateIndexTest(
			"sess-prov-child",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(time.Minute),
		)
		child.Lineage = &store.SessionLineage{
			ParentSessionID: "sess-prov-root",
			RootSessionID:   "sess-prov-root",
			SpawnDepth:      1,
		}
		grandchild := sessionInfoForWorkspaceStateIndexTest(
			"sess-prov-grandchild",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(2*time.Minute),
		)
		grandchild.Lineage = &store.SessionLineage{
			ParentSessionID: "sess-prov-child",
			RootSessionID:   "sess-prov-root",
			SpawnDepth:      2,
		}
		foreign := sessionInfoForWorkspaceStateIndexTest(
			"sess-prov-foreign",
			workspaceID,
			globalDBSessionStateActive,
			baseAt.Add(3*time.Minute),
		)
		for _, sessionInfo := range []store.SessionInfo{root, child, grandchild, foreign} {
			if err := globalDB.RegisterSession(ctx, sessionInfo); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", sessionInfo.ID, err)
			}
		}

		parentPage, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			WorkspaceID:     workspaceID,
			ParentSessionID: "sess-prov-root",
			Sort:            "recent",
			Limit:           10,
		})
		if err != nil {
			t.Fatalf("PageSessions(parent) error = %v", err)
		}
		wantChildren := []string{"sess-prov-child"}
		if got := sessionIDsForWorkspaceStateIndexTest(parentPage.Sessions); !slices.Equal(got, wantChildren) {
			t.Fatalf("PageSessions(parent) ids = %#v, want %#v", got, wantChildren)
		}
		if parentPage.Total != 1 {
			t.Fatalf("PageSessions(parent).Total = %d, want 1", parentPage.Total)
		}

		rootPage, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			WorkspaceID:   workspaceID,
			RootSessionID: "sess-prov-root",
			Sort:          "recent",
			Limit:         10,
		})
		if err != nil {
			t.Fatalf("PageSessions(root) error = %v", err)
		}
		wantTree := []string{"sess-prov-grandchild", "sess-prov-child", "sess-prov-root"}
		if got := sessionIDsForWorkspaceStateIndexTest(rootPage.Sessions); !slices.Equal(got, wantTree) {
			t.Fatalf("PageSessions(root) ids = %#v, want %#v", got, wantTree)
		}
		if rootPage.Total != 3 {
			t.Fatalf("PageSessions(root).Total = %d, want 3", rootPage.Total)
		}
	})

	t.Run("Should count only resumable sessions before the page cut", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		now := time.Date(2026, 7, 10, 12, 30, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-resumable-page",
			filepath.Join(t.TempDir(), "workspace-resumable-page"),
		)
		available := sessionInfoForWorkspaceStateIndexTest(
			"sess-available",
			workspaceID,
			globalDBSessionStateActive,
			now,
		)
		locked := sessionInfoForWorkspaceStateIndexTest(
			"sess-locked",
			workspaceID,
			globalDBSessionStateActive,
			now,
		)
		for _, sessionInfo := range []store.SessionInfo{available, locked} {
			if err := globalDB.RegisterSession(ctx, sessionInfo); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", sessionInfo.ID, err)
			}
		}
		if _, err := globalDB.AttachSession(ctx, store.SessionAttachRequest{
			SessionID:  locked.ID,
			AttachedTo: "uds:test",
			Now:        now,
			TTL:        time.Hour,
		}); err != nil {
			t.Fatalf("AttachSession(locked) error = %v", err)
		}

		page, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			WorkspaceID: workspaceID,
			Resumable:   true,
			Sort:        "last_activity",
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("PageSessions(resumable) error = %v", err)
		}
		got := sessionIDsForWorkspaceStateIndexTest(page.Sessions)
		want := []string{available.ID}
		if !slices.Equal(got, want) {
			t.Fatalf("PageSessions(resumable) ids = %#v, want %#v", got, want)
		}
		if page.Total != 1 {
			t.Fatalf("PageSessions(resumable).Total = %d, want 1", page.Total)
		}
	})

	t.Run("Should aggregate exact visible metrics while excluding the live overlay", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-agent-counts",
			filepath.Join(t.TempDir(), "workspace-agent-counts"),
		)
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-agent-counts-foreign",
			filepath.Join(t.TempDir(), "workspace-agent-counts-foreign"),
		)
		baseAt := time.Date(2026, 7, 11, 16, 0, 0, 0, time.UTC)
		now := baseAt.Add(10 * time.Minute)
		globalDB.now = func() time.Time { return now }
		coderActive := sessionInfoForWorkspaceStateIndexTest(
			"sess-coder-active",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		coderStopped := sessionInfoForWorkspaceStateIndexTest(
			"sess-coder-stopped",
			workspaceID,
			globalDBSessionStateStopped,
			baseAt,
		)
		coderStopped.StopReason = store.StopError
		reviewer := sessionInfoForWorkspaceStateIndexTest(
			"sess-reviewer",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		reviewer.AgentName = "reviewer"
		memory := sessionInfoForWorkspaceStateIndexTest(
			"sess-memory",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		memory.Lineage = &store.SessionLineage{SpawnRole: "memory-extractor"}
		dream := sessionInfoForWorkspaceStateIndexTest(
			"sess-dream",
			workspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		dream.SessionType = "dream"
		foreign := sessionInfoForWorkspaceStateIndexTest(
			"sess-foreign",
			foreignWorkspaceID,
			globalDBSessionStateActive,
			baseAt,
		)
		for _, info := range []store.SessionInfo{
			coderActive,
			coderStopped,
			reviewer,
			memory,
			dream,
			foreign,
		} {
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
			}
		}

		metrics, err := globalDB.AggregateSessionsByAgent(ctx, store.SessionAgentMetricsQuery{
			WorkspaceID:         workspaceID,
			ExcludeIDs:          []string{reviewer.ID},
			ExcludeSessionTypes: []string{"dream"},
			ExcludeSpawnRoles:   []string{"memory-extractor"},
		})
		if err != nil {
			t.Fatalf("AggregateSessionsByAgent() error = %v", err)
		}
		wantRuntime := int64(now.Sub(coderActive.CreatedAt).Seconds()) +
			int64(coderStopped.UpdatedAt.Sub(coderStopped.CreatedAt).Seconds())
		if len(metrics) != 1 || metrics[0].AgentName != "coder" ||
			metrics[0].Total != 2 || metrics[0].Active != 1 || metrics[0].Failed != 1 ||
			metrics[0].RuntimeSeconds != wantRuntime ||
			!metrics[0].LastActivityAt.Equal(coderStopped.UpdatedAt) {
			t.Fatalf(
				"AggregateSessionsByAgent() = %#v, want coder total=2 active=1 failed=1 runtime=%d last=%s",
				metrics,
				wantRuntime,
				coderStopped.UpdatedAt,
			)
		}
	})
}

func TestDeleteSessionRemovesDurableCatalogTruth(t *testing.T) {
	t.Run("Should delete only the target session and its owned history", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-session-delete",
			filepath.Join(t.TempDir(), "workspace-session-delete"),
		)
		info := sessionInfoForWorkspaceStateIndexTest(
			"sess-delete",
			workspaceID,
			globalDBSessionStateStopped,
			time.Date(2026, 7, 13, 15, 0, 0, 0, time.UTC),
		)
		info.SessionType = "user"
		if err := globalDB.RegisterSession(ctx, info); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		foreign := sessionInfoForWorkspaceStateIndexTest(
			"sess-delete-foreign",
			workspaceID,
			globalDBSessionStateStopped,
			info.UpdatedAt.Add(time.Minute),
		)
		foreign.SessionType = "user"
		if err := globalDB.RegisterSession(ctx, foreign); err != nil {
			t.Fatalf("RegisterSession(foreign) error = %v", err)
		}
		for _, sessionInfo := range []store.SessionInfo{info, foreign} {
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO permission_log (
				id, session_id, agent_name, action, resource, decision, policy_used, timestamp
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				"perm-"+sessionInfo.ID,
				sessionInfo.ID,
				sessionInfo.AgentName,
				"invoke",
				"compozy__task_run_complete",
				"allow",
				"test",
				sessionInfo.UpdatedAt.Format(time.RFC3339Nano),
			); err != nil {
				t.Fatalf("Insert permission log for %q error = %v", sessionInfo.ID, err)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`INSERT INTO token_stats (
				id, session_id, agent_name, turn_count, updated_at
			) VALUES (?, ?, ?, ?, ?)`,
				"tokens-"+sessionInfo.ID,
				sessionInfo.ID,
				sessionInfo.AgentName,
				1,
				sessionInfo.UpdatedAt.Format(time.RFC3339Nano),
			); err != nil {
				t.Fatalf("Insert token stats for %q error = %v", sessionInfo.ID, err)
			}
		}
		if err := globalDB.DeleteSession(ctx, info.ID); err != nil {
			t.Fatalf("DeleteSession() error = %v", err)
		}

		page, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			WorkspaceID: workspaceID,
			SessionType: "user",
			Sort:        "recent",
			Limit:       1,
		})
		if err != nil {
			t.Fatalf("PageSessions(after delete) error = %v", err)
		}
		if page.Total != 1 || len(page.Sessions) != 1 || page.Sessions[0].ID != foreign.ID {
			t.Fatalf("PageSessions(after delete) = %#v, want only foreign session %q", page, foreign.ID)
		}
		for table, want := range map[string]int{
			"permission_log": 0,
			"token_stats":    0,
		} {
			var got int
			query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE session_id = ?", table)
			if err := globalDB.db.QueryRowContext(ctx, query, info.ID).Scan(&got); err != nil {
				t.Fatalf("Count deleted %s rows error = %v", table, err)
			}
			if got != want {
				t.Fatalf("%s rows for deleted session = %d, want %d", table, got, want)
			}
			if err := globalDB.db.QueryRowContext(ctx, query, foreign.ID).Scan(&got); err != nil {
				t.Fatalf("Count preserved %s rows error = %v", table, err)
			}
			if got != 1 {
				t.Fatalf("%s rows for foreign session = %d, want 1", table, got)
			}
		}
		if err := globalDB.DeleteSession(ctx, info.ID); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("DeleteSession(missing) error = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestPageSessionsStableKeyset(t *testing.T) {
	t.Parallel()

	t.Run("Should walk matches after an anchor mutation without gaps or duplicates", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-session-page",
			filepath.Join(t.TempDir(), "workspace-session-page"),
		)
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-session-page-foreign",
			filepath.Join(t.TempDir(), "workspace-session-page-foreign"),
		)
		baseAt := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)
		matching := make([]store.SessionInfo, 0, 5)
		for index := range 5 {
			info := sessionInfoForWorkspaceStateIndexTest(
				fmt.Sprintf("sess-match-%d", index),
				workspaceID,
				globalDBSessionStateActive,
				baseAt,
			)
			info.Name = fmt.Sprintf("Match %d", index)
			info.UpdatedAt = baseAt.Add(time.Duration(index) * time.Minute)
			matching = append(matching, info)
		}
		nonMatching := []store.SessionInfo{
			sessionInfoForWorkspaceStateIndexTest(
				"sess-foreign",
				foreignWorkspaceID,
				globalDBSessionStateActive,
				baseAt,
			),
			sessionInfoForWorkspaceStateIndexTest(
				"sess-stopped",
				workspaceID,
				globalDBSessionStateStopped,
				baseAt,
			),
			sessionInfoForWorkspaceStateIndexTest(
				"sess-other-agent",
				workspaceID,
				globalDBSessionStateActive,
				baseAt,
			),
		}
		nonMatching[0].Name = "Match foreign"
		nonMatching[1].Name = "Match stopped"
		nonMatching[2].Name = "Match other agent"
		nonMatching[2].AgentName = "reviewer"
		for _, info := range append(matching, nonMatching...) {
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
			}
		}

		query := store.SessionCatalogPageQuery{
			WorkspaceID: workspaceID,
			State:       globalDBSessionStateActive,
			AgentName:   "coder",
			Search:      "MATCH",
			Sort:        "recent",
			Limit:       2,
		}
		first, err := globalDB.PageSessions(ctx, query)
		if err != nil {
			t.Fatalf("PageSessions(first) error = %v", err)
		}
		if first.Total != len(matching) || len(first.Sessions) != 2 {
			t.Fatalf(
				"PageSessions(first) = total %d rows %d, want %d/2",
				first.Total,
				len(first.Sessions),
				len(matching),
			)
		}
		anchor := first.Sessions[len(first.Sessions)-1]
		query.After = &store.SessionCatalogPosition{
			PrimaryAt:   anchor.UpdatedAt,
			SecondaryAt: anchor.CreatedAt,
			CreatedAt:   anchor.CreatedAt,
			ID:          anchor.ID,
		}
		if err := globalDB.UpdateSessionState(ctx, store.SessionStateUpdate{
			ID:        anchor.ID,
			State:     anchor.State,
			UpdatedAt: baseAt.Add(24 * time.Hour),
		}); err != nil {
			t.Fatalf("UpdateSessionState(anchor) error = %v", err)
		}

		seen := sessionIDsForWorkspaceStateIndexTest(first.Sessions)
		for {
			page, pageErr := globalDB.PageSessions(ctx, query)
			if pageErr != nil {
				t.Fatalf("PageSessions(next) error = %v", pageErr)
			}
			if page.Total != len(matching) {
				t.Fatalf("PageSessions(next).Total = %d, want %d", page.Total, len(matching))
			}
			if len(page.Sessions) == 0 {
				break
			}
			seen = append(seen, sessionIDsForWorkspaceStateIndexTest(page.Sessions)...)
			last := page.Sessions[len(page.Sessions)-1]
			query.After = &store.SessionCatalogPosition{
				PrimaryAt:   last.UpdatedAt,
				SecondaryAt: last.CreatedAt,
				CreatedAt:   last.CreatedAt,
				ID:          last.ID,
			}
		}
		if len(seen) != len(matching) {
			t.Fatalf("walked session ids = %#v, want %d rows", seen, len(matching))
		}
		seenSet := make(map[string]struct{}, len(seen))
		for _, id := range seen {
			if _, duplicate := seenSet[id]; duplicate {
				t.Fatalf("walked session ids contain duplicate %q: %#v", id, seen)
			}
			seenSet[id] = struct{}{}
		}
	})
}

func TestSessionCatalogPagingIndexesFreshDB(t *testing.T) {
	t.Parallel()

	t.Run("Should create both paging indexes on a fresh database", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		assertIndexesPresent(
			t,
			globalDB.db,
			"sessions",
			sessionCatalogRecentIndex,
			sessionCatalogActivityIndex,
		)
	})
}

func sessionInfoForWorkspaceStateIndexTest(
	id string,
	workspaceID string,
	state string,
	baseAt time.Time,
) store.SessionInfo {
	return store.SessionInfo{
		ID:            id,
		Name:          id,
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		SessionType:   "root",
		State:         state,
		CreatedAt:     baseAt,
		UpdatedAt:     baseAt.Add(time.Duration(len(id)) * time.Second),
	}
}

func explainQueryPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()

	rows, err := db.QueryContext(testutil.Context(t), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN QUERY PLAN error = %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Fatalf("rows.Close() error = %v", closeErr)
		}
	}()

	var details []string
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("rows.Scan() error = %v", err)
		}
		details = append(details, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err() error = %v", err)
	}
	return strings.Join(details, "\n")
}

func assertIndexAbsent(t *testing.T, db *sql.DB, index string) {
	t.Helper()

	var found int
	if err := db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`,
		index,
	).Scan(&found); err != nil {
		t.Fatalf("query sqlite_master index %s error = %v", index, err)
	}
	if found != 0 {
		t.Fatalf("index %s exists, want absent", index)
	}
}

func sessionIDsForWorkspaceStateIndexTest(sessions []store.SessionInfo) []string {
	ids := make([]string, 0, len(sessions))
	for _, sessionInfo := range sessions {
		ids = append(ids, sessionInfo.ID)
	}
	return ids
}
