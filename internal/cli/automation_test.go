package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

func TestAutomationJobsCreateParsesWorkspaceScopeAndRetry(t *testing.T) {
	t.Parallel()

	var request AutomationJobCreateRequest
	deps := newTestDeps(t, &stubClient{
		getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			if ref != "alpha" {
				t.Fatalf("GetWorkspace ref = %q, want %q", ref, "alpha")
			}
			return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
		},
		createAutomationJobFn: func(_ context.Context, got AutomationJobCreateRequest) (JobRecord, error) {
			request = got
			return sampleAutomationJobRecord(), nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"automation", "jobs", "create",
		"--name", "nightly-review",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--schedule", "every:30m",
		"--catch-up-policy", "run_once_on_catchup",
		"--misfire-grace-seconds", "90",
		"--agent", "coder",
		"--prompt", "review repo",
		"--retry", "backoff:3:2s",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(automation jobs create) error = %v", err)
	}

	if request.Scope != automationpkg.AutomationScopeWorkspace || request.WorkspaceID != "ws-alpha" {
		t.Fatalf("request scope/workspace = %#v, want workspace ws-alpha", request)
	}
	if request.Schedule.Mode != automationpkg.ScheduleModeEvery || request.Schedule.Interval != "30m" {
		t.Fatalf("request schedule = %#v, want every 30m", request.Schedule)
	}
	if request.Schedule.CatchUpPolicy != automationpkg.SchedulerCatchUpPolicyRunOnce ||
		request.Schedule.MisfireGraceSeconds != 90 {
		t.Fatalf("request schedule reliability = %#v, want run once with 90 second grace", request.Schedule)
	}
	if request.Retry == nil || request.Retry.Strategy != automationpkg.RetryStrategyBackoff ||
		request.Retry.MaxRetries != 3 ||
		request.Retry.BaseDelay != "2s" {
		t.Fatalf("request retry = %#v, want backoff 3 2s", request.Retry)
	}

	var created JobRecord
	if err := json.Unmarshal([]byte(stdout), &created); err != nil {
		t.Fatalf("json.Unmarshal(job create) error = %v", err)
	}
	if created.ID != "job-1" {
		t.Fatalf("created.ID = %q, want %q", created.ID, "job-1")
	}
}

func TestAutomationSuggestionsResolveWorkspaceAndPreserveStructuredOutput(t *testing.T) {
	t.Parallel()

	pending := sampleAutomationSuggestionRecord()
	resolvedAt := fixedTestNow
	accepted := pending
	accepted.Status = automationpkg.SuggestionStatusAccepted
	accepted.ResolvedAt = &resolvedAt
	dismissed := pending
	dismissed.Status = automationpkg.SuggestionStatusDismissed
	dismissed.ResolvedAt = &resolvedAt
	workspaceLookups := 0
	deps := newTestDeps(t, &stubClient{
		getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			workspaceLookups++
			if ref != "alpha" {
				t.Fatalf("GetWorkspace ref = %q, want alpha", ref)
			}
			return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
		},
		listAutomationSuggestionsFn: func(
			_ context.Context,
			workspaceID string,
			status automationpkg.SuggestionStatus,
		) (AutomationSuggestionListRecord, error) {
			if workspaceID != "ws-alpha" || status != automationpkg.SuggestionStatusPending {
				t.Fatalf("ListAutomationSuggestions(%q, %q), want ws-alpha/pending", workspaceID, status)
			}
			return AutomationSuggestionListRecord{Suggestions: []SuggestionRecord{pending}}, nil
		},
		acceptAutomationSuggestionFn: func(
			_ context.Context,
			workspaceID string,
			suggestionID string,
		) (AutomationSuggestionAcceptanceRecord, error) {
			if workspaceID != "ws-alpha" || suggestionID != pending.ID {
				t.Fatalf("AcceptAutomationSuggestion(%q, %q), want ws-alpha/%s", workspaceID, suggestionID, pending.ID)
			}
			return AutomationSuggestionAcceptanceRecord{Suggestion: accepted, Job: accepted.Payload}, nil
		},
		dismissAutomationSuggestionFn: func(
			_ context.Context,
			workspaceID string,
			suggestionID string,
		) (AutomationSuggestionRecord, error) {
			if workspaceID != "ws-alpha" || suggestionID != pending.ID {
				t.Fatalf("DismissAutomationSuggestion(%q, %q), want ws-alpha/%s", workspaceID, suggestionID, pending.ID)
			}
			return AutomationSuggestionRecord{Suggestion: dismissed}, nil
		},
	})

	listOutput, _, err := executeRootCommand(
		t,
		deps,
		"automation", "suggestions", "--workspace", "alpha", "-o", "json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(automation suggestions) error = %v", err)
	}
	var listRecord AutomationSuggestionListRecord
	if err := json.Unmarshal([]byte(listOutput), &listRecord); err != nil {
		t.Fatalf("json.Unmarshal(automation suggestions) error = %v", err)
	}
	if len(listRecord.Suggestions) != 1 || listRecord.Suggestions[0].ID != pending.ID {
		t.Fatalf("suggestion list = %#v, want %s", listRecord, pending.ID)
	}

	acceptOutput, _, err := executeRootCommand(
		t,
		deps,
		"automation", "suggestions", "--workspace", "alpha", "accept", pending.ID, "-o", "json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(automation suggestions accept) error = %v", err)
	}
	var acceptance AutomationSuggestionAcceptanceRecord
	if err := json.Unmarshal([]byte(acceptOutput), &acceptance); err != nil {
		t.Fatalf("json.Unmarshal(automation suggestions accept) error = %v", err)
	}
	if acceptance.Suggestion.Status != automationpkg.SuggestionStatusAccepted ||
		acceptance.Job.ID != pending.Payload.ID {
		t.Fatalf("suggestion acceptance = %#v, want accepted with created Job", acceptance)
	}

	dismissOutput, _, err := executeRootCommand(
		t,
		deps,
		"automation", "suggestions", "--workspace", "alpha", "dismiss", pending.ID, "-o", "toon",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(automation suggestions dismiss) error = %v", err)
	}
	if !strings.Contains(dismissOutput, "automation_suggestion") ||
		!strings.Contains(dismissOutput, string(automationpkg.SuggestionStatusDismissed)) {
		t.Fatalf("dismiss TOON output = %q, want dismissed automation suggestion", dismissOutput)
	}
	if workspaceLookups != 3 {
		t.Fatalf("workspace lookups = %d, want 3 exact command resolutions", workspaceLookups)
	}
}

