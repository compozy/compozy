package core_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/heartbeat"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func TestAgentInstanceReports(t *testing.T) {
	t.Run("Should page durable reports and reject a cursor from another instance", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instances := db.Instances()
		input := agentinstance.Instance{
			AgentName:         "coder",
			ProfileID:         store.DefaultProfileID,
			CreatedByKind:     "human",
			PermissionCeiling: "{}",
		}
		first, _, err := instances.AdmitCreate(t.Context(), input, 10)
		if err != nil {
			t.Fatal(err)
		}
		input.AgentName = "other"
		other, _, err := instances.AdmitCreate(t.Context(), input, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"occ:first", "occ:second"} {
			if _, err := db.InstanceReports().
				AppendReport(t.Context(), first.ID, agentinstance.ReportInput{SourceKey: key, Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"provider exited"}`)}); err != nil {
				t.Fatal(err)
			}
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store:     instances,
			Reports:   db.InstanceReports(),
			Authority: agentinstance.Authority{Callers: instances},
		}
		fixture.Engine.GET("/instances/:id/reports", fixture.Handlers.ListAgentInstanceReports)
		read := func(path string) (int, contract.AgentInstanceReportsResponse) {
			t.Helper()
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), "GET", path, nil))
			var body contract.AgentInstanceReportsResponse
			if response.Code == 200 {
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
			}
			if response.Code >= 400 {
				var failure contract.ErrorPayload
				if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
					t.Fatal(err)
				}
				if failure.Error == "" {
					t.Fatalf("missing error body: %s", response.Body.String())
				}
			}
			return response.Code, body
		}
		status, filtered := read("/instances/" + first.ID + "/reports?state=delivered")
		if status != 200 || len(filtered.Reports) != 0 {
			t.Fatalf("delivered filter: %d %#v", status, filtered)
		}
		status, filtered = read("/instances/" + first.ID + "/reports?state=pending")
		if status != 200 || len(filtered.Reports) != 2 {
			t.Fatalf("pending filter: %d %#v", status, filtered)
		}
		status, _ = read("/instances/" + first.ID + "/reports?state=unknown")
		if status != 422 {
			t.Fatalf("invalid state: %d", status)
		}
		status, page := read("/instances/" + first.ID + "/reports?limit=1")
		if status != 200 || len(page.Reports) != 1 || page.NextCursor == "" {
			t.Fatalf("first page: %d %#v", status, page)
		}
		if page.Reports[0].Detail != "provider exited" || page.Reports[0].OccurrenceID != "first" {
			t.Fatalf("payload: %#v", page.Reports[0])
		}
		status, _ = read("/instances/" + first.ID + "/reports?state=pending&cursor=" + page.NextCursor)
		if status != 422 {
			t.Fatalf("changed cursor filter: %d", status)
		}
		status, next := read("/instances/" + first.ID + "/reports?limit=1&cursor=" + page.NextCursor)
		if status != 200 || len(next.Reports) != 1 || next.Reports[0].ID == page.Reports[0].ID ||
			next.NextCursor != "" {
			t.Fatalf("next page: %d %#v", status, next)
		}
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(
			response,
			httptest.NewRequestWithContext(
				t.Context(),
				"GET",
				"/instances/"+other.ID+"/reports?cursor="+page.NextCursor,
				nil,
			),
		)
		var denial contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &denial); err != nil {
			t.Fatal(err)
		}
		if response.Code != 422 || denial.Error == "" {
			t.Fatalf("cursor denial: %d %#v", response.Code, denial)
		}
	})
}

type instanceReadFailure struct {
	agentinstance.InstanceStore
	err error
}

var _ agentinstance.InstanceStore = instanceReadFailure{}

func (s instanceReadFailure) Get(context.Context, string) (agentinstance.Instance, error) {
	return agentinstance.Instance{}, s.err
}

func TestAgentInstanceErrorResponses(t *testing.T) {
	// UT-086: domain errors cross the HTTP boundary with stable status and code.
	for _, tc := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"Should report forbidden access", agentinstance.ErrForbidden, 403, "An agent can only act on its own instance, within its limits."},
		{"Should report an archived workspace as a conflict", store.ErrWorkspaceArchived, 409, "This workspace is archived."},
		{"Should report workspace denial", agentinstance.ErrWorkspaceAccessDenied, 403, "This agent can't reach that workspace."},
		{"Should report child adoption", agentinstance.ErrSessionNotRoot, 422, "Only root sessions can be adopted."},
		{"Should report mismatched adoption scope", agentinstance.ErrScopeMismatch, 422, "This session can't be adopted by this agent and profile."},
		{"Should report a manually paused instance", agentinstance.ErrPaused, 409, "This agent is paused. Dry runs still work."},
		{"Should report excessive instance depth", agentinstance.ErrDepth, 403, "An instance created by an agent can't create more instances."},
		{"Should report a missing instance", agentinstance.ErrNotFound, 404, "This agent instance was not found."},
		{"Should report a missing definition", agentinstance.ErrDefinitionMissing, 404, "This agent's definition is gone. Reinstall it to resume. Its memory is kept."},
		{"Should report an existing instance", agentinstance.ErrExists, 409, "This agent already runs in that scope."},
		{"Should report an instance still starting", agentinstance.ErrStarting, 409, "This agent is already starting there."},
		{"Should report the active instance cap", agentinstance.ErrLimit, 429, "You have 7 active always-on agents. Pause one or raise `agents.always_on.max_active_instances`."},
		{"Should report an on demand definition", agentinstance.ErrNotAlwaysOn, 422, "This agent is on-demand. Only always-on agents have instances."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
			)
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
				MaxActiveInstances: 7, Store: instanceReadFailure{err: fmt.Errorf("repository: %w", tc.err)},
			}
			fixture.Engine.GET("/instances/:id", fixture.Handlers.GetAgentInstance)
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(
				response,
				httptest.NewRequestWithContext(t.Context(), "GET", "/instances/missing", nil),
			)
			var body contract.ErrorPayload
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			wantCode := tc.err.Error()
			if errors.Is(tc.err, store.ErrWorkspaceArchived) {
				wantCode = "workspace_archived"
			}
			if response.Code != tc.status || body.Code != wantCode || body.Error != tc.message {
				t.Fatalf("response: status=%d body=%#v, want status=%d code=%s", response.Code, body, tc.status, tc.err)
			}
		})
	}
}

func TestAgentInstanceDefinitionDeletion(t *testing.T) {
	for _, state := range []string{"starting", "active", "paused", "archived"} {
		t.Run("Should enforce definition deletion for "+state+" instances", func(t *testing.T) {
			t.Parallel()
			db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			instance, _, err := db.Instances().
				AdmitCreate(t.Context(), agentinstance.Instance{AgentName: "coder", ProfileID: store.DefaultProfileID, CreatedByKind: "human", PermissionCeiling: "{}"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if state == "archived" {
				if _, err := db.Instances().Archive(t.Context(), instance.ID); err != nil {
					t.Fatal(err)
				}
			} else if state != "starting" {
				if _, err := db.DB().
					ExecContext(t.Context(), "UPDATE agent_instances SET state=? WHERE id=?", state, instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
			)
			path := filepath.Join(fixture.HomePaths.AgentsDir, "coder", compozyconfig.AgentDefinitionFileName)
			if _, err := compozyconfig.CreateAgentDefFile(
				path,
				compozyconfig.AgentDefinitionDraft{
					Name:     "coder",
					Provider: "codex",
					Prompt:   "Preserve this definition.",
				},
				false,
			); err != nil {
				t.Fatal(err)
			}
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: db.Instances()}
			response := performRequest(t, fixture.Engine, "DELETE", "/agents/coder", nil)
			if state == "archived" {
				var body contract.DeleteAgentResponse
				decodeJSON(t, response.Body.Bytes(), &body)
				if response.Code != 200 || body.Name != "coder" {
					t.Fatalf("delete: %d %#v", response.Code, body)
				}
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("definition remains: %v", err)
				}
				return
			}
			var body contract.ErrorPayload
			decodeJSON(t, response.Body.Bytes(), &body)
			if response.Code != 409 || body.Code != "agent_has_instances" {
				t.Fatalf("delete: %d %#v", response.Code, body)
			}
			definition, err := compozyconfig.LoadAgentDefFile(path)
			if err != nil || definition.Prompt != "Preserve this definition." {
				t.Fatalf("definition changed: %#v, %v", definition, err)
			}
		})
	}
}

func TestAgentInstanceDeletionPagination(t *testing.T) {
	t.Run("Should reject deletion when the blocking instance is beyond the first page", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		for n := range 101 {
			if _, _, err := db.Instances().
				AdmitCreate(t.Context(), agentinstance.Instance{AgentName: fmt.Sprintf("agent-%d", n), ProfileID: store.DefaultProfileID, CreatedByKind: "human", PermissionCeiling: "{}"}, 101); err != nil {
				t.Fatal(err)
			}
		}
		first, err := db.Instances().
			List(t.Context(), agentinstance.ListQuery{ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 100})
		if err != nil || len(first) != 100 {
			t.Fatalf("first page: %d %v", len(first), err)
		}
		last, err := db.Instances().
			List(t.Context(), agentinstance.ListQuery{ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 100, AfterID: first[99].ID})
		if err != nil || len(last) != 1 {
			t.Fatalf("last page: %#v %v", last, err)
		}
		name := last[0].AgentName
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		path := filepath.Join(fixture.HomePaths.AgentsDir, name, compozyconfig.AgentDefinitionFileName)
		if _, err := compozyconfig.CreateAgentDefFile(
			path,
			compozyconfig.AgentDefinitionDraft{Name: name, Provider: "codex", Prompt: "Keep this agent."},
			false,
		); err != nil {
			t.Fatal(err)
		}
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: db.Instances()}
		response := performRequest(t, fixture.Engine, "DELETE", "/agents/"+name, nil)
		var body contract.ErrorPayload
		decodeJSON(t, response.Body.Bytes(), &body)
		if response.Code != 409 || body.Code != "agent_has_instances" {
			t.Fatalf("delete past first page: %d %#v", response.Code, body)
		}
		if _, err := compozyconfig.LoadAgentDefFile(path); err != nil {
			t.Fatalf("blocked definition was removed: %v", err)
		}
	})
}

type instanceCheckStub struct {
	result heartbeat.CheckResult
	dry    bool
	id     string
}

var _ core.AgentInstanceChecker = (*instanceCheckStub)(nil)

func (s *instanceCheckStub) Check(_ context.Context, id string, dry bool) (heartbeat.CheckResult, error) {
	s.id, s.dry = id, dry
	return s.result, nil
}

func TestAgentInstanceCheck(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		result     heartbeat.CheckResult
		status     int
		dry        bool
	}{
		{name: "Should return an admitted occurrence", body: `{"dry_run":false}`, result: heartbeat.CheckResult{OccurrenceID: "instance_check-test"}, status: 202},
		{name: "Should return a gate without an occurrence", body: `{"dry_run":false}`, result: heartbeat.CheckResult{Gate: "no_slot"}, status: 200},
		{name: "Should forward dry runs and return their gate", body: `{"dry_run":true}`, result: heartbeat.CheckResult{Gate: "would_run"}, status: 200, dry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "check.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			instance, _, err := db.Instances().
				AdmitCreate(t.Context(), agentinstance.Instance{AgentName: "coder", ProfileID: store.DefaultProfileID, CreatedByKind: "human", PermissionCeiling: "{}"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			checker := &instanceCheckStub{result: tc.result}
			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
			)
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
				Store:     db.Instances(),
				Checks:    checker,
				Authority: agentinstance.Authority{Callers: db.Instances()},
			}
			fixture.Engine.POST("/instances/:id/check", fixture.Handlers.CheckAgentInstance)
			response := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(
				t.Context(),
				"POST",
				"/instances/"+instance.ID+"/check",
				strings.NewReader(tc.body),
			)
			req.Header.Set("Content-Type", "application/json")
			fixture.Engine.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			var body contract.AgentInstanceCheckResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Gate != tc.result.Gate || body.OccurrenceID != tc.result.OccurrenceID ||
				checker.id != instance.ID ||
				checker.dry != tc.dry {
				t.Fatalf("response=%+v call=%+v", body, checker)
			}
		})
	}
}

func TestAgentInstanceCheckSettings(t *testing.T) {
	t.Run("Should reject a missing checks setting before mutation", func(t *testing.T) {
		t.Parallel()
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Engine.PATCH("/instances/:id", fixture.Handlers.UpdateAgentInstance)
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(
			response,
			httptest.NewRequestWithContext(t.Context(), "PATCH", "/instances/unknown", strings.NewReader(`{}`)),
		)
		var body contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 422 || !strings.Contains(body.Error, "checks_enabled") {
			t.Fatalf("response=%d %s", response.Code, response.Body.String())
		}
	})
}

func TestAgentInstanceDailyBudgetSettings(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "budget-api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Errorf("close agent instance budget DB: %v", err)
		}
	})
	instance, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
		AgentName: "coder", ProfileID: store.DefaultProfileID,
		CreatedByKind: "human", PermissionCeiling: "{}",
	}, 10)
	if err != nil {
		t.Fatal(err)
	}
	fixture := newHandlerFixture(
		t,
		&testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
	)
	fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
		Store: db.Instances(), Authority: agentinstance.Authority{Callers: db.Instances()},
	}
	fixture.Engine.PATCH("/instances/:id", fixture.Handlers.UpdateAgentInstance)
	fixture.Engine.GET("/instances/:id", fixture.Handlers.GetAgentInstance)
	url := "/instances/" + instance.ID
	for _, tc := range []struct {
		name, body string
		wantStatus int
		wantTokens *int64
		wantUSD    *float64
	}{
		{"reject both mutations", `{"checks_enabled":false,"daily_budget":{"tokens":100}}`, 422, nil, nil},
		{"reject invalid cap", `{"daily_budget":{"tokens":-1}}`, 422, nil, nil},
		{"set both caps", `{"daily_budget":{"tokens":100,"usd":1.25}}`, 200, new(int64(100)), new(float64(1.25))},
		{"reject typo without clearing caps", `{"daily_budget":{"token":100}}`, 422, new(int64(100)), new(float64(1.25))},
		{"reject misplaced cap without clearing caps", `{"daily_budget":{},"usd":1.25}`, 422,
			new(int64(100)), new(float64(1.25))},
		{"clear USD but keep tokens", `{"daily_budget":{"tokens":0}}`, 200, new(int64(0)), nil},
		{"clear all caps", `{"daily_budget":{}}`, 200, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performRequest(t, fixture.Engine, "PATCH", url, []byte(tc.body))
			if resp.Code != tc.wantStatus {
				t.Fatalf("PATCH status=%d body=%s", resp.Code, resp.Body.String())
			}
			get := performRequest(t, fixture.Engine, "GET", url, nil)
			if get.Code != 200 {
				t.Fatalf("GET status=%d body=%s", get.Code, get.Body.String())
			}
			var payload contract.AgentInstanceResponse
			decodeJSON(t, get.Body.Bytes(), &payload)
			if strings.Contains(get.Body.String(), `"Tokens"`) || strings.Contains(get.Body.String(), `"USD"`) {
				t.Fatalf("noncanonical daily budget JSON keys: %s", get.Body.String())
			}
			limits := payload.Instance.DailyBudget
			if limits == nil {
				if tc.wantTokens != nil || tc.wantUSD != nil {
					t.Fatal("configured budget missing from GET")
				}
				return
			}
			if (tc.wantTokens == nil) != (limits.Tokens == nil) ||
				(tc.wantUSD == nil) != (limits.USD == nil) ||
				(tc.wantTokens != nil && *tc.wantTokens != *limits.Tokens) ||
				(tc.wantUSD != nil && *tc.wantUSD != *limits.USD) {
				t.Fatalf("limits=%+v, want tokens=%v usd=%v", limits, tc.wantTokens, tc.wantUSD)
			}
		})
	}
	t.Run("Should keep paired app credentials from replacing operator limits", func(t *testing.T) {
		request := httptest.NewRequestWithContext(t.Context(), "PATCH", url,
			strings.NewReader(`{"daily_budget":{"tokens":0}}`))
		request.Header.Set("Content-Type", "application/json")
		request = request.WithContext(core.ContextWithAppToken(request.Context(), gateway.AppToken{}))
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, request)
		var failure contract.ErrorPayload
		decodeJSON(t, response.Body.Bytes(), &failure)
		if response.Code != http.StatusForbidden || failure.Error == "" {
			t.Fatalf("paired app budget update status=%d response=%s", response.Code, response.Body.String())
		}
	})
	fixture.Handlers.AgentInstances.WithDailyBudgetUpdate = func(context.Context, string, bool, func(context.Context) error) error {
		return errors.New("ledger requires recovery")
	}
	blocked := performRequest(t, fixture.Engine, "PATCH", url,
		[]byte(`{"daily_budget":{"tokens":250}}`))
	if blocked.Code != http.StatusInternalServerError {
		t.Fatalf("unsafe budget enable status=%d body=%s", blocked.Code, blocked.Body.String())
	}
	limits, err := db.Instances().DailyBudgetLimits(ctx, instance.ID)
	if err != nil || limits.Tokens != nil || limits.USD != nil {
		t.Fatalf("failed pre-enable reconciliation altered limits: %+v err=%v", limits, err)
	}
	fixture.Handlers.AgentInstances.WithDailyBudgetUpdate = nil
	t.Run(
		"Should keep usage errors local while preserving successful mutations and instance reads",
		func(t *testing.T) {
			now := time.Now()
			fixture.Handlers.AgentInstances.Clock = func() time.Time { return now }
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO agent_instance_daily_budget_usage
			(instance_id, profile_id, day, total_tokens, has_unpriced_usage, invalid)
			VALUES (?, ?, ?, 0, 0, 1)`, instance.ID, instance.ProfileID, store.LocalDay(now)); err != nil {
				t.Fatal(err)
			}
			peer, _, err := db.Instances().AdmitCreate(ctx, agentinstance.Instance{
				AgentName: "peer", ProfileID: instance.ProfileID,
				CreatedByKind: "human", PermissionCeiling: "{}",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			for _, method := range []string{"PATCH", "GET"} {
				var body []byte
				if method == "PATCH" {
					body = []byte(`{"daily_budget":{"tokens":250}}`)
				}
				response := performRequest(t, fixture.Engine, method, url, body)
				var payload contract.AgentInstanceResponse
				decodeJSON(t, response.Body.Bytes(), &payload)
				configured := payload.Instance.DailyBudget
				if response.Code != 200 || configured == nil || configured.Tokens == nil ||
					*configured.Tokens != 250 || payload.Instance.DailyBudgetUsage != nil {
					t.Fatalf("%s status=%d response=%s", method, response.Code, response.Body.String())
				}
			}
			page, err := fixture.Handlers.AgentInstances.ListPage(
				ctx,
				agentinstance.ListQuery{
					ProfileID: instance.ProfileID,
					Limit:     10,
				},
				workspaceaccess.ActorRef{Operator: true},
			)
			if err != nil || len(page.Instances) != 2 {
				t.Fatalf("instance page=%+v err=%v", page, err)
			}
			for _, item := range page.Instances {
				if item.ID == peer.ID && item.DailyBudget != nil {
					t.Fatalf("peer unexpectedly acquired a budget: %+v", item)
				}
			}
		},
	)
	t.Run("Should return committed caps even when the instance projection cannot reload", func(t *testing.T) {
		api := fixture.Handlers.AgentInstances
		api.ReadPayload = func(context.Context, agentinstance.Instance) (contract.AgentInstancePayload, error) {
			return contract.AgentInstancePayload{}, errors.New("instance projection unavailable")
		}
		t.Cleanup(func() { api.ReadPayload = nil })
		response := performRequest(t, fixture.Engine, "PATCH", url, []byte(`{"daily_budget":{"tokens":125}}`))
		var payload contract.AgentInstanceResponse
		decodeJSON(t, response.Body.Bytes(), &payload)
		configured := payload.Instance.DailyBudget
		if response.Code != http.StatusOK || payload.Instance.ID != instance.ID ||
			configured == nil || configured.Tokens == nil || *configured.Tokens != 125 ||
			payload.Instance.DailyBudgetUsage != nil || payload.Instance.Sources == nil ||
			payload.Instance.ResolvedAppearance == nil {
			t.Fatalf("budget update status=%d body=%s", response.Code, response.Body.String())
		}
		persisted, err := db.Instances().DailyBudgetLimits(ctx, instance.ID)
		if err != nil || persisted.Tokens == nil || *persisted.Tokens != 125 {
			t.Fatalf("persisted caps=%+v err=%v, want 125 tokens", persisted, err)
		}
	})
}

func TestAgentInstanceAway(t *testing.T) {
	t.Run("Should keep reads passive and hide seen reports after marking", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "away.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instance, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		report, err := db.InstanceReports().
			AppendReport(t.Context(), instance.ID, agentinstance.ReportInput{SourceKey: "turn:away-report", Kind: "failure", Payload: []byte(`{"reason":"failed","detail":"provider exited"}`)})
		if err != nil {
			t.Fatal(err)
		}
		for id, result := range map[string]string{"event-quiet": "quiet", "event-skipped": "skipped"} {
			_, err := db.DB().
				ExecContext(t.Context(), `INSERT INTO agent_heartbeat_wake_events (id,workspace_id,agent_name,instance_id,source,result,reason,created_at,expires_at) VALUES (?,'','coder',?,'timer',?,'digest_empty',?,'2099-01-01T00:00:00Z')`, id, instance.ID, result, store.FormatTimestamp(report.CreatedAt))
			if err != nil {
				t.Fatal(err)
			}
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store:     db.Instances(),
			Reports:   db.InstanceReports(),
			Authority: agentinstance.Authority{Callers: db.Instances()},
		}
		at := report.CreatedAt.Add(time.Second)
		fixture.Handlers.Now = func() time.Time { return at }
		other, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "other", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		for i := range 105 {
			id, result, timestamp := instance.ID, "quiet", at
			if i == 102 {
				timestamp = at.Add(time.Second)
			}
			if i == 103 {
				id = other.ID
			}
			if i == 104 {
				result = "failed"
			}
			if _, err := db.DB().
				ExecContext(t.Context(), `INSERT INTO agent_heartbeat_wake_events (id,workspace_id,agent_name,instance_id,source,result,reason,created_at,expires_at) VALUES (?,'','coder',?,'timer',?,'quiet',?,?)`, fmt.Sprintf("quiet-%03d", i), id, result, store.FormatTimestamp(timestamp), store.FormatTimestamp(at.Add(time.Hour))); err != nil {
				t.Fatal(err)
			}
		}
		updates := 0
		fixture.Handlers.AgentInstances.AfterSeen = func(_ context.Context, got agentinstance.Instance, _ []store.SessionAttentionCommit) {
			if got.ID != instance.ID {
				t.Errorf("updated instance=%q", got.ID)
			}
			updates++
		}
		fixture.Engine.GET("/instances/:id/away", fixture.Handlers.GetAgentInstanceAway)
		fixture.Engine.POST("/instances/:id/away", fixture.Handlers.MarkAgentInstanceSeen)
		for _, method := range []string{"GET", "POST", "GET"} {
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(
				response,
				httptest.NewRequestWithContext(t.Context(), method, "/instances/"+instance.ID+"/away", nil),
			)
			if response.Code != 200 {
				t.Fatalf("%s: %d %s", method, response.Code, response.Body.String())
			}
			var body contract.AgentInstanceAwayResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if method == "GET" && body.LastSeenAt == nil {
				if body.QuietCheckIns != 104 || len(body.Reports) != 1 || body.Reports[0].ID != report.ID {
					t.Fatalf("unseen body = %+v", body)
				}
				seen, err := db.Instances().LastSeen(t.Context(), instance.ID, agentinstance.LocalOperator)
				if err != nil || seen != nil {
					t.Fatalf("GET changed seen: %v %v", seen, err)
				}
			} else {
				if method == "GET" && body.QuietCheckIns != 1 {
					t.Fatalf("quiet after seen=%d", body.QuietCheckIns)
				}
				if body.LastSeenAt == nil || !body.LastSeenAt.Equal(at) || len(body.Reports) != 0 {
					t.Fatalf("seen body = %+v", body)
				}
			}
		}
		if updates != 1 {
			t.Fatalf("catalog updates=%d", updates)
		}
		if attention, err := db.Instances().NeedsAttention(t.Context(), instance.ID); err != nil || attention {
			t.Fatalf("attention after HTTP acknowledgement=%v, %v", attention, err)
		}
	})
}

