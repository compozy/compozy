package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/sessiondb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestManagerListAllRequiresContext(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	var nilCtx context.Context
	if _, err := h.manager.ListAll(nilCtx); err == nil {
		t.Fatal("ListAll(nil) error = nil, want non-nil")
	}
}

func TestManagerListAllReturnsActiveWhenSessionsDirMissing(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
			!errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Stop(%q) error = %v", session.ID, err)
		}
	})

	if err := os.RemoveAll(h.homePaths.SessionsDir); err != nil {
		t.Fatalf("RemoveAll(sessions dir) error = %v", err)
	}

	infos, err := h.manager.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("ListAll() = %d sessions, want 1", len(infos))
	}
	if got := infos[0].ID; got != session.ID {
		t.Fatalf("ListAll()[0].ID = %q, want %q", got, session.ID)
	}
	if got := infos[0].State; got != StateActive {
		t.Fatalf("ListAll()[0].State = %q, want %q", got, StateActive)
	}
}

func TestManagerListAllMergesActiveAndStoppedSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	active := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil &&
			!errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Stop(%q) error = %v", active.ID, err)
		}
	})

	stopped, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked-stopped",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create(networked stopped) error = %v", err)
	}
	if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
		t.Fatalf("Stop(stopped) error = %v", err)
	}

	orphanDir := filepath.Join(h.homePaths.SessionsDir, "orphan")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(orphan) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(h.homePaths.SessionsDir, "notes.txt"), []byte("skip me"), 0o644); err != nil {
		t.Fatalf("WriteFile(notes) error = %v", err)
	}
	if err := os.WriteFile(active.MetaPath(), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(corrupt active meta) error = %v", err)
	}

	badStoppedDir := filepath.Join(h.homePaths.SessionsDir, "bad-stopped")
	if err := os.MkdirAll(badStoppedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(bad-stopped) error = %v", err)
	}
	if err := os.WriteFile(store.SessionMetaFile(badStoppedDir), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(corrupt stopped meta) error = %v", err)
	}

	infos, err := h.manager.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("ListAll() = %d sessions, want 2", len(infos))
	}

	if got := infos[0].ID; got != active.ID {
		t.Fatalf("ListAll()[0].ID = %q, want %q", got, active.ID)
	}
	if got := infos[0].State; got != StateActive {
		t.Fatalf("ListAll()[0].State = %q, want %q", got, StateActive)
	}
	if got, want := infos[0].NetworkParticipation, participation.LocalSpec(); got != want {
		t.Fatalf("ListAll()[0].NetworkParticipation = %#v, want %#v", got, want)
	}
	if got := infos[1].ID; got != stopped.ID {
		t.Fatalf("ListAll()[1].ID = %q, want %q", got, stopped.ID)
	}
	if got := infos[1].State; got != StateStopped {
		t.Fatalf("ListAll()[1].State = %q, want %q", got, StateStopped)
	}
	if got, want := infos[1].NetworkParticipation, testLiveParticipation(h.workspaceID, "builders"); got != want {
		t.Fatalf("ListAll()[1].NetworkParticipation = %#v, want %#v", got, want)
	}
}

