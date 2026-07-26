package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestAutomationJobManagedLifecycleAllowsOnlyEnabledUpdates(t *testing.T) {
	t.Parallel()

	for _, source := range []automationpkg.JobSource{
		automationpkg.JobSourceConfig,
		automationpkg.JobSourcePackage,
	} {
		t.Run("Should enforce the managed lifecycle for "+string(source)+" jobs", func(t *testing.T) {
			t.Parallel()

			current := automationpkg.Job{
				ID:        "job-managed",
				Scope:     automationpkg.AutomationScopeGlobal,
				Name:      "managed-job",
				AgentName: "coder",
				Prompt:    "do work",
				Schedule: &automationpkg.ScheduleSpec{
					Mode:     automationpkg.ScheduleModeEvery,
					Interval: "1h",
				},
				Enabled:   true,
				Retry:     automationpkg.DefaultRetryConfig(),
				FireLimit: automationpkg.DefaultFireLimitConfig(),
				Source:    source,
			}
			setCalled := false
			router := newAutomationCoreTestRouter(t, stubAutomationManager{
				GetJobFn: func(_ context.Context, id string) (automationpkg.Job, error) {
					if id != current.ID {
						t.Fatalf("GetJob() id = %q, want %q", id, current.ID)
					}
					return current, nil
				},
				SetJobEnabledFn: func(_ context.Context, id string, enabled bool) (automationpkg.Job, error) {
					setCalled = true
					if id != current.ID || enabled {
						t.Fatalf("SetJobEnabled() = (%q, %v), want (%q, false)", id, enabled, current.ID)
					}
					next := current
					next.Enabled = false
					return next, nil
				},
				UpdateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
					t.Fatal("UpdateJob() should not be called for managed job validation")
					return automationpkg.Job{}, nil
				},
				DeleteJobFn: func(context.Context, string) error {
					t.Fatal("DeleteJob() should not be called for a managed job")
					return nil
				},
			})

			invalid := performAutomationCoreRequest(t, router, http.MethodPatch,
				"/automation/jobs/"+current.ID, []byte(`{"enabled":false,"prompt":"changed"}`), nil)
			if invalid.Code != http.StatusBadRequest {
				t.Fatalf(
					"invalid status = %d, want %d; body=%s",
					invalid.Code,
					http.StatusBadRequest,
					invalid.Body.String(),
				)
			}
			if !strings.Contains(invalid.Body.String(), "managed automation jobs only accept enabled updates") {
				t.Fatalf("invalid body = %q, want managed enabled-only error", invalid.Body.String())
			}
			if setCalled {
				t.Fatal("SetJobEnabled() called for rejected managed definition edit")
			}

			valid := performAutomationCoreRequest(t, router, http.MethodPatch,
				"/automation/jobs/"+current.ID, []byte(`{"enabled":false}`), nil)
			if valid.Code != http.StatusOK {
				t.Fatalf(
					"valid status = %d, want %d; body=%s",
					valid.Code,
					http.StatusOK,
					valid.Body.String(),
				)
			}
			if !setCalled {
				t.Fatal("SetJobEnabled() not called for enabled-only managed update")
			}

			var response struct {
				Job struct {
					Enabled bool `json:"enabled"`
				} `json:"job"`
			}
			decodeAutomationCoreJSON(t, valid, &response)
			if response.Job.Enabled {
				t.Fatalf("response.job.enabled = %v, want false", response.Job.Enabled)
			}

			deleted := performAutomationCoreRequest(t, router, http.MethodDelete,
				"/automation/jobs/"+current.ID, nil, nil)
			if deleted.Code != http.StatusBadRequest {
				t.Fatalf(
					"delete status = %d, want %d; body=%s",
					deleted.Code,
					http.StatusBadRequest,
					deleted.Body.String(),
				)
			}
			if !strings.Contains(deleted.Body.String(), "managed automation jobs cannot be deleted") {
				t.Fatalf("delete body = %q, want managed delete error", deleted.Body.String())
			}
		})
	}
}

func TestAutomationTriggerManagedLifecycleAllowsOnlyEnabledUpdates(t *testing.T) {
	t.Parallel()

	for _, source := range []automationpkg.JobSource{
		automationpkg.JobSourceConfig,
		automationpkg.JobSourcePackage,
	} {
		t.Run("Should enforce the managed lifecycle for "+string(source)+" triggers", func(t *testing.T) {
			t.Parallel()

			current := automationpkg.Trigger{
				ID:        "trigger-managed",
				Scope:     automationpkg.AutomationScopeGlobal,
				Name:      "managed-trigger",
				AgentName: "coder",
				Prompt:    `review {{ index .Data "session_id" }}`,
				Event:     "session.stopped",
				Enabled:   true,
				Retry:     automationpkg.DefaultRetryConfig(),
				FireLimit: automationpkg.DefaultFireLimitConfig(),
				Source:    source,
			}
			setCalled := false
			router := newAutomationCoreTestRouter(t, stubAutomationManager{
				GetTriggerFn: func(_ context.Context, id string) (automationpkg.Trigger, error) {
					if id != current.ID {
						t.Fatalf("GetTrigger() id = %q, want %q", id, current.ID)
					}
					return current, nil
				},
				SetTriggerEnabledFn: func(
					_ context.Context,
					id string,
					enabled bool,
				) (automationpkg.Trigger, error) {
					setCalled = true
					if id != current.ID || enabled {
						t.Fatalf("SetTriggerEnabled() = (%q, %v), want (%q, false)", id, enabled, current.ID)
					}
					next := current
					next.Enabled = false
					return next, nil
				},
				UpdateTriggerFn: func(
					context.Context,
					automationpkg.Trigger,
					*automationpkg.WebhookSecretWrite,
				) (automationpkg.Trigger, error) {
					t.Fatal("UpdateTrigger() should not be called for managed trigger validation")
					return automationpkg.Trigger{}, nil
				},
				DeleteTriggerFn: func(context.Context, string) error {
					t.Fatal("DeleteTrigger() should not be called for a managed trigger")
					return nil
				},
			})

			invalid := performAutomationCoreRequest(t, router, http.MethodPatch,
				"/automation/triggers/"+current.ID, []byte(`{"enabled":false,"prompt":"changed"}`), nil)
			if invalid.Code != http.StatusBadRequest {
				t.Fatalf(
					"invalid status = %d, want %d; body=%s",
					invalid.Code,
					http.StatusBadRequest,
					invalid.Body.String(),
				)
			}
			if !strings.Contains(invalid.Body.String(), "managed automation triggers only accept enabled updates") {
				t.Fatalf("invalid body = %q, want managed enabled-only error", invalid.Body.String())
			}
			if setCalled {
				t.Fatal("SetTriggerEnabled() called for rejected managed definition edit")
			}

			valid := performAutomationCoreRequest(t, router, http.MethodPatch,
				"/automation/triggers/"+current.ID, []byte(`{"enabled":false}`), nil)
			if valid.Code != http.StatusOK {
				t.Fatalf(
					"valid status = %d, want %d; body=%s",
					valid.Code,
					http.StatusOK,
					valid.Body.String(),
				)
			}
			if !setCalled {
				t.Fatal("SetTriggerEnabled() not called for enabled-only managed update")
			}

			deleted := performAutomationCoreRequest(t, router, http.MethodDelete,
				"/automation/triggers/"+current.ID, nil, nil)
			if deleted.Code != http.StatusBadRequest {
				t.Fatalf(
					"delete status = %d, want %d; body=%s",
					deleted.Code,
					http.StatusBadRequest,
					deleted.Body.String(),
				)
			}
			if !strings.Contains(deleted.Body.String(), "managed automation triggers cannot be deleted") {
				t.Fatalf("delete body = %q, want managed delete error", deleted.Body.String())
			}
		})
	}
}

func TestCreateAutomationTriggerRejectsInvalidWebhookID(t *testing.T) {
	t.Parallel()

	router := newAutomationCoreTestRouter(t, stubAutomationManager{
		CreateTriggerFn: func(context.Context, automationpkg.Trigger, automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error) {
			t.Fatal("CreateTrigger() should not be called for invalid webhook id")
			return automationpkg.Trigger{}, nil
		},
	})

	response := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/automation/triggers",
		[]byte(
			`{"scope":"global","name":"deploy-review","agent_name":"coder","prompt":"review {{ .Kind }}","event":"webhook","endpoint_slug":"deploy-review","webhook_id":"qa-webhook-id","webhook_secret_value":"shared-secret"}`,
		),
		nil,
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	if body := response.Body.String(); !strings.Contains(body, "webhook_id") {
		t.Fatalf("body = %q, want webhook_id validation detail", body)
	}
}