func TestAgentInstanceAllProfilesLens(t *testing.T) {
	t.Run(
		"Should read another profile's instance under all profiles but refuse to write through it",
		func(t *testing.T) {
			t.Parallel()
			db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "lens.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			// The fixture acts as default; the instance belongs to another profile.
			foreign := "uuuuuuuuuuuuuuuuuuuuuuuuuu"
			if _, err := db.DB().ExecContext(
				t.Context(),
				`INSERT INTO profiles(id,name,color,icon,state,created_at) VALUES (?, 'foreign', '#8E8EB5', 'circle', 'active', ?)`,
				foreign, store.FormatTimestamp(time.Now()),
			); err != nil {
				t.Fatal(err)
			}
			instance, _, err := db.Instances().
				AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: foreign, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
			if err != nil {
				t.Fatal(err)
			}
			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
			)
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
				Store:     db.Instances(),
				Reports:   db.InstanceReports(),
				Authority: agentinstance.Authority{Callers: db.Instances()},
			}
			fixture.Engine.GET("/instances/:id/away", fixture.Handlers.GetAgentInstanceAway)
			fixture.Engine.POST("/instances/:id/away", fixture.Handlers.MarkAgentInstanceSeen)
			serve := func(method, query string) *httptest.ResponseRecorder {
				response := httptest.NewRecorder()
				fixture.Engine.ServeHTTP(response, httptest.NewRequestWithContext(
					t.Context(), method, "/instances/"+instance.ID+"/away"+query, nil,
				))
				return response
			}
			if response := serve("GET", ""); response.Code != 404 {
				t.Fatalf("scoped read: %d %s, want the foreign instance hidden", response.Code, response.Body.String())
			}
			if response := serve("GET", "?all_profiles=true"); response.Code != 200 {
				t.Fatalf("aggregate read: %d %s", response.Code, response.Body.String())
			}
			response := serve("POST", "?all_profiles=true")
			if response.Code != 400 ||
				!strings.Contains(response.Body.String(), "all_profiles is only available for reads") {
				t.Fatalf(
					"aggregate write: %d %s, want the read-only lens refusal",
					response.Code,
					response.Body.String(),
				)
			}
			seen, err := db.Instances().LastSeen(t.Context(), instance.ID, agentinstance.LocalOperator)
			if err != nil || seen != nil {
				t.Fatalf("refused write changed seen: %v %v", seen, err)
			}
		},
	)
}