func TestManagerListPageOverlaysActiveAndBindsCursor(t *testing.T) {
	t.Parallel()

	t.Run("Should treat onboarding as an ordinary user agent name", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		ctx := testutil.Context(t)
		resolved, err := h.resolver.Resolve(ctx, h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(workspace) error = %v", err)
		}
		resolved.Agents = append(resolved.Agents, aghconfig.AgentDef{
			Name:     "onboarding",
			Provider: "claude",
			Prompt:   "Internal onboarding.",
		})
		h.resolver.upsert(&resolved)

		onboarding, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "onboarding",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(onboarding) error = %v", err)
		}
		user, err := h.manager.Create(ctx, CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(user) error = %v", err)
		}
		for _, created := range []*Session{onboarding, user} {
			t.Cleanup(func() {
				if stopErr := h.manager.Stop(testutil.Context(t), created.ID); stopErr != nil &&
					!errors.Is(stopErr, ErrSessionNotFound) {
					t.Errorf("Stop(%q) error = %v", created.ID, stopErr)
				}
			})
		}

		if got, want := onboarding.Info().Type, SessionTypeUser; got != want {
			t.Errorf("onboarding session type = %q, want %q", got, want)
		}
		if got, want := readMeta(t, onboarding.MetaPath()).SessionType, string(SessionTypeUser); got != want {
			t.Errorf("onboarding metadata session type = %q, want %q", got, want)
		}
		if got, want := user.Info().Type, SessionTypeUser; got != want {
			t.Errorf("user session type = %q, want %q", got, want)
		}

		userPage, err := h.manager.ListPage(ctx, ListQuery{
			State:       string(StateActive),
			SessionType: SessionTypeUser,
			Sort:        ListSortRecent,
		})
		if err != nil {
			t.Fatalf("ListPage(user) error = %v", err)
		}
		if userPage.Total != 2 || len(userPage.Sessions) != 2 {
			t.Errorf("ListPage(user) = %#v, want onboarding and coder sessions", userPage)
		}

		allPage, err := h.manager.ListPage(ctx, ListQuery{
			State: string(StateActive),
			Sort:  ListSortRecent,
		})
		if err != nil {
			t.Fatalf("ListPage(all) error = %v", err)
		}
		allIDs := make([]string, 0, len(allPage.Sessions))
		for _, info := range allPage.Sessions {
			allIDs = append(allIDs, info.ID)
		}
		if allPage.Total != 2 || !slices.Contains(allIDs, onboarding.ID) || !slices.Contains(allIDs, user.ID) {
			t.Errorf("ListPage(all) = %#v, want onboarding %q and user %q", allPage, onboarding.ID, user.ID)
		}
	})

	t.Run("Should page the active and durable union from the indexed projection", func(t *testing.T) {
		t.Parallel()

		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithSessionCatalog(catalog))
		active, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "catalog active",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create(active) error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		stopped, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "catalog stopped",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create(stopped) error = %v", err)
		}
		if err := h.manager.Stop(testutil.Context(t), stopped.ID); err != nil {
			t.Fatalf("Stop(stopped) error = %v", err)
		}
		internal, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "catalog internal",
			Workspace: h.workspaceID,
			Type:      SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("Create(internal) error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), internal.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(internal cleanup) error = %v", stopErr)
			}
		})
		catalog.durable = sessionCatalogInfoFromRuntime(stopped.Info())
		metaReads := 0
		h.manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			metaReads++
			return store.ReadSessionMeta(path)
		}
		h.resolver.mu.Lock()
		resolverCallsBefore := len(h.resolver.resolveCalls)
		h.resolver.mu.Unlock()

		query := ListQuery{
			WorkspaceID: h.workspaceID,
			SessionType: SessionTypeUser,
			AgentName:   "coder",
			Search:      "CATALOG",
			Sort:        ListSortRecent,
			Limit:       1,
		}
		first, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(first) error = %v", err)
		}
		if first.Total != 2 || len(first.Sessions) != 1 || !first.HasMore || first.NextCursor == "" {
			t.Fatalf("ListPage(first) = %#v, want one of two sessions with next cursor", first)
		}
		if !slices.Contains(catalog.lastQuery.ExcludeIDs, active.ID) {
			t.Fatalf("PageSessions().ExcludeIDs = %#v, want active id %q", catalog.lastQuery.ExcludeIDs, active.ID)
		}
		if catalog.lastQuery.SessionType != string(SessionTypeUser) {
			t.Fatalf("PageSessions().SessionType = %q, want %q", catalog.lastQuery.SessionType, SessionTypeUser)
		}

		query.Cursor = first.NextCursor
		second, err := h.manager.ListPage(testutil.Context(t), query)
		if err != nil {
			t.Fatalf("ListPage(second) error = %v", err)
		}
		if second.Total != 2 || len(second.Sessions) != 1 || second.HasMore {
			t.Fatalf("ListPage(second) = %#v, want final one-of-two page", second)
		}
		ids := []string{first.Sessions[0].ID, second.Sessions[0].ID}
		if !slices.Contains(ids, active.ID) || !slices.Contains(ids, stopped.ID) {
			t.Fatalf("paged ids = %#v, want active %q and durable %q", ids, active.ID, stopped.ID)
		}
		for _, page := range []ListPage{first, second} {
			for _, info := range page.Sessions {
				if info.ID == stopped.ID && info.Workspace != "" {
					t.Fatalf("projected stopped info = %#v, want no meta-only workspace path", info)
				}
			}
		}
		if metaReads != 0 {
			t.Fatalf("session metadata reads = %d, want zero for catalog pages", metaReads)
		}
		h.resolver.mu.Lock()
		resolverCallsAfter := len(h.resolver.resolveCalls)
		h.resolver.mu.Unlock()
		if resolverCallsAfter != resolverCallsBefore {
			t.Fatalf(
				"workspace resolver calls = %d after catalog pages, want unchanged %d",
				resolverCallsAfter,
				resolverCallsBefore,
			)
		}

		callsBeforeMismatch := catalog.calls
		query.Search = "different"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(query mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.Search = "catalog"
		query.WorkspaceID = "ws-foreign"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(workspace mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.WorkspaceID = h.workspaceID
		query.SessionType = SessionTypeSystem
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(type mismatch) error = %v, want ErrListCursorInvalid", err)
		}
		query.SessionType = SessionTypeUser
		query.Cursor = "not-an-opaque-cursor"
		if _, err := h.manager.ListPage(testutil.Context(t), query); !errors.Is(err, ErrListCursorInvalid) {
			t.Fatalf("ListPage(malformed cursor) error = %v, want ErrListCursorInvalid", err)
		}
		if catalog.calls != callsBeforeMismatch {
			t.Fatalf("PageSessions calls after mismatched cursors = %d, want %d", catalog.calls, callsBeforeMismatch)
		}
	})

	t.Run("Should restore active detail health and resumable catalog truth after attach expiry", func(t *testing.T) {
		t.Parallel()

		clock := newSessionHealthTestClock(time.Date(2026, 7, 11, 14, 0, 0, 0, time.UTC))
		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithNow(clock.Now), WithSessionCatalog(catalog))
		active := createSession(t, h)
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		attach, err := h.manager.AttachSession(testutil.Context(t), store.SessionAttachRequest{
			SessionID:  active.ID,
			AttachedTo: "uds:operator",
			Now:        clock.Now(),
			TTL:        time.Minute,
		})
		if err != nil {
			t.Fatalf("AttachSession() error = %v", err)
		}

		locked, err := h.manager.ListPage(testutil.Context(t), ListQuery{
			WorkspaceID: h.workspaceID,
			Resumable:   true,
		})
		if err != nil {
			t.Fatalf("ListPage(locked resumable) error = %v", err)
		}
		if locked.Total != 0 || len(locked.Sessions) != 0 {
			t.Fatalf("ListPage(locked resumable) = %#v, want empty", locked)
		}

		clock.Set(attach.AttachExpiresAt.Add(time.Second))
		status, err := h.manager.Status(testutil.Context(t), active.ID)
		if err != nil {
			t.Fatalf("Status(after expiry) error = %v", err)
		}
		if status.AttachedTo != "" || status.AttachExpiresAt != nil || !AttachableForInfo(status, clock.Now()) {
			t.Fatalf("Status(after expiry) = %#v, want normalized attachable session", status)
		}
		resumable, err := h.manager.ListPage(testutil.Context(t), ListQuery{
			WorkspaceID: h.workspaceID,
			Resumable:   true,
		})
		if err != nil {
			t.Fatalf("ListPage(after expiry) error = %v", err)
		}
		if resumable.Total != 1 || len(resumable.Sessions) != 1 || resumable.Sessions[0].ID != active.ID ||
			resumable.Sessions[0].AttachedTo != "" || resumable.Sessions[0].AttachExpiresAt != nil {
			t.Fatalf("ListPage(after expiry) = %#v, want active session unlocked", resumable)
		}
		healthByID, err := h.manager.SessionHealthForPage(testutil.Context(t), resumable.Sessions)
		if err != nil {
			t.Fatalf("SessionHealthForPage(after expiry) error = %v", err)
		}
		if !healthByID[active.ID].Attachable {
			t.Fatalf("SessionHealthForPage(after expiry) = %#v, want attachable", healthByID[active.ID])
		}
	})
}