func TestAutomationJobsUpdateScheduleReliability(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve existing reliability when only the schedule changes", func(t *testing.T) {
		t.Parallel()

		var request AutomationJobUpdateRequest
		deps := newTestDeps(t, &stubClient{
			getAutomationJobFn: func(context.Context, string) (JobRecord, error) {
				job := sampleAutomationJobRecord()
				job.Schedule.CatchUpPolicy = automationpkg.SchedulerCatchUpPolicyReplay
				job.Schedule.MisfireGraceSeconds = 30
				return job, nil
			},
			updateAutomationJobFn: func(
				_ context.Context,
				_ string,
				got AutomationJobUpdateRequest,
			) (JobRecord, error) {
				request = got
				return sampleAutomationJobRecord(), nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"automation", "jobs", "update", "job-1",
			"--schedule", "every:1h",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(automation jobs update schedule) error = %v", err)
		}
		if request.Schedule == nil || request.Schedule.Mode != automationpkg.ScheduleModeEvery ||
			request.Schedule.Interval != "1h" {
			t.Fatalf("request.Schedule = %#v, want every 1h", request.Schedule)
		}
		if request.Schedule.CatchUpPolicy != automationpkg.SchedulerCatchUpPolicyReplay ||
			request.Schedule.MisfireGraceSeconds != 30 {
			t.Fatalf("request.Schedule = %#v, want preserved replay policy with 30 second grace", request.Schedule)
		}
	})

	t.Run("Should preserve the existing recurring schedule when only reliability changes", func(t *testing.T) {
		t.Parallel()

		var request AutomationJobUpdateRequest
		deps := newTestDeps(t, &stubClient{
			getAutomationJobFn: func(_ context.Context, id string) (JobRecord, error) {
				if id != "job-1" {
					t.Fatalf("GetAutomationJob id = %q, want job-1", id)
				}
				job := sampleAutomationJobRecord()
				job.Schedule.CatchUpPolicy = automationpkg.SchedulerCatchUpPolicyReplay
				job.Schedule.MisfireGraceSeconds = 30
				return job, nil
			},
			updateAutomationJobFn: func(
				_ context.Context,
				id string,
				got AutomationJobUpdateRequest,
			) (JobRecord, error) {
				if id != "job-1" {
					t.Fatalf("UpdateAutomationJob id = %q, want job-1", id)
				}
				request = got
				return sampleAutomationJobRecord(), nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"automation", "jobs", "update", "job-1",
			"--catch-up-policy", "run_once_on_catchup",
			"--misfire-grace-seconds", "120",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(automation jobs update reliability) error = %v", err)
		}
		if request.Schedule == nil {
			t.Fatal("request.Schedule = nil, want recurring schedule patch")
		}
		if request.Schedule.Mode != automationpkg.ScheduleModeEvery || request.Schedule.Interval != "30m" {
			t.Fatalf("request.Schedule = %#v, want preserved every 30m schedule", request.Schedule)
		}
		if request.Schedule.CatchUpPolicy != automationpkg.SchedulerCatchUpPolicyRunOnce ||
			request.Schedule.MisfireGraceSeconds != 120 {
			t.Fatalf("request.Schedule = %#v, want run once with 120 second grace", request.Schedule)
		}
	})

	t.Run("Should reset an explicit catch up policy to the target default", func(t *testing.T) {
		t.Parallel()

		var request AutomationJobUpdateRequest
		deps := newTestDeps(t, &stubClient{
			getAutomationJobFn: func(context.Context, string) (JobRecord, error) {
				job := sampleAutomationJobRecord()
				job.Schedule.CatchUpPolicy = automationpkg.SchedulerCatchUpPolicyReplay
				job.Schedule.MisfireGraceSeconds = 30
				return job, nil
			},
			updateAutomationJobFn: func(
				_ context.Context,
				_ string,
				got AutomationJobUpdateRequest,
			) (JobRecord, error) {
				request = got
				return sampleAutomationJobRecord(), nil
			},
		})

		_, _, err := executeRootCommand(
			t,
			deps,
			"automation", "jobs", "update", "job-1",
			"--catch-up-policy", "default",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(automation jobs reset catch up policy) error = %v", err)
		}
		if request.Schedule == nil {
			t.Fatal("request.Schedule = nil, want recurring schedule patch")
		}
		if request.Schedule.CatchUpPolicy != "" || request.Schedule.MisfireGraceSeconds != 30 {
			t.Fatalf("request.Schedule = %#v, want default policy with preserved grace", request.Schedule)
		}
	})
}

func TestAutomationJobsCreateRejectsMissingWorkspaceForWorkspaceScope(t *testing.T) {
	t.Parallel()

	_, _, err := executeRootCommand(
		t,
		newTestDeps(t, &stubClient{}),
		"automation", "jobs", "create",
		"--name", "nightly-review",
		"--scope", "workspace",
		"--schedule", "every:30m",
		"--agent", "coder",
		"--prompt", "review repo",
	)
	if err == nil || !strings.Contains(err.Error(), "--workspace is required when --scope is workspace") {
		t.Fatalf("missing workspace error = %v, want workspace requirement", err)
	}
}

func TestAutomationJobsCreateSupportsHumanAndJSONOutput(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		createAutomationJobFn: func(context.Context, AutomationJobCreateRequest) (JobRecord, error) {
			return sampleAutomationJobRecord(), nil
		},
	})

	humanOut, _, err := executeRootCommand(
		t,
		deps,
		"automation", "jobs", "create",
		"--name", "nightly-review",
		"--scope", "global",
		"--schedule", "every:30m",
		"--agent", "coder",
		"--prompt", "review repo",
		"-o", "human",
	)
	if err != nil {
		t.Fatalf("automation jobs create human error = %v", err)
	}
	if !strings.Contains(humanOut, "Automation Job") || !strings.Contains(humanOut, "nightly-review") {
		t.Fatalf("human output = %q, want Automation Job details", humanOut)
	}

	jsonOut, _, err := executeRootCommand(
		t,
		deps,
		"automation", "jobs", "create",
		"--name", "nightly-review",
		"--scope", "global",
		"--schedule", "every:30m",
		"--agent", "coder",
		"--prompt", "review repo",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("automation jobs create json error = %v", err)
	}

	var created JobRecord
	if err := json.Unmarshal([]byte(jsonOut), &created); err != nil {
		t.Fatalf("json.Unmarshal(job create json) error = %v", err)
	}
	if created.ID != "job-1" {
		t.Fatalf("created.ID = %q, want %q", created.ID, "job-1")
	}
}