func TestAutomationJobWriteHandlersIgnoreNextRunLookupFailures(t *testing.T) {
	t.Parallel()

	current := automationpkg.Job{
		ID:        "job-dynamic",
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      "nightly-review",
		AgentName: "coder",
		Prompt:    "review repo",
		Schedule: &automationpkg.ScheduleSpec{
			Mode:     automationpkg.ScheduleModeEvery,
			Interval: "1h",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourceDynamic,
	}

	t.Run("Should reject removed participation fields before creating a job", func(t *testing.T) {
		t.Parallel()

		router := newAutomationCoreTestRouter(t, stubAutomationManager{
			CreateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
				t.Fatal("CreateJob() should not run for a removed participation field")
				return automationpkg.Job{}, nil
			},
		})
		response := performAutomationCoreRequest(
			t,
			router,
			http.MethodPost,
			"/automation/jobs",
			[]byte(
				`{"scope":"global","name":"nightly-review","agent_name":"coder","prompt":"review repo","schedule":{"mode":"every","interval":"1h"},"task":{"title":"Review","network_channel":"builders"}}`,
			),
			nil,
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
		}
		body := response.Body.String()
		if !strings.Contains(body, "unknown_field") || !strings.Contains(body, "network_channel") {
			t.Fatalf("body = %q, want unknown_field with removed field named", body)
		}
	})

	t.Run("Should create a job even when next-run enrichment fails", func(t *testing.T) {
		t.Parallel()

		router := newAutomationCoreTestRouter(t, stubAutomationManager{
			CreateJobFn: func(_ context.Context, _ automationpkg.Job) (automationpkg.Job, error) {
				return current, nil
			},
			StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
				return automationpkg.ManagerStatus{}, errors.New("status unavailable")
			},
		})

		response := performAutomationCoreRequest(
			t,
			router,
			http.MethodPost,
			"/automation/jobs",
			[]byte(
				`{"scope":"global","name":"nightly-review","agent_name":"coder","prompt":"review repo","schedule":{"mode":"every","interval":"1h"}}`,
			),
			nil,
		)
		if response.Code != http.StatusCreated {
			t.Fatalf("create status = %d, want %d; body=%s", response.Code, http.StatusCreated, response.Body.String())
		}

		var payload map[string]any
		decodeAutomationCoreJSON(t, response, &payload)

		job, ok := payload["job"].(map[string]any)
		if !ok {
			t.Fatalf("job payload type = %T, want object", payload["job"])
		}
		if job["id"] != current.ID {
			t.Fatalf("job.id = %#v, want %q", job["id"], current.ID)
		}
		if nextRun, exists := job["next_run"]; exists && nextRun != nil {
			t.Fatalf("job.next_run = %#v, want omitted or nil", nextRun)
		}
	})

	t.Run("Should update a job even when next-run enrichment fails", func(t *testing.T) {
		t.Parallel()

		updated := current
		updated.Prompt = "updated prompt"

		router := newAutomationCoreTestRouter(t, stubAutomationManager{
			GetJobFn: func(_ context.Context, id string) (automationpkg.Job, error) {
				if id != current.ID {
					t.Fatalf("GetJob() id = %q, want %q", id, current.ID)
				}
				return current, nil
			},
			UpdateJobFn: func(_ context.Context, job automationpkg.Job) (automationpkg.Job, error) {
				if job.Prompt != updated.Prompt {
					t.Fatalf("UpdateJob() prompt = %q, want %q", job.Prompt, updated.Prompt)
				}
				return updated, nil
			},
			StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
				return automationpkg.ManagerStatus{}, errors.New("status unavailable")
			},
		})

		response := performAutomationCoreRequest(
			t,
			router,
			http.MethodPatch,
			"/automation/jobs/"+current.ID,
			[]byte(`{"prompt":"updated prompt"}`),
			nil,
		)
		if response.Code != http.StatusOK {
			t.Fatalf("update status = %d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
		}

		var payload map[string]any
		decodeAutomationCoreJSON(t, response, &payload)

		job, ok := payload["job"].(map[string]any)
		if !ok {
			t.Fatalf("job payload type = %T, want object", payload["job"])
		}
		if job["prompt"] != updated.Prompt {
			t.Fatalf("job.prompt = %#v, want %q", job["prompt"], updated.Prompt)
		}
		if nextRun, exists := job["next_run"]; exists && nextRun != nil {
			t.Fatalf("job.next_run = %#v, want omitted or nil", nextRun)
		}
	})
}

func TestWebhookRequestValidationRejectsInvalidScopeAndMalformedEndpointBeforeDispatch(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/webhooks/global/not-used",
		http.NoBody,
	)
	req.Header.Set(WebhookTimestampHeader, time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Format(time.RFC3339))
	req.Header.Set(WebhookSignatureHeader, "sha256=deadbeef")
	req.Header.Set(WebhookDeliveryIDHeader, "delivery-validation")
	ctx.Request = req
	ctx.Params = gin.Params{{Key: "endpoint", Value: "deploy-review--wbh_test"}}

	if _, err := webhookRequestFromHTTP(
		ctx,
		automationpkg.Scope("bogus"),
	); !errors.Is(
		err,
		ErrAutomationValidation,
	) {
		t.Fatalf("webhookRequestFromHTTP(invalid scope) error = %v, want ErrAutomationValidation", err)
	}

	called := false
	router := newAutomationCoreTestRouter(t, stubAutomationManager{
		HandleWebhookFn: func(context.Context, automationpkg.WebhookRequest) (automationpkg.TriggerResult, error) {
			called = true
			return automationpkg.TriggerResult{}, nil
		},
	})

	response := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/webhooks/global/not-an-endpoint",
		[]byte(`{"payload":"deploy"}`),
		map[string]string{
			WebhookTimestampHeader:  time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			WebhookSignatureHeader:  "sha256=deadbeef",
			WebhookDeliveryIDHeader: "delivery-malformed-endpoint",
		},
	)
	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"malformed endpoint status = %d, want %d; body=%s",
			response.Code,
			http.StatusBadRequest,
			response.Body.String(),
		)
	}
	if called {
		t.Fatal("HandleWebhook() called for malformed endpoint path")
	}
}