func TestManagerAggregateSessionsByAgent(t *testing.T) {
	t.Parallel()

	t.Run("Should overlay live sessions onto exact durable workspace metrics", func(t *testing.T) {
		t.Parallel()

		baseAt := time.Date(2026, 7, 12, 14, 0, 0, 0, time.UTC)
		clock := newSessionHealthTestClock(baseAt)
		catalog := &pagedRecordingSessionCatalog{recordingSessionCatalog: newRecordingSessionCatalog()}
		h := newHarness(t, WithNow(clock.Now), WithSessionCatalog(catalog))
		active, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Name:      "live coder",
			Workspace: h.workspaceID,
		})
		if err != nil {
			t.Fatalf("Create(active) error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), active.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop(active cleanup) error = %v", stopErr)
			}
		})
		lastActivityAt := baseAt.Add(20 * time.Second)
		catalog.agentMetrics = []store.SessionAgentMetrics{{
			AgentName:      "coder",
			Total:          1,
			Failed:         1,
			RuntimeSeconds: 12,
			LastActivityAt: lastActivityAt,
		}}
		clock.Set(baseAt.Add(30 * time.Second))

		metrics, err := h.manager.AggregateSessionsByAgent(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("AggregateSessionsByAgent() error = %v", err)
		}
		coder := metrics["coder"]
		if coder.Total != 2 || coder.Active != 1 || coder.Failed != 1 ||
			coder.RuntimeSeconds != 42 || !coder.LastActivityAt.Equal(lastActivityAt) {
			t.Fatalf(
				"AggregateSessionsByAgent() = %#v, want coder total=2 active=1 failed=1 runtime=42 last=%s",
				metrics,
				lastActivityAt,
			)
		}
		if catalog.lastAgentMetricsQuery.WorkspaceID != h.workspaceID ||
			!slices.Contains(catalog.lastAgentMetricsQuery.ExcludeIDs, active.ID) ||
			!slices.Contains(catalog.lastAgentMetricsQuery.ExcludeSessionTypes, string(SessionTypeDream)) ||
			!slices.Contains(catalog.lastAgentMetricsQuery.ExcludeSpawnRoles, SpawnRoleMemoryExtractor) ||
			!slices.Contains(catalog.lastAgentMetricsQuery.ExcludeSpawnRoles, SpawnRoleAutoTitle) {
			t.Fatalf(
				"AggregateSessionsByAgent() query = %#v, want workspace and live/internal exclusions",
				catalog.lastAgentMetricsQuery,
			)
		}
	})
}

func TestSessionMatchesListQuery(t *testing.T) {
	t.Parallel()

	base := &Info{
		ID:                   "sess-visible",
		Name:                 "Review launch",
		AgentName:            "coder",
		Provider:             "codex",
		WorkspaceID:          "ws-alpha",
		NetworkParticipation: testLiveParticipation("ws-alpha", "builders"),
		Type:                 SessionTypeUser,
		State:                StateActive,
	}
	now := time.Date(2026, 7, 10, 15, 0, 0, 0, time.UTC)

	t.Run("Should apply exact workspace state agent and literal search filters", func(t *testing.T) {
		t.Parallel()

		if !sessionMatchesListQuery(base, ListQuery{
			WorkspaceID: "ws-alpha",
			State:       "active",
			SessionType: SessionTypeUser,
			AgentName:   "coder",
			Search:      "review",
		}, now) {
			t.Fatal("sessionMatchesListQuery() = false, want exact filters to match")
		}
		if !sessionMatchesListQuery(base, ListQuery{Search: "BUILDERS"}, now) {
			t.Fatal("sessionMatchesListQuery(BUILDERS) = false, want case-insensitive participation-channel match")
		}
		for _, query := range []ListQuery{
			{WorkspaceID: "ws-foreign"},
			{State: "stopped"},
			{SessionType: SessionTypeSystem},
			{AgentName: "reviewer"},
			{Search: "missing"},
		} {
			if sessionMatchesListQuery(base, query, now) {
				t.Fatalf("sessionMatchesListQuery(%#v) = true, want false", query)
			}
		}
	})

	t.Run("Should hide daemon-owned internal sessions before the page cut", func(t *testing.T) {
		t.Parallel()

		dream := *base
		dream.Type = SessionTypeDream
		if sessionMatchesListQuery(&dream, ListQuery{}, now) {
			t.Fatal("sessionMatchesListQuery(dream) = true, want false")
		}
		for _, role := range []string{SpawnRoleMemoryExtractor, SpawnRoleAutoTitle} {
			internal := *base
			internal.Lineage = &store.SessionLineage{SpawnRole: role}
			if sessionMatchesListQuery(&internal, ListQuery{}, now) {
				t.Fatalf("sessionMatchesListQuery(%s) = true, want false", role)
			}
		}
	})

	t.Run("Should apply resumable eligibility before the page cut", func(t *testing.T) {
		t.Parallel()

		if !sessionMatchesListQuery(base, ListQuery{Resumable: true}, now) {
			t.Fatal("sessionMatchesListQuery(resumable) = false, want attachable active session")
		}
		locked := *base
		locked.AttachedTo = "uds:123"
		expiresAt := now.Add(time.Hour)
		locked.AttachExpiresAt = &expiresAt
		if sessionMatchesListQuery(&locked, ListQuery{Resumable: true}, now) {
			t.Fatal("sessionMatchesListQuery(locked resumable) = true, want false")
		}
	})
}

func TestNormalizeListQuery(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize defaults and case-insensitive search", func(t *testing.T) {
		t.Parallel()

		query, err := normalizeListQuery(ListQuery{Search: "  Review  "})
		if err != nil {
			t.Fatalf("normalizeListQuery() error = %v", err)
		}
		if query.Search != "review" || query.Sort != ListSortRecent || query.Limit != DefaultListLimit {
			t.Fatalf("normalizeListQuery() = %#v, want normalized search/sort/limit", query)
		}
	})

	for _, testCase := range []struct {
		name  string
		query ListQuery
	}{
		{name: "state", query: ListQuery{State: "unknown"}},
		{name: "type", query: ListQuery{SessionType: "unknown"}},
		{name: "dream type", query: ListQuery{SessionType: SessionTypeDream}},
		{name: "sort", query: ListQuery{Sort: "oldest"}},
		{name: "negative limit", query: ListQuery{Limit: -1}},
		{name: "oversized limit", query: ListQuery{Limit: MaxListLimit + 1}},
	} {
		t.Run("Should reject invalid "+testCase.name, func(t *testing.T) {
			t.Parallel()

			if _, err := normalizeListQuery(testCase.query); !errors.Is(err, ErrListQueryInvalid) {
				t.Fatalf("normalizeListQuery(%#v) error = %v, want ErrListQueryInvalid", testCase.query, err)
			}
		})
	}
}

type pagedRecordingSessionCatalog struct {
	*recordingSessionCatalog
	durable               store.SessionInfo
	agentMetrics          []store.SessionAgentMetrics
	lastQuery             store.SessionCatalogPageQuery
	lastAgentMetricsQuery store.SessionAgentMetricsQuery
	calls                 int
}

func (c *pagedRecordingSessionCatalog) PageSessions(
	_ context.Context,
	query store.SessionCatalogPageQuery,
) (store.SessionCatalogPage, error) {
	c.calls++
	c.lastQuery = query
	if c.durable.ID == "" {
		return store.SessionCatalogPage{}, nil
	}
	page := store.SessionCatalogPage{Total: 1}
	position := sessionCatalogPosition(c.durable, query.Sort)
	if query.After == nil || compareSessionCatalogPosition(position, *query.After) > 0 {
		page.Sessions = []store.SessionInfo{c.durable}
	}
	return page, nil
}

func (c *pagedRecordingSessionCatalog) AggregateSessionsByAgent(
	_ context.Context,
	query store.SessionAgentMetricsQuery,
) ([]store.SessionAgentMetrics, error) {
	c.lastAgentMetricsQuery = query
	return append([]store.SessionAgentMetrics(nil), c.agentMetrics...), nil
}