func TestAutomationTriggersCreateParsesFilters(t *testing.T) {
	t.Parallel()

	var request AutomationTriggerCreateRequest
	deps := newTestDeps(t, &stubClient{
		getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			if ref != "alpha" {
				t.Fatalf("GetWorkspace ref = %q, want %q", ref, "alpha")
			}
			return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
		},
		createAutomationTriggerFn: func(_ context.Context, got AutomationTriggerCreateRequest) (TriggerRecord, error) {
			request = got
			return sampleAutomationTriggerRecord(), nil
		},
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"automation", "triggers", "create",
		"--name", "branch-review",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--event", "session.stopped",
		"--filter", "data.branch=main,data.repo=agh",
		"--agent", "coder",
		"--prompt", `review {{ index .Data "session_id" }}`,
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("executeRootCommand(automation triggers create) error = %v", err)
	}

	if request.Scope != automationpkg.AutomationScopeWorkspace || request.WorkspaceID != "ws-alpha" {
		t.Fatalf("request scope/workspace = %#v, want workspace ws-alpha", request)
	}
	if request.Filter["data.branch"] != "main" || request.Filter["data.repo"] != "agh" {
		t.Fatalf("request.Filter = %#v, want parsed filter map", request.Filter)
	}

	var created TriggerRecord
	if err := json.Unmarshal([]byte(stdout), &created); err != nil {
		t.Fatalf("json.Unmarshal(trigger create) error = %v", err)
	}
	if created.ID != "trg-1" {
		t.Fatalf("created.ID = %q, want %q", created.ID, "trg-1")
	}
}

func TestAutomationTriggersCreateRejectsMalformedFilter(t *testing.T) {
	t.Parallel()

	_, _, err := executeRootCommand(
		t,
		newTestDeps(t, &stubClient{}),
		"automation", "triggers", "create",
		"--name", "branch-review",
		"--scope", "global",
		"--event", "session.stopped",
		"--filter", "data.branch",
		"--agent", "coder",
		"--prompt", `review {{ index .Data "session_id" }}`,
	)
	if err == nil || !strings.Contains(err.Error(), "invalid filter") {
		t.Fatalf("malformed filter error = %v, want invalid filter", err)
	}
}