func TestAutomationDynamicHandlersRoundTripAndHelperCoverage(t *testing.T) {
	t.Parallel()

	nextRun := time.Date(2026, 4, 11, 13, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 4, 11, 12, 15, 0, 0, time.UTC)
	endedAt := time.Date(2026, 4, 11, 12, 16, 0, 0, time.UTC)

	job := automationpkg.Job{
		ID:        "job-dynamic",
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      "nightly-review",
		AgentName: "coder",
		Prompt:    "review repo",
		Schedule: &automationpkg.ScheduleSpec{
			Mode:     automationpkg.ScheduleModeEvery,
			Interval: "1h",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourceDynamic,
	}
	trigger := automationpkg.Trigger{
		ID:           "trigger-dynamic",
		Scope:        automationpkg.AutomationScopeWorkspace,
		Name:         "deploy-review",
		AgentName:    "coder",
		WorkspaceID:  "ws-alpha",
		Prompt:       `review {{ index .Data "payload" }}`,
		Event:        "webhook",
		Filter:       map[string]string{"data.branch": "main"},
		Enabled:      true,
		Retry:        automationpkg.DefaultRetryConfig(),
		FireLimit:    automationpkg.DefaultFireLimitConfig(),
		Source:       automationpkg.JobSourceDynamic,
		WebhookID:    "wbh_123",
		EndpointSlug: "deploy-review",
	}
	jobRun := automationpkg.Run{
		ID:        "run-job",
		JobID:     job.ID,
		Status:    automationpkg.RunCompleted,
		Attempt:   1,
		StartedAt: &startedAt,
		EndedAt:   &endedAt,
	}
	triggerRun := automationpkg.Run{
		ID:        "run-trigger",
		TriggerID: trigger.ID,
		Status:    automationpkg.RunCompleted,
		Attempt:   1,
		StartedAt: &startedAt,
		EndedAt:   &endedAt,
	}

	var listJobsQuery automationpkg.JobListQuery
	var listTriggersQuery automationpkg.TriggerListQuery
	var runsQueries []automationpkg.RunQuery
	var webhookRequest automationpkg.WebhookRequest
	jobDeleted := false
	triggerDeleted := false

	router := newAutomationCoreTestRouter(t, stubAutomationManager{
		ListJobsFn: func(_ context.Context, query automationpkg.JobListQuery) (automationpkg.JobListPage, error) {
			listJobsQuery = query
			return automationpkg.JobListPage{
				Jobs:       []automationpkg.Job{job},
				Total:      7,
				Limit:      query.Limit,
				HasMore:    true,
				NextCursor: "job-next",
			}, nil
		},
		GetJobFn: func(_ context.Context, id string) (automationpkg.Job, error) {
			if id != job.ID {
				t.Fatalf("GetJob() id = %q, want %q", id, job.ID)
			}
			return job, nil
		},
		CreateJobFn: func(_ context.Context, created automationpkg.Job) (automationpkg.Job, error) {
			if created.Scope != automationpkg.AutomationScopeGlobal ||
				created.Source != automationpkg.JobSourceDynamic {
				t.Fatalf("CreateJob() job = %#v", created)
			}
			if created.Name != "nightly-review" || created.AgentName != "coder" || created.Prompt != "review repo" {
				t.Fatalf("CreateJob() trimming failed: %#v", created)
			}
			if created.Schedule == nil || created.Schedule.Interval != "1h" {
				t.Fatalf("CreateJob() schedule = %#v", created.Schedule)
			}
			if !created.Enabled {
				t.Fatalf("CreateJob() enabled = %v, want true", created.Enabled)
			}
			return job, nil
		},
		DeleteJobFn: func(_ context.Context, id string) error {
			jobDeleted = true
			if id != job.ID {
				t.Fatalf("DeleteJob() id = %q, want %q", id, job.ID)
			}
			return nil
		},
		TriggerJobFn: func(_ context.Context, id string) (automationpkg.Run, error) {
			if id != job.ID {
				t.Fatalf("TriggerJob() id = %q, want %q", id, job.ID)
			}
			return jobRun, nil
		},
		ListTriggersFn: func(_ context.Context, query automationpkg.TriggerListQuery) (automationpkg.TriggerListPage, error) {
			listTriggersQuery = query
			return automationpkg.TriggerListPage{
				Triggers:   []automationpkg.Trigger{trigger},
				Total:      9,
				Limit:      query.Limit,
				HasMore:    true,
				NextCursor: "trigger-next",
			}, nil
		},
		GetTriggerFn: func(_ context.Context, id string) (automationpkg.Trigger, error) {
			if id != trigger.ID {
				t.Fatalf("GetTrigger() id = %q, want %q", id, trigger.ID)
			}
			return trigger, nil
		},
		CreateTriggerFn: func(_ context.Context, created automationpkg.Trigger, secret automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error) {
			if secret.Value == nil || *secret.Value != "shared-secret" {
				t.Fatalf("CreateTrigger() secret = %#v, want shared-secret write", secret)
			}
			if created.Scope != automationpkg.AutomationScopeWorkspace || created.WorkspaceID != "ws-alpha" {
				t.Fatalf("CreateTrigger() scope/workspace = %#v", created)
			}
			if created.Source != automationpkg.JobSourceDynamic || created.WebhookID != "wbh_123" ||
				created.EndpointSlug != "deploy-review" {
				t.Fatalf("CreateTrigger() webhook fields = %#v", created)
			}
			if created.Filter["data.branch"] != "main" {
				t.Fatalf("CreateTrigger() filter = %#v", created.Filter)
			}
			return trigger, nil
		},
		DeleteTriggerFn: func(_ context.Context, id string) error {
			triggerDeleted = true
			if id != trigger.ID {
				t.Fatalf("DeleteTrigger() id = %q, want %q", id, trigger.ID)
			}
			return nil
		},
		ListRunsFn: func(_ context.Context, query automationpkg.RunQuery) ([]automationpkg.Run, error) {
			runsQueries = append(runsQueries, query)
			switch {
			case query.JobID == job.ID:
				return []automationpkg.Run{jobRun}, nil
			case query.TriggerID == trigger.ID:
				return []automationpkg.Run{triggerRun}, nil
			default:
				return []automationpkg.Run{jobRun, triggerRun}, nil
			}
		},
		GetRunFn: func(_ context.Context, id string) (automationpkg.Run, error) {
			if id != triggerRun.ID {
				t.Fatalf("GetRun() id = %q, want %q", id, triggerRun.ID)
			}
			return triggerRun, nil
		},
		StatusFn: func(context.Context) (automationpkg.ManagerStatus, error) {
			return automationpkg.ManagerStatus{
				Running:          true,
				SchedulerRunning: true,
				Jobs:             automationpkg.ResourceStatus{Total: 1, Enabled: 1},
				Triggers:         automationpkg.ResourceStatus{Total: 1, Enabled: 1},
				ScheduledJobs: []automationpkg.ScheduledJobState{{
					JobID:      job.ID,
					Registered: true,
					NextRun:    &nextRun,
				}},
			}, nil
		},
		HandleWebhookFn: func(_ context.Context, request automationpkg.WebhookRequest) (automationpkg.TriggerResult, error) {
			webhookRequest = request
			return automationpkg.TriggerResult{Matched: 1, Runs: []automationpkg.Run{triggerRun}}, nil
		},
	})

	jobList := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/automation/jobs?scope=global&source=package&enabled=false&q=review&limit=3&loop=triage",
		nil,
		nil,
	)
	if jobList.Code != http.StatusOK {
		t.Fatalf("job list status = %d, want %d; body=%s", jobList.Code, http.StatusOK, jobList.Body.String())
	}
	var jobsResponse contract.JobsResponse
	decodeAutomationCoreJSON(t, jobList, &jobsResponse)
	if len(jobsResponse.Jobs) != 1 || jobsResponse.Jobs[0].NextRun == nil {
		t.Fatalf("jobs response = %#v", jobsResponse.Jobs)
	}
	if jobsResponse.Jobs[0].Scheduler == nil ||
		jobsResponse.Jobs[0].Scheduler.JobID != job.ID ||
		!jobsResponse.Jobs[0].Scheduler.Registered {
		t.Fatalf("jobs response scheduler = %#v, want durable scheduler metadata", jobsResponse.Jobs[0].Scheduler)
	}
	if jobsResponse.Page.Total != 7 || jobsResponse.Page.Limit != 3 ||
		!jobsResponse.Page.HasMore || jobsResponse.Page.NextCursor != "job-next" {
		t.Fatalf("jobs response page = %#v", jobsResponse.Page)
	}
	if listJobsQuery.Scope != automationpkg.AutomationScopeGlobal ||
		listJobsQuery.Source != automationpkg.JobSourcePackage ||
		listJobsQuery.Enabled == nil || *listJobsQuery.Enabled ||
		listJobsQuery.LoopName != "triage" ||
		listJobsQuery.Search != "review" ||
		listJobsQuery.Limit != 3 {
		t.Fatalf("ListJobs() query = %#v", listJobsQuery)
	}

	jobCreate := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/automation/jobs",
		[]byte(
			`{"scope":"global","name":" nightly-review ","agent_name":" coder ","prompt":" review repo ","schedule":{"mode":"every","interval":"1h"}}`,
		),
		nil,
	)
	if jobCreate.Code != http.StatusCreated {
		t.Fatalf(
			"job create status = %d, want %d; body=%s",
			jobCreate.Code,
			http.StatusCreated,
			jobCreate.Body.String(),
		)
	}

	jobGet := performAutomationCoreRequest(t, router, http.MethodGet, "/automation/jobs/"+job.ID, nil, nil)
	if jobGet.Code != http.StatusOK {
		t.Fatalf("job get status = %d, want %d; body=%s", jobGet.Code, http.StatusOK, jobGet.Body.String())
	}

	jobTrigger := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/automation/jobs/"+job.ID+"/trigger",
		nil,
		nil,
	)
	if jobTrigger.Code != http.StatusOK {
		t.Fatalf("job trigger status = %d, want %d; body=%s", jobTrigger.Code, http.StatusOK, jobTrigger.Body.String())
	}

	jobRuns := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/automation/jobs/"+job.ID+"/runs?status=completed&limit=2",
		nil,
		nil,
	)
	if jobRuns.Code != http.StatusOK {
		t.Fatalf("job runs status = %d, want %d; body=%s", jobRuns.Code, http.StatusOK, jobRuns.Body.String())
	}

	jobDelete := performAutomationCoreRequest(t, router, http.MethodDelete, "/automation/jobs/"+job.ID, nil, nil)
	if jobDelete.Code != http.StatusNoContent {
		t.Fatalf(
			"job delete status = %d, want %d; body=%s",
			jobDelete.Code,
			http.StatusNoContent,
			jobDelete.Body.String(),
		)
	}
	if !jobDeleted {
		t.Fatal("DeleteJob() not called")
	}

	triggerList := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/automation/triggers?scope=workspace&workspace_id=ws-alpha&source=package&enabled=true&event=webhook&q=review&limit=2&loop=triage",
		nil,
		nil,
	)
	if triggerList.Code != http.StatusOK {
		t.Fatalf(
			"trigger list status = %d, want %d; body=%s",
			triggerList.Code,
			http.StatusOK,
			triggerList.Body.String(),
		)
	}
	var triggersResponse contract.TriggersResponse
	decodeAutomationCoreJSON(t, triggerList, &triggersResponse)
	if len(triggersResponse.Triggers) != 1 {
		t.Fatalf("triggers response = %#v", triggersResponse.Triggers)
	}
	if triggersResponse.Page.Total != 9 || triggersResponse.Page.Limit != 2 ||
		!triggersResponse.Page.HasMore || triggersResponse.Page.NextCursor != "trigger-next" {
		t.Fatalf("triggers response page = %#v", triggersResponse.Page)
	}
	if listTriggersQuery.Scope != automationpkg.AutomationScopeWorkspace ||
		listTriggersQuery.WorkspaceID != "ws-alpha" ||
		listTriggersQuery.Source != automationpkg.JobSourcePackage ||
		listTriggersQuery.Enabled == nil || !*listTriggersQuery.Enabled ||
		listTriggersQuery.Event != "webhook" ||
		listTriggersQuery.LoopName != "triage" ||
		listTriggersQuery.Search != "review" ||
		listTriggersQuery.Limit != 2 {
		t.Fatalf("ListTriggers() query = %#v", listTriggersQuery)
	}

	invalidEnabled := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/automation/jobs?enabled=not-a-boolean",
		nil,
		nil,
	)
	if invalidEnabled.Code != http.StatusBadRequest {
		t.Fatalf(
			"invalid enabled status = %d, want %d; body=%s",
			invalidEnabled.Code,
			http.StatusBadRequest,
			invalidEnabled.Body.String(),
		)
	}

	triggerCreate := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/automation/triggers",
		[]byte(
			`{"scope":"workspace","workspace_id":" ws-alpha ","name":" deploy-review ","agent_name":" coder ","prompt":" review {{ index .Data \"payload\" }} ","event":"webhook","filter":{"data.branch":"main"},"webhook_id":" wbh_123 ","endpoint_slug":" deploy-review ","webhook_secret_value":"shared-secret"}`,
		),
		nil,
	)
	if triggerCreate.Code != http.StatusCreated {
		t.Fatalf(
			"trigger create status = %d, want %d; body=%s",
			triggerCreate.Code,
			http.StatusCreated,
			triggerCreate.Body.String(),
		)
	}

	triggerGet := performAutomationCoreRequest(t, router, http.MethodGet, "/automation/triggers/"+trigger.ID, nil, nil)
	if triggerGet.Code != http.StatusOK {
		t.Fatalf("trigger get status = %d, want %d; body=%s", triggerGet.Code, http.StatusOK, triggerGet.Body.String())
	}

	triggerRuns := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/automation/triggers/"+trigger.ID+"/runs?status=completed&limit=1",
		nil,
		nil,
	)
	if triggerRuns.Code != http.StatusOK {
		t.Fatalf(
			"trigger runs status = %d, want %d; body=%s",
			triggerRuns.Code,
			http.StatusOK,
			triggerRuns.Body.String(),
		)
	}

	allRuns := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/automation/runs?status=completed&job_id="+job.ID+"&limit=5&since=2026-04-11T12:00:00Z&until=2026-04-11T13:00:00Z",
		nil,
		nil,
	)
	if allRuns.Code != http.StatusOK {
		t.Fatalf("all runs status = %d, want %d; body=%s", allRuns.Code, http.StatusOK, allRuns.Body.String())
	}
	if len(runsQueries) != 3 {
		t.Fatalf("ListRuns() calls = %d, want 3", len(runsQueries))
	}
	if runsQueries[2].Status != automationpkg.RunCompleted || runsQueries[2].Limit != 5 ||
		runsQueries[2].JobID != job.ID ||
		runsQueries[2].Since.IsZero() ||
		runsQueries[2].Until.IsZero() {
		t.Fatalf("ListRuns() final query = %#v", runsQueries[2])
	}

	runGet := performAutomationCoreRequest(t, router, http.MethodGet, "/automation/runs/"+triggerRun.ID, nil, nil)
	if runGet.Code != http.StatusOK {
		t.Fatalf("run get status = %d, want %d; body=%s", runGet.Code, http.StatusOK, runGet.Body.String())
	}

	triggerDelete := performAutomationCoreRequest(
		t,
		router,
		http.MethodDelete,
		"/automation/triggers/"+trigger.ID,
		nil,
		nil,
	)
	if triggerDelete.Code != http.StatusNoContent {
		t.Fatalf(
			"trigger delete status = %d, want %d; body=%s",
			triggerDelete.Code,
			http.StatusNoContent,
			triggerDelete.Body.String(),
		)
	}
	if !triggerDeleted {
		t.Fatal("DeleteTrigger() not called")
	}

	webhookPayload := []byte(`{"payload":"deploy"}`)
	webhookDelivery := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/webhooks/workspaces/ws-alpha/deploy-review--wbh_123",
		webhookPayload,
		map[string]string{
			WebhookTimestampHeader:  strconv.FormatInt(time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Unix(), 10),
			WebhookSignatureHeader:  "sha256=deadbeef",
			WebhookDeliveryIDHeader: "delivery-roundtrip",
		},
	)
	if webhookDelivery.Code != http.StatusOK {
		t.Fatalf(
			"workspace webhook status = %d, want %d; body=%s",
			webhookDelivery.Code,
			http.StatusOK,
			webhookDelivery.Body.String(),
		)
	}
	if webhookRequest.Scope != automationpkg.AutomationScopeWorkspace || webhookRequest.WorkspaceID != "ws-alpha" {
		t.Fatalf("webhook request scope/workspace = %#v", webhookRequest)
	}
	if webhookRequest.Endpoint != "deploy-review--wbh_123" || webhookRequest.Signature != "sha256=deadbeef" ||
		webhookRequest.DeliveryID != "delivery-roundtrip" {
		t.Fatalf("webhook request routing = %#v", webhookRequest)
	}
	if payload := webhookRequest.Data["payload"]; payload != "deploy" {
		t.Fatalf("webhook request data = %#v", webhookRequest.Data)
	}
}