func TestManagerStatusReturnsActiveAndStoredSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	info, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(active) error = %v", err)
	}
	if got := info.State; got != StateActive {
		t.Fatalf("Status(active).State = %q, want %q", got, StateActive)
	}
	if got, want := info.NetworkParticipation, participation.LocalSpec(); got != want {
		t.Fatalf("Status(active).NetworkParticipation = %#v, want %#v", got, want)
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	info, err = h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(stopped) error = %v", err)
	}
	if got := info.State; got != StateStopped {
		t.Fatalf("Status(stopped).State = %q, want %q", got, StateStopped)
	}
	if got, want := info.NetworkParticipation, participation.LocalSpec(); got != want {
		t.Fatalf("Status(stopped).NetworkParticipation = %#v, want %#v", got, want)
	}

	var nilCtx context.Context
	if _, err := h.manager.Status(nilCtx, session.ID); err == nil {
		t.Fatal("Status(nil, id) error = nil, want non-nil")
	}
	if _, err := h.manager.Status(testutil.Context(t), "   "); err == nil {
		t.Fatal("Status(blank id) error = nil, want non-nil")
	}
	if _, err := h.manager.Status(testutil.Context(t), "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Status(missing) error = %v, want ErrSessionNotFound", err)
	}
}

func TestManagerStatusRejectsTraversalSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	escapedID := createEscapedStoredSession(t, h)

	info, err := h.manager.Status(testutil.Context(t), "../"+escapedID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Status(traversal) error = %v, want ErrSessionNotFound", err)
	}
	if info != nil {
		t.Fatalf("Status(traversal) info = %#v, want nil", info)
	}
}

func TestNormalizeStoredSessionIDRejectsWindowsDriveRelativePath(t *testing.T) {
	t.Parallel()

	if _, err := normalizeStoredSessionID("C:escape"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("normalizeStoredSessionID(windows drive-relative) error = %v, want ErrSessionNotFound", err)
	}
}

func TestManagerStatusRepairsIncompleteStartMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta, err := store.ReadSessionMeta(session.MetaPath())
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	meta.State = string(StateStarting)
	meta.StopReason = nil
	meta.StopDetail = ""
	meta.ACPSessionID = stringPointer(originalACP)
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	info, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(repaired) error = %v", err)
	}
	if got := info.State; got != StateStopped {
		t.Fatalf("Status(repaired).State = %q, want %q", got, StateStopped)
	}
	if got := info.StopReason; got != store.StopError {
		t.Fatalf("Status(repaired).StopReason = %q, want %q", got, store.StopError)
	}
	if got := info.StopDetail; got != resumeStopDetailStartIncomplete {
		t.Fatalf("Status(repaired).StopDetail = %q, want %q", got, resumeStopDetailStartIncomplete)
	}
	if got := info.ACPSessionID; got != "" {
		t.Fatalf("Status(repaired).ACPSessionID = %q, want empty", got)
	}

	repairedMeta, err := store.ReadSessionMeta(session.MetaPath())
	if err != nil {
		t.Fatalf("ReadSessionMeta(repaired) error = %v", err)
	}
	if got := repairedMeta.State; got != string(StateStopped) {
		t.Fatalf("repaired meta state = %q, want %q", got, StateStopped)
	}
	if repairedMeta.StopReason == nil || *repairedMeta.StopReason != store.StopError {
		t.Fatalf("repaired meta stop reason = %#v, want %q", repairedMeta.StopReason, store.StopError)
	}
	if got := repairedMeta.StopDetail; got != resumeStopDetailStartIncomplete {
		t.Fatalf("repaired meta stop detail = %q, want %q", got, resumeStopDetailStartIncomplete)
	}
	if repairedMeta.ACPSessionID != nil {
		t.Fatalf("repaired meta ACPSessionID = %#v, want nil", repairedMeta.ACPSessionID)
	}
}

func TestManagerStatusRepairsInterruptedSessionAsStalledWhenLiveSubprocessIsStale(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	meta, err := store.ReadSessionMeta(session.MetaPath())
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	lastUpdate := time.Now().UTC().Add(-DefaultLivenessStallAfter - time.Minute)
	startedAt := time.Now().UTC().Add(-10 * time.Minute)
	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	meta.Liveness = &store.SessionLivenessMeta{
		SubprocessPID:       os.Getpid(),
		SubprocessStartedAt: &startedAt,
		LastUpdateAt:        &lastUpdate,
	}
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	info, err := h.manager.Status(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Status(stalled) error = %v", err)
	}
	if got, want := info.State, StateStopped; got != want {
		t.Fatalf("Status(stalled).State = %q, want %q", got, want)
	}
	if got, want := info.StopReason, store.StopAgentCrashed; got != want {
		t.Fatalf("Status(stalled).StopReason = %q, want %q", got, want)
	}
	if got, want := info.StopDetail, resumeStopDetailAgentStalled; got != want {
		t.Fatalf("Status(stalled).StopDetail = %q, want %q", got, want)
	}
	if info.Liveness == nil {
		t.Fatal("Status(stalled).Liveness = nil, want liveness metadata")
	}
	if got, want := info.Liveness.StallState, store.SessionStallStateDetected; got != want {
		t.Fatalf("Status(stalled).Liveness.StallState = %q, want %q", got, want)
	}
	if got, want := info.Liveness.StallReason, store.SessionStallReasonActivityTimeout; got != want {
		t.Fatalf("Status(stalled).Liveness.StallReason = %q, want %q", got, want)
	}
}

func TestManagerStatusDoesNotRepairPendingStartMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sessionID := "sess-pending"
	sessionDir := filepath.Join(h.homePaths.SessionsDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}

	acpSessionID := "acp-pending"
	meta := store.SessionMeta{
		ID:                   sessionID,
		Name:                 "pending",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		NetworkParticipation: testLocalParticipationPtr(),
		State:                string(StateStarting),
		ACPSessionID:         stringPointer(acpSessionID),
		CreatedAt:            time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 20, 12, 0, 1, 0, time.UTC),
	}
	metaPath := store.SessionMetaFile(sessionDir)
	if err := store.WriteSessionMeta(metaPath, meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	h.manager.mu.Lock()
	h.manager.pending[sessionID] = sessionReservation{workspaceID: h.workspaceID}
	h.manager.mu.Unlock()

	info, err := h.manager.Status(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("Status(pending) error = %v", err)
	}
	if got := info.State; got != StateStarting {
		t.Fatalf("Status(pending).State = %q, want %q", got, StateStarting)
	}
	if got := info.ACPSessionID; got != acpSessionID {
		t.Fatalf("Status(pending).ACPSessionID = %q, want %q", got, acpSessionID)
	}
	if got := info.StopDetail; got != "" {
		t.Fatalf("Status(pending).StopDetail = %q, want empty", got)
	}

	storedMeta, err := store.ReadSessionMeta(metaPath)
	if err != nil {
		t.Fatalf("ReadSessionMeta() error = %v", err)
	}
	if got := storedMeta.State; got != string(StateStarting) {
		t.Fatalf("stored meta state = %q, want %q", got, StateStarting)
	}
	if storedMeta.StopReason != nil {
		t.Fatalf("stored meta stop reason = %#v, want nil", storedMeta.StopReason)
	}
	if got := stringValue(storedMeta.ACPSessionID); got != acpSessionID {
		t.Fatalf("stored meta ACPSessionID = %q, want %q", got, acpSessionID)
	}
}

