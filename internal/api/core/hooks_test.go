package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestHookParsersAndPayloadConverters(t *testing.T) {
	t.Parallel()

	newContext := func(rawURL string) *gin.Context {
		t.Helper()
		recorder := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(recorder)
		ginCtx.Request = httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			rawURL,
			http.NoBody,
		)
		return ginCtx
	}

	eventValue := hookspkg.HookToolPreCall.String()

	catalogCtx := newContext("/hooks/catalog?agent=coder&event=" + eventValue + "&source=skill&mode=sync")
	catalogFilter, err := core.ParseHookCatalogFilter(catalogCtx)
	if err != nil {
		t.Fatalf("ParseHookCatalogFilter() error = %v", err)
	}
	if catalogFilter.AgentName != "coder" || catalogFilter.Event != hookspkg.HookToolPreCall ||
		catalogFilter.Source == nil ||
		*catalogFilter.Source != hookspkg.HookSourceSkill ||
		catalogFilter.Mode != hookspkg.HookModeSync {
		t.Fatalf("catalog filter = %#v", catalogFilter)
	}
	if _, err := core.ParseHookCatalogFilter(newContext("/hooks/catalog?source=bogus")); err == nil {
		t.Fatal("ParseHookCatalogFilter(invalid source) error = nil, want non-nil")
	}

	runsCtx := newContext(
		"/hooks/runs?session=sess-1&event=" + eventValue + "&outcome=applied&since=2026-04-03T12:00:00Z&last=2",
	)
	runsQuery, err := core.ParseHookRunsQuery(runsCtx)
	if err != nil {
		t.Fatalf("ParseHookRunsQuery() error = %v", err)
	}
	if runsQuery.SessionID != "sess-1" || runsQuery.Event != hookspkg.HookToolPreCall.String() ||
		runsQuery.Outcome != hookspkg.HookRunOutcomeApplied ||
		runsQuery.Limit != 2 ||
		runsQuery.Since.IsZero() {
		t.Fatalf("hook run query = %#v", runsQuery)
	}
	if _, err := core.ParseHookRunsQuery(newContext("/hooks/runs?session=sess-1&outcome=bogus")); err == nil {
		t.Fatal("ParseHookRunsQuery(invalid outcome) error = nil, want non-nil")
	}

	eventCtx := newContext("/hooks/events?family=tool&sync_only=true")
	eventFilter, err := core.ParseHookEventFilter(eventCtx)
	if err != nil {
		t.Fatalf("ParseHookEventFilter() error = %v", err)
	}
	if eventFilter.Family != hookspkg.HookEventFamilyTool || !eventFilter.SyncOnly {
		t.Fatalf("event filter = %#v", eventFilter)
	}
	if value, err := core.ParseOptionalBool("true"); err != nil || !value {
		t.Fatalf("ParseOptionalBool(true) = %v, %v", value, err)
	}
	if _, err := core.ParseHookEventFilter(newContext("/hooks/events?sync_only=nope")); err == nil {
		t.Fatal("ParseHookEventFilter(invalid bool) error = nil, want non-nil")
	}

	metadata := map[string]string{"origin": "workspace"}
	patchApplied := json.RawMessage(`{"allow":true}`)

	catalogPayloads := core.HookCatalogPayloadsFromEntries([]hookspkg.CatalogEntry{{
		Order:        1,
		Name:         "channel-opt-in",
		Event:        hookspkg.HookToolPreCall,
		Source:       hookspkg.HookSourceSkill,
		SkillSource:  hookspkg.HookSkillSourceWorkspace,
		Mode:         hookspkg.HookModeSync,
		Required:     true,
		Priority:     10,
		Timeout:      1500 * time.Millisecond,
		ExecutorKind: hookspkg.HookExecutorNative,
		Matcher:      hookspkg.HookMatcher{AgentName: "coder"},
		Metadata:     metadata,
	}})
	if got, want := len(catalogPayloads), 1; got != want {
		t.Fatalf("len(catalogPayloads) = %d, want %d", got, want)
	}
	if catalogPayloads[0].SkillSource != string(hookspkg.HookSkillSourceWorkspace) ||
		catalogPayloads[0].TimeoutMS != 1500 ||
		catalogPayloads[0].Metadata["origin"] != "workspace" {
		t.Fatalf("catalog payload = %#v", catalogPayloads[0])
	}
	metadata["origin"] = "mutated"
	if catalogPayloads[0].Metadata["origin"] != "workspace" {
		t.Fatalf("catalog payload metadata was not cloned: %#v", catalogPayloads[0].Metadata)
	}

	runPayloads := core.HookRunPayloadsFromRecords([]hookspkg.HookRunRecord{{
		HookName:      "channel-opt-in",
		Event:         hookspkg.HookToolPreCall,
		Source:        hookspkg.HookSourceSkill,
		Mode:          hookspkg.HookModeSync,
		Duration:      250 * time.Millisecond,
		Outcome:       hookspkg.HookRunOutcomeApplied,
		DispatchDepth: 2,
		PatchApplied:  patchApplied,
		Error:         "",
		Required:      true,
		RecordedAt:    time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}})
	if got, want := len(runPayloads), 1; got != want {
		t.Fatalf("len(runPayloads) = %d, want %d", got, want)
	}
	if string(runPayloads[0].PatchApplied) != `{"allow":true}` || runPayloads[0].DurationMS != 250 {
		t.Fatalf("hook run payload = %#v", runPayloads[0])
	}
	patchApplied[2] = 'X'
	if string(runPayloads[0].PatchApplied) != `{"allow":true}` {
		t.Fatalf("hook run payload patch was not cloned: %s", string(runPayloads[0].PatchApplied))
	}

	eventPayloads := core.HookEventPayloadsFromDescriptors([]hookspkg.EventDescriptor{{
		Event:         hookspkg.HookToolPreCall,
		Family:        hookspkg.HookEventFamilyTool,
		SyncEligible:  true,
		PayloadSchema: "ToolPreCallPayload",
		PatchSchema:   "ToolCallPatch",
	}})
	if got, want := len(eventPayloads), 1; got != want {
		t.Fatalf("len(eventPayloads) = %d, want %d", got, want)
	}
	if eventPayloads[0].Event != hookspkg.HookToolPreCall.String() || !eventPayloads[0].SyncEligible {
		t.Fatalf("hook event payload = %#v", eventPayloads[0])
	}
}

