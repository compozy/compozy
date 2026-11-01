package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/session"
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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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

	t.Run("Should hide persisted verdict-only judges while preserving actionable system sessions", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		workspace := registerWorkspaceForGlobalTests(t, db, "system-audit", filepath.Join(t.TempDir(), "workspace"))
		base := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		for _, id := range []string{"user", "automation", "judge"} {
			info := sessionInfoForWorkspaceStateIndexTest(id, workspace, globalDBSessionStateStopped, base)
			info.SessionType = "system"
			if id == "user" {
				info.SessionType = "user"
			}
			if id != "judge" {
				if err := db.RegisterSession(ctx, info); err != nil {
					t.Fatal(err)
				}
				continue
			}
			profile := store.SessionCreationProfile{
				Version:     store.SessionCreationProfileVersion,
				AgentName:   "judge",
				Provider:    "mock",
				ProfileID:   store.DefaultProfileID,
				WorkspaceID: workspace,
				CWD:         t.TempDir(),
				RuntimeMode: "verdict-only",
			}
			ref, err := db.PutSessionCreationProfile(ctx, profile)
			if err != nil {
				t.Fatal(err)
			}
			policy, err := profile.PolicySpecDigest()
			if err != nil {
				t.Fatal(err)
			}
			digest, err := profile.CreationDigest(
				store.SessionCreationOptions{SessionID: id, SessionType: info.SessionType},
			)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.RegisterSessionWithCreationIdentity(ctx, info, store.SessionCreationIdentity{
				CreationProfileRef: ref, PolicySpecDigest: policy, CreationDigest: digest,
			}); err != nil {
				t.Fatal(err)
			}
		}
		query := store.SessionCatalogPageQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: workspace, Sort: sessionCatalogSortRecent, Limit: 1, ExcludeVerdictOnly: true}
		page, err := db.PageSessions(ctx, query)
		if err != nil || page.Total != 2 || len(page.Sessions) != 1 || page.Sessions[0].ID == "judge" {
			t.Fatalf("page = %#v, %v", page, err)
		}
		facets, err := db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.All != 2 {
			t.Fatalf("facets = %#v, %v", facets, err)
		}
		query.ExcludeVerdictOnly = false
		query.SessionType = "system"
		query.Limit = 10
		page, err = db.PageSessions(ctx, query)
		if err != nil || page.Total != 2 || len(page.Sessions) != 2 {
			t.Fatalf("audit = %#v, %v", page, err)
		}
		if !slices.ContainsFunc(
			page.Sessions,
			func(info store.SessionInfo) bool { return info.ID == "judge" && info.VerdictOnly },
		) {
			t.Fatalf("missing typed judge = %#v", page)
		}
	})

	t.Run("Should count all scoped terminal approvals without excluding live durable records", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		workspace := registerWorkspaceForGlobalTests(t, db, "terminal-count", filepath.Join(t.TempDir(), "workspace"))
		foreignWorkspace := registerWorkspaceForGlobalTests(
			t,
			db,
			"terminal-foreign",
			filepath.Join(t.TempDir(), "foreign"),
		)
		base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
		const foreignProfile = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		if _, err := db.db.ExecContext(
			ctx,
			`INSERT INTO profiles (id,name,color,icon,state,created_at) VALUES (?, 'foreign','#8E8EB5','circle','active',?)`,
			foreignProfile,
			store.FormatTimestamp(base),
		); err != nil {
			t.Fatal(err)
		}
		for index := range 115 {
			id := fmt.Sprintf("terminal-count-%03d", index)
			info := sessionInfoForWorkspaceStateIndexTest(id, workspace, globalDBSessionStateActive, base)
			if index == 111 {
				info.State = globalDBSessionStateStopped
			}
			if index == 110 {
				info.WorkspaceID = foreignWorkspace
			}
			if index == 112 {
				info.SessionType = "dream"
			}
			if index == 113 {
				info.ProfileID = foreignProfile
			}
			if err := db.RegisterSession(ctx, info); err != nil {
				t.Fatal(err)
			}
			tool := "compozy__terminal_write"
			if index == 114 {
				tool = "compozyXterminal_write"
			}
			if _, err := db.CreatePendingInteraction(
				ctx,
				store.PendingInteractionCreate{
					InteractionID:     id + "-approval",
					SessionID:         id,
					Kind:              "permission",
					ProviderRequestID: id,
					Payload:           store.PendingInteractionPayload{ToolID: tool},
					CreatedAt:         base,
				},
			); err != nil {
				t.Fatal(err)
			}
			if index == 111 {
				if _, err := db.SetSessionArchived(ctx, workspace, id, true); err != nil {
					t.Fatal(err)
				}
			}
		}
		query := store.SessionCatalogPageQuery{
			ReadScope:           store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID:         workspace,
			Archive:             store.SessionArchiveExclude,
			Limit:               1,
			Sort:                sessionCatalogSortRecent,
			ExcludeSessionTypes: []string{"dream"},
			ExcludeIDs:          []string{"terminal-count-000"},
		}
		facets, err := db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 110 || len(facets.ByWorkspace) != 1 ||
			facets.ByWorkspace[0].Facets.TerminalApprovals != 110 {
			t.Fatalf("scoped approval facets=%#v error=%v", facets, err)
		}
		// The shared summary must retain the same exact workspace/profile population.
		if len(facets.TerminalApprovals) != 1 || facets.TerminalApprovals[0] != (store.TerminalApprovalCount{
			WorkspaceID: workspace, ProfileID: store.DefaultProfileID, Count: 110,
		}) {
			t.Fatalf("scoped terminal approval groups = %#v", facets.TerminalApprovals)
		}
		if _, err := db.TransitionPendingInteraction(
			ctx,
			store.PendingInteractionTransition{
				InteractionID: "terminal-count-001-approval",
				Status:        store.PendingInteractionStatusResolved,
				Resolution:    "allow",
				ResolvedBy:    "operator",
				At:            base.Add(time.Minute),
			},
		); err != nil {
			t.Fatal(err)
		}
		facets, err = db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 109 {
			t.Fatalf("resolved approval facets=%#v error=%v", facets, err)
		}
		if _, err := db.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
			InteractionID: "terminal-count-002-approval",
			Status:        store.PendingInteractionStatusOrphaned,
			At:            base.Add(2 * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
		facets, err = db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 108 {
			t.Fatalf("orphaned approval facets=%#v error=%v", facets, err)
		}
		query.ExcludeIDs = nil
		where, args, err := sessionCatalogPageFilters(query, store.FormatTimestamp(base))
		if err != nil {
			t.Fatal(err)
		}
		plan := explainQueryPlan(t, db.db, sessionTerminalApprovalFacetStatement(where), args...)
		if !strings.Contains(plan, "uq_session_pending_interactions_active_provider_request") {
			t.Fatalf("terminal approval query plan = %s, want the active interaction index", plan)
		}
		query.ReadScope = store.ReadScope{ProfileID: foreignProfile}
		facets, err = db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 1 {
			t.Fatalf("foreign profile facets=%#v error=%v", facets, err)
		}
		query.ReadScope = store.ReadScope{AllProfiles: true}
		facets, err = db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 109 {
			t.Fatalf("all profile facets=%#v error=%v", facets, err)
		}
		query.WorkspaceID = ""
		facets, err = db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 110 || len(facets.ByWorkspace) != 2 {
			t.Fatalf("all workspace facets=%#v error=%v", facets, err)
		}
		if len(facets.TerminalApprovals) != 3 {
			t.Fatalf(
				"terminal approval intersections = %#v, want three workspace/profile groups",
				facets.TerminalApprovals,
			)
		}
		query.WorkspaceID = registerWorkspaceForGlobalTests(t, db, "terminal-empty", t.TempDir())
		facets, err = db.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.TerminalApprovals != 0 || len(facets.ByWorkspace) != 0 {
			t.Fatalf("empty workspace facets=%#v error=%v", facets, err)
		}
	})

	t.Run("Should search visible titles and preserve scoped metadata facets", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"facet-search",
			filepath.Join(t.TempDir(), "facet-search"),
		)
		base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
		for _, tc := range []struct{ id, name, agent, state string }{
			{"session-identity", "session-identity", "coder", globalDBSessionStateStopped},
			{"session-title", "Árvore ΟΣ", "coder", globalDBSessionStateActive},
			{"session-agent", "Équipe İ", "Équipe İ", globalDBSessionStateStopped},
		} {
			info := sessionInfoForWorkspaceStateIndexTest(tc.id, workspaceID, tc.state, base)
			info.Name = tc.name
			info.AgentName = tc.agent
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatal(err)
			}
		}
		query := store.SessionCatalogPageQuery{
			ReadScope:    store.ReadScope{AllProfiles: true},
			WorkspaceID:  workspaceID,
			Archive:      store.SessionArchiveExclude,
			Sort:         sessionCatalogSortRecent,
			Limit:        1,
			SkipTotal:    true,
			SearchFields: "title_agent",
			Search:       "New session",
		}
		page, err := globalDB.PageSessions(ctx, query)
		if err != nil || len(page.Sessions) != 1 || page.Sessions[0].ID == "session-title" {
			t.Fatalf("visible fallback page=%#v error=%v", page, err)
		}
		for _, match := range []struct{ query, id string }{{"árvore", "session-title"}, {"équipe", "session-agent"}, {"ος", "session-title"}, {"i̇", "session-agent"}, {"ΟΣ", "session-title"}, {"İ", "session-agent"}} {
			query.Search = match.query
			page, err = globalDB.PageSessions(ctx, query)
			if err != nil || len(page.Sessions) != 1 || page.Sessions[0].ID != match.id {
				t.Fatalf("Unicode visible search %q page=%#v error=%v", match.query, page, err)
			}
		}
		query.Search = "session-identity"
		page, err = globalDB.PageSessions(ctx, query)
		if err != nil || len(page.Sessions) != 0 {
			t.Fatalf("hidden identity search=%#v error=%v", page, err)
		}
		query.SearchFields = ""
		page, err = globalDB.PageSessions(ctx, query)
		if err != nil || len(page.Sessions) != 1 {
			t.Fatalf("legacy identity search=%#v error=%v", page, err)
		}
		if _, err := globalDB.SetSessionArchived(ctx, workspaceID, "session-agent", true); err != nil {
			t.Fatal(err)
		}
		query.Archive = store.SessionArchiveExclude
		facets, err := globalDB.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.All != 2 || facets.Facets.Idle != 1 {
			t.Fatalf("scoped facets=%#v error=%v", facets, err)
		}
		query.Archive = store.SessionArchiveOnly
		facets, err = globalDB.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.All != 1 {
			t.Fatalf("archive facets=%#v error=%v", facets, err)
		}
		query.ReadScope = store.ReadScope{ProfileID: "other-profile"}
		facets, err = globalDB.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.All != 0 {
			t.Fatalf("foreign profile facets=%#v error=%v", facets, err)
		}
	})

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
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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

	t.Run("Should park stopped sessions in Later, hide them by default, and return due ones once", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-session-later",
			filepath.Join(t.TempDir(), "workspace-session-later"),
		)
		baseAt := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return baseAt.Add(time.Minute) }
		parked := sessionInfoForWorkspaceStateIndexTest("sess-later", workspaceID, globalDBSessionStateStopped, baseAt)
		active := sessionInfoForWorkspaceStateIndexTest("sess-live", workspaceID, globalDBSessionStateActive, baseAt)
		for _, info := range []store.SessionInfo{parked, active} {
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", info.ID, err)
			}
		}
		returnAt := baseAt.Add(time.Hour)
		info, err := globalDB.ParkSession(ctx, store.SessionLaterPark{
			WorkspaceID: workspaceID, SessionID: parked.ID, Reason: "revisit after billing", ReturnAt: &returnAt,
		})
		if err != nil {
			t.Fatalf("ParkSession() error = %v", err)
		}
		parkedLater := info.LaterState()
		if parkedLater == nil || parkedLater.Reason != "revisit after billing" || parkedLater.ReturnAt == nil ||
			!parkedLater.ReturnAt.Equal(returnAt) || parkedLater.PendingMessages != 0 {
			t.Fatalf("ParkSession() later = %#v, want reason and return time", parkedLater)
		}
		if _, err := globalDB.ParkSession(ctx, store.SessionLaterPark{
			WorkspaceID: workspaceID, SessionID: active.ID,
		}); !errors.Is(err, store.ErrSessionLaterRequiresStopped) {
			t.Fatalf("ParkSession(active) error = %v, want ErrSessionLaterRequiresStopped", err)
		}
		if err := globalDB.UpdateSessionState(ctx, store.SessionStateUpdate{
			ID: parked.ID, State: globalDBSessionStateActive, UpdatedAt: baseAt.Add(2 * time.Minute),
		}); !errors.Is(err, store.ErrSessionInLater) {
			t.Fatalf("UpdateSessionState(parked) error = %v, want ErrSessionInLater", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO session_input_queue
			(id, session_id, status, mode, text, origin_class, enqueued_at, updated_at)
			VALUES ('inq-later', ?, 'queued', 'queue', 'ping', 'agent', ?, ?)`,
			parked.ID, formatTimestamp(baseAt), formatTimestamp(baseAt)); err != nil {
			t.Fatalf("insert queued agent message error = %v", err)
		}

		for _, testCase := range []struct {
			name  string
			later store.SessionLaterFilter
			want  []string
		}{
			{name: "exclude", later: store.SessionLaterExclude, want: []string{active.ID}},
			{name: "only", later: store.SessionLaterOnly, want: []string{parked.ID}},
			{name: "include", later: store.SessionLaterInclude, want: []string{parked.ID, active.ID}},
		} {
			page, pageErr := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
				WorkspaceID: workspaceID,
				Later:       testCase.later,
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
			if testCase.later == store.SessionLaterOnly && page.Sessions[0].LaterState().PendingMessages != 1 {
				t.Fatalf("PageSessions(only) pending = %d, want 1", page.Sessions[0].LaterState().PendingMessages)
			}
			// Every facets read reports the scope's parked count, so the Later toggle
			// needs no second facets request beside the active one.
			facets, facetsErr := globalDB.SessionCatalogFacets(ctx, store.SessionCatalogPageQuery{
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
				WorkspaceID: workspaceID,
				Later:       testCase.later,
				Sort:        sessionCatalogSortRecent,
				Limit:       10,
			})
			if facetsErr != nil || facets.Later != 1 || facets.Facets.All != len(testCase.want) {
				t.Fatalf("SessionCatalogFacets(%s) later=%d all=%d, %v; want later=1 all=%d",
					testCase.name, facets.Later, facets.Facets.All, facetsErr, len(testCase.want))
			}
		}

		// A clear advances the input generation; rows from the old generation are never delivered.
		if _, err := globalDB.db.ExecContext(
			ctx, "UPDATE sessions SET input_generation = input_generation + 1 WHERE id = ?", parked.ID,
		); err != nil {
			t.Fatalf("advance input generation error = %v", err)
		}
		if later, _, err := globalDB.SessionLaterState(ctx, workspaceID, parked.ID); err != nil ||
			later == nil || later.PendingMessages != 0 {
			t.Fatalf("SessionLaterState(after clear) = %#v, %v, want no pending messages", later, err)
		}
		returned, err := globalDB.ReturnDueLaterSessions(ctx, baseAt.Add(30*time.Minute), 10)
		if err != nil || len(returned) != 0 {
			t.Fatalf("ReturnDueLaterSessions(before return) = %#v, %v, want none", returned, err)
		}
		returned, err = globalDB.ReturnDueLaterSessions(ctx, returnAt, 10)
		if err != nil || len(returned) != 1 || returned[0].SessionID != parked.ID ||
			returned[0].Reason != "revisit after billing" {
			t.Fatalf("ReturnDueLaterSessions(at return) = %#v, %v, want the parked session", returned, err)
		}
		again, err := globalDB.ReturnDueLaterSessions(ctx, returnAt.Add(time.Hour), 10)
		if err != nil || len(again) != 0 {
			t.Fatalf("ReturnDueLaterSessions(repeat) = %#v, %v, want no second return", again, err)
		}
		later, returnedAt, err := globalDB.SessionLaterState(ctx, workspaceID, parked.ID)
		if err != nil || later != nil || returnedAt == nil || !returnedAt.Equal(returnAt) {
			t.Fatalf(
				"SessionLaterState(returned) = %#v, %v, %v, want unparked with return mark",
				later,
				returnedAt,
				err,
			)
		}
		needsYou, err := globalDB.PageSessions(ctx, store.SessionCatalogPageQuery{
			ReadScope:     store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID:   workspaceID,
			AttentionOnly: true,
			Sort:          sessionCatalogSortAttention,
			Limit:         10,
		})
		if err != nil || !slices.Equal(sessionIDsForWorkspaceStateIndexTest(needsYou.Sessions), []string{parked.ID}) {
			t.Fatalf("PageSessions(attention) = %#v, %v, want the session back from Later", needsYou.Sessions, err)
		}
		cleared, err := globalDB.ClearLaterReturn(ctx, parked.ID)
		if err != nil || !cleared {
			t.Fatalf("ClearLaterReturn() = %v, %v, want cleared", cleared, err)
		}

		first, err := globalDB.ParkSession(ctx, store.SessionLaterPark{WorkspaceID: workspaceID, SessionID: parked.ID})
		if err != nil {
			t.Fatalf("ParkSession(again) error = %v", err)
		}
		globalDB.now = func() time.Time { return baseAt.Add(3 * time.Hour) }
		if _, err := globalDB.ParkSession(ctx, store.SessionLaterPark{
			WorkspaceID: workspaceID, SessionID: parked.ID, Reason: "newer park",
		}); err != nil {
			t.Fatalf("ParkSession(newer) error = %v", err)
		}
		undone, err := globalDB.UnparkSessionParkedAt(ctx, workspaceID, parked.ID, first.LaterState().ParkedAt)
		if err != nil || undone {
			t.Fatalf("UnparkSessionParkedAt(stale park) = %v, %v, want the newer park kept", undone, err)
		}
		archived, err := globalDB.SetSessionArchived(ctx, workspaceID, parked.ID, true)
		if err != nil || archived.ArchivedAt == nil || archived.LaterState() != nil {
			t.Fatalf("SetSessionArchived(parked) = %#v, %v, want archived without Later", archived, err)
		}
		if _, err := globalDB.ParkSession(ctx, store.SessionLaterPark{
			WorkspaceID: workspaceID, SessionID: parked.ID,
		}); !errors.Is(err, store.ErrSessionArchived) {
			t.Fatalf("ParkSession(archived) error = %v, want ErrSessionArchived", err)
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
			ReadScope:           store.ReadScope{ProfileID: store.DefaultProfileID},
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
				ProfileID: store.DefaultProfileID,
				CreatedAt: baseAt, UpdatedAt: baseAt,
			},
			{
				ID: "wt-other", WorkspaceID: workspaceID, Name: "other", Branch: "feature/other",
				Path: filepath.Join(t.TempDir(), "other"), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				ProfileID: store.DefaultProfileID,
				CreatedAt: baseAt, UpdatedAt: baseAt,
			},
			{
				ID: "wt-foreign", WorkspaceID: foreignWorkspaceID, Name: "foreign", Branch: "feature/foreign",
				Path: filepath.Join(t.TempDir(), "foreign"), State: worktreepkg.StateReady,
				Origin: worktreepkg.OriginManual, SetupState: worktreepkg.SetupNone,
				ProfileID: store.DefaultProfileID,
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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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
			Kind:            store.LineageKindProvenance,
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
			Kind:            store.LineageKindProvenance,
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
			ReadScope:       store.ReadScope{ProfileID: store.DefaultProfileID},
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
			ReadScope:     store.ReadScope{ProfileID: store.DefaultProfileID},
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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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
			ReadScope:           store.ReadScope{ProfileID: store.DefaultProfileID},
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
		globalMetrics, err := globalDB.AggregateSessionsByAgent(ctx, store.SessionAgentMetricsQuery{
			ReadScope:           store.ReadScope{ProfileID: store.DefaultProfileID},
			ExcludeIDs:          []string{reviewer.ID},
			ExcludeSessionTypes: []string{"dream"},
			ExcludeSpawnRoles:   []string{"memory-extractor"},
		})
		if err != nil {
			t.Fatalf("AggregateSessionsByAgent(Global) error = %v", err)
		}
		if len(globalMetrics) != 1 || globalMetrics[0].Total != 3 ||
			globalMetrics[0].Active != 2 || globalMetrics[0].Failed != 1 ||
			globalMetrics[0].RuntimeSeconds != wantRuntime+int64(now.Sub(foreign.CreatedAt).Seconds()) {
			t.Fatalf("Global agent metrics = %#v, want both project populations with exclusions", globalMetrics)
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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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

	t.Run(
		"Should page navigator bands before recent history and preserve latest-created selection",
		func(t *testing.T) {
			t.Parallel()
			ctx := testutil.Context(t)
			db := openTestGlobalDB(t)
			workspace := registerWorkspaceForGlobalTests(
				t,
				db,
				"navigator-history",
				filepath.Join(t.TempDir(), "workspace"),
			)
			base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
			for index := range 110 {
				info := sessionInfoForWorkspaceStateIndexTest(
					fmt.Sprintf("quiet-%03d", index),
					workspace,
					globalDBSessionStateStopped,
					base.Add(time.Duration(index)*time.Minute),
				)
				info.UpdatedAt = base.Add(48 * time.Hour)
				if err := db.RegisterSession(ctx, info); err != nil {
					t.Fatal(err)
				}
			}
			for _, id := range []string{"needs-old", "finished-old", "working-a", "working-b"} {
				info := sessionInfoForWorkspaceStateIndexTest(id, workspace, globalDBSessionStateActive, base)
				if strings.HasPrefix(id, "working") {
					info.State = "starting"
					info.CreatedAt = base.Add(72 * time.Hour)
				}
				if err := db.RegisterSession(ctx, info); err != nil {
					t.Fatal(err)
				}
				if id == "needs-old" {
					if _, err := db.CreatePendingInteraction(
						ctx,
						store.PendingInteractionCreate{
							InteractionID:     "navigator-pending",
							SessionID:         id,
							Kind:              "clarify",
							ProviderRequestID: "navigator-request",
							CreatedAt:         base,
						},
					); err != nil {
						t.Fatal(err)
					}
				}
				if id == "finished-old" {
					if _, err := db.db.ExecContext(
						ctx,
						"UPDATE sessions SET last_settled_revision=1 WHERE id=?",
						id,
					); err != nil {
						t.Fatal(err)
					}
				}
			}
			query := store.SessionCatalogPageQuery{
				ReadScope:   store.ReadScope{AllProfiles: true},
				WorkspaceID: workspace,
				Archive:     store.SessionArchiveExclude,
				Limit:       1,
				Sort:        sessionCatalogSortNavigator,
				SkipTotal:   true,
			}
			seen := []string{}
			for {
				page, err := db.PageSessions(ctx, query)
				if err != nil {
					t.Fatal(err)
				}
				if len(page.Sessions) == 0 {
					break
				}
				info := page.Sessions[0]
				seen = append(seen, info.ID)
				band := 3
				switch {
				case info.ID == "needs-old":
					band = 0
				case info.ID == "finished-old":
					band = 1
				case strings.HasPrefix(info.ID, "working"):
					band = 2
				}
				primary := info.UpdatedAt
				if info.AttentionSnapshot().AttentionChangedAt != nil {
					primary = *info.AttentionSnapshot().AttentionChangedAt
				}
				query.After = &store.SessionCatalogPosition{
					NavigatorBand: &band,
					PrimaryAt:     primary,
					SecondaryAt:   info.CreatedAt,
					CreatedAt:     info.CreatedAt,
					ID:            info.ID,
				}
			}
			if len(seen) != 114 ||
				!slices.Equal(seen[:4], []string{"needs-old", "finished-old", "working-a", "working-b"}) {
				t.Fatalf("navigator walk=%#v", seen)
			}
			for index, id := range seen[4:] {
				if id != fmt.Sprintf("quiet-%03d", index) {
					t.Fatalf("rest tie order=%#v", seen[4:])
				}
			}
			query.Sort = sessionCatalogSortCreated
			query.After = nil
			page, err := db.PageSessions(ctx, query)
			if err != nil || len(page.Sessions) != 1 || page.Sessions[0].ID != "working-b" {
				t.Fatalf("latest-created page=%#v error=%v", page, err)
			}
			info := page.Sessions[0]
			query.After = &store.SessionCatalogPosition{
				PrimaryAt:   info.CreatedAt,
				SecondaryAt: info.CreatedAt,
				CreatedAt:   info.CreatedAt,
				ID:          info.ID,
			}
			page, err = db.PageSessions(ctx, query)
			if err != nil || len(page.Sessions) != 1 || page.Sessions[0].ID != "working-a" {
				t.Fatalf("created tie cursor page=%#v error=%v", page, err)
			}
			order, err := sessionCatalogOrderClause(sessionCatalogSortNavigator)
			if err != nil {
				t.Fatal(err)
			}
			plan := explainQueryPlan(
				t,
				db.db,
				"SELECT id FROM sessions WHERE workspace_id = ? AND archived_at IS NULL"+order+" LIMIT 1",
				workspace,
			)
			if !strings.Contains(plan, "catalog_navigator") || strings.Contains(plan, "TEMP B-TREE") {
				t.Fatalf("navigator index plan=%s", plan)
			}
		},
	)

	t.Run("Should filter attention in one bounded count-free durable page", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"catalog-load",
			filepath.Join(t.TempDir(), "catalog-load"),
		)
		base := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
		for index := range 305 {
			info := sessionInfoForWorkspaceStateIndexTest(
				fmt.Sprintf("catalog-%03d", index),
				workspaceID,
				globalDBSessionStateStopped,
				base,
			)
			if index < 5 {
				info.State = globalDBSessionStateActive
				info.Attention = &store.SessionAttention{PendingClarifyCount: 1}
			}
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession error=%v", err)
			}
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			"UPDATE sessions SET pending_clarify_count = 1 WHERE id < 'catalog-005'",
		); err != nil {
			t.Fatalf("seed pending attention error=%v", err)
		}

		query := store.SessionCatalogPageQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			Archive:       store.SessionArchiveExclude,
			AttentionOnly: true,
			Sort:          sessionCatalogSortAttention,
			Limit:         3,
			SkipTotal:     true,
		}
		page, err := globalDB.PageSessions(ctx, query)
		if err != nil {
			t.Fatalf("PageSessions error=%v", err)
		}
		if len(page.Sessions) != 3 || page.Total != 0 {
			t.Fatalf("count-free page=%#v", page)
		}
		for _, info := range page.Sessions {
			if info.AttentionSnapshot().PendingClarifyCount != 1 {
				t.Fatalf("unexpected quiet row=%s", info.ID)
			}
		}
		query.SkipTotal = false
		counted, err := globalDB.PageSessions(ctx, query)
		if err != nil || counted.Total != 5 {
			t.Fatalf("counted page=%#v error=%v", counted, err)
		}
		// Facets are exact metadata, independent of selected badge/text and rich JSON.
		if _, err := globalDB.db.ExecContext(
			ctx,
			"UPDATE sessions SET acp_options_json = '{}' WHERE id = 'catalog-304'",
		); err != nil {
			t.Fatal(err)
		}
		query.Badges = []string{"idle"}
		query.Search = "not a visible title"
		facets, err := globalDB.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.All != 305 || facets.Facets.NeedsYou != 5 || len(facets.ByWorkspace) != 1 ||
			facets.ByWorkspace[0].WorkspaceID != workspaceID {
			t.Fatalf("facets=%#v error=%v", facets, err)
		}
		query.ExcludeIDs = []string{"catalog-000"}
		facets, err = globalDB.SessionCatalogFacets(ctx, query)
		if err != nil || facets.Facets.All != 304 || facets.Facets.NeedsYou != 4 {
			t.Fatalf("overlay-excluded facets=%#v error=%v", facets, err)
		}

		order, err := sessionCatalogOrderClause(query.Sort)
		if err != nil {
			t.Fatal(err)
		}
		plan := explainQueryPlan(t, globalDB.db, "SELECT id FROM sessions WHERE archived_at IS NULL"+order+" LIMIT 3")
		if !strings.Contains(plan, "idx_sessions_global_catalog_attention") || strings.Contains(plan, "TEMP B-TREE") {
			t.Fatalf("attention cursor plan=%s", plan)
		}
	})

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
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
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

	t.Run("Should preserve attention class order through the durable cursor", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"workspace-attention-page",
			filepath.Join(t.TempDir(), "workspace-attention-page"),
		)
		baseAt := time.Date(2026, 8, 15, 22, 0, 0, 0, time.UTC)
		type attentionSeed struct {
			id                     string
			state                  string
			failureKind            store.FailureKind
			liveness               *store.SessionLivenessMeta
			pendingPermissionCount int
			pendingClarifyCount    int
			stopVerificationFailed bool
			lastSettledRevision    int64
			offset                 time.Duration
		}
		seeds := []attentionSeed{
			{id: "sess-stop-unverified", state: "stopping", stopVerificationFailed: true, offset: 0},
			{id: "sess-quiet-newest", state: globalDBSessionStateActive, offset: 7 * time.Minute},
			{
				id:    "sess-prompting-unseen",
				state: globalDBSessionStateActive,
				liveness: &store.SessionLivenessMeta{
					Activity: &store.SessionActivityMeta{TurnID: "turn-active"},
				},
				lastSettledRevision: 1,
				offset:              6 * time.Minute,
			},
			{
				id:    "sess-stalled-unseen",
				state: globalDBSessionStateActive,
				liveness: &store.SessionLivenessMeta{
					StallState:  store.SessionStallStateDetected,
					StallReason: "stalled",
				},
				lastSettledRevision: 1,
				offset:              5 * time.Minute,
			},
			{id: "sess-finished", state: globalDBSessionStateActive, lastSettledRevision: 1, offset: 4 * time.Minute},
			{
				id:                  "sess-needs-input",
				state:               globalDBSessionStateActive,
				pendingClarifyCount: 1,
				offset:              3 * time.Minute,
			},
			{
				id:                     "sess-needs-auth",
				state:                  globalDBSessionStateActive,
				pendingPermissionCount: 1,
				offset:                 2 * time.Minute,
			},
			{
				id:          "sess-failed",
				state:       globalDBSessionStateStopped,
				failureKind: store.FailurePrompt,
				offset:      time.Minute,
			},
		}
		for _, seed := range seeds {
			info := sessionInfoForWorkspaceStateIndexTest(seed.id, workspaceID, seed.state, baseAt)
			info.UpdatedAt = baseAt.Add(seed.offset)
			info.Liveness = seed.liveness
			info.StopVerificationFailed = seed.stopVerificationFailed
			if seed.failureKind != "" {
				info.Failure = &store.SessionFailure{Kind: seed.failureKind}
			}
			if err := globalDB.RegisterSession(ctx, info); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", seed.id, err)
			}
			changedAt := store.FormatTimestamp(baseAt.Add(seed.offset))
			if _, err := globalDB.db.ExecContext(
				ctx,
				`UPDATE sessions SET pending_permission_count = ?, pending_clarify_count = ?,
last_settled_revision = ?, attention_changed_at = ? WHERE id = ?`,
				seed.pendingPermissionCount,
				seed.pendingClarifyCount,
				seed.lastSettledRevision,
				changedAt,
				seed.id,
			); err != nil {
				t.Fatalf("seed attention for %q error = %v", seed.id, err)
			}
		}
		rankRows, err := globalDB.db.QueryContext(
			ctx,
			"SELECT id, "+sessionCatalogAttentionRankExpression+
				" FROM sessions WHERE workspace_id = ? ORDER BY id",
			workspaceID,
		)
		if err != nil {
			t.Fatalf("query durable attention ranks error = %v", err)
		}
		defer func() {
			if closeErr := rankRows.Close(); closeErr != nil {
				t.Errorf("close durable attention rank rows error = %v", closeErr)
			}
		}()
		gotRanks := make(map[string]store.SessionCatalogAttentionRank, len(seeds))
		for rankRows.Next() {
			var id string
			var rank store.SessionCatalogAttentionRank
			if scanErr := rankRows.Scan(&id, &rank); scanErr != nil {
				t.Fatalf("scan durable attention rank error = %v", scanErr)
			}
			gotRanks[id] = rank
		}
		if err := rankRows.Err(); err != nil {
			t.Fatalf("iterate durable attention ranks error = %v", err)
		}
		wantRanks := map[string]store.SessionCatalogAttentionRank{
			"sess-stop-unverified":  store.SessionCatalogAttentionRankNeedsYou,
			"sess-needs-input":      store.SessionCatalogAttentionRankNeedsYou,
			"sess-needs-auth":       store.SessionCatalogAttentionRankNeedsYou,
			"sess-failed":           store.SessionCatalogAttentionRankNeedsYou,
			"sess-finished":         store.SessionCatalogAttentionRankFinished,
			"sess-quiet-newest":     store.SessionCatalogAttentionRankNone,
			"sess-prompting-unseen": store.SessionCatalogAttentionRankNone,
			"sess-stalled-unseen":   store.SessionCatalogAttentionRankNone,
		}
		if !reflect.DeepEqual(gotRanks, wantRanks) {
			t.Fatalf("durable attention ranks = %#v, want %#v", gotRanks, wantRanks)
		}

		query := store.SessionCatalogPageQuery{
			ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: workspaceID,
			Sort:        sessionCatalogSortAttention,
			Limit:       3,
		}
		facets, err := globalDB.SessionCatalogFacets(ctx, query)
		wantFacets := store.SessionCatalogFacets{All: 8, NeedsYou: 4, Working: 1, Finished: 1, Idle: 1,
			Board: store.SessionBoardCounts{NeedsYou: 4, Working: 2, Done: 2}}
		if err != nil || facets.Facets != wantFacets {
			t.Fatalf("canonical badge facets=%#v error=%v, want %#v", facets, err, wantFacets)
		}
		first, err := globalDB.PageSessions(ctx, query)
		if err != nil {
			t.Fatalf("PageSessions(first attention) error = %v", err)
		}
		if got, want := sessionIDsForWorkspaceStateIndexTest(
			first.Sessions,
		), []string{
			"sess-needs-input",
			"sess-needs-auth",
			"sess-failed",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("PageSessions(first attention) ids = %#v, want %#v", got, want)
		}
		anchor := first.Sessions[len(first.Sessions)-1]
		anchorAttention := anchor.AttentionSnapshot()
		query.After = &store.SessionCatalogPosition{
			AttentionRank: store.SessionCatalogAttentionRankNeedsYou,
			PrimaryAt:     *anchorAttention.AttentionChangedAt,
			SecondaryAt:   anchor.UpdatedAt,
			CreatedAt:     anchor.CreatedAt,
			ID:            anchor.ID,
		}
		second, err := globalDB.PageSessions(ctx, query)
		if err != nil {
			t.Fatalf("PageSessions(second attention) error = %v", err)
		}
		if got, want := sessionIDsForWorkspaceStateIndexTest(
			second.Sessions,
		), []string{
			"sess-stop-unverified",
			"sess-finished",
			"sess-quiet-newest",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("PageSessions(second attention) ids = %#v, want %#v", got, want)
		}
		if !second.Sessions[0].StopVerificationFailed {
			t.Fatal("catalog page lost durable stop verification failure")
		}
		anchor = second.Sessions[len(second.Sessions)-1]
		anchorAttention = anchor.AttentionSnapshot()
		query.After = &store.SessionCatalogPosition{
			AttentionRank: store.SessionCatalogAttentionRankNone,
			PrimaryAt:     *anchorAttention.AttentionChangedAt,
			SecondaryAt:   anchor.UpdatedAt,
			CreatedAt:     anchor.CreatedAt,
			ID:            anchor.ID,
		}
		third, err := globalDB.PageSessions(ctx, query)
		if err != nil {
			t.Fatalf("PageSessions(third attention) error = %v", err)
		}
		if got, want := sessionIDsForWorkspaceStateIndexTest(
			third.Sessions,
		), []string{
			"sess-prompting-unseen",
			"sess-stalled-unseen",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("PageSessions(third attention) ids = %#v, want %#v", got, want)
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

func TestSessionAttentionSeenFenceIsMonotonicAndPassiveReadsArePure(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	sessionID := registerAttentionSession(t, globalDB, store.SessionAttention{
		AttentionRevision:   3,
		LastSettledRevision: 3,
		LastSeenRevision:    1,
	})
	beforeRead, err := globalDB.GetSessionAttention(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionAttention(before read) error = %v", err)
	}
	if _, err := globalDB.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		ID:        sessionID,
		Limit:     1,
	}); err != nil {
		t.Fatalf("ListSessions(passive) error = %v", err)
	}
	afterRead, err := globalDB.GetSessionAttention(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionAttention(after read) error = %v", err)
	}
	if afterRead.AttentionRevision != beforeRead.AttentionRevision ||
		afterRead.LastSeenRevision != beforeRead.LastSeenRevision || afterRead.LastSeenAt != nil {
		t.Fatalf("passive read changed attention = %#v, before %#v", afterRead, beforeRead)
	}

	seenAt := time.Date(2026, 8, 15, 18, 5, 0, 0, time.UTC)
	commit, err := globalDB.MarkSessionSeen(ctx, sessionID, seenAt)
	if err != nil {
		t.Fatalf("MarkSessionSeen() error = %v", err)
	}
	if commit.After.AttentionRevision != 4 || commit.After.LastSeenRevision != 4 ||
		commit.After.LastSeenAt == nil || !commit.After.LastSeenAt.Equal(seenAt) || commit.After.Unseen() {
		t.Fatalf("MarkSessionSeen() after = %#v, want advanced seen fence", commit.After)
	}
	idempotent, err := globalDB.MarkSessionSeen(ctx, sessionID, seenAt.Add(time.Minute))
	if err != nil {
		t.Fatalf("MarkSessionSeen(idempotent) error = %v", err)
	}
	if idempotent.Outcome != "already-seen" || idempotent.After.AttentionRevision != 4 ||
		idempotent.After.LastSeenRevision != 4 || !idempotent.After.LastSeenAt.Equal(seenAt) {
		t.Fatalf("MarkSessionSeen(idempotent) = %#v, want unchanged fence", idempotent)
	}
}

// Invariant: the early-stop mark is durable attention that the SQL catalog badge, attention
// filter, and attention rank read exactly like session.CanonicalBadge; clearing it restores done.
func TestSessionAttentionEndedEarlyMarkDrivesDurableCatalog(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	sessionID := registerAttentionSession(t, globalDB, store.SessionAttention{
		AttentionRevision: 2, LastSettledRevision: 2, LastSeenRevision: 1,
	})
	page := func(query store.SessionCatalogPageQuery) []string {
		t.Helper()
		query.ReadScope = store.ReadScope{ProfileID: store.DefaultProfileID}
		query.Limit = 10
		if query.Sort == "" {
			query.Sort = sessionCatalogSortRecent
		}
		result, err := globalDB.PageSessions(ctx, query)
		if err != nil {
			t.Fatalf("PageSessions(%#v) error = %v", query, err)
		}
		ids := make([]string, 0, len(result.Sessions))
		for _, session := range result.Sessions {
			ids = append(ids, session.ID)
		}
		return ids
	}

	commit, err := globalDB.MarkSessionEndedEarly(ctx, sessionID, "turn-early")
	if err != nil {
		t.Fatalf("MarkSessionEndedEarly() error = %v", err)
	}
	if commit.Before.EndedEarlyTurnID != "" || commit.After.EndedEarlyTurnID != "turn-early" ||
		commit.After.AttentionRevision != commit.Before.AttentionRevision {
		t.Fatalf("MarkSessionEndedEarly() commit = %#v, want the mark without a revision bump", commit)
	}
	listed, err := globalDB.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, ID: sessionID, Limit: 1,
	})
	if err != nil || len(listed) != 1 || listed[0].AttentionSnapshot().EndedEarlyTurnID != "turn-early" {
		t.Fatalf("ListSessions() = %#v, %v; want the durable early-stop mark", listed, err)
	}
	if got := page(
		store.SessionCatalogPageQuery{Badges: []string{"needs-attention"}},
	); !slices.Equal(
		got,
		[]string{sessionID},
	) {
		t.Fatalf("needs-attention page = %#v, want the early-stopped session", got)
	}
	if got := page(
		store.SessionCatalogPageQuery{AttentionOnly: true, Sort: sessionCatalogSortAttention},
	); !slices.Equal(
		got,
		[]string{sessionID},
	) {
		t.Fatalf("attention page = %#v, want the early-stopped session", got)
	}

	if _, err := globalDB.MarkSessionEndedEarly(ctx, sessionID, ""); err != nil {
		t.Fatalf("MarkSessionEndedEarly(clear) error = %v", err)
	}
	if got := page(store.SessionCatalogPageQuery{Badges: []string{"needs-attention"}}); len(got) != 0 {
		t.Fatalf("needs-attention page after clear = %#v, want none", got)
	}
	if got := page(store.SessionCatalogPageQuery{Badges: []string{"done"}}); !slices.Equal(got, []string{sessionID}) {
		t.Fatalf("done page after clear = %#v, want the unseen settled session", got)
	}
}

func TestSessionAttentionLifecycleUpdateBumpsOrderingFenceAtomically(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	sessionID := registerAttentionSession(t, globalDB, store.SessionAttention{AttentionRevision: 7})
	changedAt := time.Date(2026, 8, 15, 18, 6, 0, 0, time.UTC)
	if err := globalDB.UpdateSessionState(ctx, store.SessionStateUpdate{
		ID: sessionID, State: globalDBSessionStateStopped,
		AttentionTransition: true, UpdatedAt: changedAt,
	}); err != nil {
		t.Fatalf("UpdateSessionState(attention edge) error = %v", err)
	}
	attention, err := globalDB.GetSessionAttention(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionAttention() error = %v", err)
	}
	if attention.AttentionRevision != 8 || attention.AttentionChangedAt == nil ||
		!attention.AttentionChangedAt.Equal(changedAt) {
		t.Fatalf("attention after lifecycle edge = %#v, want revision 8 at %v", attention, changedAt)
	}
}

func TestPendingInteractionCanonicalTransactionRedactsAndRollsBackTogether(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	sessionID := registerAttentionSession(t, globalDB, store.SessionAttention{})
	createdAt := time.Date(2026, 8, 15, 18, 7, 0, 0, time.UTC)
	if _, err := globalDB.db.ExecContext(ctx, `CREATE TRIGGER fail_attention_projection
BEFORE UPDATE OF pending_clarify_count ON sessions
BEGIN SELECT RAISE(ABORT, 'injected attention projection failure'); END`); err != nil {
		t.Fatalf("create failure trigger error = %v", err)
	}
	_, err := globalDB.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
		InteractionID: "int-rollback", SessionID: sessionID,
		Kind: store.PendingInteractionKindClarify, ProviderRequestID: "clarify-rollback",
		Title: "rollback", CreatedAt: createdAt,
	})
	if err == nil {
		t.Fatal("CreatePendingInteraction(injected failure) error = nil")
	}
	if _, err := globalDB.db.ExecContext(ctx, `DROP TRIGGER fail_attention_projection`); err != nil {
		t.Fatalf("drop failure trigger error = %v", err)
	}
	assertPendingInteractionRowCount(t, globalDB.db, sessionID, 0)
	attention, err := globalDB.GetSessionAttention(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSessionAttention(after rollback) error = %v", err)
	}
	if attention.PendingClarifyCount != 0 || attention.AttentionRevision != 0 ||
		attention.AttentionChangedAt != nil {
		t.Fatalf("attention after rollback = %#v, want zero projection", attention)
	}

	const secret = "compozy_claim_PENDING_SECRET_123"
	commit, err := globalDB.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
		InteractionID: "int-redacted", SessionID: sessionID,
		Kind: store.PendingInteractionKindClarify, ProviderRequestID: "clarify-redacted",
		TurnID: "turn-1", Title: strings.Repeat("title ", 50) + secret,
		Payload:   store.PendingInteractionPayload{Choices: []string{strings.Repeat("choice ", 30) + secret}},
		CreatedAt: createdAt.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreatePendingInteraction(redacted) error = %v", err)
	}
	if commit.After.PendingClarifyCount != 1 || commit.After.AttentionRevision != 1 ||
		commit.After.AttentionChangedAt == nil {
		t.Fatalf("CreatePendingInteraction() commit = %#v, want coupled projection", commit)
	}
	listed, err := globalDB.ListPendingInteractions(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("ListPendingInteractions() error = %v", err)
	}
	if len(listed) != 1 || strings.Contains(listed[0].Title, secret) ||
		len(listed[0].Title) > store.PendingInteractionTitleMaxBytes || len(listed[0].Payload.Choices) != 1 ||
		strings.Contains(listed[0].Payload.Choices[0], secret) ||
		len(listed[0].Payload.Choices[0]) > store.PendingInteractionChoiceMaxBytes {
		t.Fatalf("listed interaction = %#v, want bounded redacted content", listed)
	}
	resolved, err := globalDB.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
		InteractionID: "int-redacted",
		Status:        store.PendingInteractionStatusResolved,
		Resolution:    strings.Repeat("resolution ", 40) + secret,
		ResolvedBy:    "operator:test",
		At:            createdAt.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("TransitionPendingInteraction(resolved) error = %v", err)
	}
	if resolved.After.PendingClarifyCount != 0 || resolved.After.AttentionRevision != 2 ||
		resolved.Interaction == nil || strings.Contains(resolved.Interaction.Resolution, secret) ||
		len(resolved.Interaction.Resolution) > store.PendingInteractionResolutionMaxBytes {
		t.Fatalf("resolved interaction commit = %#v, want bounded redacted resolution", resolved)
	}

	for index, requestID := range []string{"permission-concurrent-a", "permission-concurrent-b"} {
		if _, err := globalDB.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
			InteractionID: "int-" + requestID, SessionID: sessionID,
			Kind: store.PendingInteractionKindPermission, ProviderRequestID: requestID,
			Title: "Approve concurrent request", CreatedAt: createdAt.Add(time.Duration(index+3) * time.Second),
		}); err != nil {
			t.Fatalf("CreatePendingInteraction(%s) error = %v", requestID, err)
		}
	}
	concurrentAttention, err := globalDB.GetSessionAttention(ctx, sessionID)
	if err != nil || concurrentAttention.PendingPermissionCount != 2 {
		t.Fatalf("attention with concurrent permissions = %#v, %v, want count 2", concurrentAttention, err)
	}
	if _, err := globalDB.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
		InteractionID: "int-permission-concurrent-a",
		Status:        store.PendingInteractionStatusResolved,
		Resolution:    "allow-once",
		ResolvedBy:    "operator:test",
		At:            createdAt.Add(6 * time.Second),
	}); err != nil {
		t.Fatalf("TransitionPendingInteraction(concurrent first) error = %v", err)
	}
	actionable, err := globalDB.ListPendingInteractions(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("ListPendingInteractions(concurrent remaining) error = %v", err)
	}
	if len(actionable) != 1 || actionable[0].InteractionID != "int-permission-concurrent-b" {
		t.Fatalf("actionable concurrent interactions = %#v, want only request b", actionable)
	}
}

func TestOrphanInteractionResolutionCouplesQueueAndIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	sessionID := registerAttentionSession(t, globalDB, store.SessionAttention{})
	now := time.Date(2026, 8, 15, 18, 8, 0, 0, time.UTC)
	if _, err := globalDB.CreatePendingInteraction(ctx, store.PendingInteractionCreate{
		InteractionID: "int-orphan", SessionID: sessionID,
		Kind: store.PendingInteractionKindPermission, ProviderRequestID: "permission-orphan",
		Title: "Approve command", CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreatePendingInteraction() error = %v", err)
	}
	if _, err := globalDB.TransitionPendingInteraction(ctx, store.PendingInteractionTransition{
		InteractionID: "int-orphan", Status: store.PendingInteractionStatusOrphaned, At: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("TransitionPendingInteraction(orphan) error = %v", err)
	}
	if _, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
		ID: "input-existing", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
		Text: "existing", QueueCap: 1, Now: now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("EnqueueSessionInput(existing) error = %v", err)
	}
	transition := store.PendingInteractionTransition{
		InteractionID: "int-orphan", Status: store.PendingInteractionStatusResolved,
		Resolution: "allow-once", ResolvedBy: "operator:test", At: now.Add(3 * time.Second),
		OrphanInput: &store.SessionInputQueueInsert{
			ID: "input-orphan", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
			Text: "allow-once", QueueCap: 1, Now: now.Add(3 * time.Second),
		},
	}
	if _, err := globalDB.TransitionPendingInteraction(ctx, transition); !errors.Is(
		err,
		store.ErrSessionInputQueueFull,
	) {
		t.Fatalf("TransitionPendingInteraction(queue full) error = %v", err)
	}
	actionable, err := globalDB.ListPendingInteractions(ctx, sessionID, nil)
	if err != nil {
		t.Fatalf("ListPendingInteractions(after queue full) error = %v", err)
	}
	if len(actionable) != 1 || actionable[0].Status != store.PendingInteractionStatusOrphaned {
		t.Fatalf("actionable after queue full = %#v, want untouched orphan", actionable)
	}
	attention, err := globalDB.GetSessionAttention(ctx, sessionID)
	if err != nil || attention.PendingPermissionCount != 1 {
		t.Fatalf("attention after queue full = %#v, %v, want pending count 1", attention, err)
	}

	transition.OrphanInput.QueueCap = 2
	resolved, err := globalDB.TransitionPendingInteraction(ctx, transition)
	if err != nil {
		t.Fatalf("TransitionPendingInteraction(after restart) error = %v", err)
	}
	if resolved.Outcome != "resolved-after-restart" || resolved.After.PendingPermissionCount != 0 {
		t.Fatalf("resolved outcome = %#v, want resolved-after-restart", resolved)
	}
	duplicate, err := globalDB.TransitionPendingInteraction(ctx, transition)
	if err != nil {
		t.Fatalf("TransitionPendingInteraction(duplicate) error = %v", err)
	}
	if duplicate.Outcome != "already-resolved" {
		t.Fatalf("duplicate outcome = %q, want already-resolved", duplicate.Outcome)
	}
	queued, err := globalDB.ListPendingSessionInputs(ctx, sessionID)
	if err != nil {
		t.Fatalf("ListPendingSessionInputs() error = %v", err)
	}
	if len(queued) != 2 || queued[1].ID != "input-orphan" {
		t.Fatalf("pending inputs = %#v, want one atomic orphan-resolution enqueue", queued)
	}
}

func registerAttentionSession(
	t *testing.T,
	globalDB *GlobalDB,
	attention store.SessionAttention,
) string {
	t.Helper()
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"attention-workspace",
		filepath.Join(t.TempDir(), "attention-workspace"),
	)
	sessionID := "sess-attention"
	now := time.Date(2026, 8, 15, 18, 0, 0, 0, time.UTC)
	if err := globalDB.RegisterSession(testutil.Context(t), store.SessionInfo{
		ProfileID: store.DefaultProfileID,
		ID:        sessionID, Name: "Attention", AgentName: "coder",
		RuntimeStatus: store.SessionRuntimeUnbound, WorkspaceID: workspaceID,
		SessionType: "user", State: globalDBSessionStateActive,
		Attention: store.CloneSessionAttention(&attention),
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`UPDATE sessions SET pending_permission_count = ?, pending_clarify_count = ?,
attention_revision = ?, last_settled_revision = ?, last_seen_revision = ?,
last_seen_at = ?, attention_changed_at = ? WHERE id = ?`,
		attention.PendingPermissionCount,
		attention.PendingClarifyCount,
		attention.AttentionRevision,
		attention.LastSettledRevision,
		attention.LastSeenRevision,
		nullableAttentionTestTime(attention.LastSeenAt),
		nullableAttentionTestTime(attention.AttentionChangedAt),
		sessionID,
	); err != nil {
		t.Fatalf("seed session attention error = %v", err)
	}
	return sessionID
}

func nullableAttentionTestTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return store.FormatTimestamp(*value)
}

func assertPendingInteractionRowCount(t *testing.T, db *sql.DB, sessionID string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(
		testutil.Context(t),
		"SELECT COUNT(*) FROM session_pending_interactions WHERE session_id = ?",
		sessionID,
	).Scan(&got); err != nil {
		t.Fatalf("count pending interaction rows error = %v", err)
	}
	if got != want {
		t.Fatalf("pending interaction rows = %d, want %d", got, want)
	}
}

func sessionInfoForWorkspaceStateIndexTest(
	id string,
	workspaceID string,
	state string,
	baseAt time.Time,
) store.SessionInfo {
	return store.SessionInfo{
		ID:            id,
		ProfileID:     store.DefaultProfileID,
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

// UT-053, IT-029: subagent visibility is filtered before pagination and summaries are batched.
func TestGlobalDBSubagentCatalog(t *testing.T) {
	t.Run("Should filter subagents before the limit and summarize parents", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		for i := range 119 {
			id := fmt.Sprintf("catalog-%03d", i)
			role, parentID := "", ""
			if i < 30 {
				role, parentID = store.SubagentSpawnRole, parent
			}
			registerSubagentSession(t, db, workspace, id, parentID, role, now.Add(time.Duration(i+1)*time.Second))
			if i < 30 {
				r := reserveSubagent(t, db.SessionRepo, workspace, parent, "sub-"+id, now)
				if _, err := db.LinkChild(ctx, r.ID, id, now); err != nil {
					t.Fatal(err)
				}
				if i >= 3 {
					status := store.SubagentStatusCompleted
					if i == 3 {
						status = store.SubagentStatusFailed
					}
					if _, _, err := db.FinalizeSubagent(
						ctx,
						// Settlement records the idle child, as the live finalize path does.
						store.SubagentFinalize{
							ID: r.ID, Status: status, ChildActivity: store.SubagentChildActivityIdle, SettledAt: now,
						},
					); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
		for _, filter := range []string{"", "include", "exclude", "only"} {
			expected := 100
			if filter == "exclude" {
				expected = 90
			}
			if filter == "only" {
				expected = 30
			}
			listed, err := db.ListSessions(
				ctx,
				store.SessionListQuery{
					ReadScope:   store.ReadScope{AllProfiles: true},
					WorkspaceID: workspace,
					Limit:       100,
					Subagents:   filter,
					Sort:        "recent",
				},
			)
			if err != nil || len(listed) != expected {
				t.Fatalf("list %s = %d %v", filter, len(listed), err)
			}
			page, err := db.PageSessions(
				ctx,
				store.SessionCatalogPageQuery{
					ReadScope:   store.ReadScope{AllProfiles: true},
					WorkspaceID: workspace,
					Limit:       100,
					Subagents:   filter,
					Sort:        "recent",
				},
			)
			if err != nil || len(page.Sessions) != expected {
				t.Fatalf("page %s = %#v %v", filter, page, err)
			}
			total := 120
			if filter == "exclude" {
				total = 90
			}
			if filter == "only" {
				total = 30
			}
			if page.Total != total {
				t.Fatalf("total %s = %d", filter, page.Total)
			}
			facets, err := db.SessionCatalogFacets(ctx, store.SessionCatalogPageQuery{
				ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspace,
				Limit: 1, Subagents: filter, Sort: "recent",
			})
			if err != nil || facets.Facets.All != total || len(facets.ByWorkspace) != 1 ||
				facets.ByWorkspace[0].WorkspaceID != workspace || facets.ByWorkspace[0].Facets.All != total {
				t.Fatalf("facets %q = %#v, error=%v; want global and workspace total %d", filter, facets, err, total)
			}
			foundParent := false
			for _, r := range page.Sessions {
				isSub := r.Lineage != nil && r.Lineage.SpawnRole == store.SubagentSpawnRole
				if filter == "exclude" && isSub || filter == "only" && !isSub {
					t.Fatalf("filter %s returned %s", filter, r.ID)
				}
				foundParent = foundParent || r.ID == parent
			}
			if filter == "exclude" && !foundParent {
				t.Fatal("parent lost to child pagination")
			}
		}
		if _, err := db.ListSessions(
			ctx,
			store.SessionListQuery{ReadScope: store.ReadScope{AllProfiles: true}, Limit: 100, Subagents: "bogus"},
		); err == nil {
			t.Fatal("accepted invalid filter")
		}
		if _, err := db.PageSessions(
			ctx,
			store.SessionCatalogPageQuery{
				ReadScope: store.ReadScope{AllProfiles: true},
				Limit:     100,
				Subagents: "bogus",
			},
		); err == nil {
			t.Fatal("accepted invalid page filter")
		}
		if _, err := db.SessionCatalogFacets(ctx, store.SessionCatalogPageQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Limit: 100, Subagents: "bogus",
		}); err == nil {
			t.Fatal("accepted invalid facets filter")
		}
		sums, err := db.Summaries(ctx, []string{parent, "empty"})
		if err != nil {
			t.Fatal(err)
		}
		want := store.SubagentSummary{Total: 30, Live: 3, Failed: 1, MostUrgent: "failed"}
		if sums[parent] != want {
			t.Fatalf("summary=%#v want=%#v", sums[parent], want)
		}
		if _, ok := sums["empty"]; ok {
			t.Fatal("empty summary should be omitted")
		}
		if _, err := db.db.ExecContext(
			ctx,
			`UPDATE session_subagents SET status='waiting' WHERE id='sub-catalog-000'`,
		); err != nil {
			t.Fatal(err)
		}
		sums, err = db.Summaries(ctx, []string{parent})
		if err != nil || sums[parent].Attention != 1 || sums[parent].Live != 3 ||
			sums[parent].MostUrgent != "attention" {
			t.Fatalf("attention=%v %v", sums, err)
		}
	})
}

// UT-054, IT-030: archive changes are atomic across subagent descendants only.
func TestGlobalDBSubagentArchive(t *testing.T) {
	t.Run(
		"Should cascade archive and restore while rejecting live children and standalone changes",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db, workspace, parent, now := subagentFixture(t)
			for _, lookup := range []struct{ workspace, session string }{
				{"foreign-workspace", parent},
				{workspace, "missing-session"},
			} {
				if _, err := db.SessionArchivedAt(
					ctx,
					lookup.workspace,
					lookup.session,
				); !errors.Is(
					err,
					store.ErrSessionNotFound,
				) {
					t.Fatalf("archive metadata outside scope = %v, want ErrSessionNotFound", err)
				}
			}
			registerSubagentSession(t, db, workspace, "child", parent, store.SubagentSpawnRole, now)
			registerSubagentSession(t, db, workspace, "grandchild", "child", store.SubagentSpawnRole, now)
			registerSubagentSession(t, db, workspace, "plain", parent, "worker", now)
			if _, err := db.db.ExecContext(
				ctx,
				`UPDATE sessions SET state='stopping' WHERE id='grandchild'`,
			); err != nil {
				t.Fatal(err)
			}
			if _, err := db.SetSessionArchived(
				ctx,
				workspace,
				parent,
				true,
			); !errors.Is(err, store.ErrSessionArchiveRequiresStopped) ||
				!strings.Contains(err.Error(), "grandchild") {
				t.Fatalf("live descendant=%v", err)
			} else if live, ok := errors.AsType[*store.SessionArchiveLiveMemberError](err); !ok ||
				live.SessionID != "grandchild" {
				t.Fatalf("live descendant member = %#v, want grandchild", live)
			}
			for _, id := range []string{parent, "child", "grandchild", "plain"} {
				at, err := db.SessionArchivedAt(ctx, workspace, id)
				if err != nil || at != nil {
					t.Fatalf("partial archive %s = %v %v", id, at, err)
				}
			}
			if _, err := db.db.ExecContext(
				ctx,
				`UPDATE sessions SET state='stopped' WHERE id='grandchild'`,
			); err != nil {
				t.Fatal(err)
			}
			for _, archived := range []bool{true, false} {
				if _, err := db.SetSessionArchived(ctx, workspace, parent, archived); err != nil {
					t.Fatal(err)
				}
				for _, id := range []string{parent, "child", "grandchild"} {
					at, err := db.SessionArchivedAt(ctx, workspace, id)
					if err != nil || (at != nil) != archived {
						t.Fatalf("cascade %s = %v %v", id, at, err)
					}
				}
				if _, err := db.SetSessionArchived(
					ctx,
					workspace,
					"child",
					archived,
				); !errors.Is(
					err,
					store.ErrSubagentArchiveFollowsParent,
				) {
					t.Fatalf("standalone=%v", err)
				}
				at, err := db.SessionArchivedAt(ctx, workspace, "plain")
				if err != nil || at != nil {
					t.Fatalf("plain child=%v %v", at, err)
				}
			}
		},
	)
}

func TestGlobalDBSessionTree(t *testing.T) {
	t.Parallel()

	// Invariant: the tree is the session list hierarchy, inside the root's workspace
	// and profile, and archiving it is all-or-nothing.
	t.Run("Should walk hierarchy children only, inside the workspace", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		other := registerWorkspaceForGlobalTests(t, db, "tree-other", filepath.Join(t.TempDir(), "other"))
		registerSubagentSession(t, db, workspace, "child", parent, "worker", now)
		registerSubagentSession(t, db, workspace, "grandchild", "child", store.SubagentSpawnRole, now)
		registerSubagentSession(t, db, workspace, "continued", parent, "", now)
		registerSubagentSession(t, db, workspace, "forked", parent, "", now)
		registerSubagentSession(t, db, workspace, "title", parent, "auto-title", now)
		registerSubagentSession(t, db, other, "foreign", parent, "worker", now)
		registerSubagentSession(t, db, workspace, "loop", "grandchild", "worker", now)
		for _, statement := range []string{
			`UPDATE sessions SET lineage_kind='continue' WHERE id='continued'`,
			`UPDATE sessions SET lineage_kind='fork' WHERE id='forked'`,
			// A corrupt cycle through the root ends the walk instead of looping.
			`UPDATE sessions SET parent_session_id='loop' WHERE id='parent'`,
			`UPDATE sessions SET spawn_depth=1 WHERE id='child'`,
			`UPDATE sessions SET spawn_depth=2 WHERE id='grandchild'`,
			`UPDATE sessions SET spawn_depth=3 WHERE id='loop'`,
		} {
			if _, err := db.db.ExecContext(ctx, statement); err != nil {
				t.Fatal(err)
			}
		}
		descendants, err := db.ListSessionDescendants(ctx, workspace, parent)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, 0, len(descendants))
		for i := range descendants {
			got = append(got, descendants[i].ID)
		}
		if want := []string{"child", "grandchild", "loop"}; !slices.Equal(got, want) {
			t.Fatalf("descendants = %v, want %v shallowest first", got, want)
		}
		foreign, err := db.ListSessionDescendants(ctx, other, parent)
		if err != nil || len(foreign) != 0 {
			t.Fatalf("descendants outside the root's workspace = %v, %v, want none", foreign, err)
		}
	})

	t.Run("Should not walk a child in another profile", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		theirs := store.SessionInfo{
			ID: "theirs", ProfileID: insertLayoutTestProfile(ctx, t, db), AgentName: "coder", Provider: "claude",
			WorkspaceID: workspace, RuntimeStatus: store.SessionRuntimeUnbound, SessionType: defaultSessionType,
			State: "stopped", CreatedAt: now, UpdatedAt: now,
			Lineage: &store.SessionLineage{
				Kind: store.LineageKindSpawn, ParentSessionID: parent, RootSessionID: parent, SpawnDepth: 1,
			},
		}
		if err := db.RegisterSession(ctx, theirs); err != nil {
			t.Fatal(err)
		}
		descendants, err := db.ListSessionDescendants(ctx, workspace, parent)
		if err != nil || len(descendants) != 0 {
			t.Fatalf("descendants across profiles = %v, %v, want none", descendants, err)
		}
	})

	t.Run("Should archive the whole tree or nothing and leave unarchive to each session", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		registerSubagentSession(t, db, workspace, "child", parent, "worker", now)
		registerSubagentSession(t, db, workspace, "grandchild", "child", "worker", now)
		if _, err := db.db.ExecContext(ctx, `UPDATE sessions SET state='active' WHERE id='grandchild'`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ArchiveSessionTree(ctx, workspace, parent); !errors.Is(err, store.ErrSessionArchiveRequiresStopped) {
			t.Fatalf("live member = %v", err)
		} else if live, ok := errors.AsType[*store.SessionArchiveLiveMemberError](err); !ok ||
			live.SessionID != "grandchild" {
			t.Fatalf("live member = %#v, want grandchild", live)
		}
		for _, id := range []string{parent, "child", "grandchild"} {
			if at, err := db.SessionArchivedAt(ctx, workspace, id); err != nil || at != nil {
				t.Fatalf("partial archive %s = %v %v", id, at, err)
			}
		}
		if _, err := db.db.ExecContext(ctx, `UPDATE sessions SET state='stopped' WHERE id='grandchild'`); err != nil {
			t.Fatal(err)
		}
		tree, err := db.ArchiveSessionTree(ctx, workspace, parent)
		if err != nil || tree.Root.ArchivedAt == nil || len(tree.Descendants) != 2 {
			t.Fatalf("ArchiveSessionTree() = %#v, %v", tree, err)
		}
		for i := range tree.Descendants {
			if tree.Descendants[i].ArchivedAt == nil {
				t.Fatalf("member %s stayed unarchived", tree.Descendants[i].ID)
			}
		}
		// Restoring the parent keeps today's subagent-family reach.
		if _, err := db.SetSessionArchived(ctx, workspace, parent, false); err != nil {
			t.Fatal(err)
		}
		if at, err := db.SessionArchivedAt(ctx, workspace, "child"); err != nil || at == nil {
			t.Fatalf("worker child after parent unarchive = %v %v, want still archived", at, err)
		}
	})

	t.Run("Should keep children listed after their parent is archived or deleted", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		registerSubagentSession(t, db, workspace, "child", parent, "worker", now)
		if _, err := db.SetSessionArchived(ctx, workspace, parent, true); err != nil {
			t.Fatal(err)
		}
		if err := db.DeleteSession(ctx, parent); err != nil {
			t.Fatal(err)
		}
		rows, err := db.ListSessions(ctx, store.SessionListQuery{
			ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: workspace,
		})
		if err != nil || len(rows) != 1 || rows[0].ID != "child" || rows[0].ArchivedAt != nil {
			t.Fatalf("rows after parent delete = %#v, %v, want the child still listed", rows, err)
		}
	})
}

func TestPageSessionsManualOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should order folders, then unplaced and pinned root sessions across keyset pages", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db, scope := openSessionLayoutTestDB(t, "manual-page")
		base := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
		for index, id := range []string{"old", "pinned-a", "pinned-b", "in-x", "in-y", "new"} {
			info := sessionInfoForWorkspaceStateIndexTest(id, scope.WorkspaceID, globalDBSessionStateStopped,
				base.Add(time.Duration(index)*time.Minute))
			if err := db.RegisterSession(ctx, info); err != nil {
				t.Fatal(err)
			}
		}
		createLayoutFolder(ctx, t, db, scope, "sfold-y", "Y")
		createLayoutFolder(ctx, t, db, scope, "sfold-x", "X")
		if err := db.MoveSession(ctx, scope, "in-x", "sfold-x"); err != nil {
			t.Fatal(err)
		}
		if err := db.MoveSession(ctx, scope, "in-y", "sfold-y"); err != nil {
			t.Fatal(err)
		}
		if err := db.ReorderSessions(ctx, scope, "", []string{"pinned-b", "pinned-a"}); err != nil {
			t.Fatal(err)
		}

		want := []string{"in-x", "in-y", "new", "old", "pinned-b", "pinned-a"}
		full := pageManual(ctx, t, db, scope.WorkspaceID, nil)
		if got := sessionIDsFromInfos(full.Sessions); !slices.Equal(got, want) {
			t.Fatalf("manual order = %v, want %v", got, want)
		}

		var walked []string
		var after *store.SessionCatalogPosition
		for range len(want) {
			page, err := db.PageSessions(ctx, store.SessionCatalogPageQuery{
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
				WorkspaceID: scope.WorkspaceID,
				Archive:     store.SessionArchiveInclude,
				Sort:        "manual",
				Limit:       1,
				After:       after,
			})
			if err != nil {
				t.Fatalf("PageSessions(manual, limit 1) error = %v", err)
			}
			if len(page.Sessions) == 0 {
				break
			}
			last := page.Sessions[0]
			walked = append(walked, last.ID)
			placements := readPlacements(ctx, t, db, last.ID)
			rank, position := store.SessionManualRootRank, store.SessionManualUnplacedPosition
			if placement, ok := placements[last.ID]; ok {
				position = placement.Position
				if placement.FolderID != "" {
					rank = placement.FolderPosition
				}
			}
			after = &store.SessionCatalogPosition{
				ManualFolderRank: &rank,
				ManualPosition:   &position,
				PrimaryAt:        last.CreatedAt,
				SecondaryAt:      last.CreatedAt,
				CreatedAt:        last.CreatedAt,
				ID:               last.ID,
			}
		}
		if !slices.Equal(walked, want) {
			t.Fatalf("keyset walk = %v, want %v", walked, want)
		}
	})
}

func sessionIDsFromInfos(infos []store.SessionInfo) []string {
	ids := make([]string, 0, len(infos))
	for index := range infos {
		ids = append(ids, infos[index].ID)
	}
	return ids
}

func TestSessionCatalogBoard(t *testing.T) {
	t.Parallel()
	t.Run("Should bind repository and default branches to literal workspace IDs", func(t *testing.T) {
		t.Parallel()
		ctx, db := t.Context(), openTestGlobalDB(t)
		ids := []string{`workspace.one`, `workspace[2]`, `workspace"three`, `workspace\four`}
		query := store.SessionCatalogPageQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Archive: store.SessionArchiveExclude,
			Sort: sessionCatalogSortRecent, Limit: 10, Columns: []string{"in_review"},
			RepositoryKeys: map[string]string{}, DefaultBranches: map[string][]string{},
		}
		for _, id := range ids {
			query.RepositoryKeys[id] = "github.com/owner/repo"
			if _, err := db.db.ExecContext(
				ctx,
				`INSERT INTO workspaces(id,root_dir,name,created_at,updated_at) VALUES (?, ?, ?, '2026-10-26T00:00:00Z', '2026-10-26T00:00:00Z')`,
				id,
				"/"+id,
				id,
			); err != nil {
				t.Fatal(err)
			}
			info := sessionInfoForWorkspaceStateIndexTest(
				id,
				id,
				globalDBSessionStateStopped,
				time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
			)
			if err := db.RegisterSession(ctx, info); err != nil {
				t.Fatal(err)
			}
			if err := db.SaveSessionBranch(ctx, id, id, "topic"); err != nil {
				t.Fatal(err)
			}
			if _, err := db.db.ExecContext(
				ctx,
				`INSERT INTO forge_pull_requests(workspace_id,repo_key,branch,pr_state) VALUES (?, ?, 'topic', 'open')`,
				id,
				query.RepositoryKeys[id],
			); err != nil {
				t.Fatal(err)
			}
		}
		for _, id := range ids {
			query.DefaultBranches[id] = []string{"topic"}
			page, err := db.PageSessions(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != len(ids)-len(query.DefaultBranches) {
				t.Fatalf("default exclusion for %q: total=%d", id, page.Total)
			}
		}
		query.DefaultBranches = nil
		delete(query.RepositoryKeys, ids[0])
		page, err := db.PageSessions(ctx, query)
		if err != nil || page.Total != len(ids)-1 {
			t.Fatalf("missing repository: total=%d error=%v", page.Total, err)
		}
		source, args, err := sessionCatalogBoardSource(query.RepositoryKeys, query.DefaultBranches)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(
			explainQueryPlan(
				t,
				db.db,
				"SELECT COUNT(1) FROM "+source+" WHERE "+sessionCatalogBoardColumnExpression+" = 'in_review'",
				args...),
		)
	})
	t.Run("Should use only a pending or ready worktree branch and never its root snapshot", func(t *testing.T) {
		t.Parallel()
		ctx, db := t.Context(), openTestGlobalDB(t)
		workspace := registerWorkspaceForGlobalTests(t, db, "board-branch", filepath.Join(t.TempDir(), "workspace"))
		base := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
		if _, err := db.db.ExecContext(
			ctx,
			`INSERT INTO worktrees(id, profile_id, workspace_id, name, branch, path, state, origin, created_at, updated_at) VALUES ('board-tree', ?, ?, 'board-tree', 'feat/tree', '/board/tree', 'ready', 'manual', ?, ?)`,
			store.DefaultProfileID,
			workspace,
			store.FormatTimestamp(base),
			store.FormatTimestamp(base),
		); err != nil {
			t.Fatal(err)
		}
		info := sessionInfoForWorkspaceStateIndexTest("tree-session", workspace, globalDBSessionStateStopped, base)
		info.WorktreeID = "board-tree"
		if err := db.RegisterSession(ctx, info); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, "UPDATE sessions SET branch='feat/root' WHERE id=?", info.ID); err != nil {
			t.Fatal(err)
		}
		for _, branch := range []string{"feat/root", "feat/tree"} {
			if _, err := db.db.ExecContext(
				ctx,
				"INSERT INTO forge_pull_requests(workspace_id,repo_key,branch,pr_state) VALUES (?,'github.com/owner/repo',?,'open')",
				workspace,
				branch,
			); err != nil {
				t.Fatal(err)
			}
		}
		query := store.SessionCatalogPageQuery{
			ReadScope:      store.ReadScope{AllProfiles: true},
			WorkspaceID:    workspace,
			Archive:        store.SessionArchiveExclude,
			Sort:           sessionCatalogSortRecent,
			Limit:          10,
			Columns:        []string{"in_review"},
			RepositoryKeys: map[string]string{workspace: "github.com/owner/repo"},
		}
		for _, state := range []string{"ready", "pending", "failed", "missing", "removed"} {
			if _, err := db.db.ExecContext(
				ctx,
				"UPDATE worktrees SET state=? WHERE id='board-tree'",
				state,
			); err != nil {
				t.Fatal(err)
			}
			page, err := db.PageSessions(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if state == "ready" || state == "pending" {
				want = 1
			}
			if page.Total != want {
				t.Fatalf("worktree %s returned %d, want %d", state, page.Total, want)
			}
		}
	})

	t.Run("Should scope direct child rollups and preserve archived parent titles", func(t *testing.T) {
		t.Parallel()
		ctx, db := t.Context(), openTestGlobalDB(t)
		workspace := registerWorkspaceForGlobalTests(t, db, "relations", filepath.Join(t.TempDir(), "workspace"))
		base := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
		const foreignProfile = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		if _, err := db.db.ExecContext(
			ctx,
			`INSERT INTO profiles (id,name,color,icon,state,created_at) VALUES (?, 'foreign','#8E8EB5','circle','active',?)`,
			foreignProfile,
			store.FormatTimestamp(base),
		); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{
			"parent", "needs", "working-1", "working-2", "working-3", "archived", "parked", "grandchild", "foreign",
		} {
			state := globalDBSessionStateStopped
			if strings.HasPrefix(id, "working") {
				state = "starting"
			}
			info := sessionInfoForWorkspaceStateIndexTest(id, workspace, state, base)
			if id == "foreign" {
				info.ProfileID = foreignProfile
			}
			if err := db.RegisterSession(ctx, info); err != nil {
				t.Fatal(err)
			}
			if id != "parent" {
				parent := "parent"
				if id == "grandchild" {
					parent = "working-1"
				}
				if _, err := db.db.ExecContext(
					ctx,
					"UPDATE sessions SET parent_session_id=? WHERE id=?",
					parent,
					id,
				); err != nil {
					t.Fatal(err)
				}
			}
		}
		if _, err := db.db.ExecContext(
			ctx,
			"UPDATE sessions SET archived_at=? WHERE id IN ('parent','archived')",
			store.FormatTimestamp(base),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			"UPDATE sessions SET state='active', pending_clarify_count=1 WHERE id='needs'",
		); err != nil {
			t.Fatal(err)
		}
		// A child in Later is hidden from the board, so the parent must not count it.
		if _, err := db.db.ExecContext(
			ctx, "UPDATE sessions SET parked_at=? WHERE id='parked'", store.FormatTimestamp(base),
		); err != nil {
			t.Fatal(err)
		}
		query := store.SessionCatalogRelationsQuery{
			ReadScope:  store.ReadScope{ProfileID: store.DefaultProfileID},
			SessionIDs: []string{"parent"},
			ParentIDs:  []string{"parent"},
		}
		relations, err := db.SessionCatalogRelations(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if counts := relations.Children["parent"]; counts.NeedsYou != 1 || counts.Working != 3 || counts.Done != 0 {
			t.Fatalf("children = %#v", counts)
		}
		if relations.ParentTitles["parent"] == "" {
			t.Fatal("archived parent lost title")
		}
		pageQuery := store.SessionCatalogPageQuery{
			ReadScope:    query.ReadScope,
			WorkspaceID:  workspace,
			Archive:      store.SessionArchiveInclude,
			HideChildren: true,
			Sort:         sessionCatalogSortRecent,
			Limit:        10,
		}
		page, err := db.PageSessions(ctx, pageQuery)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 1 || len(page.Sessions) != 1 || page.Sessions[0].ID != "parent" {
			t.Fatalf("root page = %#v", page)
		}
		facets, err := db.SessionCatalogFacets(ctx, pageQuery)
		if err != nil {
			t.Fatal(err)
		}
		if facets.Facets.All != 1 || facets.Facets.Board.Done != 1 {
			t.Fatalf("root facets = %#v", facets)
		}
	})

	t.Run("Should keep forked and continued sessions top-level when hiding children", func(t *testing.T) {
		t.Parallel()
		ctx, db := t.Context(), openTestGlobalDB(t)
		workspace := registerWorkspaceForGlobalTests(t, db, "lineage", filepath.Join(t.TempDir(), "workspace"))
		base := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
		lineage := map[string][2]string{
			"provenance": {string(store.LineageKindProvenance), ""},
			"spawn":      {string(store.LineageKindSpawn), ""},
			"subagent":   {string(store.LineageKindSpawn), store.SubagentSpawnRole},
			"recovery":   {string(store.LineageKindRecovery), ""},
			"fork":       {string(store.LineageKindFork), ""},
			"continue":   {string(store.LineageKindContinue), ""},
		}
		for _, id := range []string{"parent", "provenance", "spawn", "subagent", "recovery", "fork", "continue"} {
			if err := db.RegisterSession(
				ctx,
				sessionInfoForWorkspaceStateIndexTest(id, workspace, globalDBSessionStateStopped, base),
			); err != nil {
				t.Fatal(err)
			}
			if relation, ok := lineage[id]; ok {
				if _, err := db.db.ExecContext(
					ctx,
					`UPDATE sessions SET parent_session_id='parent', root_session_id='parent', spawn_depth=1,
 lineage_kind=?, spawn_role=?, origin_agent_name=CASE WHEN ? IN ('fork','continue') THEN 'coder' ELSE '' END
 WHERE id=?`,
					relation[0],
					relation[1],
					relation[0],
					id,
				); err != nil {
					t.Fatal(err)
				}
			}
		}
		scope := store.ReadScope{ProfileID: store.DefaultProfileID}
		for _, tc := range []struct {
			name         string
			hideChildren bool
			want         []string
		}{
			{
				name: "Should list every child when children are included",
				want: []string{"continue", "fork", "parent", "provenance", "recovery", "spawn", "subagent"},
			},
			{
				name:         "Should keep forks and continuations when children are hidden",
				hideChildren: true,
				want:         []string{"continue", "fork", "parent"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				pageQuery := store.SessionCatalogPageQuery{
					ReadScope:    scope,
					WorkspaceID:  workspace,
					HideChildren: tc.hideChildren,
					Sort:         sessionCatalogSortRecent,
					Limit:        10,
				}
				page, err := db.PageSessions(ctx, pageQuery)
				if err != nil {
					t.Fatal(err)
				}
				got := make([]string, 0, len(page.Sessions))
				for _, info := range page.Sessions {
					got = append(got, info.ID)
				}
				slices.Sort(got)
				if page.Total != len(tc.want) || !slices.Equal(got, tc.want) {
					t.Fatalf("page total %d ids %v, want %v", page.Total, got, tc.want)
				}
				// Walk one row per page: every page counts the filtered total and the
				// keyset walk visits each filtered session exactly once.
				pageQuery.Limit = 1
				walked := make([]string, 0, len(tc.want))
				for range len(tc.want) + 1 {
					bounded, err := db.PageSessions(ctx, pageQuery)
					if err != nil {
						t.Fatal(err)
					}
					if bounded.Total != len(tc.want) {
						t.Fatalf("bounded page total = %d, want %d", bounded.Total, len(tc.want))
					}
					if len(bounded.Sessions) == 0 {
						break
					}
					anchor := bounded.Sessions[0]
					walked = append(walked, anchor.ID)
					pageQuery.After = &store.SessionCatalogPosition{
						PrimaryAt:   anchor.UpdatedAt,
						SecondaryAt: anchor.CreatedAt,
						CreatedAt:   anchor.CreatedAt,
						ID:          anchor.ID,
					}
				}
				slices.Sort(walked)
				if !slices.Equal(walked, tc.want) {
					t.Fatalf("keyset walk = %v, want %v", walked, tc.want)
				}
				pageQuery.After = nil
				facets, err := db.SessionCatalogFacets(ctx, pageQuery)
				if err != nil {
					t.Fatal(err)
				}
				if facets.Facets.All != len(tc.want) || facets.Facets.Board.Done != len(tc.want) {
					t.Fatalf("facets = %#v", facets.Facets)
				}
			})
		}
		relations, err := db.SessionCatalogRelations(ctx, store.SessionCatalogRelationsQuery{
			ReadScope:  scope,
			SessionIDs: []string{"parent"},
			ParentIDs:  []string{"parent"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if counts := relations.Children["parent"]; counts.Done != 4 ||
			counts.NeedsYou+counts.Working+counts.InReview != 0 {
			t.Fatalf("parent rollup = %#v, want only provenance, spawn, subagent and recovery children", counts)
		}
	})

	t.Run("Should agree with Go for every badge and PR state", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		for _, badge := range []session.Badge{session.BadgeRunning, session.BadgeIdle, session.BadgeUnhealthy,
			session.BadgeHung, session.BadgeWaitingForAuth, session.BadgeWaitingForInput, session.BadgeDone,
			session.BadgeStopped, session.BadgeFailed, session.BadgeUnknown, session.BadgeNeedsAttention, session.BadgeWaiting} {
			for _, pr := range []session.PullRequestState{"", session.PullRequestOpen, session.PullRequestClosed, session.PullRequestMerged} {
				var got string
				expression := sessionCatalogBoardColumnSQL("badge", "pr_state")
				if err := db.db.QueryRowContext(t.Context(), "SELECT "+expression+" FROM (SELECT ? AS badge, ? AS pr_state)", string(badge), string(pr)).
					Scan(&got); err != nil {
					t.Fatal(err)
				}
				if want := session.BoardColumnFor(badge, pr); got != string(want) {
					t.Fatalf("%s/%s: SQL %s, Go %s", badge, pr, got, want)
				}
			}
		}
	})
	t.Run("Should filter before limit and count with repository isolation", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		workspace := registerWorkspaceForGlobalTests(t, db, "board", filepath.Join(t.TempDir(), "workspace"))
		base := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
		for index := range 120 {
			id := fmt.Sprintf("board-%03d", index)
			info := sessionInfoForWorkspaceStateIndexTest(
				id,
				workspace,
				globalDBSessionStateStopped,
				base.Add(-time.Duration(index)*time.Minute),
			)
			if err := db.RegisterSession(ctx, info); err != nil {
				t.Fatal(err)
			}
			if index >= 100 {
				if _, err := db.db.ExecContext(
					ctx,
					"UPDATE sessions SET branch='feat/review' WHERE id=?",
					id,
				); err != nil {
					t.Fatal(err)
				}
			}
		}
		if _, err := db.db.ExecContext(
			ctx,
			"INSERT INTO forge_pull_requests(workspace_id, repo_key, branch, pr_state) VALUES (?, 'github.com/owner/repo', 'feat/review', 'open')",
			workspace,
		); err != nil {
			t.Fatal(err)
		}
		query := store.SessionCatalogPageQuery{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspace,
			Archive: store.SessionArchiveExclude, Sort: sessionCatalogSortRecent, Limit: 10,
			Columns: []string{"in_review"}, RepositoryKeys: map[string]string{workspace: "github.com/owner/repo"}}
		page, err := db.PageSessions(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 20 || len(page.Sessions) != 10 || page.Sessions[0].ID != "board-100" {
			t.Fatalf("first page = %#v", page)
		}
		last := page.Sessions[9]
		query.After = &store.SessionCatalogPosition{
			ID:          last.ID,
			PrimaryAt:   last.UpdatedAt,
			SecondaryAt: last.CreatedAt,
			CreatedAt:   last.CreatedAt,
		}
		page, err = db.PageSessions(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 20 || len(page.Sessions) != 10 || page.Sessions[0].ID != "board-110" {
			t.Fatalf("second page = %#v", page)
		}
		query.Search = "no-match"
		facets, err := db.SessionCatalogFacets(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if facets.Facets.Board.InReview != 20 || facets.Facets.Board.Done != 100 {
			t.Fatalf("facets = %#v", facets)
		}
		query.After, query.Search = nil, ""
		query.DefaultBranches = map[string][]string{workspace: {"feat/review"}}
		page, err = db.PageSessions(ctx, query)
		if err != nil || page.Total != 0 {
			t.Fatalf("default branch cached PR returned %#v, %v", page, err)
		}
		query.DefaultBranches = nil
		query.RepositoryKeys[workspace] = "github.com/owner/other"
		page, err = db.PageSessions(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 0 {
			t.Fatalf("foreign repository returned %d sessions", page.Total)
		}
	})
}

// BenchmarkSessionCatalogBoard measures the database owner with many repository-scoped rows.
func BenchmarkSessionCatalogBoard(b *testing.B) {
	ctx := b.Context()
	db, err := OpenGlobalDB(ctx, filepath.Join(b.TempDir(), GlobalDatabaseName))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			b.Error(err)
		}
	})
	query := store.SessionCatalogPageQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, Archive: store.SessionArchiveExclude,
		Sort: sessionCatalogSortRecent, Limit: 50, Columns: []string{"in_review"},
		RepositoryKeys: map[string]string{}, DefaultBranches: map[string][]string{},
	}
	for index := range 16 {
		id := fmt.Sprintf("ws-board-%02d", index)
		query.RepositoryKeys[id] = "github.com/owner/" + id
		query.DefaultBranches[id] = []string{"main", "master"}
		if _, err := db.db.ExecContext(
			ctx,
			`INSERT INTO workspaces(id,root_dir,name,created_at,updated_at) VALUES (?, ?, ?, '2026-10-26T00:00:00Z', '2026-10-26T00:00:00Z')`,
			id,
			"/"+id,
			id,
		); err != nil {
			b.Fatal(err)
		}
		if _, err := db.db.ExecContext(
			ctx,
			`INSERT INTO forge_pull_requests(workspace_id,repo_key,branch,pr_state) VALUES (?, ?, 'topic', 'open')`,
			id,
			query.RepositoryKeys[id],
		); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := db.db.ExecContext(
		ctx,
		`WITH RECURSIVE fixture(i) AS (SELECT 0 UNION ALL SELECT i+1 FROM fixture WHERE i < 1023)
 INSERT INTO sessions(id,profile_id,workspace_id,agent_name,provider,state,branch,created_at,updated_at)
 SELECT printf('sess-board-%04d',i), ?, printf('ws-board-%02d',i%16), 'coder', 'claude', 'stopped', 'topic', '2026-10-26T00:00:00Z', '2026-10-26T00:00:00Z' FROM fixture`,
		store.DefaultProfileID,
	); err != nil {
		b.Fatal(err)
	}
	b.Run("Facets", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			facets, err := db.SessionCatalogFacets(b.Context(), query)
			if err != nil || facets.Facets.All != 1024 || facets.Facets.Board.InReview != 1024 {
				b.Fatalf("facets=%+v error=%v", facets.Facets, err)
			}
		}
	})
	b.Run("Page", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			page, err := db.PageSessions(b.Context(), query)
			if err != nil || page.Total != 1024 || len(page.Sessions) != 50 {
				b.Fatalf("total=%d rows=%d error=%v", page.Total, len(page.Sessions), err)
			}
		}
	})
}