// Activity authorization belongs to the HTTP read boundary; aggregation uses real SQLite.
type activityReachPolicy struct{}

var _ workspaceaccess.Policy = activityReachPolicy{}

func (activityReachPolicy) Authorize(_ context.Context, req workspaceaccess.Request) (workspaceaccess.Decision, error) {
	return workspaceaccess.Decision{
		Allowed: req.Target.Kind == workspaceaccess.TargetAggregate || req.Target.WorkspaceID == "allowed",
		Source:  workspaceaccess.SourceAgentGrant,
	}, nil
}

func TestAgentInstanceActivityReach(t *testing.T) {
	t.Run("Should preserve internal error masking and reject invalid workspace input", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []string{"list", "label", "invalid", "resolve"} {
			t.Run("Should classify "+kind+" failure", func(t *testing.T) {
				t.Parallel()
				db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "errors.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.Close(context.Background()); err != nil {
						t.Error(err)
					}
				})
				fixture := newHandlerFixture(
					t,
					&testutil.StubSessionManager{},
					testutil.StubObserver{},
					testutil.StubWorkspaceService{
						ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
							if kind == "list" {
								return nil, errors.New("private catalog cause")
							}
							return []workspacepkg.Workspace{{ID: "allowed"}}, nil
						},
						ResolveFn: func(context.Context, string) (workspacepkg.ResolvedWorkspace, error) {
							if kind == "invalid" {
								return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
							}
							return workspacepkg.ResolvedWorkspace{}, errors.New("private resolve cause")
						},
					},
				)
				fixture.Handlers.MaskInternalErrors = true
				fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: db.Instances()}
				fixture.Engine.GET("/activity/:name", fixture.Handlers.GetAgentActivity)
				fixture.Engine.POST("/activity", fixture.Handlers.BatchAgentActivity)
				workspace := "allowed"
				want := 500
				if kind == "invalid" || kind == "resolve" {
					workspace = "missing"
				}
				if kind == "invalid" {
					want = 422
				}
				for _, method := range []string{"GET", "POST"} {
					target := "/activity/coder?workspace=" + workspace
					body := ""
					if method == "POST" {
						target = "/activity"
						body = `{"items":[{"name":"coder","workspaces":["` + workspace + `"]}]}`
					}
					request := httptest.NewRequestWithContext(t.Context(), method, target, strings.NewReader(body))
					request.Header.Set("Content-Type", "application/json")
					response := httptest.NewRecorder()
					fixture.Engine.ServeHTTP(response, request)
					if response.Code != want {
						t.Fatalf("%s %s status=%d body=%s", kind, method, response.Code, response.Body.String())
					}
					if want == 500 &&
						(!strings.Contains(response.Body.String(), "Internal Server Error") || strings.Contains(response.Body.String(), "private")) {
						t.Fatalf("unmasked failure: %s", response.Body.String())
					}
					if want == 422 && !strings.Contains(response.Body.String(), "invalid activity request") {
						t.Fatalf("invalid input body=%s", response.Body.String())
					}
				}
			})
		}
	})

	t.Run("Should serve sparse activity from a million telemetry events without cancellations", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), "million.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.WithoutCancel(ctx)); err != nil {
				t.Error(err)
			}
		})
		if _, err := db.DB().ExecContext(ctx, "ATTACH DATABASE ? AS seed", db.Path()+".telemetry.db"); err != nil {
			t.Fatal(err)
		}
		started := time.Now()
		if _, err := db.DB().
			ExecContext(ctx, `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1000000)
          INSERT INTO seed.event_summaries(id,profile_id,workspace_id,agent_name,type,timestamp,content_json)
          SELECT 'load-'||x, ?, '', CASE WHEN x<=10 THEN 'sparse' ELSE 'busy' END, 'thought', '2026-11-01T12:00:00Z', '{"text":"retained telemetry payload"}' FROM n`, store.DefaultProfileID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(ctx, "DETACH DATABASE seed"); err != nil {
			t.Fatal(err)
		}
		t.Logf("seeded 1000000 events in %s", time.Since(started))
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: db.Instances()}
		fixture.Handlers.Now = func() time.Time { return time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC) }
		fixture.Engine.GET("/api/agents/:name/activity", fixture.Handlers.GetAgentActivity)
		fixture.Engine.POST("/api/agents/activity", fixture.Handlers.BatchAgentActivity)
		samples := make([]time.Duration, 100)
		for i := range samples {
			request := httptest.NewRequestWithContext(ctx, "GET", "/api/agents/sparse/activity", nil)
			response := httptest.NewRecorder()
			started := time.Now()
			fixture.Engine.ServeHTTP(response, request)
			samples[i] = time.Since(started)
			if response.Code != 200 {
				t.Fatalf("read %d status=%d body=%s", i, response.Code, response.Body.String())
			}
			var result agentinstance.Activity
			decodeJSON(t, response.Body.Bytes(), &result)
			if len(result.BusyByHour) != 1 || result.BusyByHour[0].Events != 10 {
				t.Fatalf("sparse activity=%+v", result)
			}
		}
		slices.Sort(samples)
		if samples[94] >= 100*time.Millisecond {
			t.Fatalf("sparse activity p95=%s, want below 100ms", samples[94])
		}
		t.Logf(
			"GET /api/agents/sparse/activity: n=100 p50=%s p95=%s max=%s status499=0",
			samples[49],
			samples[94],
			samples[99],
		)
		for range 20 {
			request := httptest.NewRequestWithContext(
				ctx,
				"POST",
				"/api/agents/activity",
				strings.NewReader(
					`{"days":14,"items":[{"name":"sparse","workspaces":["@user"]},{"name":"quiet-a","workspaces":["@user"]},{"name":"quiet-b","workspaces":["@user"]},{"name":"quiet-c","workspaces":["@user"]}]}`,
				),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(response, request)
			if response.Code != 200 {
				t.Fatalf("team batch status=%d body=%s", response.Code, response.Body.String())
			}
		}
	})

	t.Run(
		"Should isolate activity by caller profile and authorized workspace and return empty arrays",
		func(t *testing.T) {
			t.Parallel()
			db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "activity.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.Local)
			foreign := "uuuuuuuuuuuuuuuuuuuuuuuuuu"
			if _, err := db.DB().
				ExecContext(t.Context(), `INSERT INTO profiles(id,name,color,icon,state,created_at) VALUES (?, 'foreign', '#8E8EB5', 'circle', 'active', ?)`, foreign, store.FormatTimestamp(now)); err != nil {
				t.Fatal(err)
			}
			for _, row := range []struct {
				profile, workspace string
				tokens             int64
			}{
				{store.DefaultProfileID, "allowed", 10}, {store.DefaultProfileID, "denied", 20}, {foreign, "allowed", 40}, {store.DefaultProfileID, "", 80},
			} {
				if err := db.UpsertTokenUsageDaily(
					t.Context(),
					store.TokenUsageDailyUpdate{
						ProfileID:   row.profile,
						WorkspaceID: row.workspace,
						AgentName:   "coder",
						Day:         store.LocalDay(now),
						TotalTokens: &row.tokens,
						CostStatus:  store.TokenCostStatusIncluded,
						CostSource:  store.TokenCostSourceNone,
					},
				); err != nil {
					t.Fatal(err)
				}
			}
			lists, resolves := 0, 0

			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{StatusFn: func(context.Context, string) (*session.Info, error) {
					return &session.Info{
						ID:                       "caller",
						AgentName:                "coder",
						ProfileID:                store.DefaultProfileID,
						WorkspaceID:              "allowed",
						State:                    session.StateActive,
						EffectiveWorkspaceAccess: workspaceaccess.Unrestricted,
					}, nil
				}},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{
					ListFn: func(context.Context) ([]workspacepkg.Workspace, error) {
						lists++
						return []workspacepkg.Workspace{{ID: "allowed"}, {ID: "denied"}}, nil
					},
					ResolveFn: func(_ context.Context, id string) (workspacepkg.ResolvedWorkspace, error) {
						resolves++
						return workspacepkg.ResolvedWorkspace{ID: id, Name: id}, nil
					},
				},
			)
			fixture.Handlers.Now = func() time.Time { return now }
			fixture.Handlers.WorkspaceAccess = activityReachPolicy{}
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: db.Instances()}
			fixture.Engine.GET("/activity/:name", fixture.Handlers.GetAgentActivity)
			fixture.Engine.POST("/activity", fixture.Handlers.BatchAgentActivity)
			for _, body := range []string{
				`{"days":30,"items":[{"name":"coder","workspaces":["allowed","denied","@user"]},{"name":"coder","workspaces":["denied"]},{"name":"empty","workspaces":["allowed"]}]}`,
			} {
				req := httptest.NewRequestWithContext(t.Context(), "POST", "/activity", strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set(agentidentity.HeaderSessionID, "caller")
				req.Header.Set(agentidentity.HeaderAgent, "coder")
				response := httptest.NewRecorder()
				fixture.Engine.ServeHTTP(response, req)
				var got contract.AgentActivityBatchResponse
				if response.Code != 200 {
					t.Fatalf("batch status=%d body=%s", response.Code, response.Body.String())
				}
				decodeJSON(t, response.Body.Bytes(), &got)
				if len(got.Activities) != 3 || len(got.Activities[0].CostByProject) != 1 ||
					got.Activities[0].CostByProject[0].Tokens != 10 ||
					len(got.Activities[1].CostByProject) != 0 ||
					len(got.Activities[2].CostByProject) != 0 {
					t.Fatalf("batch reach leak: %+v", got)
				}
			}

			if lists != 1 || resolves != 1 {
				t.Fatalf("batch repeated workspace reads: lists=%d resolves=%d", lists, resolves)
			}

			// Invariant: an explicit workspace restricts the already-authorized activity set.
			for _, tc := range []struct {
				name, query  string
				wantActivity bool
			}{
				{"coder", "", true}, {"empty", "", false},
				{"coder", "?workspace=allowed", true}, {"coder", "?workspace=denied", false},
			} {
				req := httptest.NewRequestWithContext(t.Context(), "GET", "/activity/"+tc.name+tc.query, nil)
				req.Header.Set(agentidentity.HeaderSessionID, "caller")
				req.Header.Set(agentidentity.HeaderAgent, "coder")
				response := httptest.NewRecorder()
				fixture.Engine.ServeHTTP(response, req)
				var body agentinstance.Activity
				if response.Code != 200 {
					t.Fatalf("activity status=%d body=%s", response.Code, response.Body.String())
				}
				decodeJSON(t, response.Body.Bytes(), &body)
				if body.ReportsByDay == nil || body.BusyByHour == nil || body.CostByProject == nil {
					t.Fatalf("null arrays: %+v", body)
				}
				if tc.wantActivity {
					if len(body.CostByProject) != 1 || body.CostByProject[0].Tokens != 10 ||
						body.CostByProject[0].WorkspaceID != "allowed" {
						t.Fatalf("scope leak: %+v", body)
					}
				} else if len(body.CostByProject) != 0 || len(body.ReportsByDay) != 0 || len(body.BusyByHour) != 0 {
					t.Fatalf("invented activity: %+v", body)
				}
			}
		},
	)
}