func TestAutomationJobUpdateSurfacesConfigBackedMutationError(t *testing.T) {
	t.Parallel()

	expected := "automation validation error: config-backed automation jobs only accept enabled updates"
	deps := newTestDeps(t, &stubClient{
		updateAutomationJobFn: func(context.Context, string, AutomationJobUpdateRequest) (JobRecord, error) {
			return JobRecord{}, errors.New(expected)
		},
	})

	_, _, err := executeRootCommand(
		t,
		deps,
		"automation", "jobs", "update", "job-config",
		"--prompt", "changed",
	)
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("config-backed update error = %v, want %q", err, expected)
	}
}

func TestAutomationJobsListAndUpdateCommands(t *testing.T) {
	t.Parallel()

	var (
		listQuery     AutomationJobQuery
		updateRequest AutomationJobUpdateRequest
	)

	deps := newTestDeps(t, &stubClient{
		getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			if ref != "alpha" {
				t.Fatalf("GetWorkspace ref = %q, want %q", ref, "alpha")
			}
			return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
		},
		listAutomationJobsFn: func(_ context.Context, query AutomationJobQuery) (AutomationJobListRecord, error) {
			listQuery = query
			return AutomationJobListRecord{
				Jobs: []JobRecord{sampleAutomationJobRecord()},
				Page: contract.CountedCursorPagePayload{
					Total:      4,
					Limit:      query.Limit,
					HasMore:    true,
					NextCursor: "job-next",
				},
			}, nil
		},
		updateAutomationJobFn: func(_ context.Context, _ string, request AutomationJobUpdateRequest) (JobRecord, error) {
			updateRequest = request
			updated := sampleAutomationJobRecord()
			updated.Name = *request.Name
			updated.Prompt = *request.Prompt
			updated.Schedule = request.Schedule
			updated.Retry = *request.Retry
			updated.Enabled = *request.Enabled
			return updated, nil
		},
	})
	jobCursor := automationJobCursorForTest(t, AutomationJobQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Source:      automationpkg.JobSourcePackage,
		Enabled:     new(false),
		LoopName:    "triage",
		Search:      "digest",
	})

	listJSON, _, err := executeRootCommand(
		t,
		deps,
		"automation", "jobs",
		"--scope", "workspace",
		"--workspace", "alpha",
		"--source", "package",
		"--enabled=false",
		"--loop", "triage",
		"--query", "digest",
		"--cursor", jobCursor,
		"--limit", "3",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("automation jobs list error = %v", err)
	}

	var listed contract.JobsResponse
	if err := json.Unmarshal([]byte(listJSON), &listed); err != nil {
		t.Fatalf("json.Unmarshal(job list) error = %v", err)
	}
	if len(listed.Jobs) != 1 || listQuery.Scope != automationpkg.AutomationScopeWorkspace ||
		listQuery.WorkspaceID != "ws-alpha" ||
		listQuery.Source != automationpkg.JobSourcePackage ||
		listQuery.Enabled == nil || *listQuery.Enabled ||
		listQuery.LoopName != "triage" ||
		listQuery.Search != "digest" ||
		listQuery.Cursor != jobCursor ||
		listQuery.Limit != 3 {
		t.Fatalf("listQuery = %#v, want resolved scope/workspace/source/limit", listQuery)
	}

	updatedJSON, _, err := executeRootCommand(
		t,
		deps,
		"automation", "jobs", "update", "job-1",
		"--name", "nightly-digest",
		"--prompt", "digest repo activity",
		"--schedule", "at:2026-04-15T15:00",
		"--retry", "none",
		"--enabled=false",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("automation jobs update error = %v", err)
	}

	var updated JobRecord
	if err := json.Unmarshal([]byte(updatedJSON), &updated); err != nil {
		t.Fatalf("json.Unmarshal(job update) error = %v", err)
	}
	if updated.Name != "nightly-digest" || updated.AgentName != "coder" || updated.WorkspaceID != "ws-alpha" ||
		updated.Schedule == nil ||
		updated.Schedule.Mode != automationpkg.ScheduleModeAt {
		t.Fatalf("updated job = %#v, want updated values", updated)
	}
	if updateRequest.Schedule == nil || updateRequest.Schedule.Mode != automationpkg.ScheduleModeAt ||
		updateRequest.Schedule.Time != "2026-04-15T15:00:00Z" {
		t.Fatalf("updateRequest.Schedule = %#v, want normalized at schedule", updateRequest.Schedule)
	}
	if updateRequest.Retry == nil || updateRequest.Retry.Strategy != automationpkg.RetryStrategyNone {
		t.Fatalf("updateRequest.Retry = %#v, want retry none", updateRequest.Retry)
	}
	if updateRequest.Enabled == nil || *updateRequest.Enabled {
		t.Fatalf("updateRequest.Enabled = %#v, want false pointer", updateRequest.Enabled)
	}
}

