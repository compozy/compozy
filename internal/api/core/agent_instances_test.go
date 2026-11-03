package core_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
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
		if page.Reports[0].Detail != "provider exited" {
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
		{"Should report the active instance cap", agentinstance.ErrLimit, 429, "You have 7 active stays-on agents. Pause one or raise `agents.stays_on.max_active_instances`."},
		{"Should report an on demand definition", agentinstance.ErrNotStaysOn, 422, "This agent is on-demand. Only stays-on agents have instances."},
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
	})
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
						return []workspacepkg.Workspace{{ID: "allowed"}, {ID: "denied"}}, nil
					},
					ResolveFn: func(_ context.Context, id string) (workspacepkg.ResolvedWorkspace, error) {
						return workspacepkg.ResolvedWorkspace{ID: id, Name: id}, nil
					},
				},
			)
			fixture.Handlers.Now = func() time.Time { return now }
			fixture.Handlers.WorkspaceAccess = activityReachPolicy{}
			fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{Store: db.Instances()}
			fixture.Engine.GET("/activity/:name", fixture.Handlers.GetAgentActivity)
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
	t.Run("Should count attention per instance and exclude main sessions from board counts", func(t *testing.T) {
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
		for _, name := range []string{"attention", "quiet"} {
			if _, _, err := db.Instances().
				AdmitCreate(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: name, PermissionCeiling: "{}", CreatedByKind: "human"}, 10); err != nil {
				t.Fatal(err)
			}
		}
		fixture := newHandlerFixture(
			t,
			&testutil.StubSessionManager{
				ListPageFn: func(_ context.Context, q session.ListQuery) (session.ListPage, error) {
					if !q.ExcludeInstanceMain || len(q.InstanceMainIDs) != 2 {
						t.Fatalf("listed main exclusions: %+v", q)
					}
					return session.ListPage{Limit: 1}, nil
				},
				CatalogFacetsFn: func(_ context.Context, q session.ListQuery) (store.SessionCatalogFacetResult, error) {
					if len(q.BoardExcludedIDs) != 2 {
						t.Fatalf("main exclusions: %v", q.BoardExcludedIDs)
					}
					return store.SessionCatalogFacetResult{Facets: store.SessionCatalogFacets{All: 7}}, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
		)
		fixture.Handlers.AgentInstances = &core.AgentInstanceAPI{
			Store: db.Instances(),
			ReadPayload: func(_ context.Context, i agentinstance.Instance) (contract.AgentInstancePayload, error) {
				p := core.AgentInstancePayload(i)
				p.NeedsAttention = i.AgentName == "attention"
				p.MainSessionID = "main-" + i.AgentName
				return p, nil
			},
		}
		fixture.Engine.GET("/facets", fixture.Handlers.ListSessionFacets)
		fixture.Engine.GET("/list", fixture.Handlers.ListSessions)
		listed := performRequest(t, fixture.Engine, "GET", "/list?all_workspaces=true&exclude_instance_main=true", nil)
		if listed.Code != 200 {
			t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
		}
		response := performRequest(
			t,
			fixture.Engine,
			"GET",
			"/facets?all_workspaces=true&exclude_instance_main=true",
			nil,
		)
		var body contract.SessionCatalogFacetsResponse
		decodeJSON(t, response.Body.Bytes(), &body)
		if response.Code != 200 || body.Facets.All != 7 || body.Facets.InstancesNeedingAttention != 1 ||
			len(body.ByWorkspace) != 1 ||
			body.ByWorkspace[0].WorkspaceID != "" ||
			body.ByWorkspace[0].Facets.InstancesNeedingAttention != 1 ||
			len(body.ByProfile) != 1 ||
			body.ByProfile[0].ProfileID != store.DefaultProfileID ||
			body.ByProfile[0].Facets.InstancesNeedingAttention != 1 {
			t.Fatalf("facets: %d %+v", response.Code, body)
		}
	})
}

func TestAgentInstanceCheckIns(t *testing.T) {
	t.Run("Should return persisted check-ins and a usable next cursor", func(t *testing.T) {
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
					if response.Code != 422 || body.Code != agentinstance.ErrNotStaysOn.Error() {
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
	return agentinstance.Instance{}, agentinstance.ErrNotStaysOn
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
	for _, tc := range []struct{ name, monitorAt, checkAt, want string }{
		{"Should prefer an earlier timer", "2026-11-03T10:00:00Z", "2026-11-03T11:00:00Z", "timer"},
		{"Should prefer an earlier check", "2026-11-03T12:00:00Z", "2026-11-03T11:00:00Z", "check"},
		{"Should preserve an untimed monitor without a check", "", "", "timer"},
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
					return heartbeat.InstanceWakeState{NextWakeAt: tc.checkAt}, nil
				},
			}
			payload, err := api.Payload(t.Context(), agentinstance.Instance{State: "active", ChecksEnabled: true})
			if err != nil {
				t.Fatal(err)
			}
			if payload.NextWake == nil || payload.NextWake.Kind != tc.want {
				t.Fatalf("next wake = %+v", payload.NextWake)
			}
		})
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
			Authority: authority,
			DraftRoot: t.TempDir(),
			ScanRoots: []string{scanRoot},
		}
		var reviews []agentinstance.SkillReview
		for _, name := range []string{"first", "second"} {
			review, err := service.Propose(
				t.Context(),
				workspaceaccess.ActorRef{Operator: true},
				instance,
				scanRoot,
				agentinstance.SkillProposalInput{
					Name:        name,
					Description: "Review changes",
					Body:        "Read the diff.",
					Files:       []agentinstance.SkillDraftFile{{Path: "scripts/read.sh", Content: "echo reviewed"}},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			reviews = append(reviews, review)
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
		mismatch := request("GET", "/reviews?state=pending&cursor="+page.NextCursor, "", true)
		if mismatch.Code != 422 {
			t.Fatalf("changed filters: %d %s", mismatch.Code, mismatch.Body.String())
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
		if accepted.Code != 200 || decision.Review.State != "approved" || len(decision.Review.Files) != 2 {
			t.Fatalf("approval: %d %+v", accepted.Code, decision)
		}
		installed, err := os.ReadFile(filepath.Join(scanRoot, "first", "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(installed) != decision.Review.Files[0].Content {
			t.Fatal("installed bytes differ from review")
		}
	})
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
		for _, id := range []string{"allowed", "denied"} {
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
		for i, workspace := range []string{"denied", "allowed", "denied", "allowed"} {
			_, _, err := repo.AdmitCreate(
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
		first, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{})
		if err != nil || len(first.Peers) != 1 || first.Peers[0].Scope != "allowed" || first.NextCursor == "" ||
			len(first.Instances) != 0 {
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
		query.AfterID = first.NextCursor
		next, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{})
		if err != nil || len(next.Peers) != 1 || next.Peers[0].Name == first.Peers[0].Name || next.NextCursor != "" {
			t.Fatalf("next peers: %+v %v", next, err)
		}
		query.State = "paused"
		if _, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}); err == nil {
			t.Fatal("cursor crossed filters")
		}
		query.State = ""
		actor.SessionID = "another"
		if _, err := api.PeersPage(t.Context(), query, actor, activityReachPolicy{}); err == nil {
			t.Fatal("cursor crossed callers")
		}
	})
}