func TestAgentInstanceReadFacets(t *testing.T) {
	// Invariant: authorized instance reads expose durable spend and held-input counts.
	// Owning layer/canonical suite: instance API projection, TestAgentInstanceReadFacets.
	t.Run("Should expose unattended turns and held wakes", func(t *testing.T) {
		t.Parallel()
		api := core.AgentInstanceAPI{
			UnattendedTurnLimit: 300,
			WakeState: func(context.Context, string) (heartbeat.InstanceWakeState, error) {
				return heartbeat.InstanceWakeState{UnattendedTurns: 300}, nil
			},
			HeldWakes: func(context.Context, string) (int64, error) { return 7, nil },
		}
		payload, err := api.Payload(t.Context(), agentinstance.Instance{ID: "paused", State: "paused"})
		if err != nil || payload.UnattendedTurns != 300 || payload.UnattendedTurnLimit != 300 ||
			payload.HeldWakes != 7 {
			t.Fatalf("spend projection = %+v, %v", payload, err)
		}
	})
	// Invariant: authorized rows fill pages and only a visible successor creates a cursor.
	// Owner: instance API projection; canonical suite: TestAgentInstanceReadFacets.
	t.Run("Should page after authorization and omit a full terminal cursor", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "pages.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		for _, name := range []string{"one", "two", "three"} {
			if _, _, err := db.Instances().
				AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: name, PermissionCeiling: "{}", CreatedByKind: "human"}, 10); err != nil {
				t.Fatal(err)
			}
		}
		query := agentinstance.ListQuery{ProfileID: store.DefaultProfileID, Limit: 3}
		rows, err := db.Instances().List(t.Context(), query)
		if err != nil {
			t.Fatal(err)
		}
		api := core.AgentInstanceAPI{
			Store:     db.Instances(),
			Authority: agentinstance.Authority{Callers: instanceAPIInputs{instance: rows[2]}},
		}
		operator, err := api.ListPage(t.Context(), query, workspaceaccess.ActorRef{Operator: true})
		if err != nil || len(operator.Instances) != 3 || operator.NextCursor != "" {
			t.Fatalf("operator=%+v error=%v", operator, err)
		}
		query.Limit = 1
		caller, err := api.ListPage(
			t.Context(),
			query,
			workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: "caller"},
		)
		if err != nil || len(caller.Instances) != 1 || caller.Instances[0].ID != rows[2].ID || caller.NextCursor != "" {
			t.Fatalf("caller=%+v error=%v", caller, err)
		}
	})
	t.Run("Should resolve Global instance mains for session facet exclusions", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "facets.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		const foreignProfile = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
		if _, err := db.DB().ExecContext(
			t.Context(),
			`INSERT INTO profiles (id, name, color, icon, state, created_at) VALUES (?, 'foreign', '#8E8EB5', 'circle', 'active', ?)`,
			foreignProfile,
			store.FormatTimestamp(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)),
		); err != nil {
			t.Fatal(err)
		}
		var callerInstance agentinstance.Instance
		for _, seed := range []struct{ name, profileID string }{
			{"attention", store.DefaultProfileID},
			{"quiet", store.DefaultProfileID},
			{"foreign", foreignProfile},
		} {
			instance, _, err := db.Instances().AdmitCreate(t.Context(), agentinstance.Instance{
				ProfileID: seed.profileID, AgentName: seed.name, PermissionCeiling: "{}", CreatedByKind: "human",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if seed.name == "quiet" {
				callerInstance = instance
			}
			if err := db.RegisterSession(t.Context(), new(store.SessionInfo{
				ProfileID: seed.profileID, ID: "main-" + seed.name,
				AgentName: seed.name, Provider: "claude", RuntimeStatus: store.SessionRuntimeUnbound,
				State: "active",
			})); err != nil {
				t.Fatal(err)
			}
			if _, err := db.DB().ExecContext(t.Context(),
				"UPDATE agent_instances SET main_session_id=? WHERE id=?", "main-"+seed.name, instance.ID); err != nil {
				t.Fatal(err)
			}
		}
		for _, tc := range []struct {
			name        string
			query       string
			flag        bool
			archived    bool
			agentCaller bool
			wantMainIDs []string
			attention   int
		}{
			{
				name: "Should exclude Global mains in User scope", query: "workspace_id=@user",
				flag: true, wantMainIDs: []string{"main-attention", "main-quiet"}, attention: 1,
			},
			{
				name: "Should exclude Global mains in All projects", query: "all_workspaces=true",
				flag: true, wantMainIDs: []string{"main-attention", "main-quiet"}, attention: 1,
			},
			{
				name:  "Should exclude Global mains in an explicit User aggregate",
				query: "all_workspaces=true&workspace_ids=@user", flag: true,
				wantMainIDs: []string{"main-attention", "main-quiet"}, attention: 1,
			},
			{
				name:  "Should exclude Global mains when All projects omits archived projects",
				query: "all_workspaces=true", flag: true, archived: true,
				wantMainIDs: []string{"main-attention", "main-quiet"}, attention: 1,
			},
			{
				name:  "Should keep Global instances outside a project-only aggregate",
				query: "all_workspaces=true&workspace_ids=ws-project", flag: true,
			},
			{
				name:  "Should preserve the default aggregate when the flag is omitted",
				query: "all_workspaces=true", attention: 1,
			},
			{
				name:  "Should preserve the narrowed aggregate when exclusion is false",
				query: "all_workspaces=true&workspace_ids=@user&exclude_instance_main=false",
			},
			{
				name:        "Should exclude in-scope mains even when a granted agent cannot list their instances",
				query:       "all_workspaces=true&all_profiles=true",
				flag:        true,
				agentCaller: true,
				wantMainIDs: []string{"main-attention", "main-foreign", "main-quiet"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				workspaces := testutil.StubWorkspaceService{}
				if tc.archived {
					workspaces.ListFn = func(context.Context) ([]workspacepkg.Workspace, error) {
						return []workspacepkg.Workspace{{ID: "ws-archived", ArchivedAt: new(time.Time{})}}, nil
					}
				}
				fixture := newHandlerFixture(t, &testutil.StubSessionManager{
					StatusFn: func(_ context.Context, id string) (*session.Info, error) {
						return &session.Info{
							ID: id, AgentName: callerInstance.AgentName, ProfileID: callerInstance.ProfileID,
							State: session.StateActive, EffectiveWorkspaceAccess: workspaceaccess.Unrestricted,
						}, nil
					},
					ListPageFn: func(_ context.Context, q session.ListQuery) (session.ListPage, error) {
						mainIDs := slices.Sorted(slices.Values(q.InstanceMainIDs))
						if !q.ExcludeInstanceMain || !slices.Equal(mainIDs, tc.wantMainIDs) {
							t.Fatalf("listed main exclusions: %+v", q)
						}
						return session.ListPage{Limit: 1}, nil
					},
					CatalogFacetsFn: func(_ context.Context, q session.ListQuery) (store.SessionCatalogFacetResult, error) {
						mainIDs := slices.Sorted(slices.Values(q.InstanceMainIDs))
						if q.ExcludeInstanceMain != tc.flag || !slices.Equal(mainIDs, tc.wantMainIDs) {
							t.Fatalf("facet main exclusions: %+v", q)
						}
						return store.SessionCatalogFacetResult{Facets: store.SessionCatalogFacets{All: 7}}, nil
					},
				}, testutil.StubObserver{}, workspaces)
				fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
					Store:     db.Instances(),
					Authority: agentinstance.Authority{Callers: instanceAPIInputs{instance: callerInstance}},
					ReadPayload: func(context.Context, agentinstance.Instance) (contract.AgentInstancePayload, error) {
						t.Fatal("facets hydrated the full instance payload")
						return contract.AgentInstancePayload{}, nil
					},
					ReadAttention: func(ctx context.Context, id string) (bool, error) {
						row, err := db.Instances().Get(ctx, id)
						return row.AgentName != callerInstance.AgentName, err
					},
				}
				if tc.agentCaller {
					fixture.Handlers.Profiles = &workspaceHintProfileServiceStub{
						ProfileService: sessionProfileServiceStub{},
						profiles: []profilepkg.WithCounts{
							{ID: store.DefaultProfileID, Name: "default", State: profilepkg.StateActive},
							{ID: foreignProfile, Name: "foreign", State: profilepkg.StateActive},
						},
					}
					fixture.Handlers.WorkspaceAccess = testutil.StubWorkspaceAccessPolicy{
						AuthorizeFn: func(context.Context, workspaceaccess.Request) (workspaceaccess.Decision, error) {
							return workspaceaccess.Decision{
								Allowed: true,
								Source:  workspaceaccess.SourceAgentGrant,
							}, nil
						},
					}
				}
				fixture.Engine.GET("/facets", fixture.Handlers.ListSessionFacets)
				fixture.Engine.GET("/list", fixture.Handlers.ListSessions)
				read := func(path string) *httptest.ResponseRecorder {
					t.Helper()
					request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
					if tc.agentCaller {
						request.Header.Set(agentidentity.HeaderSessionID, "main-quiet")
						request.Header.Set(agentidentity.HeaderAgent, callerInstance.AgentName)
					}
					response := httptest.NewRecorder()
					fixture.Engine.ServeHTTP(response, request)
					return response
				}
				query := tc.query
				if tc.flag {
					query += "&exclude_instance_main=true"
					readAttention := fixture.Handlers.AgentInstances.ReadAttention
					fixture.Handlers.AgentInstances.ReadAttention = func(context.Context, string) (bool, error) {
						return false, errors.New("instance attention unavailable")
					}
					listed := read("/list?" + query)
					fixture.Handlers.AgentInstances.ReadAttention = readAttention
					var page contract.SessionCatalogResponse
					decodeJSON(t, listed.Body.Bytes(), &page)
					if listed.Code != http.StatusOK || page.Page.Limit != 1 {
						t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
					}
				}
				response := read("/facets?" + query)
				var body contract.SessionCatalogFacetsResponse
				decodeJSON(t, response.Body.Bytes(), &body)
				if response.Code != http.StatusOK || body.Facets.All != 7 ||
					body.Facets.InstancesNeedingAttention != tc.attention ||
					len(body.ByWorkspace) != tc.attention || len(body.ByProfile) != tc.attention {
					t.Fatalf("facets: %d %+v", response.Code, body)
				}
				if tc.attention > 0 &&
					(body.ByWorkspace[0].WorkspaceID != "" ||
						body.ByWorkspace[0].Facets.InstancesNeedingAttention != 1 ||
						body.ByProfile[0].ProfileID != store.DefaultProfileID ||
						body.ByProfile[0].Facets.InstancesNeedingAttention != 1) {
					t.Fatalf("instance attention groups: %+v", body)
				}
			})
		}
	})
}