func TestAutomationCommandsSupportToonOutput(t *testing.T) {
	t.Parallel()

	deps := newTestDeps(t, &stubClient{
		listAutomationJobsFn: func(_ context.Context, query AutomationJobQuery) (AutomationJobListRecord, error) {
			return AutomationJobListRecord{
				Jobs: []JobRecord{sampleAutomationJobRecord()},
				Page: contract.CountedCursorPagePayload{Total: 1, Limit: query.Limit},
			}, nil
		},
		getAutomationTriggerFn: func(context.Context, string) (TriggerRecord, error) {
			return sampleAutomationTriggerRecord(), nil
		},
		listAutomationRunsFn: func(context.Context, AutomationRunQuery) ([]RunRecord, error) {
			return []RunRecord{sampleAutomationRunRecord()}, nil
		},
	})

	jobsToon, _, err := executeRootCommand(t, deps, "automation", "jobs", "-o", "toon")
	if err != nil {
		t.Fatalf("automation jobs toon error = %v", err)
	}
	if !strings.Contains(
		jobsToon,
		"automation_jobs[1]{id,name,scope,workspace_id,schedule,agent_name,enabled,source,next_run}:",
	) {
		t.Fatalf("jobs toon output = %q, want automation_jobs TOON array", jobsToon)
	}

	triggerToon, _, err := executeRootCommand(t, deps, "automation", "triggers", "get", "trg-1", "-o", "toon")
	if err != nil {
		t.Fatalf("automation triggers get toon error = %v", err)
	}
	if !strings.Contains(
		triggerToon,
		"automation_trigger{id,name,scope,workspace_id,agent_name,event,enabled,source,retry,fire_limit,webhook_id,endpoint_slug,webhook_path,created_at,updated_at,prompt}:",
	) {
		t.Fatalf("trigger toon output = %q, want automation_trigger TOON object", triggerToon)
	}

	runsToon, _, err := executeRootCommand(t, deps, "automation", "runs", "-o", "toon")
	if err != nil {
		t.Fatalf("automation runs toon error = %v", err)
	}
	if !strings.Contains(
		runsToon,
		"automation_runs[1]{id,target,status,attempt,session_id,scheduled_at,started_at,ended_at,error,delivery_error}:",
	) {
		t.Fatalf("runs toon output = %q, want automation_runs TOON array", runsToon)
	}
}