func TestManagerEventsAndHistoryUseStoredEvents(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	runtimeEvents := collectEvents(t, eventsCh)
	if len(runtimeEvents) != 2 {
		t.Fatalf("Prompt() events = %d, want 2", len(runtimeEvents))
	}

	activeEvents, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
	if err != nil {
		t.Fatalf("Events(active) error = %v", err)
	}
	if len(activeEvents) != 3 {
		t.Fatalf("Events(active) = %d events, want 3", len(activeEvents))
	}
	activeHistory, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
	if err != nil {
		t.Fatalf("History(active) error = %v", err)
	}
	if len(activeHistory) != 1 {
		t.Fatalf("History(active) = %d turns, want 1", len(activeHistory))
	}

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	stopOnly, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{
		Type:  EventTypeSessionStopped,
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("Events(stopOnly) error = %v", err)
	}
	if len(stopOnly) != 1 {
		t.Fatalf("Events(stopOnly) = %d events, want 1", len(stopOnly))
	}
	if got := stopOnly[0].Type; got != EventTypeSessionStopped {
		t.Fatalf("Events(stopOnly)[0].Type = %q, want %q", got, EventTypeSessionStopped)
	}
	if got := countEventType(stopOnly, EventTypeSessionStopped); got != 1 {
		t.Fatalf("Events(stopOnly) %q count = %d, want 1", EventTypeSessionStopped, got)
	}

	afterPrompt, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{
		AfterSequence: activeEvents[len(activeEvents)-1].Sequence,
	})
	if err != nil {
		t.Fatalf("Events(after prompt) error = %v", err)
	}
	if len(afterPrompt) != 2 {
		t.Fatalf("Events(after prompt) = %d events, want 2", len(afterPrompt))
	}
	if got := afterPrompt[0].Type; got != EventTypeSessionStopped {
		t.Fatalf("Events(after prompt)[0].Type = %q, want %q", got, EventTypeSessionStopped)
	}
	if got := afterPrompt[1].Type; got != events.TranscriptMarkerCreated {
		t.Fatalf("Events(after prompt)[1].Type = %q, want %q", got, events.TranscriptMarkerCreated)
	}

	stoppedHistory, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
	if err != nil {
		t.Fatalf("History(stopped) error = %v", err)
	}
	if len(stoppedHistory) != 2 {
		t.Fatalf("History(stopped) = %d turns, want 2", len(stoppedHistory))
	}
	if got := stoppedHistory[1].Events[0].Type; got != EventTypeSessionStopped {
		t.Fatalf("History(stopped)[1] first event type = %q, want %q", got, EventTypeSessionStopped)
	}
}

func TestManagerLatestSessionEventByType(t *testing.T) {
	t.Parallel()

	t.Run("Should return the newest typed event for active and stopped sessions", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		ctx := testutil.Context(t)
		for revision := 1; revision <= 2; revision++ {
			if err := h.manager.AppendSessionEventIfAbsent(ctx, GoalEvent{
				EventID:         fmt.Sprintf("goal-snapshot-%d", revision),
				SessionID:       sess.ID,
				SyntheticTurnID: "goal-snapshot:" + sess.ID,
				AgentName:       "system",
				Type:            EventTypeGoalSnapshotChanged,
				Content:         json.RawMessage(fmt.Sprintf(`{"revision":%d}`, revision)),
				CreatedAt:       time.Now().UTC().Add(time.Duration(revision) * time.Second),
			}); err != nil {
				t.Fatalf("AppendSessionEventIfAbsent(%d) error = %v", revision, err)
			}
		}

		assertLatest := func(state string) {
			t.Helper()
			event, err := h.manager.LatestSessionEventByType(ctx, sess.ID, EventTypeGoalSnapshotChanged)
			if err != nil {
				t.Fatalf("LatestSessionEventByType(%s) error = %v", state, err)
			}
			if event == nil || event.ID != "goal-snapshot-2" {
				t.Fatalf("LatestSessionEventByType(%s) = %#v, want goal-snapshot-2", state, event)
			}
		}

		assertLatest("active")
		if err := h.manager.Stop(ctx, sess.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		assertLatest("stopped")
	})
}

func TestManagerEventsAndHistoryRetryClosedActiveRecorder(t *testing.T) {
	t.Parallel()

	t.Run("Should retry when finalization closes the active recorder during a query", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		originalRecorder := session.recorderHandle()
		if originalRecorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := originalRecorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(original recorder) error = %v", err)
		}

		wantEvent := store.SessionEvent{
			ID:        "ev-retry",
			SessionID: session.ID,
			Sequence:  1,
			TurnID:    "turn-retry",
			Type:      "agent_message",
			AgentName: "coder",
			Content:   "{}",
		}
		stub := &queryRecorderStub{
			events:    []store.SessionEvent{wantEvent},
			history:   []store.TurnHistory{{TurnID: wantEvent.TurnID, Events: []store.SessionEvent{wantEvent}}},
			queryErrs: []error{store.ErrClosed},
			historyErrs: []error{
				store.ErrClosed,
			},
		}
		session.setRecorder(stub)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("cleanup Stop() error = %v", err)
			}
		})

		events, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events() error = %v", err)
		}
		if len(events) != 1 || events[0].ID != wantEvent.ID {
			t.Fatalf("Events() = %#v, want retry event", events)
		}
		if got := len(stub.queryCalls); got != 2 {
			t.Fatalf("Query() calls = %d, want 2", got)
		}

		history, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if len(history) != 1 || history[0].TurnID != wantEvent.TurnID {
			t.Fatalf("History() = %#v, want retry history", history)
		}
		if got := len(stub.historyCall); got != 2 {
			t.Fatalf("History() calls = %d, want 2", got)
		}
	})
}

func TestManagerEventsRejectTraversalSessionID(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	escapedID := createEscapedStoredSession(t, h)

	events, err := h.manager.Events(testutil.Context(t), "../"+escapedID, store.EventQuery{})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Events(traversal) error = %v, want ErrSessionNotFound", err)
	}
	if events != nil {
		t.Fatalf("Events(traversal) = %#v, want nil", events)
	}
}