func TestAgentInstanceCheckIns(t *testing.T) {
	t.Run("Should return check-ins, a next cursor and one occurrence with proposals", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instance, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{AgentName: "coder", ProfileID: store.DefaultProfileID, CreatedByKind: "human", PermissionCeiling: "{}"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"event-a", "event-b"} {
			_, err := db.DB().
				ExecContext(t.Context(), `INSERT INTO agent_heartbeat_wake_events (id,workspace_id,agent_name,instance_id,source,result,reason,detail,created_at,expires_at) VALUES (?,'','coder',?,'timer','skipped','quiet','Bounded audit detail','2026-11-01T00:00:00Z','2026-11-02T00:00:00Z')`, id, instance.ID)
			if err != nil {
				t.Fatal(err)
			}
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store:     db.Instances(),
			Authority: agentinstance.Authority{Callers: db.Instances()},
		}
		fixture.Engine.GET("/instances/:id/check-ins", fixture.Handlers.ListAgentInstanceCheckIns)
		cursor := ""
		for _, want := range []string{"event-b", "event-a"} {
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(
				response,
				httptest.NewRequestWithContext(
					t.Context(),
					"GET",
					"/instances/"+instance.ID+"/check-ins?limit=1&cursor="+cursor,
					nil,
				),
			)
			var body contract.AgentInstanceCheckInsResponse
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != 200 || len(body.CheckIns) != 1 || body.CheckIns[0].ID != want ||
				body.CheckIns[0].Detail != "Bounded audit detail" {
				t.Fatalf("response = %d %s", response.Code, response.Body.String())
			}
			cursor = body.NextCursor
		}
		if cursor != "" {
			t.Fatalf("terminal cursor = %s", cursor)
		}
		if _, err := db.DB().ExecContext(
			t.Context(),
			`UPDATE agent_heartbeat_wake_events SET occurrence_id='occ-b' WHERE id='event-b'`,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().ExecContext(
			t.Context(),
			`INSERT INTO agent_instance_reports (id,instance_id,source_key,kind,payload,created_at) VALUES ('arpt-b',?,'occ:occ-b','proposal','{"proposals":[{"title":"a"},{"title":"b"}]}','2026-11-01T00:00:00Z')`,
			instance.ID,
		); err != nil {
			t.Fatal(err)
		}
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(
			response,
			httptest.NewRequestWithContext(
				t.Context(),
				"GET",
				"/instances/"+instance.ID+"/check-ins?occurrence=occ-b",
				nil,
			),
		)
		var body contract.AgentInstanceCheckInsResponse
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || len(body.CheckIns) != 1 || body.CheckIns[0].ID != "event-b" ||
			body.CheckIns[0].Proposals.Waiting != 2 {
			t.Fatalf("occurrence response = %d %s", response.Code, response.Body.String())
		}
	})
}

func TestAgentInstanceCheckProjection(t *testing.T) {
	t.Run("Should expose persisted check state and profile capacity across reads and updates", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "projection.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		repo := db.Instances()
		instance, _, err := repo.AdmitCreate(
			t.Context(),
			agentinstance.Instance{
				AgentName:         "coder",
				ProfileID:         store.DefaultProfileID,
				CreatedByKind:     "human",
				PermissionCeiling: "{}",
			},
			7,
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := repo.AdmitCreate(
			t.Context(),
			agentinstance.Instance{
				AgentName:         "other",
				ProfileID:         store.DefaultProfileID,
				CreatedByKind:     "human",
				PermissionCeiling: "{}",
			},
			7,
		); err != nil {
			t.Fatal(err)
		}
		const last = "2026-11-01T12:00:00Z"
		const next = "2026-11-01T13:00:00Z"
		if _, err := db.DB().
			ExecContext(t.Context(), `UPDATE agent_instance_wake_state SET last_wake_at=?, next_wake_at=?, quiet_streak=3, failure_streak=2 WHERE instance_id=?`, last, next, instance.ID); err != nil {
			t.Fatal(err)
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Service: &agentinstance.Service{}, Store: repo, WakeState: repo.WakeState, CountActive: repo.CountActive,
			MaxActiveInstances: 7, FatigueLimit: 4, CheckSettings: instanceCheckSettingsRepo{repo},
		}
		fixture.Engine.GET("/instances", fixture.Handlers.ListAgentInstances)
		fixture.Engine.GET("/instances/:id", fixture.Handlers.GetAgentInstance)
		fixture.Engine.PATCH("/instances/:id", fixture.Handlers.UpdateAgentInstance)
		assertPayload := func(payload contract.AgentInstancePayload, enabled bool) {
			t.Helper()
			if payload.ID != instance.ID || payload.ChecksEnabled != enabled || payload.LastWakeAt != last ||
				payload.NextWakeAt != next ||
				payload.QuietStreak != 3 ||
				payload.FailureStreak != 2 ||
				payload.FatigueLimit != 4 {
				t.Fatalf("unexpected check projection: %+v", payload)
			}
		}
		response := performRequest(t, fixture.Engine, "GET", "/instances?agent=coder&state=starting&limit=1", nil)
		if response.Code != 200 {
			t.Fatalf("list: %d %s", response.Code, response.Body.String())
		}
		var list contract.AgentInstanceListResponse
		decodeJSON(t, response.Body.Bytes(), &list)
		if len(list.Instances) != 1 || list.ActiveCount != 2 || list.MaxActiveInstances != 7 {
			t.Fatalf("filtered capacity: %+v", list)
		}
		assertPayload(list.Instances[0], true)
		for _, method := range []string{"GET", "PATCH", "GET"} {
			response := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(
				response,
				httptest.NewRequestWithContext(
					t.Context(),
					method,
					"/instances/"+instance.ID,
					strings.NewReader(`{"checks_enabled":false}`),
				),
			)
			if response.Code != 200 {
				t.Fatalf("%s: %d %s", method, response.Code, response.Body.String())
			}
			var body contract.AgentInstanceResponse
			decodeJSON(t, response.Body.Bytes(), &body)
			current, err := repo.Get(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			assertPayload(body.Instance, current.ChecksEnabled)
		}
		current, err := repo.Get(t.Context(), instance.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.ChecksEnabled {
			t.Fatal("checks remained enabled after PATCH")
		}
	})
}

type instanceCheckSettingsRepo struct{ repo *globaldb.InstanceRepo }

var _ core.AgentInstanceCheckSettings = instanceCheckSettingsRepo{}

func (r instanceCheckSettingsRepo) SetChecks(
	ctx context.Context,
	id string,
	enabled bool,
) (agentinstance.Instance, error) {
	return r.repo.SetChecks(ctx, id, enabled, time.Hour)
}

func TestAgentInstanceApprovalAdmission(t *testing.T) {
	for _, mode := range []string{"operator", "approve-all", "approve-reads", "deny-all"} {
		t.Run("Should enforce instance approval for "+mode, func(t *testing.T) {
			t.Parallel()
			db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "approval.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(context.Background()); err != nil {
					t.Error(err)
				}
			})
			repo := db.Instances()
			instance, _, err := repo.AdmitCreate(
				t.Context(),
				agentinstance.Instance{
					AgentName:         "coder",
					ProfileID:         store.DefaultProfileID,
					CreatedByKind:     "human",
					PermissionCeiling: "{}",
				},
				10,
			)
			if err != nil {
				t.Fatal(err)
			}
			boundary := instanceAPIInputs{instance: instance}
			service, err := agentinstance.NewService(
				agentinstance.WithStore(repo),
				agentinstance.WithAdmission(boundary),
				agentinstance.WithMainSessions(boundary),
				agentinstance.WithActiveLimit(10),
			)
			if err != nil {
				t.Fatal(err)
			}
			fixture := newHandlerFixture(
				t,
				&testutil.StubSessionManager{StatusFn: func(context.Context, string) (*session.Info, error) {
					return &session.Info{
						ID:                   "caller",
						AgentName:            "coder",
						ProfileID:            store.DefaultProfileID,
						State:                session.StateActive,
						EffectivePermissions: mode,
					}, nil
				}},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
			)
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
				Service:   service,
				Store:     repo,
				Authority: agentinstance.Authority{Callers: boundary},
			}
			fixture.Handlers.WorkspaceAccess = testutil.StubWorkspaceAccessPolicy{
				AuthorizeFn: func(context.Context, workspaceaccess.Request) (workspaceaccess.Decision, error) {
					return workspaceaccess.Decision{Allowed: true}, nil
				},
			}
			fixture.Engine.POST("/instances", fixture.Handlers.CreateAgentInstance)
			fixture.Engine.DELETE("/instances/:id", fixture.Handlers.ArchiveAgentInstance)
			allowed := mode == "operator" || mode == "approve-all"
			for _, operation := range []string{"create", "archive"} {
				method, path := "POST", "/instances"
				if operation == "archive" {
					method, path = "DELETE", path+"/"+instance.ID
				}
				req := httptest.NewRequestWithContext(
					t.Context(),
					method,
					path,
					strings.NewReader(`{"agent":"coder","workspace":"@user"}`),
				)
				if mode != "operator" {
					req.Header.Set(agentidentity.HeaderSessionID, "caller")
					req.Header.Set(agentidentity.HeaderAgent, "coder")
				}
				response := httptest.NewRecorder()
				fixture.Engine.ServeHTTP(response, req)
				switch {
				case !allowed:
					var body contract.ErrorPayload
					decodeJSON(t, response.Body.Bytes(), &body)
					if response.Code != 403 || body.Code != "tool_approval_required" ||
						!strings.Contains(body.Error, "compozy__agent_instance_"+operation) {
						t.Fatalf("denial: %d %s", response.Code, response.Body.String())
					}
				case operation == "create":
					var body contract.ErrorPayload
					decodeJSON(t, response.Body.Bytes(), &body)
					if response.Code != 422 || body.Code != agentinstance.ErrNotAlwaysOn.Error() {
						t.Fatalf("definition admission: %d %s", response.Code, response.Body.String())
					}
				case response.Code != 204:
					t.Fatalf("archive: %d %s", response.Code, response.Body.String())
				}
			}
			current, err := repo.Get(t.Context(), instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if (current.ArchivedAt != nil) != allowed {
				t.Fatalf("archive state: %+v", current)
			}
		})
	}
}

type instanceAPIInputs struct {
	db *globaldb.GlobalDB
	agentinstance.Admission
	instance agentinstance.Instance
}

func (i instanceAPIInputs) ResolveCreate(
	_ context.Context,
	req agentinstance.CreateRequest,
) (agentinstance.Instance, error) {
	if i.db != nil {
		return agentinstance.Instance{
			AgentName:         req.Agent,
			ProfileID:         req.Profile,
			WorkspaceID:       req.Workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}, nil
	}
	return agentinstance.Instance{}, agentinstance.ErrNotAlwaysOn
}
func (i instanceAPIInputs) Start(ctx context.Context, req agentinstance.StartMainSession) (string, error) {
	if i.db != nil {
		info := store.SessionInfo{
			ID:            "sess-api-created-main",
			ProfileID:     req.Instance.ProfileID,
			AgentName:     req.Instance.AgentName,
			State:         "starting",
			RuntimeStatus: store.SessionRuntimeUnbound,
			SessionType:   "user",
		}
		_, err := i.db.RegisterInstanceSession(ctx, &(info), req.Instance.ID, nil)
		return info.ID, err
	}
	return "", errors.New("unexpected session start")
}
func (i instanceAPIInputs) Stop(context.Context, string) error {
	return errors.New("unexpected session stop")
}
func (i instanceAPIInputs) CheckMain(context.Context, string) error { return nil }
func (i instanceAPIInputs) InstanceCaller(context.Context, string) (agentinstance.Caller, error) {
	return agentinstance.Caller{
		InstanceID: i.instance.ID,
		AgentName:  i.instance.AgentName,
		ProfileID:  i.instance.ProfileID,
	}, nil
}

var _ agentinstance.Admission = instanceAPIInputs{}
var _ agentinstance.MainSessionStarter = instanceAPIInputs{}
var _ agentinstance.CallerReader = instanceAPIInputs{}

