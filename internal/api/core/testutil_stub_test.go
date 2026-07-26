package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/session"
)

func TestTestutilStubFallbacksReturnDeterministicErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return not found when session create stub has no implementation", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/sessions",
			[]byte(`{"agent_name":"coder","workspace":"alpha"}`),
		)

		assertAPIErrorResponse(t, response, http.StatusNotFound, "session not found")
	})

	t.Run("Should return not found when session resume and clear stubs have no implementation", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			StatusFn: func(context.Context, string) (*session.Info, error) {
				return testutil.NewSessionInfo("sess-default"), nil
			},
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Engine.POST(
			"/workspaces/:workspace_id/sessions/:session_id/clear-conversation",
			fixture.Handlers.ClearSessionConversation,
		)

		attach := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-default/attach",
			nil,
		)
		assertAPIErrorResponse(t, attach, http.StatusNotFound, "session not found")

		clearResponse := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/sessions/sess-default/clear-conversation",
			nil,
		)
		assertAPIErrorResponse(t, clearResponse, http.StatusNotFound, "session not found")
	})

	t.Run("Should return not found when bridge create stub has no implementation", func(t *testing.T) {
		t.Parallel()

		_, engine := newBridgeHandlerFixture(t, testutil.StubBridgeService{})
		response := performRequest(
			t,
			engine,
			http.MethodPost,
			"/bridges",
			[]byte(
				`{"scope":"global","platform":"telegram","extension_name":"ext-telegram","display_name":"Support","enabled":true}`,
			),
		)

		assertAPIErrorResponse(t, response, http.StatusNotFound, "bridge instance not found")
	})
}

func TestTestutilAutomationToggleFallbacksReturnDeterministicErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return not found for config-backed job toggle without set implementation", func(t *testing.T) {
		t.Parallel()

		current := automationpkg.Job{
			ID:        "job-config",
			Scope:     automationpkg.AutomationScopeGlobal,
			Name:      "config-job",
			AgentName: "coder",
			Prompt:    "inspect",
			Schedule: &automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceConfig,
		}
		fixture := newHandlerFixtureWithAutomation(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubAutomationManager{
				GetJobFn: func(context.Context, string) (automationpkg.Job, error) {
					return current, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/automation/jobs/"+current.ID,
			[]byte(`{"enabled":false}`),
		)

		assertAPIErrorResponse(t, response, http.StatusNotFound, "job not found")
	})

	t.Run("Should return not found for config-backed trigger toggle without set implementation", func(t *testing.T) {
		t.Parallel()

		current := automationpkg.Trigger{
			ID:        "trigger-config",
			Scope:     automationpkg.AutomationScopeGlobal,
			Name:      "config-trigger",
			AgentName: "coder",
			Prompt:    "inspect",
			Event:     "session.stopped",
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceConfig,
		}
		fixture := newHandlerFixtureWithAutomation(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubAutomationManager{
				GetTriggerFn: func(context.Context, string) (automationpkg.Trigger, error) {
					return current, nil
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/automation/triggers/"+current.ID,
			[]byte(`{"enabled":false}`),
		)

		assertAPIErrorResponse(t, response, http.StatusNotFound, "trigger not found")
	})
}

func assertAPIErrorResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantStatus int,
	wantErrorSubstring string,
) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, wantStatus, response.Body.String())
	}
	var payload contract.ErrorPayload
	testutil.DecodeJSONResponse(t, response, &payload)
	if !strings.Contains(payload.Error, wantErrorSubstring) {
		t.Fatalf("error = %q, want substring %q", payload.Error, wantErrorSubstring)
	}
}