func TestManagerOpenQueryRecorderValidationAndCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should requires context and session id", func(t *testing.T) {
		h := newHarness(t)
		var nilCtx context.Context
		if _, _, err := h.manager.openQueryRecorder(nilCtx, "sess-1"); err == nil {
			t.Fatal("openQueryRecorder(nil, id) error = nil, want non-nil")
		}
		if _, _, err := h.manager.openQueryRecorder(testutil.Context(t), "   "); err == nil {
			t.Fatal("openQueryRecorder(ctx, blank) error = nil, want non-nil")
		}
	})

	t.Run("Should active session with nil recorder falls back to stored events", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if session.recorderHandle() == nil {
				recorder, restoreErr := h.manager.openStore(testutil.Context(t), session.ID, session.DBPath())
				if restoreErr != nil {
					t.Errorf("restore recorder for Stop(%q) cleanup error = %v", session.ID, restoreErr)
					return
				}
				session.setRecorder(recorder)
			}
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if events := collectEvents(t, eventsCh); len(events) == 0 {
			t.Fatal("Prompt() returned no events, want stored transcript data")
		}
		recorder := session.recorderHandle()
		if recorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(recorder) error = %v", err)
		}
		session.setRecorder(nil)
		queried, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(active with nil recorder) error = %v", err)
		}
		if len(queried) == 0 {
			t.Fatal("Events(active with nil recorder) = 0, want persisted events")
		}
		history, err := h.manager.History(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("History(active with nil recorder) error = %v", err)
		}
		if len(history) == 0 {
			t.Fatal("History(active with nil recorder) = 0, want persisted turns")
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), session.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(active with nil recorder) error = %v", err)
		}
		if len(page.Entries) == 0 {
			t.Fatal("TranscriptPage(active with nil recorder) = 0, want persisted entries")
		}
	})

	t.Run("Should wait for finalization before returning active recorder", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)
		done := make(chan struct{})
		h.manager.mu.Lock()
		h.manager.finalizing[session.ID] = &sessionFinalization{done: done}
		h.manager.mu.Unlock()
		t.Cleanup(func() {
			h.manager.finishFinalization(session.ID, nil)
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		ctx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		if _, _, err := h.manager.openQueryRecorder(ctx, session.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("openQueryRecorder(finalizing canceled ctx) error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should finalizing active session reopens stored events after recorder closes", func(t *testing.T) {
		h := newHarness(t)
		session := createSession(t, h)

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		if len(runtimeEvents) == 0 {
			t.Fatal("Prompt() returned no runtime events, want recorded prompt events")
		}

		activeEvents, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(active) error = %v", err)
		}
		if len(activeEvents) == 0 {
			t.Fatal("Events(active) returned no events, want persisted session events")
		}

		recorder := session.recorderHandle()
		if recorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(active recorder) error = %v", err)
		}

		done := make(chan struct{})
		h.manager.mu.Lock()
		h.manager.finalizing[session.ID] = &sessionFinalization{done: done}
		h.manager.mu.Unlock()

		type queryResult struct {
			events []store.SessionEvent
			err    error
		}
		started := make(chan struct{})
		resultCh := make(chan queryResult, 1)
		ctx, cancel := context.WithTimeout(testutil.Context(t), 5*time.Second)
		defer cancel()
		go func() {
			close(started)
			got, cleanup, err := h.manager.openQueryRecorder(ctx, session.ID)
			if err != nil {
				resultCh <- queryResult{err: err}
				return
			}
			events, err := got.Query(ctx, store.EventQuery{})
			if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
			resultCh <- queryResult{events: events, err: err}
		}()

		now := time.Now().UTC()
		if err := session.beginStopping(now); err != nil {
			t.Fatalf("beginStopping() error = %v", err)
		}
		if err := session.markStopped(now); err != nil {
			t.Fatalf("markStopped() error = %v", err)
		}
		if err := h.manager.persistSessionMetadataOnly(session); err != nil {
			t.Fatalf("writeMeta(stopped) error = %v", err)
		}
		h.manager.removeActive(session.ID)
		h.manager.finishFinalization(session.ID, nil)

		result := <-resultCh
		if result.err != nil {
			t.Fatalf("openQueryRecorder(finalizing active) error = %v", result.err)
		}
		if len(result.events) != len(activeEvents) {
			t.Fatalf("openQueryRecorder(finalizing active) events = %d, want %d", len(result.events), len(activeEvents))
		}
	})

	t.Run("Should wait for finalization before reading a closed recorder handle", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		if len(runtimeEvents) == 0 {
			t.Fatal("Prompt() returned no runtime events, want recorded prompt events")
		}

		activeEvents, err := h.manager.Events(testutil.Context(t), session.ID, store.EventQuery{})
		if err != nil {
			t.Fatalf("Events(active) error = %v", err)
		}
		if len(activeEvents) == 0 {
			t.Fatal("Events(active) returned no events, want persisted session events")
		}

		recorder := session.recorderHandle()
		if recorder == nil {
			t.Fatal("recorderHandle() = nil, want active recorder")
		}
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close(active recorder) error = %v", err)
		}

		done := make(chan struct{})
		h.manager.mu.Lock()
		h.manager.finalizing[session.ID] = &sessionFinalization{done: done}
		h.manager.mu.Unlock()

		type queryResult struct {
			events []store.SessionEvent
			err    error
		}
		started := make(chan struct{})
		resultCh := make(chan queryResult, 1)
		ctx, cancel := context.WithTimeout(testutil.Context(t), 5*time.Second)
		defer cancel()
		go func() {
			close(started)
			got, cleanup, err := h.manager.openQueryRecorder(ctx, session.ID)
			if err != nil {
				resultCh <- queryResult{err: err}
				return
			}
			events, err := got.Query(ctx, store.EventQuery{})
			if cleanupErr := cleanup(); cleanupErr != nil && err == nil {
				err = cleanupErr
			}
			resultCh <- queryResult{events: events, err: err}
		}()

		<-started
		select {
		case result := <-resultCh:
			t.Fatalf(
				"openQueryRecorder returned before finalization finished: events=%d err=%v",
				len(result.events),
				result.err,
			)
		case <-time.After(50 * time.Millisecond):
		}

		now := time.Now().UTC()
		if err := session.beginStopping(now); err != nil {
			t.Fatalf("beginStopping() error = %v", err)
		}
		if err := session.markStopped(now); err != nil {
			t.Fatalf("markStopped() error = %v", err)
		}
		if err := h.manager.persistSessionMetadataOnly(session); err != nil {
			t.Fatalf("writeMeta(stopped) error = %v", err)
		}
		h.manager.removeActive(session.ID)
		h.manager.finishFinalization(session.ID, nil)

		result := <-resultCh
		if result.err != nil {
			t.Fatalf("openQueryRecorder(finalized active) error = %v", result.err)
		}
		if err := compareQueriedSessionEvents(activeEvents, result.events); err != nil {
			t.Fatalf("openQueryRecorder(finalized active) events mismatch: %v", err)
		}
	})

	t.Run("Should missing session metadata", func(t *testing.T) {
		h := newHarness(t)
		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"missing",
		); !errors.Is(
			err,
			ErrSessionNotFound,
		) {
			t.Fatalf("openQueryRecorder(missing) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should missing database file", func(t *testing.T) {
		h := newHarness(t)
		writeStoppedSessionArtifacts(t, h, "stored-no-db", false)

		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"stored-no-db",
		); !errors.Is(
			err,
			ErrSessionNotFound,
		) {
			t.Fatalf("openQueryRecorder(no db) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should store open failure", func(t *testing.T) {
		openErr := errors.New("boom")
		h := newHarness(t, WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return nil, openErr
		}))
		writeStoppedSessionArtifacts(t, h, "stored-open-failure", true)

		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"stored-open-failure",
		); !errors.Is(
			err,
			openErr,
		) {
			t.Fatalf("openQueryRecorder(open failure) error = %v, want wrapped %v", err, openErr)
		}
	})

	t.Run("Should hide query pool quiescence behind session absence", func(t *testing.T) {
		h := newHarness(t, WithQueryStore(func(context.Context, string, string) (EventReadCloser, error) {
			return nil, sessiondb.ErrReadOnlyPoolQuiescing
		}))
		writeStoppedSessionArtifacts(t, h, "stored-quiescing", true)

		if _, _, err := h.manager.openQueryRecorder(
			testutil.Context(t),
			"stored-quiescing",
		); !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("openQueryRecorder(quiescing) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should cleanup closes reopened recorder", func(t *testing.T) {
		recorder := &queryRecorderStub{}
		h := newHarness(t, WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return recorder, nil
		}))
		writeStoppedSessionArtifacts(t, h, "stored-cleanup", true)

		got, cleanup, err := h.manager.openQueryRecorder(testutil.Context(t), "stored-cleanup")
		if err != nil {
			t.Fatalf("openQueryRecorder(cleanup) error = %v", err)
		}
		if got != recorder {
			t.Fatalf("openQueryRecorder(cleanup) recorder = %T, want queryRecorderStub", got)
		}
		if cleanup == nil {
			t.Fatal("openQueryRecorder(cleanup) cleanup = nil, want non-nil")
		}
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
		if recorder.closeCalls != 1 {
			t.Fatalf("cleanup() close calls = %d, want 1", recorder.closeCalls)
		}
	})
}