func TestAgentInstanceCreateResult(t *testing.T) {
	t.Run("Should distinguish new creation from an idempotent repeat", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "create.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		boundary := instanceAPIInputs{db: db}
		service, err := agentinstance.NewService(
			agentinstance.WithStore(db.Instances()),
			agentinstance.WithAdmission(boundary),
			agentinstance.WithMainSessions(boundary),
			agentinstance.WithActiveLimit(1),
		)
		if err != nil {
			t.Fatal(err)
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Service:   service,
			Store:     db.Instances(),
			WakeState: db.Instances().WakeState,
		}
		fixture.Engine.POST("/instances", fixture.Handlers.CreateAgentInstance)
		id := ""
		for attempt, status := range []int{201, 200} {
			response := performRequest(
				t,
				fixture.Engine,
				"POST",
				"/instances",
				[]byte(`{"agent":"coder","workspace":"@user"}`),
			)
			if response.Code != status {
				t.Fatalf("create: %d %s", response.Code, response.Body.String())
			}
			var body contract.AgentInstanceResponse
			decodeJSON(t, response.Body.Bytes(), &body)
			if body.Instance.Created != (attempt == 0) || body.Instance.MainSessionID != "sess-api-created-main" ||
				body.Instance.State != "active" {
				t.Fatalf("create response: %+v", body)
			}
			if attempt == 0 {
				id = body.Instance.ID
			} else if body.Instance.ID != id {
				t.Fatalf("repeat changed identity: %s / %s", id, body.Instance.ID)
			}
			var envelope map[string]map[string]any
			decodeJSON(t, response.Body.Bytes(), &envelope)
			if _, exists := envelope["instance"]["created"]; !exists {
				t.Fatal("created flag omitted")
			}
		}
		var count int
		if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM sessions").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("main sessions = %d", count)
		}
	})
}

// Invariant: the instance read chooses the earliest timed wake and preserves untimed monitors.
func TestAgentInstanceNextWake(t *testing.T) {
	for _, tc := range []struct{ name, monitorAt, checkAt, want, reason string }{
		{"Should prefer an earlier timer", "2026-11-03T10:00:00Z", "2026-11-03T11:00:00Z", "timer", ""},
		{"Should prefer an earlier check", "2026-11-03T12:00:00Z", "2026-11-03T11:00:00Z", "check", ""},
		{"Should preserve an untimed monitor without a check", "", "", "timer", ""},
		{"Should explain a deferred configuration rejection", "", "2026-11-03T11:00:00Z", "check", heartbeat.ReasonTriageConfigUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			api := core.AgentInstanceAPI{
				ReadPayload: func(context.Context, agentinstance.Instance) (contract.AgentInstancePayload, error) {
					return contract.AgentInstancePayload{
						NextWake: &contract.AgentInstanceNextWake{
							Kind:        "timer",
							At:          tc.monitorAt,
							Description: "follow up",
						},
					}, nil
				},
				WakeState: func(context.Context, string) (heartbeat.InstanceWakeState, error) {
					return heartbeat.InstanceWakeState{NextWakeAt: tc.checkAt, LastReason: tc.reason}, nil
				},
			}
			payload, err := api.Payload(t.Context(), agentinstance.Instance{State: "active", ChecksEnabled: true})
			if err != nil {
				t.Fatal(err)
			}
			if tc.reason != "" &&
				(payload.NextWake == nil || !strings.Contains(payload.NextWake.Description, "agent_instance_check")) {
				t.Fatalf("missing manual recovery: %+v", payload.NextWake)
			}
			if payload.NextWake == nil || payload.NextWake.Kind != tc.want {
				t.Fatalf("next wake = %+v", payload.NextWake)
			}
		})
	}
}

type failingInstanceOwners struct{}

func (failingInstanceOwners) ListIdentities(context.Context) ([]profilepkg.Profile, error) {
	return nil, errors.New("profile store unavailable")
}

// Invariant: the owner label is best-effort; a failed profile lookup never fails the payload of
// a committed mutation or a read. Owning layer: shared core payload hydration.
func TestAgentInstancePayloadShouldKeepTheRowWhenTheOwnerLabelLookupFails(t *testing.T) {
	t.Parallel()
	api := core.AgentInstanceAPI{Profiles: failingInstanceOwners{}}
	payload, err := api.Payload(
		t.Context(),
		agentinstance.Instance{ID: "ainst-a", ProfileID: "profile-b", State: "paused"},
	)
	if err != nil {
		t.Fatalf("payload error = %v, want the committed row", err)
	}
	if payload.ID != "ainst-a" || payload.ProfileID != "profile-b" || payload.ProfileName != "" {
		t.Fatalf("payload = %+v, want the row without an owner label", payload)
	}
}