func TestAutomationSuggestionHandlersPreserveWorkspaceAndStructuredParity(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	suggestion := automationpkg.Suggestion{
		ID:          "suggestion-1",
		WorkspaceID: "workspace-1",
		Source:      automationpkg.SuggestionSourceCatalog,
		DedupKey:    "catalog:v1:daily-workspace-briefing",
		Status:      automationpkg.SuggestionStatusPending,
		Payload: automationpkg.Job{
			ID:          "job-suggestion-1",
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        "Daily workspace briefing",
			TargetKind:  automationpkg.TargetKindAgent,
			AgentName:   "general",
			WorkspaceID: "workspace-1",
			Prompt:      "Review the workspace.",
			Schedule: &automationpkg.ScheduleSpec{
				Mode: automationpkg.ScheduleModeCron,
				Expr: "0 8 * * *",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceDynamic,
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		CreatedAt: createdAt,
	}
	automation := stubAutomationManager{
		ListSuggestionsFn: func(
			_ context.Context,
			workspaceRef string,
			status automationpkg.SuggestionStatus,
		) ([]automationpkg.Suggestion, error) {
			if workspaceRef != suggestion.WorkspaceID || status != automationpkg.SuggestionStatusPending {
				t.Fatalf("ListSuggestions(%q, %q), want workspace-1/pending", workspaceRef, status)
			}
			return []automationpkg.Suggestion{suggestion}, nil
		},
		AcceptSuggestionFn: func(
			_ context.Context,
			workspaceRef string,
			suggestionID string,
		) (automationpkg.SuggestionAcceptance, error) {
			if workspaceRef != suggestion.WorkspaceID || suggestionID != suggestion.ID {
				t.Fatalf("AcceptSuggestion(%q, %q), want workspace-1/suggestion-1", workspaceRef, suggestionID)
			}
			accepted := suggestion
			accepted.Status = automationpkg.SuggestionStatusAccepted
			accepted.ResolvedAt = &createdAt
			return automationpkg.SuggestionAcceptance{Suggestion: accepted, Job: suggestion.Payload}, nil
		},
		DismissSuggestionFn: func(
			_ context.Context,
			workspaceRef string,
			suggestionID string,
		) (automationpkg.Suggestion, error) {
			if workspaceRef != suggestion.WorkspaceID || suggestionID != suggestion.ID {
				t.Fatalf("DismissSuggestion(%q, %q), want workspace-1/suggestion-1", workspaceRef, suggestionID)
			}
			dismissed := suggestion
			dismissed.Status = automationpkg.SuggestionStatusDismissed
			dismissed.ResolvedAt = &createdAt
			return dismissed, nil
		},
	}
	router := newAutomationCoreTestRouter(t, automation)

	listed := performAutomationCoreRequest(
		t,
		router,
		http.MethodGet,
		"/workspaces/workspace-1/automation/suggestions?status=pending",
		nil,
		nil,
	)
	if listed.Code != http.StatusOK {
		t.Fatalf("list suggestions status = %d, body=%s", listed.Code, listed.Body.String())
	}
	var listResponse contract.AutomationSuggestionsResponse
	decodeAutomationCoreJSON(t, listed, &listResponse)
	if len(listResponse.Suggestions) != 1 || listResponse.Suggestions[0].ID != suggestion.ID {
		t.Fatalf("list suggestions response = %#v, want suggestion-1", listResponse)
	}

	accepted := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/workspaces/workspace-1/automation/suggestions/suggestion-1/accept",
		nil,
		nil,
	)
	if accepted.Code != http.StatusOK {
		t.Fatalf("accept suggestion status = %d, body=%s", accepted.Code, accepted.Body.String())
	}
	var acceptResponse contract.AutomationSuggestionAcceptanceResponse
	decodeAutomationCoreJSON(t, accepted, &acceptResponse)
	if acceptResponse.Suggestion.Status != automationpkg.SuggestionStatusAccepted ||
		acceptResponse.Job.ID != suggestion.Payload.ID {
		t.Fatalf("accept suggestion response = %#v, want accepted suggestion and Job", acceptResponse)
	}

	dismissed := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/workspaces/workspace-1/automation/suggestions/suggestion-1/dismiss",
		nil,
		nil,
	)
	if dismissed.Code != http.StatusOK {
		t.Fatalf("dismiss suggestion status = %d, body=%s", dismissed.Code, dismissed.Body.String())
	}
	var dismissResponse contract.AutomationSuggestionResponse
	decodeAutomationCoreJSON(t, dismissed, &dismissResponse)
	if dismissResponse.Suggestion.Status != automationpkg.SuggestionStatusDismissed {
		t.Fatalf("dismiss suggestion response = %#v, want dismissed", dismissResponse)
	}
}