func TestAutomationAdditionalCommandsAndQueries(t *testing.T) {
	t.Parallel()

	var (
		listTriggerQuery     AutomationTriggerQuery
		jobHistoryQuery      AutomationRunQuery
		updateTriggerRequest AutomationTriggerUpdateRequest
		runsQuery            AutomationRunQuery
	)

	deps := newTestDeps(t, &stubClient{
		getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			if ref != "alpha" {
				t.Fatalf("GetWorkspace ref = %q, want %q", ref, "alpha")
			}
			return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-alpha"}}, nil
		},
		listAutomationTriggersFn: func(_ context.Context, query AutomationTriggerQuery) (AutomationTriggerListRecord, error) {
			listTriggerQuery = query
			return AutomationTriggerListRecord{
				Triggers: []TriggerRecord{sampleAutomationTriggerRecord()},
				Page: contract.CountedCursorPagePayload{
					Total:      3,
					Limit:      query.Limit,
					HasMore:    true,
					NextCursor: "trigger-next",
				},
			}, nil
		},
		getAutomationJobFn: func(context.Context, string) (JobRecord, error) {
			return sampleAutomationJobRecord(), nil
		},
		deleteAutomationJobFn: func(context.Context, string) error {
			return nil
		},
		triggerAutomationJobFn: func(context.Context, string) (RunRecord, error) {
			return sampleAutomationRunRecord(), nil
		},
		automationJobRunsFn: func(_ context.Context, _ string, query AutomationRunQuery) ([]RunRecord, error) {
			jobHistoryQuery = query
			return []RunRecord{sampleAutomationRunRecord()}, nil
		},
		getAutomationTriggerFn: func(context.Context, string) (TriggerRecord, error) {
			return sampleAutomationTriggerRecord(), nil
		},
		updateAutomationTriggerFn: func(_ context.Context, _ string, request AutomationTriggerUpdateRequest) (TriggerRecord, error) {
			updateTriggerRequest = request
			updated := sampleAutomationTriggerRecord()
			updated.Prompt = *request.Prompt
			updated.Enabled = *request.Enabled
			return updated, nil
		},
		deleteAutomationTriggerFn: func(context.Context, string) error {
			return nil
		},
		automationTriggerRunsFn: func(context.Context, string, AutomationRunQuery) ([]RunRecord, error) {
			return []RunRecord{sampleAutomationRunRecord()}, nil
		},
		listAutomationRunsFn: func(_ context.Context, query AutomationRunQuery) ([]RunRecord, error) {
			runsQuery = query
			return []RunRecord{sampleAutomationRunRecord()}, nil
		},
		getAutomationRunFn: func(context.Context, string) (RunRecord, error) {
			return sampleAutomationRunRecord(), nil
		},
	})
	triggerCursor := automationTriggerCursorForTest(t, AutomationTriggerQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Event:       "webhook",
		Source:      automationpkg.JobSourcePackage,
		Enabled:     new(true),
		LoopName:    "triage",
		Search:      "review",
	})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"automation",
		"triggers",
		"--scope",
		"workspace",
		"--workspace",
		"alpha",
		"--event",
		"webhook",
		"--source",
		"package",
		"--enabled=true",
		"--loop",
		"triage",
		"--query",
		"review",
		"--cursor",
		triggerCursor,
		"--limit",
		"2",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("automation triggers list error = %v", err)
	}
	var listed contract.TriggersResponse
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
		t.Fatalf("json.Unmarshal(trigger list) error = %v", err)
	}
	if len(listed.Triggers) != 1 || listTriggerQuery.WorkspaceID != "ws-alpha" ||
		listTriggerQuery.Scope != automationpkg.AutomationScopeWorkspace ||
		listTriggerQuery.Event != "webhook" ||
		listTriggerQuery.Source != automationpkg.JobSourcePackage ||
		listTriggerQuery.Enabled == nil || !*listTriggerQuery.Enabled ||
		listTriggerQuery.LoopName != "triage" ||
		listTriggerQuery.Search != "review" ||
		listTriggerQuery.Cursor != triggerCursor ||
		listTriggerQuery.Limit != 2 {
		t.Fatalf("listTriggerQuery = %#v, want resolved workspace/event/source/limit", listTriggerQuery)
	}

	jobHuman, _, err := executeRootCommand(t, deps, "automation", "jobs", "get", "job-1", "-o", "human")
	if err != nil {
		t.Fatalf("automation jobs get human error = %v", err)
	}
	if !strings.Contains(jobHuman, "Automation Job") {
		t.Fatalf("job human output = %q, want Automation Job section", jobHuman)
	}

	deletedJob, _, err := executeRootCommand(t, deps, "automation", "jobs", "delete", "job-1", "-o", "json")
	if err != nil {
		t.Fatalf("automation jobs delete error = %v", err)
	}
	var deleted JobRecord
	if err := json.Unmarshal([]byte(deletedJob), &deleted); err != nil {
		t.Fatalf("json.Unmarshal(job delete) error = %v", err)
	}
	if deleted.ID != "job-1" {
		t.Fatalf("deleted job = %#v, want job-1", deleted)
	}

	triggeredRun, _, err := executeRootCommand(t, deps, "automation", "jobs", "trigger", "job-1", "-o", "json")
	if err != nil {
		t.Fatalf("automation jobs trigger error = %v", err)
	}
	var run RunRecord
	if err := json.Unmarshal([]byte(triggeredRun), &run); err != nil {
		t.Fatalf("json.Unmarshal(job trigger) error = %v", err)
	}
	if run.ID != "run-1" {
		t.Fatalf("triggered run = %#v, want run-1", run)
	}

	_, _, err = executeRootCommand(
		t,
		deps,
		"automation",
		"jobs",
		"history",
		"job-1",
		"--status",
		"completed",
		"--since",
		"1h",
		"--until",
		"30m",
		"--last",
		"2",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("automation jobs history error = %v", err)
	}
	if jobHistoryQuery.Status != automationpkg.RunCompleted || jobHistoryQuery.Limit != 2 ||
		jobHistoryQuery.Since.IsZero() ||
		jobHistoryQuery.Until.IsZero() {
		t.Fatalf("jobHistoryQuery = %#v, want parsed run filters", jobHistoryQuery)
	}

	updatedTriggerJSON, _, err := executeRootCommand(
		t,
		deps,
		"automation", "triggers", "update", "trg-1",
		"--prompt", `inspect {{ index .Data "session_id" }}`,
		"--event", "webhook",
		"--filter", "data.branch=main",
		"--retry", "backoff",
		"--enabled=false",
		"--webhook-id", "wbh_123",
		"--endpoint-slug", "branch-review",
		"--webhook-secret-value", "shared-secret",
		"-o", "json",
	)
	if err != nil {
		t.Fatalf("automation triggers update error = %v", err)
	}
	var updated TriggerRecord
	if err := json.Unmarshal([]byte(updatedTriggerJSON), &updated); err != nil {
		t.Fatalf("json.Unmarshal(trigger update) error = %v", err)
	}
	if updated.ID != "trg-1" ||
		updateTriggerRequest.Event == nil ||
		*updateTriggerRequest.Event != "webhook" ||
		updateTriggerRequest.Retry == nil ||
		updateTriggerRequest.Retry.Strategy != automationpkg.RetryStrategyBackoff ||
		updateTriggerRequest.Filter["data.branch"] != "main" ||
		updateTriggerRequest.WebhookID == nil ||
		*updateTriggerRequest.WebhookID != "wbh_123" ||
		updateTriggerRequest.EndpointSlug == nil ||
		*updateTriggerRequest.EndpointSlug != "branch-review" ||
		updateTriggerRequest.WebhookSecretValue == nil ||
		*updateTriggerRequest.WebhookSecretValue != "shared-secret" {
		t.Fatalf("updateTriggerRequest = %#v, want parsed trigger update request", updateTriggerRequest)
	}

	deletedTriggerJSON, _, err := executeRootCommand(t, deps, "automation", "triggers", "delete", "trg-1", "-o", "json")
	if err != nil {
		t.Fatalf("automation triggers delete error = %v", err)
	}
	var deletedTrigger TriggerRecord
	if err := json.Unmarshal([]byte(deletedTriggerJSON), &deletedTrigger); err != nil {
		t.Fatalf("json.Unmarshal(trigger delete) error = %v", err)
	}
	if deletedTrigger.ID != "trg-1" {
		t.Fatalf("deleted trigger = %#v, want trg-1", deletedTrigger)
	}

	triggerHistoryHuman, _, err := executeRootCommand(
		t,
		deps,
		"automation",
		"triggers",
		"history",
		"trg-1",
		"--status",
		"completed",
		"--last",
		"1",
		"-o",
		"human",
	)
	if err != nil {
		t.Fatalf("automation triggers history human error = %v", err)
	}
	if !strings.Contains(triggerHistoryHuman, "Automation Runs") {
		t.Fatalf("trigger history human output = %q, want Automation Runs table", triggerHistoryHuman)
	}

	_, _, err = executeRootCommand(
		t,
		deps,
		"automation",
		"runs",
		"--job-id",
		"job-1",
		"--trigger-id",
		"trg-1",
		"--status",
		"completed",
		"--since",
		"1h",
		"--until",
		"30m",
		"--last",
		"4",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("automation runs list error = %v", err)
	}
	if runsQuery.JobID != "job-1" || runsQuery.TriggerID != "trg-1" || runsQuery.Status != automationpkg.RunCompleted ||
		runsQuery.Limit != 4 {
		t.Fatalf("runsQuery = %#v, want parsed runs query", runsQuery)
	}

	runHuman, _, err := executeRootCommand(t, deps, "automation", "runs", "get", "run-1", "-o", "human")
	if err != nil {
		t.Fatalf("automation runs get human error = %v", err)
	}
	if !strings.Contains(runHuman, "Automation Run") {
		t.Fatalf("run human output = %q, want Automation Run section", runHuman)
	}
}