func TestReadMetaAndQueryHelpers(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if _, err := h.manager.readMetaWithContext(testutil.Context(t), "   "); err == nil {
		t.Fatal("readMeta(blank) error = nil, want non-nil")
	}
	if _, err := h.manager.readMetaWithContext(testutil.Context(t), "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("readMetaWithContext(missing) error = %v, want ErrSessionNotFound", err)
	}

	invalidDir := filepath.Join(h.homePaths.SessionsDir, "invalid")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(invalid) error = %v", err)
	}
	if err := os.WriteFile(store.SessionMetaFile(invalidDir), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile(invalid meta) error = %v", err)
	}
	if _, err := h.manager.readMetaWithContext(testutil.Context(t), "invalid"); err == nil {
		t.Fatal("readMetaWithContext(invalid) error = nil, want non-nil")
	}

	acpID := "  acp-123  "
	stopReason := store.StopTimeout
	createdAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	info := sessionInfoFromMeta(store.SessionMeta{
		ID:                   "sess-1",
		Name:                 "stored",
		AgentName:            "coder",
		Provider:             "codex",
		Model:                "  gpt-4o  ",
		ReasoningEffort:      "  high  ",
		WorkspaceID:          "ws-1",
		NetworkParticipation: testLocalParticipationPtr(),
		CreationOptions: &store.SessionCreationOptions{
			NetworkOwnerKey: " session:sess-1 ",
		},
		State:        string(StateStopped),
		StopReason:   &stopReason,
		StopDetail:   "deadline exceeded",
		ACPSessionID: &acpID,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	})
	if got := info.ACPSessionID; got != "acp-123" {
		t.Fatalf("sessionInfoFromMeta().ACPSessionID = %q, want %q", got, "acp-123")
	}
	if got := info.Provider; got != "codex" {
		t.Fatalf("sessionInfoFromMeta().Provider = %q, want %q", got, "codex")
	}
	if got := info.Model; got != "gpt-4o" {
		t.Fatalf("sessionInfoFromMeta().Model = %q, want %q", got, "gpt-4o")
	}
	if got := info.ReasoningEffort; got != "high" {
		t.Fatalf("sessionInfoFromMeta().ReasoningEffort = %q, want %q", got, "high")
	}
	if got := info.NetworkOwnerKey; got != "session:sess-1" {
		t.Fatalf("sessionInfoFromMeta().NetworkOwnerKey = %q, want %q", got, "session:sess-1")
	}
	if got := info.State; got != StateStopped {
		t.Fatalf("sessionInfoFromMeta().State = %q, want %q", got, StateStopped)
	}
	if got := info.Type; got != SessionTypeUser {
		t.Fatalf("sessionInfoFromMeta().Type = %q, want %q", got, SessionTypeUser)
	}
	if got := info.StopReason; got != store.StopTimeout {
		t.Fatalf("sessionInfoFromMeta().StopReason = %q, want %q", got, store.StopTimeout)
	}
	if got := info.StopDetail; got != "deadline exceeded" {
		t.Fatalf("sessionInfoFromMeta().StopDetail = %q, want %q", got, "deadline exceeded")
	}

	t.Run("Should keep stop fields empty when omitted", func(t *testing.T) {
		infoWithoutStop := sessionInfoFromMeta(store.SessionMeta{
			ID:                   "sess-legacy",
			AgentName:            "coder",
			WorkspaceID:          "ws-1",
			NetworkParticipation: testLocalParticipationPtr(),
			State:                string(StateStopped),
			CreatedAt:            createdAt,
			UpdatedAt:            updatedAt,
		})
		if got := infoWithoutStop.StopReason; got != "" {
			t.Fatalf("sessionInfoFromMeta().StopReason = %q, want empty", got)
		}
		if got := infoWithoutStop.StopDetail; got != "" {
			t.Fatalf("sessionInfoFromMeta().StopDetail = %q, want empty", got)
		}
	})

	sameTime := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	sorted := sortSessionInfos([]*Info{
		{ID: "b", CreatedAt: sameTime},
		nil,
		{ID: "a", CreatedAt: sameTime},
		{ID: "c", CreatedAt: sameTime.Add(time.Minute)},
	})
	if got := []string{sorted[0].ID, sorted[1].ID, sorted[2].ID}; strings.Join(got, ",") != "a,b,c" {
		t.Fatalf("sortSessionInfos() ids = %v, want [a b c]", got)
	}

	if got := stringValue(nil); got != "" {
		t.Fatalf("stringValue(nil) = %q, want empty", got)
	}
	if got := stringValue(&acpID); got != "acp-123" {
		t.Fatalf("stringValue(channeld) = %q, want %q", got, "acp-123")
	}
}