func TestAutomationPayloadsExposeSchedulerStateAndDeliveryErrors(t *testing.T) {
	t.Parallel()

	nextRun := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 4, 11, 9, 0, 1, 0, time.UTC)
	lastScheduled := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)
	lastMisfire := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 4, 11, 9, 0, 2, 0, time.UTC)
	deliveryErrorAt := time.Date(2026, 4, 11, 9, 0, 3, 0, time.UTC)

	schedulerState := automationpkg.ScheduledJobState{
		JobID:               "job-scheduler",
		Registered:          true,
		NextRun:             &nextRun,
		LastRun:             &lastRun,
		LastScheduledAt:     &lastScheduled,
		LastFireID:          "fire-previous",
		CatchUpPolicy:       automationpkg.SchedulerCatchUpPolicySkipMissed,
		MisfireGraceSeconds: 30,
		LastMisfireAt:       &lastMisfire,
		MisfireCount:        2,
		Durable: &automationpkg.SchedulerState{
			JobID:                     "job-scheduler",
			NextRunAt:                 &nextRun,
			LastRunAt:                 &lastRun,
			LastScheduledAt:           &lastScheduled,
			LastFireID:                "fire-previous",
			CatchUpPolicy:             automationpkg.SchedulerCatchUpPolicySkipMissed,
			MisfireGraceSeconds:       30,
			ConsecutiveResumeFailures: 1,
			LastMisfireAt:             &lastMisfire,
			MisfireCount:              2,
			UpdatedAt:                 updatedAt,
		},
	}
	health := AutomationHealthPayloadFromStatus(true, automationpkg.ManagerStatus{
		SchedulerRunning: true,
		NextFire:         &nextRun,
		ScheduledJobs:    []automationpkg.ScheduledJobState{schedulerState},
	})
	var exposedScheduler contract.AutomationSchedulerStatePayload
	t.Run("Should expose scheduler state in health payload", func(t *testing.T) {
		if got, want := len(health.ScheduledJobs), 1; got != want {
			t.Fatalf("len(health.ScheduledJobs) = %d, want %d", got, want)
		}
		exposedScheduler = health.ScheduledJobs[0]
		if exposedScheduler.JobID != schedulerState.JobID ||
			!exposedScheduler.Registered ||
			exposedScheduler.LastFireID != schedulerState.LastFireID ||
			exposedScheduler.CatchUpPolicy != automationpkg.SchedulerCatchUpPolicySkipMissed ||
			exposedScheduler.MisfireCount != 2 ||
			exposedScheduler.ConsecutiveResumeFailures != 1 ||
			exposedScheduler.UpdatedAt == nil ||
			!exposedScheduler.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("AutomationHealthPayloadFromStatus().ScheduledJobs[0] = %#v", exposedScheduler)
		}
	})

	job := automationpkg.Job{
		ID:        schedulerState.JobID,
		Scope:     automationpkg.AutomationScopeGlobal,
		Name:      "nightly-review",
		AgentName: "coder",
		Prompt:    "review repo",
		Schedule: &automationpkg.ScheduleSpec{
			Mode:     automationpkg.ScheduleModeEvery,
			Interval: "24h",
		},
		Enabled:   true,
		Retry:     automationpkg.DefaultRetryConfig(),
		FireLimit: automationpkg.DefaultFireLimitConfig(),
		Source:    automationpkg.JobSourceDynamic,
		CreatedAt: updatedAt,
		UpdatedAt: updatedAt,
	}
	t.Run("Should include scheduler state in job payload", func(t *testing.T) {
		jobs := JobPayloadsFromJobs(
			[]automationpkg.Job{job},
			map[string]contract.AutomationSchedulerStatePayload{job.ID: health.ScheduledJobs[0]},
		)
		if got, want := len(jobs), 1; got != want {
			t.Fatalf("len(JobPayloadsFromJobs()) = %d, want %d", got, want)
		}
		if jobs[0].Scheduler == nil ||
			jobs[0].NextRun == nil ||
			!jobs[0].NextRun.Equal(nextRun) ||
			jobs[0].Scheduler.LastScheduledAt == nil ||
			!jobs[0].Scheduler.LastScheduledAt.Equal(lastScheduled) {
			t.Fatalf("JobPayloadsFromJobs()[0] scheduler fields = %#v", jobs[0])
		}
	})

	t.Run("Should expose delivery error in run payload", func(t *testing.T) {
		metadata := map[string]any{
			"reason":          "loop_concurrency_conflict",
			"catch_up_policy": "coalesce",
		}
		request := automationTestNamedParticipation("ops-automation")
		run := RunPayloadFromRun(automationpkg.Run{
			ID:                   "run-scheduler",
			JobID:                job.ID,
			FireID:               "fire-scheduler",
			Status:               automationpkg.RunFailed,
			Attempt:              1,
			NetworkParticipation: request,
			ScheduledAt:          &lastScheduled,
			StartedAt:            &lastRun,
			DeliveryError:        "dispatcher unavailable",
			DeliveryErrorAt:      &deliveryErrorAt,
			Metadata:             metadata,
		})
		if run.FireID != "fire-scheduler" ||
			run.ScheduledAt == nil ||
			!run.ScheduledAt.Equal(lastScheduled) ||
			run.DeliveryError != "dispatcher unavailable" ||
			run.DeliveryErrorAt == nil ||
			!run.DeliveryErrorAt.Equal(deliveryErrorAt) ||
			automationTestParticipationChannel(run.NetworkParticipation) != "ops-automation" ||
			run.Metadata["reason"] != "loop_concurrency_conflict" ||
			run.Metadata["catch_up_policy"] != "coalesce" {
			t.Fatalf("RunPayloadFromRun() scheduler diagnostics = %#v", run)
		}
		metadata["reason"] = "mutated"
		*request.ChannelID = "mutated"
		if run.Metadata["reason"] != "loop_concurrency_conflict" {
			t.Fatalf("RunPayloadFromRun() metadata was not cloned: %#v", run.Metadata)
		}
		if got := automationTestParticipationChannel(run.NetworkParticipation); got != "ops-automation" {
			t.Fatalf("RunPayloadFromRun() participation was not cloned: %q", got)
		}
	})
}

func TestAutomationHelperFunctionsAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should wrap automation validation errors", func(t *testing.T) {
		rootCause := errors.New("bad request")
		validationErr := NewAutomationValidationError(rootCause)
		if !errors.Is(validationErr, ErrAutomationValidation) {
			t.Fatalf("NewAutomationValidationError() = %v, want ErrAutomationValidation", validationErr)
		}
		if !errors.Is(validationErr, rootCause) {
			t.Fatalf("NewAutomationValidationError() = %v, want wrapped root cause", validationErr)
		}
	})

	t.Run("Should parse webhook timestamps from unix seconds and reject invalid values", func(t *testing.T) {
		if _, err := parseWebhookTimestampHeader(""); err == nil {
			t.Fatal("parseWebhookTimestampHeader(empty) error = nil, want error")
		}

		seconds := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Unix()
		parsed, err := parseWebhookTimestampHeader(strconv.FormatInt(seconds, 10))
		if err != nil {
			t.Fatalf("parseWebhookTimestampHeader(unix) error = %v", err)
		}
		if parsed.Unix() != seconds {
			t.Fatalf("parseWebhookTimestampHeader(unix) = %v, want unix %d", parsed, seconds)
		}
		if _, err := parseWebhookTimestampHeader("not-a-time"); err == nil {
			t.Fatal("parseWebhookTimestampHeader(invalid) error = nil, want error")
		}
	})

	t.Run("Should decode webhook payload JSON and ignore empty inputs", func(t *testing.T) {
		if data := decodeWebhookPayloadData(nil); data != nil {
			t.Fatalf("decodeWebhookPayloadData(nil) = %#v, want nil", data)
		}
		if data := decodeWebhookPayloadData([]byte("not-json")); data != nil {
			t.Fatalf("decodeWebhookPayloadData(invalid) = %#v, want nil", data)
		}
		data := decodeWebhookPayloadData([]byte(`{"payload":"deploy"}`))
		if data["payload"] != "deploy" {
			t.Fatalf("decodeWebhookPayloadData(json) = %#v", data)
		}
	})

	t.Run("Should map create requests to trimmed task-backed jobs", func(t *testing.T) {
		createdJob, err := jobFromCreateRequest(contract.CreateJobRequest{
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        " build review ",
			AgentName:   " coder ",
			WorkspaceID: " ws-alpha ",
			Prompt:      " inspect repo ",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "2h",
			},
			Task: &automationpkg.JobTaskConfig{
				Title:                " Review repo ",
				NetworkParticipation: automationTestNamedParticipation(" ops-automation "),
				Owner: &taskpkg.Ownership{
					Kind: taskpkg.OwnerKindAutomation,
					Ref:  " rule:build-review ",
				},
			},
		})
		if err != nil {
			t.Fatalf("jobFromCreateRequest() error = %v", err)
		}
		if createdJob.Scope != automationpkg.AutomationScopeWorkspace || createdJob.Name != "build review" ||
			createdJob.AgentName != "coder" ||
			createdJob.WorkspaceID != "ws-alpha" ||
			createdJob.Prompt != "inspect repo" ||
			createdJob.Schedule == nil ||
			createdJob.Schedule.Interval != "2h" ||
			createdJob.Task == nil ||
			createdJob.Task.Title != "Review repo" ||
			automationTestParticipationChannel(createdJob.Task.NetworkParticipation) != "ops-automation" ||
			createdJob.Task.Owner == nil ||
			createdJob.Task.Owner.Kind != taskpkg.OwnerKindAutomation ||
			createdJob.Task.Owner.Ref != "rule:build-review" {
			t.Fatalf("jobFromCreateRequest() = %#v", createdJob)
		}
	})

	t.Run("Should clone patched job task configuration instead of aliasing the request", func(t *testing.T) {
		jobName := " renamed "
		jobPrompt := " next prompt "
		jobEnabled := false
		jobSchedule := automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeCron, Expr: "0 * * * *"}
		jobTask := automationpkg.JobTaskConfig{
			Title:                "Delegate review",
			NetworkParticipation: automationTestNamedParticipation("ops-queue"),
			Owner: &taskpkg.Ownership{
				Kind: taskpkg.OwnerKindPool,
				Ref:  "ops-review",
			},
		}
		jobRetry := automationpkg.RetryConfig{
			Strategy:   automationpkg.RetryStrategyBackoff,
			MaxRetries: 3,
			BaseDelay:  "2m",
		}
		jobFireLimit := automationpkg.FireLimitConfig{Max: 4, Window: "24h"}
		updatedJob, err := applyJobPatch(automationpkg.Job{
			ID:          "job-1",
			Name:        "before",
			AgentName:   "old-agent",
			WorkspaceID: "ws-alpha",
			Prompt:      "old",
			Enabled:     true,
			Schedule:    &automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeEvery, Interval: "1h"},
			Task:        &automationpkg.JobTaskConfig{Title: "Before"},
			Source:      automationpkg.JobSourceDynamic,
			Retry:       automationpkg.DefaultRetryConfig(),
			FireLimit:   automationpkg.DefaultFireLimitConfig(),
		}, contract.UpdateJobRequest{
			Name:      &jobName,
			Prompt:    &jobPrompt,
			Schedule:  &jobSchedule,
			Task:      &jobTask,
			Enabled:   &jobEnabled,
			Retry:     &jobRetry,
			FireLimit: &jobFireLimit,
		})
		if err != nil {
			t.Fatalf("applyJobPatch() error = %v", err)
		}
		if updatedJob.Name != "renamed" ||
			updatedJob.AgentName != "old-agent" ||
			updatedJob.WorkspaceID != "ws-alpha" ||
			updatedJob.Prompt != "next prompt" ||
			updatedJob.Enabled ||
			updatedJob.Schedule == nil ||
			updatedJob.Schedule.Expr != "0 * * * *" ||
			updatedJob.Task == nil ||
			updatedJob.Task.Title != "Delegate review" ||
			automationTestParticipationChannel(updatedJob.Task.NetworkParticipation) != "ops-queue" ||
			updatedJob.Task.Owner == nil ||
			updatedJob.Task.Owner.Kind != taskpkg.OwnerKindPool ||
			updatedJob.Task.Owner.Ref != "ops-review" ||
			updatedJob.Retry.MaxRetries != 3 ||
			updatedJob.FireLimit.Max != 4 {
			t.Fatalf("applyJobPatch() = %#v", updatedJob)
		}

		jobTask.Title = "mutated"
		*jobTask.NetworkParticipation.ChannelID = "mutated"
		jobTask.Owner.Ref = "mutated"
		if updatedJob.Task.Title != "Delegate review" ||
			automationTestParticipationChannel(updatedJob.Task.NetworkParticipation) != "ops-queue" ||
			updatedJob.Task.Owner.Ref != "ops-review" {
			t.Fatalf("applyJobPatch() task clone = %#v", updatedJob.Task)
		}
	})

	t.Run("Should map and clone trigger patch fields", func(t *testing.T) {
		createdTrigger := triggerFromCreateRequest(contract.CreateTriggerRequest{
			Scope:        automationpkg.AutomationScopeWorkspace,
			Name:         " deploy-review ",
			AgentName:    " coder ",
			WorkspaceID:  " ws-alpha ",
			Prompt:       ` review {{ index .Data "payload" }} `,
			Event:        " webhook ",
			Filter:       map[string]string{"data.branch": "main"},
			WebhookID:    " wbh_456 ",
			EndpointSlug: " deploy-review ",
		})
		if createdTrigger.Scope != automationpkg.AutomationScopeWorkspace || createdTrigger.Name != "deploy-review" ||
			createdTrigger.AgentName != "coder" ||
			createdTrigger.WorkspaceID != "ws-alpha" ||
			createdTrigger.Event != "webhook" ||
			createdTrigger.WebhookID != "wbh_456" ||
			createdTrigger.EndpointSlug != "deploy-review" ||
			createdTrigger.Filter["data.branch"] != "main" {
			t.Fatalf("triggerFromCreateRequest() = %#v", createdTrigger)
		}

		jobName := " renamed "
		jobPrompt := " next prompt "
		triggerEvent := "session.stopped"
		triggerFilter := map[string]string{"kind": "session"}
		triggerEnabled := false
		triggerRetry := automationpkg.RetryConfig{
			Strategy:   automationpkg.RetryStrategyBackoff,
			MaxRetries: 2,
			BaseDelay:  "30s",
		}
		triggerFireLimit := automationpkg.FireLimitConfig{Max: 2, Window: "1h"}
		updatedTrigger, err := applyTriggerPatch(automationpkg.Trigger{
			ID:           "trigger-1",
			Name:         "before",
			AgentName:    "old-agent",
			WorkspaceID:  "ws-alpha",
			Prompt:       "old",
			Event:        "webhook",
			Filter:       map[string]string{"branch": "main"},
			Enabled:      true,
			WebhookID:    "wbh_123",
			EndpointSlug: "deploy-review",
			Source:       automationpkg.JobSourceDynamic,
			Retry:        automationpkg.DefaultRetryConfig(),
			FireLimit:    automationpkg.DefaultFireLimitConfig(),
		}, contract.UpdateTriggerRequest{
			Name:      &jobName,
			Prompt:    &jobPrompt,
			Event:     &triggerEvent,
			Filter:    triggerFilter,
			Enabled:   &triggerEnabled,
			Retry:     &triggerRetry,
			FireLimit: &triggerFireLimit,
		})
		if err != nil {
			t.Fatalf("applyTriggerPatch() error = %v", err)
		}
		if updatedTrigger.Name != "renamed" || updatedTrigger.AgentName != "old-agent" ||
			updatedTrigger.WorkspaceID != "ws-alpha" ||
			updatedTrigger.Prompt != "next prompt" ||
			updatedTrigger.Event != "session.stopped" ||
			updatedTrigger.WebhookID != "" ||
			updatedTrigger.EndpointSlug != "" ||
			updatedTrigger.Enabled ||
			updatedTrigger.Retry.MaxRetries != 2 ||
			updatedTrigger.FireLimit.Max != 2 {
			t.Fatalf("applyTriggerPatch() = %#v", updatedTrigger)
		}
		triggerFilter["kind"] = "mutated"
		if updatedTrigger.Filter["kind"] != "session" {
			t.Fatalf("applyTriggerPatch() filter clone = %#v", updatedTrigger.Filter)
		}
		if clone := cloneAutomationFilter(nil); clone != nil {
			t.Fatalf("cloneAutomationFilter(nil) = %#v, want nil", clone)
		}
	})

	t.Run("Should reject a Loop target identity change", func(t *testing.T) {
		t.Parallel()

		currentTarget := &automationpkg.LoopTarget{WorkspaceID: "ws-alpha", LoopName: "review"}
		nextTarget := &automationpkg.LoopTarget{WorkspaceID: "ws-alpha", LoopName: "deploy"}
		if _, err := applyJobPatch(automationpkg.Job{
			TargetKind: automationpkg.TargetKindLoop,
			LoopTarget: currentTarget,
		}, contract.UpdateJobRequest{LoopTarget: nextTarget}); err == nil {
			t.Fatal("applyJobPatch(target identity) error = nil, want non-nil")
		}
		if _, err := applyTriggerPatch(automationpkg.Trigger{
			TargetKind: automationpkg.TargetKindLoop,
			LoopTarget: currentTarget,
		}, contract.UpdateTriggerRequest{LoopTarget: nextTarget}); err == nil {
			t.Fatal("applyTriggerPatch(target identity) error = nil, want non-nil")
		}
	})

	t.Run("Should map automation errors to HTTP status codes", func(t *testing.T) {
		validationErr := NewAutomationValidationError(errors.New("bad request"))
		if status := StatusForAutomationError(validationErr); status != http.StatusBadRequest {
			t.Fatalf("StatusForAutomationError(validation) = %d, want %d", status, http.StatusBadRequest)
		}
		if status := StatusForAutomationError(
			&http.MaxBytesError{Limit: maxWebhookPayloadSize},
		); status != http.StatusRequestEntityTooLarge {
			t.Fatalf("StatusForAutomationError(max bytes) = %d, want %d", status, http.StatusRequestEntityTooLarge)
		}
		if status := StatusForAutomationError(
			automationpkg.ErrWebhookSignatureInvalid,
		); status != http.StatusUnauthorized {
			t.Fatalf("StatusForAutomationError(signature) = %d, want %d", status, http.StatusUnauthorized)
		}
		if status := StatusForAutomationError(automationpkg.ErrRunNotFound); status != http.StatusNotFound {
			t.Fatalf("StatusForAutomationError(not found) = %d, want %d", status, http.StatusNotFound)
		}
		if status := StatusForAutomationError(automationpkg.ErrJobOverlayNotFound); status != http.StatusNotFound {
			t.Fatalf("StatusForAutomationError(job overlay not found) = %d, want %d", status, http.StatusNotFound)
		}
		if status := StatusForAutomationError(workspacepkg.ErrWorkspaceNotFound); status != http.StatusNotFound {
			t.Fatalf("StatusForAutomationError(workspace not found) = %d, want %d", status, http.StatusNotFound)
		}
		if status := StatusForAutomationError(automationpkg.ErrFireLimitReached); status != http.StatusConflict {
			t.Fatalf("StatusForAutomationError(conflict) = %d, want %d", status, http.StatusConflict)
		}
		if status := StatusForAutomationError(
			automationpkg.ErrOverlayRequiresConfigSource,
		); status != http.StatusConflict {
			t.Fatalf(
				"StatusForAutomationError(overlay requires config source) = %d, want %d",
				status,
				http.StatusConflict,
			)
		}
		if status := StatusForAutomationError(automationpkg.ErrWebhookReplayDetected); status != http.StatusConflict {
			t.Fatalf("StatusForAutomationError(webhook replay) = %d, want %d", status, http.StatusConflict)
		}
		if status := StatusForAutomationError(
			automationpkg.ErrManagerNotRunning,
		); status != http.StatusServiceUnavailable {
			t.Fatalf("StatusForAutomationError(unavailable) = %d, want %d", status, http.StatusServiceUnavailable)
		}
	})
}