func TestAutomationHelperFormattingAndParsing(t *testing.T) {
	t.Parallel()

	normalized, err := normalizeAutomationAtTime("2026-04-15T15:00")
	if err != nil {
		t.Fatalf("normalizeAutomationAtTime() error = %v", err)
	}
	if normalized != "2026-04-15T15:00:00Z" {
		t.Fatalf("normalizeAutomationAtTime() = %q, want %q", normalized, "2026-04-15T15:00:00Z")
	}

	if _, err := normalizeAutomationAtTime("bad"); err == nil {
		t.Fatal("normalizeAutomationAtTime(bad) error = nil, want non-nil")
	}

	since, err := parseAutomationOptionalTimeFlag("1h", "since", func() time.Time { return fixedTestNow })
	if err != nil {
		t.Fatalf("parseAutomationOptionalTimeFlag() error = %v", err)
	}
	if want := fixedTestNow.Add(-time.Hour); !since.Equal(want) {
		t.Fatalf("parseAutomationOptionalTimeFlag() = %v, want %v", since, want)
	}

	if _, err := parseAutomationOptionalTimeFlag("-1h", "since", func() time.Time { return fixedTestNow }); err == nil {
		t.Fatal("parseAutomationOptionalTimeFlag(-1h) error = nil, want non-nil")
	}

	if source, err := parseOptionalAutomationSource("dynamic"); err != nil || source != automationpkg.JobSourceDynamic {
		t.Fatalf("parseOptionalAutomationSource() = %q, %v", source, err)
	}
	if status, err := parseOptionalAutomationRunStatus(
		"completed",
	); err != nil ||
		status != automationpkg.RunCompleted {
		t.Fatalf("parseOptionalAutomationRunStatus() = %q, %v", status, err)
	}

	triggerListHuman, err := automationTriggerListBundle(AutomationTriggerListRecord{
		Triggers: []TriggerRecord{sampleAutomationTriggerRecord()},
		Page:     contract.CountedCursorPagePayload{Total: 1, Limit: automationpkg.DefaultListLimit},
	}).human()
	if err != nil {
		t.Fatalf("automationTriggerListBundle().human() error = %v", err)
	}
	if !strings.Contains(triggerListHuman, "Automation Triggers") {
		t.Fatalf("trigger list human output = %q, want Automation Triggers title", triggerListHuman)
	}

	runBundleHuman, err := automationRunBundle(sampleAutomationRunRecord()).human()
	if err != nil {
		t.Fatalf("automationRunBundle().human() error = %v", err)
	}
	if !strings.Contains(runBundleHuman, "Automation Run") || !strings.Contains(runBundleHuman, "job:job-1") {
		t.Fatalf("run bundle human output = %q, want Automation Run details", runBundleHuman)
	}

	if got := formatAutomationSchedule(
		&automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeCron, Expr: "0 9 * * *"},
	); got != "cron:0 9 * * *" {
		t.Fatalf("formatAutomationSchedule(cron) = %q, want cron prefix", got)
	}
	if got := displayTriggerEndpoint(
		sampleAutomationTriggerRecord(),
	); !strings.Contains(
		got,
		"/api/webhooks/workspaces/ws-alpha/",
	) {
		t.Fatalf("displayTriggerEndpoint() = %q, want workspace webhook path", got)
	}
	if got := displayRunTarget(sampleAutomationRunRecord()); got != "job:job-1" {
		t.Fatalf("displayRunTarget() = %q, want job target", got)
	}
	if ptr := new(true); ptr == nil || !*ptr {
		t.Fatalf("new(true) = %#v, want true pointer", ptr)
	}
}

