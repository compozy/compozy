package core_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	automationpkg "github.com/compozy/agh/internal/automation"
)

func TestAutomationEndpointsAdditionalCoverage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)
	job := automationpkg.Job{
		ID:        "job-1",
		Scope:     automationpkg.AutomationScopeWorkspace,
		Name:      "nightly-review",
		AgentName: "coder",
		Prompt:    "review repo",
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourceDynamic,
		CreatedAt: now,
		UpdatedAt: now,
	}
	trigger := automationpkg.Trigger{
		ID:        "trigger-1",
		Scope:     automationpkg.AutomationScopeWorkspace,
		Name:      "deploy-review",
		AgentName: "coder",
		Prompt:    "review deploy",
		Event:     "webhook",
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourceDynamic,
		CreatedAt: now,
		UpdatedAt: now,
	}
	run := automationpkg.Run{
		ID:        "run-1",
		JobID:     job.ID,
		TriggerID: trigger.ID,
		Status:    automationpkg.RunCompleted,
		Attempt:   1,
		StartedAt: &now,
		EndedAt:   &now,
	}

	newFixture := func(t *testing.T) handlerFixture {
		t.Helper()

		jobs := map[string]automationpkg.Job{job.ID: job}
		triggers := map[string]automationpkg.Trigger{trigger.ID: trigger}
		automation := testutil.StubAutomationManager{
			ListTriggersFn: func(_ context.Context, query automationpkg.TriggerListQuery) (automationpkg.TriggerListPage, error) {
				list := make([]automationpkg.Trigger, 0, len(triggers))
				for _, record := range triggers {
					list = append(list, record)
				}
				return automationpkg.TriggerListPage{Triggers: list, Total: len(list), Limit: query.Limit}, nil
			},
			GetJobFn: func(context.Context, string) (automationpkg.Job, error) {
				record, ok := jobs[job.ID]
				if !ok {
					return automationpkg.Job{}, automationpkg.ErrJobNotFound
				}
				return record, nil
			},
			DeleteJobFn: func(context.Context, string) error {
				if _, ok := jobs[job.ID]; !ok {
					return automationpkg.ErrJobNotFound
				}
				delete(jobs, job.ID)
				return nil
			},
			GetTriggerFn: func(context.Context, string) (automationpkg.Trigger, error) {
				record, ok := triggers[trigger.ID]
				if !ok {
					return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
				}
				return record, nil
			},
			DeleteTriggerFn: func(context.Context, string) error {
				if _, ok := triggers[trigger.ID]; !ok {
					return automationpkg.ErrTriggerNotFound
				}
				delete(triggers, trigger.ID)
				return nil
			},
			ListRunsFn: func(_ context.Context, query automationpkg.RunQuery) ([]automationpkg.Run, error) {
				switch {
				case query.JobID == job.ID:
					return []automationpkg.Run{run}, nil
				case query.TriggerID == trigger.ID:
					return []automationpkg.Run{run}, nil
				default:
					return []automationpkg.Run{run}, nil
				}
			},
			GetRunFn: func(context.Context, string) (automationpkg.Run, error) {
				return run, nil
			},
		}

		return newHandlerFixtureWithAutomation(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			automation,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
	}

	for _, request := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "Should return automation job details", method: http.MethodGet, path: "/automation/jobs/job-1"},
		{name: "Should delete automation jobs and hide them from subsequent reads", method: http.MethodDelete, path: "/automation/jobs/job-1"},
		{name: "Should list automation triggers", method: http.MethodGet, path: "/automation/triggers"},
		{name: "Should return automation trigger details", method: http.MethodGet, path: "/automation/triggers/trigger-1"},
		{name: "Should delete automation triggers and hide them from subsequent reads", method: http.MethodDelete, path: "/automation/triggers/trigger-1"},
		{name: "Should list job runs for one automation job", method: http.MethodGet, path: "/automation/jobs/job-1/runs"},
		{name: "Should list trigger runs for one automation trigger", method: http.MethodGet, path: "/automation/triggers/trigger-1/runs"},
		{name: "Should list automation runs", method: http.MethodGet, path: "/automation/runs?limit=1"},
		{name: "Should return automation run details", method: http.MethodGet, path: "/automation/runs/run-1"},
	} {
		t.Run(request.name, func(t *testing.T) {
			fixture := newFixture(t)
			resp := performRequest(t, fixture.Engine, request.method, request.path, nil)
			if request.method == http.MethodDelete {
				if resp.Code != http.StatusNoContent {
					t.Fatalf(
						"%s %s status = %d, want %d; body=%s",
						request.method,
						request.path,
						resp.Code,
						http.StatusNoContent,
						resp.Body.String(),
					)
				}
				followUp := performRequest(t, fixture.Engine, http.MethodGet, request.path, nil)
				if followUp.Code != http.StatusNotFound {
					t.Fatalf(
						"follow-up GET %s status = %d, want %d; body=%s",
						request.path,
						followUp.Code,
						http.StatusNotFound,
						followUp.Body.String(),
					)
				}
				return
			}
			if resp.Code != http.StatusOK {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					request.method,
					request.path,
					resp.Code,
					http.StatusOK,
					resp.Body.String(),
				)
			}

			switch request.path {
			case "/automation/jobs/job-1":
				var payload contract.JobResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal(job detail) error = %v", err)
				}
				if got, want := payload.Job.ID, job.ID; got != want {
					t.Fatalf("job payload id = %v, want %q", got, want)
				}
				if got, want := payload.Job.Name, job.Name; got != want {
					t.Fatalf("job payload name = %v, want %q", got, want)
				}
			case "/automation/triggers":
				var payload contract.TriggersResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal(trigger list) error = %v", err)
				}
				if got, want := len(payload.Triggers), 1; got != want {
					t.Fatalf("len(trigger list) = %d, want %d", got, want)
				}
				if got, want := payload.Triggers[0].ID, trigger.ID; got != want {
					t.Fatalf("trigger list id = %v, want %q", got, want)
				}
			case "/automation/triggers/trigger-1":
				var payload contract.TriggerResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal(trigger detail) error = %v", err)
				}
				if got, want := payload.Trigger.ID, trigger.ID; got != want {
					t.Fatalf("trigger payload id = %v, want %q", got, want)
				}
				if got, want := payload.Trigger.Event, trigger.Event; got != want {
					t.Fatalf("trigger payload event = %v, want %q", got, want)
				}
			case "/automation/jobs/job-1/runs", "/automation/triggers/trigger-1/runs", "/automation/runs?limit=1":
				var payload contract.RunsResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal(run list) error = %v", err)
				}
				if got, want := len(payload.Runs), 1; got != want {
					t.Fatalf("len(run list) = %d, want %d", got, want)
				}
				if got, want := payload.Runs[0].ID, run.ID; got != want {
					t.Fatalf("run list id = %v, want %q", got, want)
				}
			case "/automation/runs/run-1":
				var payload contract.RunResponse
				if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal(run detail) error = %v", err)
				}
				if got, want := payload.Run.ID, run.ID; got != want {
					t.Fatalf("run payload id = %v, want %q", got, want)
				}
				if got, want := payload.Run.Status, run.Status; got != want {
					t.Fatalf("run payload status = %v, want %q", got, want)
				}
			}
		})
	}
}