func automationTestNamedParticipation(channelID string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
	}
}

func automationTestParticipationChannel(request *participation.Request) string {
	if request == nil || request.ChannelID == nil {
		return ""
	}
	return *request.ChannelID
}

func TestStatusForAutomationErrorMapsAdditionalSentinels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "Should map missing webhook secret to bad request",
			err:  automationpkg.ErrWebhookSecretRequired,
			want: http.StatusBadRequest,
		},
		{
			name: "Should map invalid webhook endpoints to bad request",
			err:  automationpkg.ErrWebhookEndpointInvalid,
			want: http.StatusBadRequest,
		},
		{
			name: "Should map blocked daemon lifecycle commands to bad request",
			err:  automationpkg.ErrDaemonLifecycleCommandBlocked,
			want: http.StatusBadRequest,
		},
		{
			name: "Should map read-only definitions to conflict",
			err:  automationpkg.ErrDefinitionReadOnly,
			want: http.StatusConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := StatusForAutomationError(tc.err); got != tc.want {
				t.Fatalf("StatusForAutomationError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestCreateAutomationJobReturnsBlockedLifecycleClass(t *testing.T) {
	t.Run("Should return the blocked lifecycle class in the shared transport error", func(t *testing.T) {
		t.Parallel()

		blockedErr := &automationpkg.DaemonLifecycleCommandError{
			Class: automationpkg.DaemonLifecycleCommandClassAGHDaemon,
		}
		router := newAutomationCoreTestRouter(t, stubAutomationManager{
			CreateJobFn: func(context.Context, automationpkg.Job) (automationpkg.Job, error) {
				return automationpkg.Job{}, blockedErr
			},
		})
		response := performAutomationCoreRequest(
			t,
			router,
			http.MethodPost,
			"/automation/jobs",
			[]byte(`{
				"scope":"global",
				"name":"blocked-restart",
				"agent_name":"codex",
				"prompt":"Run agh daemon restart now",
				"schedule":{"mode":"every","interval":"1h"}
			}`),
			nil,
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
		}

		var payload contract.ErrorPayload
		decodeAutomationCoreJSON(t, response, &payload)
		if payload.Error != blockedErr.Error() {
			t.Fatalf("error = %q, want %q", payload.Error, blockedErr.Error())
		}
	})
}

func TestWebhookRequestValidationRejectsBodiesThatExceedTheConfiguredLimit(t *testing.T) {
	t.Parallel()

	called := false
	router := newAutomationCoreTestRouter(t, stubAutomationManager{
		HandleWebhookFn: func(context.Context, automationpkg.WebhookRequest) (automationpkg.TriggerResult, error) {
			called = true
			return automationpkg.TriggerResult{}, nil
		},
	})

	tooLargeBody := bytes.Repeat([]byte("a"), maxWebhookPayloadSize+1)
	response := performAutomationCoreRequest(
		t,
		router,
		http.MethodPost,
		"/webhooks/global/deploy-review--wbh_123",
		tooLargeBody,
		map[string]string{
			WebhookTimestampHeader:  time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
			WebhookSignatureHeader:  "sha256=deadbeef",
			WebhookDeliveryIDHeader: "delivery-too-large",
		},
	)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf(
			"oversized webhook status = %d, want %d; body=%s",
			response.Code,
			http.StatusRequestEntityTooLarge,
			response.Body.String(),
		)
	}
	if called {
		t.Fatal("HandleWebhook() called for oversized webhook body")
	}
}

func TestAutomationHandlersRequireConfiguredManager(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := aghconfig.DefaultWithHome(homePaths)

	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName: "core-automation-test",
		HomePaths:     homePaths,
		Config:        cfg,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	engine := gin.New()
	engine.GET("/automation/jobs", handlers.ListAutomationJobs)

	response := performAutomationCoreRequest(t, engine, http.MethodGet, "/automation/jobs", nil, nil)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
}

func newAutomationCoreTestRouter(t *testing.T, automation stubAutomationManager) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := aghconfig.DefaultWithHome(homePaths)

	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName: "core-automation-test",
		Automation:    automation,
		HomePaths:     homePaths,
		Config:        cfg,
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		StartedAt:     time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC),
		Now: func() time.Time {
			return time.Date(2026, 4, 11, 12, 0, 1, 0, time.UTC)
		},
	})

	engine := gin.New()
	engine.GET("/automation/jobs", handlers.ListAutomationJobs)
	engine.POST("/automation/jobs", handlers.CreateAutomationJob)
	engine.GET("/automation/jobs/:id", handlers.GetAutomationJob)
	engine.PATCH("/automation/jobs/:id", handlers.UpdateAutomationJob)
	engine.DELETE("/automation/jobs/:id", handlers.DeleteAutomationJob)
	engine.POST("/automation/jobs/:id/trigger", handlers.TriggerAutomationJob)
	engine.GET("/automation/jobs/:id/runs", handlers.AutomationJobRuns)
	engine.GET("/automation/triggers", handlers.ListAutomationTriggers)
	engine.POST("/automation/triggers", handlers.CreateAutomationTrigger)
	engine.GET("/automation/triggers/:id", handlers.GetAutomationTrigger)
	engine.PATCH("/automation/triggers/:id", handlers.UpdateAutomationTrigger)
	engine.DELETE("/automation/triggers/:id", handlers.DeleteAutomationTrigger)
	engine.GET("/automation/triggers/:id/runs", handlers.AutomationTriggerRuns)
	engine.GET("/automation/runs", handlers.ListAutomationRuns)
	engine.GET("/automation/runs/:id", handlers.GetAutomationRun)
	engine.GET("/workspaces/:workspace_id/automation/suggestions", handlers.ListAutomationSuggestions)
	engine.POST(
		"/workspaces/:workspace_id/automation/suggestions/:suggestion_id/accept",
		handlers.AcceptAutomationSuggestion,
	)
	engine.POST(
		"/workspaces/:workspace_id/automation/suggestions/:suggestion_id/dismiss",
		handlers.DismissAutomationSuggestion,
	)
	engine.POST("/webhooks/global/:endpoint", handlers.DeliverGlobalWebhook)
	engine.POST("/webhooks/workspaces/:workspace_id/:endpoint", handlers.DeliverWorkspaceWebhook)
	return engine
}

