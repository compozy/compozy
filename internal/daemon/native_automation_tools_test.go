package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	apitest "github.com/compozy/compozy/internal/api/testutil"
	automationpkg "github.com/compozy/compozy/internal/automation"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestDaemonNativeAutomationTools(t *testing.T) {
	t.Parallel()

	t.Run("Should route automation lifecycle tools through the automation manager", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
		job := nativeAutomationJobFixture("job-1", automationpkg.JobSourceDynamic)
		trigger := nativeAutomationTriggerFixture("trigger-1", automationpkg.JobSourceDynamic)
		run := automationpkg.Run{
			ID:        "run-1",
			JobID:     job.ID,
			Status:    automationpkg.RunCompleted,
			Attempt:   1,
			StartedAt: &now,
			EndedAt:   &now,
		}
		var listJobQuery automationpkg.JobListQuery
		var updateJob automationpkg.Job
		var deletedJobID string
		var triggeredJobID string
		var enabledJobID string
		var enabledJobValue bool
		var listTriggerQuery automationpkg.TriggerListQuery
		var updateTrigger automationpkg.Trigger
		var updatedTriggerSecret *automationpkg.WebhookSecretWrite
		var deletedTriggerID string
		var enabledTriggerID string
		var enabledTriggerValue bool
		var listRunQuery automationpkg.RunQuery

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				ListJobsFn: func(_ context.Context, query automationpkg.JobListQuery) (automationpkg.JobListPage, error) {
					listJobQuery = query
					return automationpkg.JobListPage{
						Jobs:       []automationpkg.Job{job},
						Total:      4,
						Limit:      query.Limit,
						HasMore:    true,
						NextCursor: "job-next",
					}, nil
				},
				GetJobFn: func(_ context.Context, id string) (automationpkg.Job, error) {
					if id != job.ID {
						return automationpkg.Job{}, automationpkg.ErrJobNotFound
					}
					return job, nil
				},
				CreateJobFn: func(_ context.Context, created automationpkg.Job) (automationpkg.Job, error) {
					created.ID = job.ID
					created.CreatedAt = now
					created.UpdatedAt = now
					return created, nil
				},
				UpdateJobFn: func(_ context.Context, updated automationpkg.Job) (automationpkg.Job, error) {
					updateJob = updated
					return updated, nil
				},
				DeleteJobFn: func(_ context.Context, id string) error {
					deletedJobID = id
					return nil
				},
				SetJobEnabledFn: func(_ context.Context, id string, enabled bool) (automationpkg.Job, error) {
					enabledJobID = id
					enabledJobValue = enabled
					next := job
					next.Enabled = enabled
					return next, nil
				},
				TriggerJobFn: func(_ context.Context, id string) (automationpkg.Run, error) {
					triggeredJobID = id
					return run, nil
				},
				ListTriggersFn: func(
					_ context.Context,
					query automationpkg.TriggerListQuery,
				) (automationpkg.TriggerListPage, error) {
					listTriggerQuery = query
					return automationpkg.TriggerListPage{
						Triggers:   []automationpkg.Trigger{trigger},
						Total:      6,
						Limit:      query.Limit,
						HasMore:    true,
						NextCursor: "trigger-next",
					}, nil
				},
				GetTriggerFn: func(_ context.Context, id string) (automationpkg.Trigger, error) {
					if id != trigger.ID {
						return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
					}
					return trigger, nil
				},
				CreateTriggerFn: func(
					_ context.Context,
					created automationpkg.Trigger,
					secret automationpkg.WebhookSecretWrite,
				) (automationpkg.Trigger, error) {
					if secret.Value != nil || secret.Ref != "" {
						t.Fatalf("CreateTrigger secret = %#v, want empty tool-managed secret", secret)
					}
					created.ID = trigger.ID
					created.CreatedAt = now
					created.UpdatedAt = now
					return created, nil
				},
				UpdateTriggerFn: func(
					_ context.Context,
					updated automationpkg.Trigger,
					secret *automationpkg.WebhookSecretWrite,
				) (automationpkg.Trigger, error) {
					updateTrigger = updated
					updatedTriggerSecret = secret
					return updated, nil
				},
				DeleteTriggerFn: func(_ context.Context, id string) error {
					deletedTriggerID = id
					return nil
				},
				SetTriggerEnabledFn: func(_ context.Context, id string, enabled bool) (automationpkg.Trigger, error) {
					enabledTriggerID = id
					enabledTriggerValue = enabled
					next := trigger
					next.Enabled = enabled
					return next, nil
				},
				ListRunsFn: func(_ context.Context, query automationpkg.RunQuery) ([]automationpkg.Run, error) {
					listRunQuery = query
					return []automationpkg.Run{run}, nil
				},
				GetRunFn: func(_ context.Context, id string) (automationpkg.Run, error) {
					if id != run.ID {
						return automationpkg.Run{}, automationpkg.ErrRunNotFound
					}
					return run, nil
				},
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, nil
				},
			},
		}, nativeApproveAllPolicyInputs())

		jobListResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsList,
				Input:  json.RawMessage(`{"scope":"global","source":"package","enabled":false,"q":"review","limit":3}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_list) error = %v", err)
		}
		requireNativeStructuredContains(t, jobListResult, []byte(`"job-1"`))
		requireNativeStructuredContains(t, jobListResult, []byte(`"page"`))
		requireNativeStructuredContains(t, jobListResult, []byte(`"total":4`))
		if listJobQuery.Scope != automationpkg.AutomationScopeGlobal ||
			listJobQuery.Source != automationpkg.JobSourcePackage ||
			listJobQuery.Enabled == nil || *listJobQuery.Enabled ||
			listJobQuery.Search != "review" ||
			listJobQuery.Limit != 3 {
			t.Fatalf("job list query = %#v, want global package disabled limit", listJobQuery)
		}

		jobGetResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsGet,
				Input:  json.RawMessage(`{"job_id":"job-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_get) error = %v", err)
		}
		requireNativeStructuredContains(t, jobGetResult, []byte(`"job-1"`))

		jobCreateResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"daily","agent_name":"codex","prompt":"run","schedule":{"mode":"every","interval":"1h"}}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_create) error = %v", err)
		}
		requireNativeStructuredContains(t, jobCreateResult, []byte(`"daily"`))

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsUpdate,
				Input:  json.RawMessage(`{"job_id":"job-1","name":"daily-updated"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_update) error = %v", err)
		}
		if updateJob.ID != job.ID || updateJob.Name != "daily-updated" {
			t.Fatalf("updated job = %#v, want renamed job-1", updateJob)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsDisable,
				Input:  json.RawMessage(`{"job_id":"job-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_disable) error = %v", err)
		}
		if enabledJobID != job.ID || enabledJobValue {
			t.Fatalf("SetJobEnabled disable = %q/%v, want job-1 false", enabledJobID, enabledJobValue)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsEnable,
				Input:  json.RawMessage(`{"job_id":"job-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_enable) error = %v", err)
		}
		if enabledJobID != job.ID || !enabledJobValue {
			t.Fatalf("SetJobEnabled enable = %q/%v, want job-1 true", enabledJobID, enabledJobValue)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsTrigger,
				Input:  json.RawMessage(`{"job_id":"job-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_trigger) error = %v", err)
		}
		if triggeredJobID != job.ID {
			t.Fatalf("TriggerJob id = %q, want job-1", triggeredJobID)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsHistory,
				Input: json.RawMessage(
					`{"job_id":"job-1","status":"completed","since":"2026-04-30T00:00:00Z","limit":2}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_history) error = %v", err)
		}
		if listRunQuery.JobID != job.ID ||
			listRunQuery.Status != automationpkg.RunCompleted ||
			listRunQuery.Limit != 2 ||
			listRunQuery.Since.IsZero() {
			t.Fatalf("job run query = %#v, want filtered job history", listRunQuery)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsDelete,
				Input:  json.RawMessage(`{"job_id":"job-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_jobs_delete) error = %v", err)
		}
		if deletedJobID != job.ID {
			t.Fatalf("DeleteJob id = %q, want job-1", deletedJobID)
		}

		triggerListResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersList,
				Input: json.RawMessage(`{
					"scope":"global",
					"event":"session.created",
					"source":"package",
					"enabled":true,
					"q":"review",
					"limit":4
				}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_list) error = %v", err)
		}
		requireNativeStructuredContains(t, triggerListResult, []byte(`"trigger-1"`))
		requireNativeStructuredContains(t, triggerListResult, []byte(`"page"`))
		requireNativeStructuredContains(t, triggerListResult, []byte(`"total":6`))
		if listTriggerQuery.Scope != automationpkg.AutomationScopeGlobal ||
			listTriggerQuery.Event != "session.created" ||
			listTriggerQuery.Source != automationpkg.JobSourcePackage ||
			listTriggerQuery.Enabled == nil || !*listTriggerQuery.Enabled ||
			listTriggerQuery.Search != "review" ||
			listTriggerQuery.Limit != 4 {
			t.Fatalf("trigger list query = %#v, want event package enabled limit", listTriggerQuery)
		}

		triggerGetResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersGet,
				Input:  json.RawMessage(`{"trigger_id":"trigger-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_get) error = %v", err)
		}
		requireNativeStructuredContains(t, triggerGetResult, []byte(`"trigger-1"`))

		triggerCreateResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"on-session","agent_name":"codex","prompt":"handle {{ .Kind }}","event":"session.created","filter":{"data.agent":"codex"}}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_create) error = %v", err)
		}
		requireNativeStructuredContains(t, triggerCreateResult, []byte(`"on-session"`))

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersUpdate,
				Input:  json.RawMessage(`{"trigger_id":"trigger-1","name":"on-session-updated"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_update) error = %v", err)
		}
		if updateTrigger.ID != trigger.ID || updateTrigger.Name != "on-session-updated" || updatedTriggerSecret != nil {
			t.Fatalf(
				"updated trigger/secret = %#v/%#v, want renamed trigger with no secret update",
				updateTrigger,
				updatedTriggerSecret,
			)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersEnable,
				Input:  json.RawMessage(`{"trigger_id":"trigger-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_enable) error = %v", err)
		}
		if enabledTriggerID != trigger.ID || !enabledTriggerValue {
			t.Fatalf("SetTriggerEnabled enable = %q/%v, want trigger-1 true", enabledTriggerID, enabledTriggerValue)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersDisable,
				Input:  json.RawMessage(`{"trigger_id":"trigger-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_disable) error = %v", err)
		}
		if enabledTriggerID != trigger.ID || enabledTriggerValue {
			t.Fatalf("SetTriggerEnabled disable = %q/%v, want trigger-1 false", enabledTriggerID, enabledTriggerValue)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersHistory,
				Input:  json.RawMessage(`{"trigger_id":"trigger-1","limit":1}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_history) error = %v", err)
		}
		if listRunQuery.TriggerID != trigger.ID || listRunQuery.JobID != "" || listRunQuery.Limit != 1 {
			t.Fatalf("trigger run query = %#v, want trigger history", listRunQuery)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersDelete,
				Input:  json.RawMessage(`{"trigger_id":"trigger-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_triggers_delete) error = %v", err)
		}
		if deletedTriggerID != trigger.ID {
			t.Fatalf("DeleteTrigger id = %q, want trigger-1", deletedTriggerID)
		}

		runsListResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationRunsList,
				Input:  json.RawMessage(`{"job_id":"job-1","limit":5}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_runs_list) error = %v", err)
		}
		requireNativeStructuredContains(t, runsListResult, []byte(`"run-1"`))
		if listRunQuery.JobID != job.ID || listRunQuery.Limit != 5 {
			t.Fatalf("runs query = %#v, want job run list", listRunQuery)
		}

		runGetResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationRunsGet,
				Input:  json.RawMessage(`{"run_id":"run-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_runs_get) error = %v", err)
		}
		requireNativeStructuredContains(t, runGetResult, []byte(`"run-1"`))
	})

	t.Run("Should enforce caller workspace across native automation operations", func(t *testing.T) {
		t.Parallel()

		foreignJob := nativeAutomationJobFixture("job-b", automationpkg.JobSourceDynamic)
		foreignJob.Scope = automationpkg.AutomationScopeWorkspace
		foreignJob.WorkspaceID = "ws-b"
		foreignTrigger := nativeAutomationTriggerFixture("trigger-b", automationpkg.JobSourceDynamic)
		foreignTrigger.Scope = automationpkg.AutomationScopeWorkspace
		foreignTrigger.WorkspaceID = "ws-b"
		foreignRun := automationpkg.Run{
			ID:        "run-b",
			JobID:     foreignJob.ID,
			TriggerID: foreignTrigger.ID,
			Status:    automationpkg.RunCompleted,
		}
		callbackAssertionErr := errors.New("automation manager callback assertion failed")
		var listQuery automationpkg.JobListQuery
		var createCalls int
		var updateCalls int
		var getJobCalls int
		var getTriggerCalls int
		var getRunCalls int
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				CreateJobFn: func(_ context.Context, job automationpkg.Job) (automationpkg.Job, error) {
					createCalls++
					return job, nil
				},
				ListJobsFn: func(_ context.Context, query automationpkg.JobListQuery) (automationpkg.JobListPage, error) {
					listQuery = query
					if query.WorkspaceID != "ws-a" {
						t.Errorf("ListJobs() workspace = %q, want ws-a", query.WorkspaceID)
						return automationpkg.JobListPage{}, callbackAssertionErr
					}
					return automationpkg.JobListPage{}, nil
				},
				GetJobFn: func(_ context.Context, id string) (automationpkg.Job, error) {
					getJobCalls++
					switch id {
					case foreignJob.ID:
						return foreignJob, nil
					case "job-missing":
						return automationpkg.Job{}, automationpkg.ErrJobNotFound
					default:
						t.Errorf("GetJob() id = %q, want foreign or missing fixture", id)
						return automationpkg.Job{}, callbackAssertionErr
					}
				},
				UpdateJobFn: func(_ context.Context, job automationpkg.Job) (automationpkg.Job, error) {
					updateCalls++
					return job, nil
				},
				ListTriggersFn: func(
					_ context.Context,
					query automationpkg.TriggerListQuery,
				) (automationpkg.TriggerListPage, error) {
					if query.WorkspaceID != "ws-a" {
						t.Errorf("ListTriggers() workspace = %q, want ws-a", query.WorkspaceID)
						return automationpkg.TriggerListPage{}, callbackAssertionErr
					}
					return automationpkg.TriggerListPage{}, nil
				},
				GetTriggerFn: func(_ context.Context, id string) (automationpkg.Trigger, error) {
					getTriggerCalls++
					switch id {
					case foreignTrigger.ID:
						return foreignTrigger, nil
					case "trigger-missing":
						return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
					default:
						t.Errorf("GetTrigger() id = %q, want foreign or missing fixture", id)
						return automationpkg.Trigger{}, callbackAssertionErr
					}
				},
				GetRunFn: func(_ context.Context, id string) (automationpkg.Run, error) {
					getRunCalls++
					switch id {
					case foreignRun.ID:
						return foreignRun, nil
					case "run-missing":
						return automationpkg.Run{}, automationpkg.ErrRunNotFound
					default:
						t.Errorf("GetRun() id = %q, want foreign or missing fixture", id)
						return automationpkg.Run{}, callbackAssertionErr
					}
				},
				ListRunsFn: func(context.Context, automationpkg.RunQuery) ([]automationpkg.Run, error) {
					t.Errorf("ListRuns() called without an owned workspace resource")
					return nil, callbackAssertionErr
				},
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, nil
				},
			},
		}, nativeApproveAllPolicyInputs())
		scope := toolspkg.Scope{SessionID: "sess-a", WorkspaceID: "ws-a", AgentName: "coder"}

		t.Run("Should deny creating global automation from a workspace-bound session", func(t *testing.T) {
			_, err := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationJobsCreate,
					Input: json.RawMessage(
						`{"scope":"global","name":"daily","agent_name":"codex","prompt":"run","schedule":{"mode":"every","interval":"1h"}}`,
					),
				},
			)
			requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonScopeMismatch)
			if createCalls != 0 {
				t.Fatalf("CreateJob calls = %d, want 0 for workspace-bound global automation", createCalls)
			}
		})

		t.Run("Should fill workspace filters from caller scope", func(t *testing.T) {
			_, err := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationJobsList,
					Input:  json.RawMessage(`{"scope":"workspace"}`),
				},
			)
			if err != nil {
				t.Fatalf("Registry.Call(automation_jobs_list inferred) error = %v", err)
			}
			if listQuery.WorkspaceID != "ws-a" {
				t.Fatalf("automation jobs workspace filter = %q, want ws-a", listQuery.WorkspaceID)
			}
		})

		t.Run("Should make foreign and absent job ids indistinguishable", func(t *testing.T) {
			callsBefore := getJobCalls
			_, foreignErr := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationJobsGet,
					Input:  json.RawMessage(`{"job_id":"job-b"}`),
				},
			)
			requireToolReason(t, foreignErr, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			_, absentErr := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationJobsGet,
					Input:  json.RawMessage(`{"job_id":"job-missing"}`),
				},
			)
			requireToolReason(t, absentErr, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			if foreignErr.Error() != absentErr.Error() {
				t.Fatalf("foreign error = %q, absent error = %q", foreignErr, absentErr)
			}
			if got := getJobCalls - callsBefore; got != 2 {
				t.Fatalf("GetJob() calls = %d, want 2 point lookups", got)
			}
		})

		t.Run("Should reject mutating a foreign workspace job before the update", func(t *testing.T) {
			_, err := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationJobsUpdate,
					Input:  json.RawMessage(`{"job_id":"job-b","name":"foreign-update"}`),
				},
			)
			requireToolReason(t, err, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			if updateCalls != 0 {
				t.Fatalf("UpdateJob calls = %d, want 0 for foreign workspace job", updateCalls)
			}
		})

		t.Run("Should make foreign and absent trigger ids indistinguishable", func(t *testing.T) {
			callsBefore := getTriggerCalls
			_, foreignErr := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationTriggersGet,
					Input:  json.RawMessage(`{"trigger_id":"trigger-b"}`),
				},
			)
			requireToolReason(t, foreignErr, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			_, absentErr := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationTriggersGet,
					Input:  json.RawMessage(`{"trigger_id":"trigger-missing"}`),
				},
			)
			requireToolReason(t, absentErr, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			if foreignErr.Error() != absentErr.Error() {
				t.Fatalf("foreign error = %q, absent error = %q", foreignErr, absentErr)
			}
			if got := getTriggerCalls - callsBefore; got != 2 {
				t.Fatalf("GetTrigger() calls = %d, want 2 point lookups", got)
			}
		})

		t.Run("Should make foreign and absent run ids indistinguishable", func(t *testing.T) {
			runCallsBefore := getRunCalls
			jobCallsBefore := getJobCalls
			_, foreignErr := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationRunsGet,
					Input:  json.RawMessage(`{"run_id":"run-b"}`),
				},
			)
			requireToolReason(t, foreignErr, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			_, absentErr := registry.Call(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID: toolspkg.ToolIDAutomationRunsGet,
					Input:  json.RawMessage(`{"run_id":"run-missing"}`),
				},
			)
			requireToolReason(t, absentErr, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)
			if foreignErr.Error() != absentErr.Error() {
				t.Fatalf("foreign error = %q, absent error = %q", foreignErr, absentErr)
			}
			if got := getRunCalls - runCallsBefore; got != 2 {
				t.Fatalf("GetRun() calls = %d, want 2 point lookups", got)
			}
			if got := getJobCalls - jobCallsBefore; got != 1 {
				t.Fatalf("GetJob() calls for foreign run ownership = %d, want 1", got)
			}
		})
	})

	t.Run("Should stop workspace pagination when cursors do not advance", func(t *testing.T) {
		t.Parallel()

		var jobCalls int
		var triggerCalls int
		manager := apitest.StubAutomationManager{
			ListJobsFn: func(_ context.Context, query automationpkg.JobListQuery) (automationpkg.JobListPage, error) {
				jobCalls++
				if jobCalls > 2 {
					return automationpkg.JobListPage{}, errors.New("job pagination did not stop")
				}
				return automationpkg.JobListPage{
					Jobs:       []automationpkg.Job{{ID: fmt.Sprintf("job-%d", jobCalls)}},
					HasMore:    true,
					NextCursor: "job-cursor",
					Limit:      query.Limit,
				}, nil
			},
			ListTriggersFn: func(
				_ context.Context,
				query automationpkg.TriggerListQuery,
			) (automationpkg.TriggerListPage, error) {
				triggerCalls++
				if triggerCalls > 2 {
					return automationpkg.TriggerListPage{}, errors.New("trigger pagination did not stop")
				}
				return automationpkg.TriggerListPage{
					Triggers:   []automationpkg.Trigger{{ID: fmt.Sprintf("trigger-%d", triggerCalls)}},
					HasMore:    true,
					NextCursor: "trigger-cursor",
					Limit:      query.Limit,
				}, nil
			},
		}
		tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{Automation: manager}}

		jobs, err := tools.nativeAutomationJobsForWorkspace(t.Context(), "ws-a")
		if err != nil {
			t.Fatalf("nativeAutomationJobsForWorkspace() error = %v", err)
		}
		if len(jobs) != 2 || jobCalls != 2 {
			t.Fatalf("jobs/calls = %d/%d, want 2/2", len(jobs), jobCalls)
		}
		triggers, err := tools.nativeAutomationTriggersForWorkspace(t.Context(), "ws-a")
		if err != nil {
			t.Fatalf("nativeAutomationTriggersForWorkspace() error = %v", err)
		}
		if len(triggers) != 2 || triggerCalls != 2 {
			t.Fatalf("triggers/calls = %d/%d, want 2/2", len(triggers), triggerCalls)
		}
	})

	t.Run("Should preserve workspace identity across consent-first suggestion tools", func(t *testing.T) {
		t.Parallel()

		workspaceID := "workspace-1"
		suggestionID := "suggestion-1"
		now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
		job := nativeAutomationJobFixture("job-suggestion-1", automationpkg.JobSourceDynamic)
		job.Scope = automationpkg.AutomationScopeWorkspace
		job.WorkspaceID = workspaceID
		suggestion := automationpkg.Suggestion{
			ID:          suggestionID,
			WorkspaceID: workspaceID,
			Source:      automationpkg.SuggestionSourceCatalog,
			DedupKey:    "catalog:v1:daily-workspace-briefing",
			Status:      automationpkg.SuggestionStatusPending,
			Payload:     job,
			CreatedAt:   now,
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				ListSuggestionsFn: func(
					_ context.Context,
					gotWorkspace string,
					status automationpkg.SuggestionStatus,
				) ([]automationpkg.Suggestion, error) {
					if gotWorkspace != workspaceID || status != automationpkg.SuggestionStatusPending {
						t.Fatalf("ListSuggestions(%q, %q), want workspace-1/pending", gotWorkspace, status)
					}
					return []automationpkg.Suggestion{suggestion}, nil
				},
				AcceptSuggestionFn: func(
					_ context.Context,
					gotWorkspace string,
					gotSuggestion string,
				) (automationpkg.SuggestionAcceptance, error) {
					if gotWorkspace != workspaceID || gotSuggestion != suggestionID {
						t.Fatalf(
							"AcceptSuggestion(%q, %q), want workspace-1/suggestion-1",
							gotWorkspace,
							gotSuggestion,
						)
					}
					accepted := suggestion
					accepted.Status = automationpkg.SuggestionStatusAccepted
					accepted.ResolvedAt = &now
					return automationpkg.SuggestionAcceptance{Suggestion: accepted, Job: job}, nil
				},
				DismissSuggestionFn: func(
					_ context.Context,
					gotWorkspace string,
					gotSuggestion string,
				) (automationpkg.Suggestion, error) {
					if gotWorkspace != workspaceID || gotSuggestion != suggestionID {
						t.Fatalf(
							"DismissSuggestion(%q, %q), want workspace-1/suggestion-1",
							gotWorkspace,
							gotSuggestion,
						)
					}
					dismissed := suggestion
					dismissed.Status = automationpkg.SuggestionStatusDismissed
					dismissed.ResolvedAt = &now
					return dismissed, nil
				},
			},
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationSuggestionsList,
				Input:  json.RawMessage(`{"workspace":"workspace-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_suggestions_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"suggestion-1"`))
		requireNativeStructuredContains(t, listResult, []byte(`"pending"`))

		acceptResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationSuggestionsAccept,
				Input:  json.RawMessage(`{"workspace":"workspace-1","suggestion_id":"suggestion-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_suggestions_accept) error = %v", err)
		}
		requireNativeStructuredContains(t, acceptResult, []byte(`"accepted"`))
		requireNativeStructuredContains(t, acceptResult, []byte(`"job-suggestion-1"`))

		dismissResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationSuggestionsDismiss,
				Input:  json.RawMessage(`{"workspace":"workspace-1","suggestion_id":"suggestion-1"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(automation_suggestions_dismiss) error = %v", err)
		}
		requireNativeStructuredContains(t, dismissResult, []byte(`"dismissed"`))
	})

	t.Run("Should deny automation mutations deterministically before manager writes when blocked", func(t *testing.T) {
		t.Parallel()

		configJob := nativeAutomationJobFixture("job-config", automationpkg.JobSourceConfig)
		configTrigger := nativeAutomationTriggerFixture("trigger-config", automationpkg.JobSourceConfig)
		var createJobCalls int
		var createTriggerCalls int
		var updateTriggerCalls int
		var deleteJobCalls int
		var deleteTriggerCalls int
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				CreateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
					createJobCalls++
					return automationpkg.Job{}, nil
				},
				CreateTriggerFn: func(context.Context, automationpkg.Trigger, automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error) {
					createTriggerCalls++
					return automationpkg.Trigger{}, nil
				},
				UpdateTriggerFn: func(context.Context, automationpkg.Trigger, *automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error) {
					updateTriggerCalls++
					return automationpkg.Trigger{}, nil
				},
				GetJobFn: func(context.Context, string) (automationpkg.Job, error) {
					return configJob, nil
				},
				GetTriggerFn: func(context.Context, string) (automationpkg.Trigger, error) {
					return configTrigger, nil
				},
				DeleteJobFn: func(context.Context, string) error {
					deleteJobCalls++
					return nil
				},
				DeleteTriggerFn: func(context.Context, string) error {
					deleteTriggerCalls++
					return nil
				},
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, nil
				},
			},
			Workspaces: nativeNetworkTestWorkspaceService(t),
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsCreate,
				Input: json.RawMessage(
					`{"scope":"global","workspace":"ws-1","name":"daily","agent_name":"codex","prompt":"run","schedule":{"mode":"every","interval":"1h"}}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonAutomationValidationFailed)
		if createJobCalls != 0 {
			t.Fatalf("CreateJob calls = %d, want 0 after validation denial", createJobCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"webhook","agent_name":"codex","prompt":"run","event":"webhook","endpoint_slug":"deploy","webhook_secret":"raw-secret"}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
		if createTriggerCalls != 0 {
			t.Fatalf("CreateTrigger calls = %d, want 0 after secret denial", createTriggerCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersUpdate,
				Input:  json.RawMessage(`{"trigger_id":"trigger-config","webhook_secret":"raw-secret"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
		if updateTriggerCalls != 0 {
			t.Fatalf("UpdateTrigger calls = %d, want 0 after secret denial", updateTriggerCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsDelete,
				Input:  json.RawMessage(`{"job_id":"job-config"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonAutomationScopeForbidden)
		if deleteJobCalls != 0 {
			t.Fatalf("DeleteJob calls = %d, want 0 after scope denial", deleteJobCalls)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersDelete,
				Input:  json.RawMessage(`{"trigger_id":"trigger-config"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonAutomationScopeForbidden)
		if deleteTriggerCalls != 0 {
			t.Fatalf("DeleteTrigger calls = %d, want 0 after scope denial", deleteTriggerCalls)
		}
	})

	t.Run("Should preserve managed enable-only update semantics", func(t *testing.T) {
		t.Parallel()

		configJob := nativeAutomationJobFixture("job-config", automationpkg.JobSourceConfig)
		configTrigger := nativeAutomationTriggerFixture("trigger-config", automationpkg.JobSourceConfig)
		var setJobEnabledCalls int
		var setJobEnabledValue bool
		var setTriggerEnabledCalls int
		var setTriggerEnabledValue bool
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				GetJobFn: func(context.Context, string) (automationpkg.Job, error) {
					return configJob, nil
				},
				GetTriggerFn: func(context.Context, string) (automationpkg.Trigger, error) {
					return configTrigger, nil
				},
				SetJobEnabledFn: func(_ context.Context, id string, enabled bool) (automationpkg.Job, error) {
					if id != configJob.ID {
						t.Fatalf("SetJobEnabled id = %q, want %q", id, configJob.ID)
					}
					setJobEnabledCalls++
					setJobEnabledValue = enabled
					next := configJob
					next.Enabled = enabled
					return next, nil
				},
				SetTriggerEnabledFn: func(
					_ context.Context,
					id string,
					enabled bool,
				) (automationpkg.Trigger, error) {
					if id != configTrigger.ID {
						t.Fatalf("SetTriggerEnabled id = %q, want %q", id, configTrigger.ID)
					}
					setTriggerEnabledCalls++
					setTriggerEnabledValue = enabled
					next := configTrigger
					next.Enabled = enabled
					return next, nil
				},
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, nil
				},
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsUpdate,
				Input:  json.RawMessage(`{"job_id":"job-config","enabled":false}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config automation_jobs_update) error = %v", err)
		}
		if setJobEnabledCalls != 1 || setJobEnabledValue {
			t.Fatalf("SetJobEnabled calls/value = %d/%v, want 1/false", setJobEnabledCalls, setJobEnabledValue)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersUpdate,
				Input:  json.RawMessage(`{"trigger_id":"trigger-config","enabled":false}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(config automation_triggers_update) error = %v", err)
		}
		if setTriggerEnabledCalls != 1 || setTriggerEnabledValue {
			t.Fatalf(
				"SetTriggerEnabled calls/value = %d/%v, want 1/false",
				setTriggerEnabledCalls,
				setTriggerEnabledValue,
			)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsUpdate,
				Input:  json.RawMessage(`{"job_id":"job-config","name":"blocked"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonAutomationValidationFailed)
		if setJobEnabledCalls != 1 {
			t.Fatalf("SetJobEnabled calls = %d, want unchanged after invalid config patch", setJobEnabledCalls)
		}

		configJob.Source = automationpkg.JobSourcePackage
		configTrigger.Source = automationpkg.JobSourcePackage
		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsUpdate,
				Input:  json.RawMessage(`{"job_id":"job-config","enabled":true}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(package automation_jobs_update) error = %v", err)
		}
		if setJobEnabledCalls != 2 || !setJobEnabledValue {
			t.Fatalf("SetJobEnabled package calls/value = %d/%v, want 2/true", setJobEnabledCalls, setJobEnabledValue)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersUpdate,
				Input:  json.RawMessage(`{"trigger_id":"trigger-config","enabled":true}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(package automation_triggers_update) error = %v", err)
		}
		if setTriggerEnabledCalls != 2 || !setTriggerEnabledValue {
			t.Fatalf(
				"SetTriggerEnabled package calls/value = %d/%v, want 2/true",
				setTriggerEnabledCalls,
				setTriggerEnabledValue,
			)
		}
	})

	t.Run("Should reject blank automation resource identifiers before manager calls", func(t *testing.T) {
		t.Parallel()

		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{},
		}, nativeApproveAllPolicyInputs())

		cases := []struct {
			name  string
			id    toolspkg.ToolID
			input json.RawMessage
		}{
			{name: "jobs get", id: toolspkg.ToolIDAutomationJobsGet, input: json.RawMessage(`{"job_id":" "}`)},
			{name: "jobs history", id: toolspkg.ToolIDAutomationJobsHistory, input: json.RawMessage(`{"job_id":" "}`)},
			{
				name:  "triggers get",
				id:    toolspkg.ToolIDAutomationTriggersGet,
				input: json.RawMessage(`{"trigger_id":" "}`),
			},
			{
				name:  "triggers history",
				id:    toolspkg.ToolIDAutomationTriggersHistory,
				input: json.RawMessage(`{"trigger_id":" "}`),
			},
			{name: "runs get", id: toolspkg.ToolIDAutomationRunsGet, input: json.RawMessage(`{"run_id":" "}`)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := registry.Call(
					t.Context(),
					toolspkg.Scope{},
					toolspkg.CallRequest{ToolID: tc.id, Input: tc.input},
				)
				requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonSchemaInvalid)
			})
		}
	})

	t.Run("Should map automation manager and run-query errors to deterministic tool reasons", func(t *testing.T) {
		t.Parallel()

		job := nativeAutomationJobFixture("job-1", automationpkg.JobSourceDynamic)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				ListJobsFn: func(_ context.Context, query automationpkg.JobListQuery) (automationpkg.JobListPage, error) {
					return automationpkg.JobListPage{Jobs: []automationpkg.Job{job}, Total: 1, Limit: query.Limit}, nil
				},
				CreateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
					return automationpkg.Job{}, automationpkg.ErrJobNameTaken
				},
				CreateTriggerFn: func(context.Context, automationpkg.Trigger, automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error) {
					return automationpkg.Trigger{}, automationpkg.ErrTriggerNameTaken
				},
				GetJobFn: func(context.Context, string) (automationpkg.Job, error) {
					return job, nil
				},
				UpdateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
					return automationpkg.Job{}, automationpkg.ErrDefinitionReadOnly
				},
				TriggerJobFn: func(context.Context, string) (automationpkg.Run, error) {
					return automationpkg.Run{}, automationpkg.ErrFireLimitReached
				},
				GetRunFn: func(context.Context, string) (automationpkg.Run, error) {
					return automationpkg.Run{}, automationpkg.ErrRunNotFound
				},
				ListRunsFn: func(context.Context, automationpkg.RunQuery) ([]automationpkg.Run, error) {
					return nil, automationpkg.ErrManagerNotRunning
				},
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, automationpkg.ErrManagerNotRunning
				},
			},
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDAutomationJobsList},
		)
		requireToolReason(t, err, toolspkg.ErrToolUnavailable, toolspkg.ReasonDependencyMissing)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsList,
				Input:  json.RawMessage(`{"cursor":"not-a-cursor","limit":1}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonAutomationValidationFailed)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"daily","agent_name":"codex","prompt":"run","schedule":{"mode":"every","interval":"1h"}}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonAutomationValidationFailed)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationTriggersCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"on-session","agent_name":"codex","prompt":"handle {{ .Kind }}","event":"session.created"}`,
				),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonAutomationValidationFailed)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsUpdate,
				Input:  json.RawMessage(`{"job_id":"job-1","name":"blocked"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonAutomationScopeForbidden)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsTrigger,
				Input:  json.RawMessage(`{"job_id":"job-1"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolConflict, toolspkg.ReasonAutomationValidationFailed)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationRunsGet,
				Input:  json.RawMessage(`{"run_id":"missing"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolNotFound, toolspkg.ReasonToolUnknown)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDAutomationRunsList},
		)
		requireToolReason(t, err, toolspkg.ErrToolUnavailable, toolspkg.ReasonDependencyMissing)

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationRunsList,
				Input:  json.RawMessage(`{"since":"not-a-date"}`),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonAutomationValidationFailed)
	})

	t.Run("Should require approval before automation mutations reach the manager", func(t *testing.T) {
		t.Parallel()

		var createJobCalls int
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: apitest.StubAutomationManager{
				CreateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
					createJobCalls++
					return nativeAutomationJobFixture("job-1", automationpkg.JobSourceDynamic), nil
				},
				StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
					return automationpkg.ManagerStatus{}, nil
				},
			},
		}, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		})

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"daily","agent_name":"codex","prompt":"run","schedule":{"mode":"every","interval":"1h"}}`,
				),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf("Registry.Call(automation_jobs_create approve-reads) error = %v, want approval required", err)
		}
		if createJobCalls != 0 {
			t.Fatalf("CreateJob calls = %d, want 0 before approval", createJobCalls)
		}
	})
}

func nativeAutomationJobFixture(id string, source automationpkg.JobSource) automationpkg.Job {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	return automationpkg.Job{
		ID:        id,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      "daily",
		AgentName: "codex",
		Prompt:    "run daily",
		Schedule: &automationpkg.ScheduleSpec{
			Mode:     automationpkg.ScheduleModeEvery,
			Interval: "1h",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func nativeAutomationTriggerFixture(id string, source automationpkg.JobSource) automationpkg.Trigger {
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	return automationpkg.Trigger{
		ID:        id,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      "on-session",
		AgentName: "codex",
		Prompt:    "handle {{ .Kind }}",
		Event:     "session.created",
		Filter: map[string]string{
			"data.agent": "codex",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    source,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