// Invariant: review pages are scoped before pagination; only operators can decide exact contents.
// Owning layer: shared HTTP/UDS core; canonical suite: TestAgentSkillReviews.
func TestAgentSkillReviews(t *testing.T) {
	t.Run("Should bind pages to their scope and require operator decisions [UT-045]", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "reviews.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		repo := db.Instances()
		instance, _, err := repo.AdmitCreate(
			t.Context(),
			agentinstance.Instance{
				AgentName:         "coder",
				ProfileID:         store.DefaultProfileID,
				CreatedByKind:     "human",
				PermissionCeiling: "{}",
			},
			10,
		)
		if err != nil {
			t.Fatal(err)
		}
		boundary := instanceAPIInputs{instance: instance}
		authority := agentinstance.Authority{Callers: boundary}
		scanRoot := filepath.Join(t.TempDir(), "skills")
		service := &agentinstance.SkillProposalService{
			Store:     db.SkillReviews(),
			Evidence:  skillEvidenceInProfile{},
			Authority: authority,
			DraftRoot: t.TempDir(),
			ScanRoots: []string{scanRoot},
		}
		lesson := agentinstance.SkillLesson{
			Cue:        "Tagging a release",
			Lesson:     "Check that CHANGELOG.md has the new version before tagging.",
			Evidence:   "Session sess-4f1c failed at the release job.",
			Confidence: "high",
		}
		var reviews []agentinstance.SkillReview
		for _, name := range []string{"first", "second"} {
			result, err := service.Propose(
				t.Context(),
				workspaceaccess.ActorRef{Operator: true},
				instance,
				scanRoot,
				agentinstance.SkillProposalInput{
					Name:        name,
					Description: "Review changes",
					Body:        "Read the diff.",
					Files: []agentinstance.SkillDraftFile{
						{Path: "scripts/read.sh", Content: "echo reviewed"},
					},
					Lesson:             lesson,
					EvidenceSessionIDs: []string{"sess-4f1c0b2a9d3e4f51", "sess-9a2e7c1d0b4f3a62"},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			reviews = append(reviews, result.Review)
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{StatusFn: func(context.Context, string) (*session.Info, error) {
				return &session.Info{
					ID:                   "caller",
					AgentName:            "coder",
					ProfileID:            store.DefaultProfileID,
					State:                session.StateActive,
					EffectivePermissions: "approve-all",
				}, nil
			}},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store:          repo,
			Authority:      authority,
			SkillProposals: service,
			SkillTarget: func(context.Context, agentinstance.Instance) (string, []string, error) {
				return scanRoot, []string{scanRoot}, nil
			},
		}
		fixture.Handlers.WorkspaceAccess = testutil.StubWorkspaceAccessPolicy{
			AuthorizeFn: func(context.Context, workspaceaccess.Request) (workspaceaccess.Decision, error) {
				return workspaceaccess.Decision{Allowed: true}, nil
			},
		}
		fixture.Engine.GET("/reviews", fixture.Handlers.ListAgentSkillReviews)
		fixture.Engine.GET("/reviews/:id", fixture.Handlers.GetAgentSkillReview)
		fixture.Engine.POST("/reviews/:id/approve", fixture.Handlers.ApproveAgentSkillReview)
		fixture.Engine.POST("/reviews/:id/reject", fixture.Handlers.RejectAgentSkillReview)
		request := func(method, path, body string, agent bool) *httptest.ResponseRecorder {
			t.Helper()
			req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
			if agent {
				req.Header.Set(agentidentity.HeaderSessionID, "caller")
				req.Header.Set(agentidentity.HeaderAgent, "coder")
			}
			recorder := httptest.NewRecorder()
			fixture.Engine.ServeHTTP(recorder, req)
			return recorder
		}
		first := request("GET", "/reviews?limit=1", "", true)
		if first.Code != 200 {
			t.Fatal(first.Code, first.Body.String())
		}
		var page contract.AgentSkillReviewsResponse
		decodeJSON(t, first.Body.Bytes(), &page)
		if len(page.Reviews) != 1 || page.NextCursor == "" || len(page.Reviews[0].Files) != 0 ||
			page.Reviews[0].RunsAutomatically.Scripts != 1 {
			t.Fatalf("first page: %+v", page)
		}
		next := request("GET", "/reviews?limit=1&cursor="+page.NextCursor, "", true)
		var second contract.AgentSkillReviewsResponse
		decodeJSON(t, next.Body.Bytes(), &second)
		if next.Code != 200 || len(second.Reviews) != 1 || second.Reviews[0].ID == page.Reviews[0].ID ||
			second.NextCursor != "" {
			t.Fatalf("next page: %d %+v", next.Code, second)
		}
		// IT-017: a cursor is bound to every filter it was issued for.
		for _, filter := range []string{"state=pending", "kind=edit", "decided_via=operator"} {
			mismatch := request("GET", "/reviews?"+filter+"&cursor="+page.NextCursor, "", true)
			var failure contract.ErrorPayload
			decodeJSON(t, mismatch.Body.Bytes(), &failure)
			if mismatch.Code != 422 || failure.Code != "invalid_cursor" {
				t.Fatalf("changed filter %s: %d %s", filter, mismatch.Code, mismatch.Body.String())
			}
		}
		// The review detail returns the lesson and the evidence sessions.
		detail := request("GET", "/reviews/"+reviews[0].ID, "", true)
		var shown contract.AgentSkillReviewResponse
		decodeJSON(t, detail.Body.Bytes(), &shown)
		if detail.Code != 200 || shown.Review.Kind != "new" || shown.Review.Lesson == nil ||
			*shown.Review.Lesson != (contract.AgentSkillLesson{
				Cue: lesson.Cue, Lesson: lesson.Lesson, Evidence: lesson.Evidence, Confidence: "high",
			}) || strings.Join(shown.Review.EvidenceSessionIDs, ",") != "sess-4f1c0b2a9d3e4f51,sess-9a2e7c1d0b4f3a62" {
			t.Fatalf("lesson detail: %d %s", detail.Code, detail.Body.String())
		}
		for _, action := range []string{"approve", "reject"} {
			denial := request("POST", "/reviews/"+reviews[0].ID+"/"+action, `{"digest":"`+reviews[0].Digest+`"}`, true)
			var failure contract.ErrorPayload
			decodeJSON(t, denial.Body.Bytes(), &failure)
			if denial.Code != 403 || failure.Code != "skill_review_operator_only" {
				t.Fatalf("agent decision: %d %+v", denial.Code, failure)
			}
		}
		stale := request("POST", "/reviews/"+reviews[0].ID+"/approve", `{"digest":"old"}`, false)
		if stale.Code != 409 {
			t.Fatalf("stale: %d %s", stale.Code, stale.Body.String())
		}
		accepted := request("POST", "/reviews/"+reviews[0].ID+"/approve", `{"digest":"`+reviews[0].Digest+`"}`, false)
		var decision contract.AgentSkillReviewResponse
		decodeJSON(t, accepted.Body.Bytes(), &decision)
		if accepted.Code != 200 || decision.Review.State != "approved" || len(decision.Review.Files) != 2 ||
			decision.Review.DecidedVia != "operator" {
			t.Fatalf("approval: %d %+v", accepted.Code, decision)
		}
		installed, err := os.ReadFile(filepath.Join(scanRoot, "first", "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(installed) != decision.Review.Files[0].Content {
			t.Fatal("installed bytes differ from review")
		}

		// An edit review details its changes and base; IT-004: approving it after the installed
		// skill changed on disk is refused with base_stale, nothing installed, and Reject still works.
		edit, err := service.Propose(t.Context(), workspaceaccess.ActorRef{Operator: true}, instance, scanRoot,
			agentinstance.SkillProposalInput{
				Name: "first", Description: "Review changes", Body: "Read the diff twice.",
				Mode: agentinstance.SkillProposalEdit, Lesson: lesson,
				EvidenceSessionIDs: []string{"sess-4f1c0b2a9d3e4f51"},
			})
		if err != nil {
			t.Fatal(err)
		}
		editDetail := request("GET", "/reviews/"+edit.Review.ID, "", true)
		var editShown contract.AgentSkillReviewResponse
		decodeJSON(t, editDetail.Body.Bytes(), &editShown)
		if editDetail.Code != 200 || editShown.Review.Kind != "edit" || editShown.Review.BaseDigest == "" ||
			len(editShown.Review.Changes) != 2 || editShown.Review.Changes[0] != (contract.AgentSkillFileChange{
			Path: "SKILL.md", Change: "modified",
		}) || editShown.Review.Changes[1].Change != "unchanged" || len(editShown.Review.BaseFiles) != 2 ||
			editShown.Review.BaseRunsAutomatically == nil || editShown.Review.BaseRunsAutomatically.Scripts != 1 ||
			editShown.Review.RunsAutomatically.Scripts != 1 {
			t.Fatalf("edit detail: %d %s", editDetail.Code, editDetail.Body.String())
		}
		if err := os.WriteFile(filepath.Join(scanRoot, "first", "notes.md"), []byte("operator"), 0o600); err != nil {
			t.Fatal(err)
		}
		editDigest := `{"digest":"` + edit.Review.Digest + `"}`
		staleBase := request("POST", "/reviews/"+edit.Review.ID+"/approve", editDigest, false)
		var staleBaseFailure contract.ErrorPayload
		decodeJSON(t, staleBase.Body.Bytes(), &staleBaseFailure)
		if staleBase.Code != 409 || staleBaseFailure.Code != "skill_review_base_stale" {
			t.Fatalf("stale base approval: %d %s", staleBase.Code, staleBase.Body.String())
		}
		kept, err := os.ReadFile(filepath.Join(scanRoot, "first", "SKILL.md"))
		if err != nil || string(kept) != string(installed) {
			t.Fatalf("stale edit changed the skill: %v", err)
		}
		rejectEdit := request("POST", "/reviews/"+edit.Review.ID+"/reject", editDigest, false)
		if rejectEdit.Code != 200 {
			t.Fatalf("reject stale edit: %d %s", rejectEdit.Code, rejectEdit.Body.String())
		}

		// A draft whose digest still matches but cannot be installed is invalid, not stale, and
		// stays rejectable.
		mismatched, err := agentinstance.PrepareSkillDraft("renamed", "Review changes", "Read the diff.", nil)
		if err != nil {
			t.Fatal(err)
		}
		draftPath, draftDigest, err := agentinstance.WriteSkillDraft(t.TempDir(), []string{scanRoot}, mismatched)
		if err != nil {
			t.Fatal(err)
		}
		invalid, _, err := db.SkillReviews().CreateReview(t.Context(), agentinstance.SkillReview{
			ID:          "skr-invalid",
			InstanceID:  instance.ID,
			ProfileID:   instance.ProfileID,
			WorkspaceID: instance.WorkspaceID,
			AgentName:   instance.AgentName,
			SkillName:   "expected",
			Digest:      draftDigest,
			DraftPath:   draftPath,
		}, agentinstance.ReportInput{
			SourceKey: "skill_review:skr-invalid",
			Kind:      agentinstance.ReportKindProposal,
			Payload:   []byte(`{"skill_review_id":"skr-invalid","proposals":[{"title":"Review","body":"Review it."}]}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		uninstallable := request("POST", "/reviews/"+invalid.ID+"/approve", `{"digest":"`+invalid.Digest+`"}`, false)
		var invalidFailure contract.ErrorPayload
		decodeJSON(t, uninstallable.Body.Bytes(), &invalidFailure)
		if uninstallable.Code != 422 || invalidFailure.Code != "skill_review_invalid" {
			t.Fatalf("uninstallable approval: %d %+v", uninstallable.Code, invalidFailure)
		}
		if _, err := os.Stat(filepath.Join(scanRoot, "expected")); !os.IsNotExist(err) {
			t.Fatalf("uninstallable draft installed: %v", err)
		}
		rejected := request("POST", "/reviews/"+invalid.ID+"/reject", `{"digest":"`+invalid.Digest+`"}`, false)
		if rejected.Code != 200 {
			t.Fatalf("reject uninstallable: %d %s", rejected.Code, rejected.Body.String())
		}

		// UT-081 over HTTP: a `new` review whose name was created meanwhile is refused unchanged.
		existing := filepath.Join(scanRoot, "second")
		if err := os.MkdirAll(existing, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(existing, "SKILL.md"), []byte("operator skill"), 0o600); err != nil {
			t.Fatal(err)
		}
		taken := request("POST", "/reviews/"+reviews[1].ID+"/approve", `{"digest":"`+reviews[1].Digest+`"}`, false)
		var takenFailure contract.ErrorPayload
		decodeJSON(t, taken.Body.Bytes(), &takenFailure)
		if taken.Code != 409 || takenFailure.Code != "skill_review_target_exists" {
			t.Fatalf("approval over an existing skill: %d %+v", taken.Code, takenFailure)
		}
		if kept, err := os.ReadFile(filepath.Join(existing, "SKILL.md")); err != nil || string(kept) != "operator skill" {
			t.Fatalf("existing skill changed: %q %v", kept, err)
		}

		// Stored draft bytes that no longer verify still read as a review, marked invalid, and reject.
		if err := os.WriteFile(reviews[1].DraftPath, []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
		listed := request("GET", "/reviews?state=pending", "", true)
		var pending contract.AgentSkillReviewsResponse
		decodeJSON(t, listed.Body.Bytes(), &pending)
		if listed.Code != 200 || len(pending.Reviews) != 1 || pending.Reviews[0].Problem != "skill_review_invalid" {
			t.Fatalf("list with a damaged draft: %d %+v", listed.Code, pending)
		}
		read := request("GET", "/reviews/"+reviews[1].ID, "", true)
		var damaged contract.AgentSkillReviewResponse
		decodeJSON(t, read.Body.Bytes(), &damaged)
		if read.Code != 200 || damaged.Review.Problem != "skill_review_invalid" || len(damaged.Review.Files) != 0 {
			t.Fatalf("read damaged draft: %d %+v", read.Code, damaged)
		}
		dropped := request("POST", "/reviews/"+reviews[1].ID+"/reject", `{"digest":"`+reviews[1].Digest+`"}`, false)
		var droppedReview contract.AgentSkillReviewResponse
		decodeJSON(t, dropped.Body.Bytes(), &droppedReview)
		if dropped.Code != 200 || droppedReview.Review.State != "rejected" {
			t.Fatalf("reject damaged draft: %d %s", dropped.Code, dropped.Body.String())
		}
		durable := request("GET", "/reviews/"+reviews[1].ID, "", true)
		var settled contract.AgentSkillReviewResponse
		decodeJSON(t, durable.Body.Bytes(), &settled)
		if durable.Code != 200 || settled.Review.State != "rejected" {
			t.Fatalf("damaged draft rejection not durable: %d %+v", durable.Code, settled)
		}

		// A review without a lesson reads with no lesson and an empty evidence list.
		legacy := request("GET", "/reviews/"+invalid.ID, "", true)
		var legacyReview contract.AgentSkillReviewResponse
		decodeJSON(t, legacy.Body.Bytes(), &legacyReview)
		if legacy.Code != 200 || legacyReview.Review.Lesson != nil || legacyReview.Review.EvidenceSessionIDs == nil ||
			len(legacyReview.Review.EvidenceSessionIDs) != 0 || legacyReview.Review.Kind != "new" {
			t.Fatalf("legacy review: %d %s", legacy.Code, legacy.Body.String())
		}

		// A deleted draft reads and rejects the same way.
		deletedResult, err := service.Propose(
			t.Context(),
			workspaceaccess.ActorRef{Operator: true},
			instance,
			scanRoot,
			agentinstance.SkillProposalInput{
				Name: "third", Description: "Review changes", Body: "Read the diff.",
				Lesson: lesson, EvidenceSessionIDs: []string{"sess-4f1c0b2a9d3e4f51"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		deleted := deletedResult.Review
		if err := os.Remove(deleted.DraftPath); err != nil {
			t.Fatal(err)
		}
		missing := request("GET", "/reviews/"+deleted.ID, "", true)
		var missingReview contract.AgentSkillReviewResponse
		decodeJSON(t, missing.Body.Bytes(), &missingReview)
		if missing.Code != 200 || missingReview.Review.Problem != "skill_review_invalid" {
			t.Fatalf("read deleted draft: %d %s", missing.Code, missing.Body.String())
		}
		gone := request("POST", "/reviews/"+deleted.ID+"/reject", `{"digest":"`+deleted.Digest+`"}`, false)
		var goneReview contract.AgentSkillReviewResponse
		decodeJSON(t, gone.Body.Bytes(), &goneReview)
		if gone.Code != 200 || goneReview.Review.State != "rejected" {
			t.Fatalf("reject deleted draft: %d %s", gone.Code, gone.Body.String())
		}
		retried := request("POST", "/reviews/"+deleted.ID+"/reject", `{"digest":"`+deleted.Digest+`"}`, false)
		if retried.Code != 409 {
			t.Fatalf("a repeated reject must report the settled state: %d %s", retried.Code, retried.Body.String())
		}

		// IT-009: decided_via pages with its own cursor and never mixes operator decisions in.
		for _, id := range []string{"skr-auto-1", "skr-auto-2", "skr-auto-3"} {
			if _, err := db.DB().ExecContext(t.Context(), `INSERT INTO agent_skill_reviews
 (id,instance_id,profile_id,workspace_id,agent_name,skill_name,digest,state,draft_path,created_at,decided_at,decided_by,decided_via)
 VALUES (?,?,?,?,'coder',?,'sha256:auto','approved','/missing','2026-11-05T10:00:00.000000000Z','2026-11-05T10:00:00.000000000Z','auto-approve','auto_approve')`,
				id, instance.ID, instance.ProfileID, instance.WorkspaceID, id); err != nil {
				t.Fatal(err)
			}
		}
		var autoIDs []string
		cursor := ""
		for range 4 {
			path := "/reviews?decided_via=auto_approve&limit=2"
			if cursor != "" {
				path += "&cursor=" + cursor
			}
			listed := request("GET", path, "", false)
			var autoPage contract.AgentSkillReviewsResponse
			decodeJSON(t, listed.Body.Bytes(), &autoPage)
			if listed.Code != 200 {
				t.Fatalf("auto-approve page: %d %s", listed.Code, listed.Body.String())
			}
			for _, review := range autoPage.Reviews {
				if review.DecidedVia != "auto_approve" {
					t.Fatalf("decided_via filter leaked %+v", review)
				}
				autoIDs = append(autoIDs, review.ID)
			}
			if cursor = autoPage.NextCursor; cursor == "" {
				break
			}
		}
		if strings.Join(autoIDs, ",") != "skr-auto-1,skr-auto-2,skr-auto-3" {
			t.Fatalf("auto-approve pages = %v", autoIDs)
		}
	})
}

// skillEvidenceInProfile accepts every evidence session as owned by the default profile.
type skillEvidenceInProfile struct{}

func (skillEvidenceInProfile) SessionScope(context.Context, string) (string, string, error) {
	return store.DefaultProfileID, "", nil
}

// Invariant UT-064: inaccessible peers never occupy pages or expose private instance details.
func TestAgentInstancePeers(t *testing.T) {
	t.Run("Should filter before pagination and bind cursors to caller and scope", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "peers.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		for _, id := range []string{"allowed", "denied", "empty"} {
			now := time.Now()
			if err := db.InsertWorkspace(
				t.Context(),
				workspacepkg.Workspace{
					ID:        id,
					Name:      id,
					RootDir:   filepath.Join(t.TempDir(), id),
					CreatedAt: now,
					UpdatedAt: now,
				},
			); err != nil {
				t.Fatal(err)
			}
		}

		repo := db.Instances()
		var withheldIDs []string
		for i, workspace := range []string{"denied", "allowed", "denied", "allowed"} {
			instance, _, err := repo.AdmitCreate(
				t.Context(),
				agentinstance.Instance{
					ProfileID:         store.DefaultProfileID,
					WorkspaceID:       workspace,
					AgentName:         fmt.Sprintf("agent-%d", i),
					PermissionCeiling: "{}",
					CreatedByKind:     "human",
				},
				10,
			)
			if err != nil {
				t.Fatal(err)
			}
			if workspace == "denied" {
				withheldIDs = append(withheldIDs, instance.ID)
			}
		}
		api := &core.AgentInstanceAPI{
			Store: repo,
			Authority: agentinstance.Authority{
				Callers: instanceAPIInputs{
					instance: agentinstance.Instance{ID: "caller-instance", ProfileID: store.DefaultProfileID},
				},
			},
		}
		actor := workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorAgentSession,
			SessionID:   "caller",
			WorkspaceID: "allowed",
		}
		query := agentinstance.ListQuery{ProfileID: store.DefaultProfileID, AllWorkspaces: true, Limit: 1}
		first, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}, "")
		if err != nil || len(first.Peers) != 1 || first.Peers[0].Scope != "allowed" || first.NextCursor == "" ||
			len(first.Instances) != 0 || first.PeersNote != "" {
			t.Fatalf("first peers: %+v %v", first, err)
		}
		raw, err := json.Marshal(first.Peers[0])
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 4 || fields["name"] == nil || fields["description"] == nil ||
			fields["scope"] == nil ||
			fields["state"] == nil {
			t.Fatalf("private peer fields: %s", raw)
		}
		t.Run(
			"Should distinguish a filtered workspace from an empty one without exposing hidden peers",
			func(t *testing.T) {
				t.Parallel()
				for _, tc := range []struct {
					name, workspace, agent, state, callerWorkspace string
					wantNote                                       bool
				}{
					{name: "Should report when access hides every candidate", workspace: "denied", callerWorkspace: "denied", wantNote: true},
					{name: "Should keep a genuinely empty workspace empty", workspace: "empty", callerWorkspace: "empty"},
					{name: "Should conceal a targeted workspace", workspace: "denied", callerWorkspace: "allowed"},
					{name: "Should conceal a targeted agent", workspace: "denied", callerWorkspace: "denied", agent: "agent-0"},
					{name: "Should conceal a targeted state", workspace: "denied", callerWorkspace: "denied", state: "active"},
				} {
					t.Run(tc.name, func(t *testing.T) {
						t.Parallel()
						query := agentinstance.ListQuery{
							ProfileID: store.DefaultProfileID, WorkspaceID: tc.workspace,
							Limit: 1, AgentName: tc.agent, State: tc.state,
						}
						actor := workspaceaccess.ActorRef{
							Kind:        workspaceaccess.ActorAgentSession,
							SessionID:   "caller",
							WorkspaceID: tc.callerWorkspace,
						}
						page, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}, "")
						if err != nil || len(page.Peers) != 0 || len(page.Instances) != 0 || page.NextCursor != "" {
							t.Fatalf("empty peers: %+v %v", page, err)
						}
						body, err := json.Marshal(page)
						if err != nil {
							t.Fatal(err)
						}
						var envelope map[string]any
						if err := json.Unmarshal(body, &envelope); err != nil {
							t.Fatal(err)
						}
						if note, present := envelope["peers_note"]; present != tc.wantNote ||
							(tc.wantNote && note != "Access restrictions hid every peer candidate.") {
							t.Fatalf("unexpected peers diagnostic: %s", body)
						}
						for _, secret := range append([]string{"agent-0", "agent-2", "denied"}, withheldIDs...) {
							if strings.Contains(string(body), secret) {
								t.Fatalf("peer result exposes %q: %s", secret, body)
							}
						}
					})
				}
			},
		)
		query.AfterID = first.NextCursor
		next, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}, "")
		if err != nil || len(next.Peers) != 1 || next.Peers[0].Name == first.Peers[0].Name || next.NextCursor != "" {
			t.Fatalf("next peers: %+v %v", next, err)
		}
		query.State = "paused"
		if _, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}, ""); err == nil {
			t.Fatal("cursor crossed filters")
		}
		query.State = ""
		actor.SessionID = "another"
		if _, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}, ""); err == nil {
			t.Fatal("cursor crossed callers")
		}
	})
}

type instanceProfileOwners []profilepkg.Profile

func (owners instanceProfileOwners) ListIdentities(context.Context) ([]profilepkg.Profile, error) {
	return owners, nil
}

func TestAgentInstanceAllProfilesReads(t *testing.T) {
	// Invariant: all_profiles is one owner-labeled read on instance lists and details, and a
	// typed selection conflict on reads that belong to one profile.
	// Owning layer/canonical suite: instance HTTP handlers, TestAgentInstanceAllProfilesReads.
	db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "all-profiles.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	foreign := "ffffffffffffffffffffffffff"
	if _, err := db.DB().ExecContext(t.Context(),
		`INSERT INTO profiles(id,name,color,icon,state,created_at) VALUES (?, 'foreign', '#8E8EB5', 'circle', 'active', ?)`,
		foreign, store.FormatTimestamp(time.Now())); err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for profileID, agent := range map[string]string{store.DefaultProfileID: "own", foreign: "theirs"} {
		instance, _, err := db.Instances().AdmitCreate(t.Context(), agentinstance.Instance{
			ProfileID: profileID, AgentName: agent, PermissionCeiling: "{}", CreatedByKind: "human",
		}, 10)
		if err != nil {
			t.Fatal(err)
		}
		ids[agent] = instance.ID
	}
	fixture := newHandlerFixture(
		t,
		&testutil.StubSessionManager{},
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
	)
	fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
		Service:            &agentinstance.Service{},
		Store:              db.Instances(),
		CountActive:        db.Instances().CountActive,
		MaxActiveInstances: 3,
		SkillProposals:     &agentinstance.SkillProposalService{},
		Profiles: instanceProfileOwners{
			{ID: store.DefaultProfileID, Name: "default"}, {ID: foreign, Name: "foreign"},
		},
	}
	fixture.Engine.GET("/api/agent-instances", fixture.Handlers.ListAgentInstances)
	fixture.Engine.GET("/api/agent-instances/:id", fixture.Handlers.GetAgentInstance)
	fixture.Engine.GET("/api/agents/:name/activity", fixture.Handlers.GetAgentActivity)
	fixture.Engine.GET("/api/agent-skill-reviews", fixture.Handlers.ListAgentSkillReviews)

	t.Run("Should list every profile's instances with their owner labels", func(t *testing.T) {
		response := performRequest(
			t,
			fixture.Engine,
			"GET",
			"/api/agent-instances?all_workspaces=true&all_profiles=true",
			nil,
		)
		var body contract.AgentInstanceListResponse
		decodeJSON(t, response.Body.Bytes(), &body)
		owners := map[string]string{}
		for _, instance := range body.Instances {
			owners[instance.AgentName] = instance.ProfileName
		}
		// The cap applies per profile, so the aggregate counts every owner's active slots.
		if response.Code != 200 || len(body.Instances) != 2 || body.ActiveCount != 2 || owners["own"] != "default" ||
			owners["theirs"] != "foreign" {
			t.Fatalf("aggregate list = %d %+v", response.Code, body)
		}
		scoped := performRequest(t, fixture.Engine, "GET", "/api/agent-instances?all_workspaces=true", nil)
		decodeJSON(t, scoped.Body.Bytes(), &body)
		if scoped.Code != 200 || len(body.Instances) != 1 || body.ActiveCount != 1 ||
			body.Instances[0].ProfileName != "default" {
			t.Fatalf("scoped list = %d %+v", scoped.Code, body)
		}
	})
	t.Run("Should read another profile's instance detail with its owner label", func(t *testing.T) {
		response := performRequest(
			t,
			fixture.Engine,
			"GET",
			"/api/agent-instances/"+ids["theirs"]+"?all_profiles=true",
			nil,
		)
		var body contract.AgentInstanceResponse
		decodeJSON(t, response.Body.Bytes(), &body)
		if response.Code != 200 || body.Instance.ProfileID != foreign || body.Instance.ProfileName != "foreign" {
			t.Fatalf("aggregate detail = %d %s", response.Code, response.Body.String())
		}
	})
	t.Run("Should refuse all_profiles on single-profile reads with the typed conflict", func(t *testing.T) {
		for _, path := range []string{
			"/api/agents/own/activity?all_profiles=true",
			"/api/agent-skill-reviews?all_profiles=true",
		} {
			response := performRequest(t, fixture.Engine, "GET", path, nil)
			if response.Code != 400 ||
				!strings.Contains(response.Body.String(), `"code":"profile_selection_conflict"`) {
				t.Fatalf("%s = %d %s, want typed 400", path, response.Code, response.Body.String())
			}
		}
	})
}

type failingSourceStatuses struct{ instanceCheckStub }

func (*failingSourceStatuses) SourceStatuses(context.Context, string) ([]heartbeat.HeartbeatSourceStatus, error) {
	return nil, errors.New("source store unavailable")
}

func TestAgentInstanceSourceStatuses(t *testing.T) {
	t.Run("Should preserve an instance payload when source statuses fail", func(t *testing.T) {
		t.Parallel()
		api := core.AgentInstanceAPI{Checks: &failingSourceStatuses{}}
		payload, err := api.Payload(t.Context(), agentinstance.Instance{ID: "instance", State: "paused"})
		if err != nil || payload.ID != "instance" || len(payload.Sources) != 0 {
			t.Fatalf("payload=%+v err=%v", payload, err)
		}
	})
}

func TestProposalPickOperatorBoundary(t *testing.T) {
	t.Run("Should return a committed pick when postcommit effects fail", func(t *testing.T) {
		t.Parallel()
		db, err := globaldb.OpenGlobalDB(t.Context(), filepath.Join(t.TempDir(), "compozy.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		instance, _, err := db.Instances().
			AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", PermissionCeiling: "{}", CreatedByKind: "human"}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.RegisterSession(
			t.Context(),
			new(
				store.SessionInfo{
					ID:            "pick-main",
					ProfileID:     store.DefaultProfileID,
					AgentName:     "coder",
					Provider:      "codex",
					State:         "active",
					RuntimeStatus: store.SessionRuntimeUnbound,
				},
			),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(t.Context(), "UPDATE agent_instances SET state='active',main_session_id='pick-main' WHERE id=?", instance.ID); err != nil {
			t.Fatal(err)
		}
		report, err := db.InstanceReports().
			AppendReport(t.Context(), instance.ID, agentinstance.ReportInput{SourceKey: "occ:pick-postcommit", Kind: "proposal", Payload: []byte(`{"proposals":[{"title":"Review this","body":"Evidence","item_keys":[]}]}`)})
		if err != nil {
			t.Fatal(err)
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store:      db.Instances(),
			QueueCap:   10,
			AfterPicks: func(context.Context, string, []agentinstance.Pick) error { return errors.New("release failed") },
		}
		fixture.Engine.POST("/instances/:id/reports/:report_id/pick", fixture.Handlers.PickAgentInstanceProposal)
		response := performRequest(
			t,
			fixture.Engine,
			"POST",
			"/instances/"+instance.ID+"/reports/"+report.ID+"/pick",
			[]byte(`{"proposal":0}`),
		)
		var result contract.AgentInstancePickResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || result.Pick.ReportID != report.ID || result.Pick.QueueEntryID == "" {
			t.Fatalf("pick=%d %s", response.Code, response.Body)
		}
		entry, err := db.GetSessionInputQueueEntry(t.Context(), "pick-main", result.Pick.QueueEntryID)
		if err != nil || entry.Status != "queued" {
			t.Fatalf("committed queue=%+v %v", entry, err)
		}
	})

	t.Run("Should reject agent credentials before target lookup [IT-009]", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Engine.POST("/instances/:id/reports/:report_id/pick", fixture.Handlers.PickAgentInstanceProposal)
		req := httptest.NewRequestWithContext(
			t.Context(),
			"POST",
			"/instances/missing/reports/missing/pick",
			strings.NewReader(`{"proposal":0}`),
		)
		req.Header.Set(agentidentity.HeaderSessionID, "sess-agent")
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, req)
		var failure contract.ErrorPayload
		if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
			t.Fatal(err)
		}
		if failure.Code != "proposal_pick_operator_only" {
			t.Fatalf("failure=%+v", failure)
		}
		if response.Code != 403 || !strings.Contains(response.Body.String(), "proposal_pick_operator_only") {
			t.Fatalf("status=%d body=%s", response.Code, response.Body)
		}
	})
}