func performAutomationCoreRequest(
	t *testing.T,
	engine http.Handler,
	method, path string,
	body []byte,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(
		context.Background(),
		method,
		path,
		bytes.NewReader(body),
	)
	if len(body) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

func decodeAutomationCoreJSON(t *testing.T, recorder *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), dest); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; body=%s", err, recorder.Body.String())
	}
}

type stubAutomationManager struct {
	ListSuggestionsFn func(
		context.Context,
		string,
		automationpkg.SuggestionStatus,
	) ([]automationpkg.Suggestion, error)
	AcceptSuggestionFn func(
		context.Context,
		string,
		string,
	) (automationpkg.SuggestionAcceptance, error)
	DismissSuggestionFn func(
		context.Context,
		string,
		string,
	) (automationpkg.Suggestion, error)
	ListJobsFn          func(context.Context, automationpkg.JobListQuery) (automationpkg.JobListPage, error)
	GetJobFn            func(context.Context, string) (automationpkg.Job, error)
	CreateJobFn         func(context.Context, automationpkg.Job) (automationpkg.Job, error)
	UpdateJobFn         func(context.Context, automationpkg.Job) (automationpkg.Job, error)
	DeleteJobFn         func(context.Context, string) error
	TriggerJobFn        func(context.Context, string) (automationpkg.Run, error)
	ListTriggersFn      func(context.Context, automationpkg.TriggerListQuery) (automationpkg.TriggerListPage, error)
	GetTriggerFn        func(context.Context, string) (automationpkg.Trigger, error)
	CreateTriggerFn     func(context.Context, automationpkg.Trigger, automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error)
	UpdateTriggerFn     func(context.Context, automationpkg.Trigger, *automationpkg.WebhookSecretWrite) (automationpkg.Trigger, error)
	DeleteTriggerFn     func(context.Context, string) error
	ListRunsFn          func(context.Context, automationpkg.RunQuery) ([]automationpkg.Run, error)
	GetRunFn            func(context.Context, string) (automationpkg.Run, error)
	StatusFn            func(context.Context) (automationpkg.ManagerStatus, error)
	SetJobEnabledFn     func(context.Context, string, bool) (automationpkg.Job, error)
	SetTriggerEnabledFn func(context.Context, string, bool) (automationpkg.Trigger, error)
	HandleWebhookFn     func(context.Context, automationpkg.WebhookRequest) (automationpkg.TriggerResult, error)
}

func (s stubAutomationManager) ListSuggestions(
	ctx context.Context,
	workspaceRef string,
	status automationpkg.SuggestionStatus,
) ([]automationpkg.Suggestion, error) {
	if s.ListSuggestionsFn == nil {
		return nil, nil
	}
	return s.ListSuggestionsFn(ctx, workspaceRef, status)
}

func (s stubAutomationManager) AcceptSuggestion(
	ctx context.Context,
	workspaceRef string,
	suggestionID string,
) (automationpkg.SuggestionAcceptance, error) {
	if s.AcceptSuggestionFn == nil {
		return automationpkg.SuggestionAcceptance{}, automationpkg.ErrSuggestionNotFound
	}
	return s.AcceptSuggestionFn(ctx, workspaceRef, suggestionID)
}

func (s stubAutomationManager) DismissSuggestion(
	ctx context.Context,
	workspaceRef string,
	suggestionID string,
) (automationpkg.Suggestion, error) {
	if s.DismissSuggestionFn == nil {
		return automationpkg.Suggestion{}, automationpkg.ErrSuggestionNotFound
	}
	return s.DismissSuggestionFn(ctx, workspaceRef, suggestionID)
}

func (s stubAutomationManager) ListJobs(
	ctx context.Context,
	query automationpkg.JobListQuery,
) (automationpkg.JobListPage, error) {
	if s.ListJobsFn == nil {
		return automationpkg.JobListPage{}, nil
	}
	return s.ListJobsFn(ctx, query)
}

func (s stubAutomationManager) GetJob(ctx context.Context, id string) (automationpkg.Job, error) {
	if s.GetJobFn == nil {
		return automationpkg.Job{}, nil
	}
	return s.GetJobFn(ctx, id)
}

func (s stubAutomationManager) CreateJob(ctx context.Context, job automationpkg.Job) (automationpkg.Job, error) {
	if s.CreateJobFn == nil {
		return automationpkg.Job{}, nil
	}
	return s.CreateJobFn(ctx, job)
}

func (s stubAutomationManager) UpdateJob(ctx context.Context, job automationpkg.Job) (automationpkg.Job, error) {
	if s.UpdateJobFn == nil {
		return automationpkg.Job{}, nil
	}
	return s.UpdateJobFn(ctx, job)
}

func (s stubAutomationManager) DeleteJob(ctx context.Context, id string) error {
	if s.DeleteJobFn == nil {
		return nil
	}
	return s.DeleteJobFn(ctx, id)
}

func (s stubAutomationManager) TriggerJob(ctx context.Context, id string) (automationpkg.Run, error) {
	if s.TriggerJobFn == nil {
		return automationpkg.Run{}, nil
	}
	return s.TriggerJobFn(ctx, id)
}

func (s stubAutomationManager) Jobs(ctx context.Context) ([]automationpkg.Job, error) {
	page, err := s.ListJobs(ctx, automationpkg.JobListQuery{})
	return page.Jobs, err
}

func (s stubAutomationManager) ListTriggers(
	ctx context.Context,
	query automationpkg.TriggerListQuery,
) (automationpkg.TriggerListPage, error) {
	if s.ListTriggersFn == nil {
		return automationpkg.TriggerListPage{}, nil
	}
	return s.ListTriggersFn(ctx, query)
}

func (s stubAutomationManager) GetTrigger(ctx context.Context, id string) (automationpkg.Trigger, error) {
	if s.GetTriggerFn == nil {
		return automationpkg.Trigger{}, nil
	}
	return s.GetTriggerFn(ctx, id)
}

func (s stubAutomationManager) CreateTrigger(
	ctx context.Context,
	trigger automationpkg.Trigger,
	secret automationpkg.WebhookSecretWrite,
) (automationpkg.Trigger, error) {
	if s.CreateTriggerFn == nil {
		return automationpkg.Trigger{}, nil
	}
	return s.CreateTriggerFn(ctx, trigger, secret)
}

func (s stubAutomationManager) UpdateTrigger(
	ctx context.Context,
	trigger automationpkg.Trigger,
	secret *automationpkg.WebhookSecretWrite,
) (automationpkg.Trigger, error) {
	if s.UpdateTriggerFn == nil {
		return automationpkg.Trigger{}, nil
	}
	return s.UpdateTriggerFn(ctx, trigger, secret)
}

func (s stubAutomationManager) DeleteTrigger(ctx context.Context, id string) error {
	if s.DeleteTriggerFn == nil {
		return nil
	}
	return s.DeleteTriggerFn(ctx, id)
}

func (s stubAutomationManager) Triggers(ctx context.Context) ([]automationpkg.Trigger, error) {
	page, err := s.ListTriggers(ctx, automationpkg.TriggerListQuery{})
	return page.Triggers, err
}

func (s stubAutomationManager) ListRuns(
	ctx context.Context,
	query automationpkg.RunQuery,
) ([]automationpkg.Run, error) {
	if s.ListRunsFn == nil {
		return nil, nil
	}
	return s.ListRunsFn(ctx, query)
}

func (s stubAutomationManager) GetRun(ctx context.Context, id string) (automationpkg.Run, error) {
	if s.GetRunFn == nil {
		return automationpkg.Run{}, nil
	}
	return s.GetRunFn(ctx, id)
}

func (s stubAutomationManager) Runs(ctx context.Context, query automationpkg.RunQuery) ([]automationpkg.Run, error) {
	return s.ListRuns(ctx, query)
}

func (s stubAutomationManager) Status(ctx context.Context) (automationpkg.ManagerStatus, error) {
	if s.StatusFn == nil {
		return automationpkg.ManagerStatus{}, nil
	}
	return s.StatusFn(ctx)
}

func (s stubAutomationManager) SetJobEnabled(ctx context.Context, id string, enabled bool) (automationpkg.Job, error) {
	if s.SetJobEnabledFn == nil {
		return automationpkg.Job{}, nil
	}
	return s.SetJobEnabledFn(ctx, id, enabled)
}

func (s stubAutomationManager) SetTriggerEnabled(
	ctx context.Context,
	id string,
	enabled bool,
) (automationpkg.Trigger, error) {
	if s.SetTriggerEnabledFn == nil {
		return automationpkg.Trigger{}, nil
	}
	return s.SetTriggerEnabledFn(ctx, id, enabled)
}

func (s stubAutomationManager) HandleWebhook(
	ctx context.Context,
	request automationpkg.WebhookRequest,
) (automationpkg.TriggerResult, error) {
	if s.HandleWebhookFn == nil {
		return automationpkg.TriggerResult{}, nil
	}
	return s.HandleWebhookFn(ctx, request)
}