func TestHookHandlers(t *testing.T) {
	t.Parallel()

	manager := testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-1" {
				t.Fatalf("Status() id = %q, want sess-1", id)
			}
			info := testutil.NewSessionInfo(id)
			info.WorkspaceID = "ws-alpha"
			return info, nil
		},
	}
	observer := testutil.StubObserver{
		QueryHookCatalogFn: func(_ context.Context, filter hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error) {
			if filter.WorkspaceID != "ws-alpha" || filter.WorkspaceRoot != "/workspace/alpha" ||
				filter.AgentName != "coder" ||
				filter.Event != hookspkg.HookToolPreCall ||
				filter.Source == nil ||
				*filter.Source != hookspkg.HookSourceSkill ||
				filter.Mode != hookspkg.HookModeSync {
				t.Fatalf("QueryHookCatalog() filter = %#v", filter)
			}
			return []hookspkg.CatalogEntry{{
				Order:        1,
				Name:         "channel-opt-in",
				Event:        hookspkg.HookToolPreCall,
				Source:       hookspkg.HookSourceSkill,
				SkillSource:  hookspkg.HookSkillSourceWorkspace,
				Mode:         hookspkg.HookModeSync,
				Required:     true,
				Priority:     10,
				Timeout:      2 * time.Second,
				ExecutorKind: hookspkg.HookExecutorNative,
				Metadata:     map[string]string{"origin": "workspace"},
			}}, nil
		},
		QueryHookRunsFn: func(_ context.Context, query store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
			if query.SessionID != "sess-1" || query.Event != hookspkg.HookToolPreCall.String() ||
				query.Outcome != hookspkg.HookRunOutcomeApplied ||
				query.Limit != 2 ||
				query.Since.IsZero() {
				t.Fatalf("QueryHookRuns() query = %#v", query)
			}
			return []hookspkg.HookRunRecord{{
				HookName:      "channel-opt-in",
				Event:         hookspkg.HookToolPreCall,
				Source:        hookspkg.HookSourceSkill,
				Mode:          hookspkg.HookModeSync,
				Duration:      250 * time.Millisecond,
				Outcome:       hookspkg.HookRunOutcomeApplied,
				DispatchDepth: 1,
				PatchApplied:  json.RawMessage(`{"allow":true}`),
				Required:      true,
				RecordedAt:    time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
			}}, nil
		},
		QueryHookEventsFn: func(_ context.Context, filter hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error) {
			if filter.Family != hookspkg.HookEventFamilyTool || !filter.SyncOnly {
				t.Fatalf("QueryHookEvents() filter = %#v", filter)
			}
			return []hookspkg.EventDescriptor{{
				Event:         hookspkg.HookToolPreCall,
				Family:        hookspkg.HookEventFamilyTool,
				SyncEligible:  true,
				PayloadSchema: "ToolPreCallPayload",
				PatchSchema:   "ToolCallPatch",
			}}, nil
		},
	}
	workspaces := testutil.StubWorkspaceService{
		ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
			if ref != "alpha" {
				t.Fatalf("Resolve() ref = %q, want alpha", ref)
			}
			return workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{
					ID:      "ws-alpha",
					RootDir: "/workspace/alpha",
				},
				WorkspaceID: "ws-alpha",
			}, nil
		},
	}

	fixture := newHandlerFixture(t, manager, observer, workspaces, nil, nil)

	eventValue := hookspkg.HookToolPreCall.String()

	catalogResp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/hooks/catalog?workspace=alpha&agent=coder&event="+eventValue+"&source=skill&mode=sync",
		nil,
	)
	if catalogResp.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, want %d; body=%s", catalogResp.Code, http.StatusOK, catalogResp.Body.String())
	}
	var catalog contract.HookCatalogResponse
	testutil.DecodeJSONResponse(t, catalogResp, &catalog)
	if got, want := len(catalog.Hooks), 1; got != want {
		t.Fatalf("len(catalog.Hooks) = %d, want %d", got, want)
	}
	if catalog.Hooks[0].SkillSource != string(hookspkg.HookSkillSourceWorkspace) ||
		catalog.Hooks[0].Metadata["origin"] != "workspace" {
		t.Fatalf("catalog hook = %#v", catalog.Hooks[0])
	}

	runsResp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/workspaces/alpha/hooks/runs?session=sess-1&event="+eventValue+"&outcome=applied&since=2026-04-03T12:00:00Z&last=2",
		nil,
	)
	if runsResp.Code != http.StatusOK {
		t.Fatalf("runs status = %d, want %d; body=%s", runsResp.Code, http.StatusOK, runsResp.Body.String())
	}
	var runs contract.HookRunsResponse
	testutil.DecodeJSONResponse(t, runsResp, &runs)
	if got, want := len(runs.Runs), 1; got != want {
		t.Fatalf("len(runs.Runs) = %d, want %d", got, want)
	}
	if string(runs.Runs[0].PatchApplied) != `{"allow":true}` || runs.Runs[0].DurationMS != 250 {
		t.Fatalf("hook run = %#v", runs.Runs[0])
	}

	eventsResp := performRequest(t, fixture.Engine, http.MethodGet, "/hooks/events?family=tool&sync_only=true", nil)
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("events status = %d, want %d; body=%s", eventsResp.Code, http.StatusOK, eventsResp.Body.String())
	}
	var events contract.HookEventsResponse
	testutil.DecodeJSONResponse(t, eventsResp, &events)
	if got, want := len(events.Events), 1; got != want {
		t.Fatalf("len(events.Events) = %d, want %d", got, want)
	}
	if events.Events[0].Event != hookspkg.HookToolPreCall.String() || !events.Events[0].SyncEligible {
		t.Fatalf("hook event = %#v", events.Events[0])
	}
}

func TestHookHandlersRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixture(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	tests := []struct {
		name string
		path string
		want int
	}{
		{
			name: "catalog invalid event",
			path: "/hooks/catalog?event=invalid",
			want: http.StatusBadRequest,
		},
		{
			name: "Should reject runs missing session",
			path: "/workspaces/ws-workspace/hooks/runs?event=" + hookspkg.HookToolPreCall.String() + "&outcome=applied&last=1",
			want: http.StatusBadRequest,
		},
		{
			name: "events invalid bool",
			path: "/hooks/events?sync_only=definitely-not-bool",
			want: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := performRequest(t, fixture.Engine, http.MethodGet, tt.path, nil)
			if resp.Code != tt.want {
				t.Fatalf("%s status = %d, want %d; body=%s", tt.path, resp.Code, tt.want, resp.Body.String())
			}
		})
	}
}