func sampleAutomationJobRecord() JobRecord {
	nextRun := fixedTestNow.Add(time.Hour)
	return JobRecord{
		ID:          "job-1",
		Scope:       automationpkg.AutomationScopeWorkspace,
		Name:        "nightly-review",
		AgentName:   "coder",
		WorkspaceID: "ws-alpha",
		Prompt:      "review repo",
		Schedule: &automationpkg.ScheduleSpec{
			Mode:     automationpkg.ScheduleModeEvery,
			Interval: "30m",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourceDynamic,
		CreatedAt: fixedTestNow,
		UpdatedAt: fixedTestNow,
		NextRun:   &nextRun,
	}
}

func sampleAutomationSuggestionRecord() SuggestionRecord {
	return SuggestionRecord{
		ID:          "suggestion-1",
		WorkspaceID: "ws-alpha",
		Source:      automationpkg.SuggestionSourceCatalog,
		DedupKey:    "catalog:v1:daily-workspace-briefing",
		Status:      automationpkg.SuggestionStatusPending,
		Payload:     sampleAutomationJobRecord(),
		CreatedAt:   fixedTestNow,
	}
}

func automationJobCursorForTest(t *testing.T, query AutomationJobQuery) string {
	t.Helper()

	source := query.Source
	if source == "" {
		source = automationpkg.JobSourceDynamic
	}
	enabled := query.Enabled != nil && *query.Enabled
	jobs := make([]automationpkg.Job, 0, 2)
	for index, name := range []string{"digest-alpha", "digest-bravo"} {
		jobs = append(jobs, automationpkg.Job{
			ID:          fmt.Sprintf("job-cursor-%d", index),
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        name,
			WorkspaceID: "ws-alpha",
			Source:      source,
			Enabled:     enabled,
			TargetKind:  automationpkg.TargetKindLoop,
			LoopTarget:  &automationpkg.LoopTarget{WorkspaceID: "ws-alpha", LoopName: "triage"},
		})
	}
	query.Limit = 1
	page, err := automationpkg.BuildJobListPage(jobs, query)
	if err != nil {
		t.Fatalf("BuildJobListPage() error = %v", err)
	}
	if page.NextCursor == "" {
		t.Fatal("BuildJobListPage().NextCursor = empty")
	}
	return page.NextCursor
}

func automationTriggerCursorForTest(t *testing.T, query AutomationTriggerQuery) string {
	t.Helper()

	source := query.Source
	if source == "" {
		source = automationpkg.JobSourceDynamic
	}
	enabled := query.Enabled != nil && *query.Enabled
	triggers := make([]automationpkg.Trigger, 0, 2)
	for index, name := range []string{"review-alpha", "review-bravo"} {
		triggers = append(triggers, automationpkg.Trigger{
			ID:          fmt.Sprintf("trigger-cursor-%d", index),
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        name,
			WorkspaceID: "ws-alpha",
			Event:       "webhook",
			Source:      source,
			Enabled:     enabled,
			TargetKind:  automationpkg.TargetKindLoop,
			LoopTarget:  &automationpkg.LoopTarget{WorkspaceID: "ws-alpha", LoopName: "triage"},
		})
	}
	query.Limit = 1
	page, err := automationpkg.BuildTriggerListPage(triggers, query)
	if err != nil {
		t.Fatalf("BuildTriggerListPage() error = %v", err)
	}
	if page.NextCursor == "" {
		t.Fatal("BuildTriggerListPage().NextCursor = empty")
	}
	return page.NextCursor
}

func sampleAutomationTriggerRecord() TriggerRecord {
	return TriggerRecord{
		ID:           "trg-1",
		Scope:        automationpkg.AutomationScopeWorkspace,
		Name:         "branch-review",
		AgentName:    "coder",
		WorkspaceID:  "ws-alpha",
		Prompt:       `review {{ index .Data "session_id" }}`,
		Event:        "webhook",
		Filter:       map[string]string{"data.branch": "main"},
		Enabled:      true,
		Retry:        automationpkg.DefaultRetryConfig(),
		FireLimit:    automationpkg.DefaultFireLimitConfig(),
		Source:       automationpkg.JobSourceDynamic,
		WebhookID:    "wbh_123",
		EndpointSlug: "branch-review",
		CreatedAt:    fixedTestNow,
		UpdatedAt:    fixedTestNow,
	}
}

func sampleAutomationRunRecord() RunRecord {
	started := fixedTestNow
	ended := fixedTestNow.Add(2 * time.Minute)
	return RunRecord{
		ID:        "run-1",
		JobID:     "job-1",
		SessionID: "sess-1",
		Status:    automationpkg.RunCompleted,
		Attempt:   1,
		StartedAt: &started,
		EndedAt:   &ended,
	}
}