func compareQueriedSessionEvents(want []store.SessionEvent, got []store.SessionEvent) error {
	if len(got) != len(want) {
		return fmt.Errorf("count = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index].ID != want[index].ID {
			return fmt.Errorf("event[%d].id = %q, want %q", index, got[index].ID, want[index].ID)
		}
		if got[index].Sequence != want[index].Sequence {
			return fmt.Errorf(
				"event[%d].sequence = %d, want %d",
				index,
				got[index].Sequence,
				want[index].Sequence,
			)
		}
		if got[index].SessionID != want[index].SessionID {
			return fmt.Errorf("event[%d].session_id = %q, want %q", index, got[index].SessionID, want[index].SessionID)
		}
		if got[index].TurnID != want[index].TurnID {
			return fmt.Errorf("event[%d].turn_id = %q, want %q", index, got[index].TurnID, want[index].TurnID)
		}
		if got[index].Type != want[index].Type {
			return fmt.Errorf("event[%d].type = %q, want %q", index, got[index].Type, want[index].Type)
		}
		if got[index].AgentName != want[index].AgentName {
			return fmt.Errorf(
				"event[%d].agent_name = %q, want %q",
				index,
				got[index].AgentName,
				want[index].AgentName,
			)
		}
		if got[index].Content != want[index].Content {
			return fmt.Errorf("event[%d].content = %q, want %q", index, got[index].Content, want[index].Content)
		}
		if !got[index].Timestamp.Equal(want[index].Timestamp) {
			return fmt.Errorf(
				"event[%d].timestamp = %s, want %s",
				index,
				got[index].Timestamp.UTC().Format(time.RFC3339Nano),
				want[index].Timestamp.UTC().Format(time.RFC3339Nano),
			)
		}
	}
	return nil
}

func TestNewAgentProcessDefaultsAndNotifierNoop(t *testing.T) {
	t.Parallel()

	args := []string{"--json"}
	proc := NewAgentProcess(AgentProcessOptions{
		PID:       42,
		AgentName: "coder",
		Args:      args,
	})
	args[0] = "--changed"

	select {
	case <-proc.Done():
	default:
		t.Fatal("Done() channel is not closed by default")
	}
	if err := proc.Wait(); err != nil {
		t.Fatalf("Wait() error = %v, want nil", err)
	}
	if got := proc.Stderr(); got != "" {
		t.Fatalf("Stderr() = %q, want empty", got)
	}
	if got := proc.Args[0]; got != "--json" {
		t.Fatalf("NewAgentProcess() copied args = %q, want %q", got, "--json")
	}
}

func writeStoppedSessionArtifacts(t *testing.T, h *harness, id string, withDB bool) string {
	t.Helper()

	sessionDir := filepath.Join(h.homePaths.SessionsDir, id)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", sessionDir, err)
	}

	now := time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), store.SessionMeta{
		ID:                   id,
		Name:                 "stored",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		NetworkParticipation: testLocalParticipationPtr(),
		State:                string(StateStopped),
		CreatedAt:            now,
		UpdatedAt:            now,
	}); err != nil {
		t.Fatalf("WriteSessionMeta(%q) error = %v", id, err)
	}

	dbPath := store.SessionDBFile(sessionDir)
	if withDB {
		if err := os.WriteFile(dbPath, nil, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", dbPath, err)
		}
	}

	return dbPath
}

func createEscapedStoredSession(t *testing.T, h *harness) string {
	t.Helper()

	escapedID := "escaped-session"
	escapedDir := filepath.Join(filepath.Dir(h.homePaths.SessionsDir), escapedID)
	if err := os.MkdirAll(escapedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", escapedDir, err)
	}

	now := time.Now().UTC()
	if err := store.WriteSessionMeta(store.SessionMetaFile(escapedDir), store.SessionMeta{
		ID:                   escapedID,
		Name:                 "escaped",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		NetworkParticipation: testLocalParticipationPtr(),
		SessionType:          string(SessionTypeUser),
		State:                string(StateStopped),
		CreatedAt:            now.Add(-time.Minute),
		UpdatedAt:            now,
	}); err != nil {
		t.Fatalf("WriteSessionMeta(%q) error = %v", escapedDir, err)
	}

	recorder, err := h.manager.openStore(testutil.Context(t), escapedID, store.SessionDBFile(escapedDir))
	if err != nil {
		t.Fatalf("openStore(%q) error = %v", escapedID, err)
	}
	t.Cleanup(func() {
		if err := recorder.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(recorder) error = %v", err)
		}
	})

	if err := recorder.Record(testutil.Context(t), store.SessionEvent{
		TurnID:    "turn-1",
		Type:      acp.EventTypeAgentMessage,
		AgentName: "coder",
		Content:   `{"type":"agent_message","text":"escaped"}`,
		Timestamp: now,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	return escapedID
}

type queryRecorderStub struct {
	events      []store.SessionEvent
	history     []store.TurnHistory
	queryErr    error
	queryErrs   []error
	historyErr  error
	historyErrs []error
	closeCalls  int
	queryCalls  []store.EventQuery
	historyCall []store.EventQuery
	appendErrs  []error
	appendCalls int
	onAppend    func()
}

func (s *queryRecorderStub) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (s *queryRecorderStub) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (s *queryRecorderStub) AppendEventIfAbsent(
	_ context.Context,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	s.appendCalls++
	if s.onAppend != nil {
		s.onAppend()
	}
	if len(s.appendErrs) > 0 {
		err := s.appendErrs[0]
		s.appendErrs = s.appendErrs[1:]
		if err != nil {
			return store.SessionEvent{}, err
		}
	}
	return event, nil
}

func (s *queryRecorderStub) Query(_ context.Context, query store.EventQuery) ([]store.SessionEvent, error) {
	s.queryCalls = append(s.queryCalls, query)
	if len(s.queryErrs) > 0 {
		err := s.queryErrs[0]
		s.queryErrs = s.queryErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	return append([]store.SessionEvent(nil), s.events...), nil
}

func (s *queryRecorderStub) History(_ context.Context, query store.EventQuery) ([]store.TurnHistory, error) {
	s.historyCall = append(s.historyCall, query)
	if len(s.historyErrs) > 0 {
		err := s.historyErrs[0]
		s.historyErrs = s.historyErrs[1:]
		if err != nil {
			return nil, err
		}
	}
	if s.historyErr != nil {
		return nil, s.historyErr
	}
	return append([]store.TurnHistory(nil), s.history...), nil
}

func (s *queryRecorderStub) Close(context.Context) error {
	s.closeCalls++
	return nil
}
