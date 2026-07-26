package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	coreapi "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestSeedAutomationFixturesRegistersDefinitionsWithoutHiddenDefaults(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 16, 18, 0, 0, 0, time.UTC)

	var seenJobRequest aghcontract.CreateJobRequest
	var seenTriggerRequest aghcontract.CreateTriggerRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/automation/jobs":
			if err := json.NewDecoder(r.Body).Decode(&seenJobRequest); err != nil {
				t.Fatalf("Decode(job request) error = %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, aghcontract.JobResponse{
				Job: aghcontract.JobPayload{
					ID:          "job-1",
					Scope:       seenJobRequest.Scope,
					Name:        seenJobRequest.Name,
					WorkspaceID: seenJobRequest.WorkspaceID,
					Prompt:      seenJobRequest.Prompt,
					Task:        seenJobRequest.Task,
					Schedule:    &seenJobRequest.Schedule,
					Enabled:     true,
					Retry:       automationpkg.DefaultRetryConfig(),
					FireLimit:   automationpkg.DefaultFireLimitConfig(),
					Source:      automationpkg.JobSourceDynamic,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/automation/triggers":
			if err := json.NewDecoder(r.Body).Decode(&seenTriggerRequest); err != nil {
				t.Fatalf("Decode(trigger request) error = %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, aghcontract.TriggerResponse{
				Trigger: aghcontract.TriggerPayload{
					ID:           "trg-1",
					Scope:        seenTriggerRequest.Scope,
					Name:         seenTriggerRequest.Name,
					AgentName:    seenTriggerRequest.AgentName,
					WorkspaceID:  seenTriggerRequest.WorkspaceID,
					Prompt:       seenTriggerRequest.Prompt,
					Event:        seenTriggerRequest.Event,
					Filter:       seenTriggerRequest.Filter,
					Enabled:      true,
					Retry:        automationpkg.DefaultRetryConfig(),
					FireLimit:    automationpkg.DefaultFireLimitConfig(),
					Source:       automationpkg.JobSourceDynamic,
					WebhookID:    "wbh_deploy-review",
					EndpointSlug: seenTriggerRequest.EndpointSlug,
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		UDSBaseURL: server.URL,
		UDSClient:  server.Client(),
	}

	taskOwner := &taskpkg.Ownership{
		Kind: taskpkg.OwnerKindAutomation,
		Ref:  "job:triage-deploy",
	}
	liveMode := participation.ModeLive
	namedStrategy := participation.StrategyNamed
	channelID := "ops-automation"
	seed := AutomationFixtureSeed{
		Jobs: []aghcontract.CreateJobRequest{{
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        "triage-deploy",
			WorkspaceID: "ws-1",
			Prompt:      "Investigate deployment drift.",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
			Task: &automationpkg.JobTaskConfig{
				Title:       "Investigate deploy drift",
				Description: "Review the latest deployment discrepancy.",
				Owner:       taskOwner,
				NetworkParticipation: &participation.Request{
					Mode:            &liveMode,
					ChannelStrategy: &namedStrategy,
					ChannelID:       &channelID,
				},
			},
		}},
		Triggers: []aghcontract.CreateTriggerRequest{{
			Scope:              automationpkg.AutomationScopeWorkspace,
			WorkspaceID:        "ws-1",
			Name:               "deploy-review",
			AgentName:          "mock-automation-runner",
			Prompt:             `Review payload {{ index .Data "payload" }} for {{ index .Data "branch" }}`,
			Event:              "webhook",
			EndpointSlug:       "deploy-review",
			WebhookSecretValue: "shared-secret",
			Filter: map[string]string{
				"data.branch": "main",
			},
		}},
	}

	created, err := harness.SeedAutomationFixtures(context.Background(), seed)
	if err != nil {
		t.Fatalf("SeedAutomationFixtures() error = %v", err)
	}

	if got, want := len(created.Jobs), 1; got != want {
		t.Fatalf("len(created.Jobs) = %d, want %d", got, want)
	}
	if got, want := len(created.Triggers), 1; got != want {
		t.Fatalf("len(created.Triggers) = %d, want %d", got, want)
	}

	if got, want := seenJobRequest.AgentName, ""; got != want {
		t.Fatalf("seenJobRequest.AgentName = %q, want %q", got, want)
	}
	if got, want := seenJobRequest.WorkspaceID, "ws-1"; got != want {
		t.Fatalf("seenJobRequest.WorkspaceID = %q, want %q", got, want)
	}
	if seenJobRequest.Task == nil {
		t.Fatal("seenJobRequest.Task = nil, want task-backed job config")
	}
	if got, want := seenJobRequest.Task.Owner.Ref, "job:triage-deploy"; got != want {
		t.Fatalf("seenJobRequest.Task.Owner.Ref = %q, want %q", got, want)
	}
	if got, want := seenTriggerRequest.WebhookSecretValue, "shared-secret"; got != want {
		t.Fatalf("seenTriggerRequest.WebhookSecretValue = %q, want %q", got, want)
	}
	if got, want := seenTriggerRequest.Filter["data.branch"], "main"; got != want {
		t.Fatalf("seenTriggerRequest.Filter[data.branch] = %q, want %q", got, want)
	}

	createdParticipation := created.Jobs[0].Task.NetworkParticipation
	if createdParticipation == nil ||
		createdParticipation.Mode == nil || *createdParticipation.Mode != participation.ModeLive ||
		createdParticipation.ChannelStrategy == nil ||
		*createdParticipation.ChannelStrategy != participation.StrategyNamed ||
		createdParticipation.ChannelID == nil || *createdParticipation.ChannelID != channelID {
		t.Fatalf(
			"created.Jobs[0].Task.NetworkParticipation = %#v, want live named channel %q",
			createdParticipation,
			channelID,
		)
	}
	if got, want := created.Triggers[0].EndpointSlug, "deploy-review"; got != want {
		t.Fatalf("created.Triggers[0].EndpointSlug = %q, want %q", got, want)
	}
}

func TestAutomationTaskHelpersUseExpectedPublicSurfaces(t *testing.T) {
	t.Parallel()

	webhookTime := time.Date(2026, 4, 16, 19, 45, 0, 0, time.UTC)
	webhookPayload := []byte(`{"payload":"deploy"}`)
	expectedSignature, err := automationpkg.SignWebhookPayload("shared-secret", webhookTime, webhookPayload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	var claimRequest aghcontract.AgentTaskClaimNextRequest
	var startRequest aghcontract.StartTaskRunRequest
	var completeRequest aghcontract.AgentTaskCompleteRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/automation/jobs/job-1/trigger":
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, aghcontract.RunResponse{
				Run: aghcontract.RunPayload{
					ID:        "run-1",
					JobID:     "job-1",
					TaskID:    "task-1",
					TaskRunID: "task-run-1",
					Status:    automationpkg.RunDelegated,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/automation/runs":
			if got, want := r.URL.RawQuery, "status=completed"; got != want {
				t.Fatalf("automation runs query = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.RunsResponse{
				Runs: []aghcontract.RunPayload{{
					ID:        "run-1",
					SessionID: "sess-1",
					Status:    automationpkg.RunCompleted,
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/automation/runs/run-1":
			writeJSON(w, aghcontract.RunResponse{
				Run: aghcontract.RunPayload{
					ID:        "run-1",
					SessionID: "sess-1",
					Status:    automationpkg.RunCompleted,
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks":
			if got, want := r.URL.RawQuery, "workspace=ws-1"; got != want {
				t.Fatalf("tasks query = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.TasksResponse{
				Tasks: []aghcontract.TaskCatalogItemPayload{{
					ID:          "task-1",
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
					Status:      taskpkg.TaskStatusReady,
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/task-1":
			writeJSON(w, aghcontract.TaskDetailResponse{
				Task: aghcontract.TaskDetailPayload{
					Task: aghcontract.TaskPayload{
						ID:          "task-1",
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-1",
						Status:      taskpkg.TaskStatusReady,
					},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks/task-1/runs":
			if got, want := r.URL.RawQuery, "status=queued"; got != want {
				t.Fatalf("task runs query = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.TaskRunsResponse{
				Runs: []aghcontract.TaskRunPayload{{
					ID:             "task-run-1",
					TaskID:         "task-1",
					Status:         taskpkg.TaskRunStatusQueued,
					IdempotencyKey: "automation-run:run-1",
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent/tasks/claim-next":
			if err := json.NewDecoder(r.Body).Decode(&claimRequest); err != nil {
				t.Fatalf("Decode(claim request) error = %v", err)
			}
			if got, want := r.Header.Get(agentidentity.HeaderSessionID), "sess-agent"; got != want {
				t.Fatalf("claim session header = %q, want %q", got, want)
			}
			if got, want := r.Header.Get(agentidentity.HeaderAgent), "worker"; got != want {
				t.Fatalf("claim agent header = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.AgentTaskClaimResponse{
				Claim: aghcontract.AgentTaskClaimPayload{
					Task: aghcontract.TaskReferencePayload{
						ID:          "task-1",
						Title:       "Deploy",
						Status:      taskpkg.TaskStatusInProgress,
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-1",
					},
					Run: aghcontract.TaskRunPayload{
						ID:     "task-run-1",
						TaskID: "task-1",
						Status: taskpkg.TaskRunStatusClaimed,
					},
					Lease: aghcontract.TaskRunLeaseSummaryPayload{
						TaskID:    "task-1",
						RunID:     "task-run-1",
						Status:    taskpkg.TaskRunStatusClaimed,
						SessionID: "sess-agent",
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/task-runs/task-run-1/start":
			if err := json.NewDecoder(r.Body).Decode(&startRequest); err != nil {
				t.Fatalf("Decode(start request) error = %v", err)
			}
			writeJSON(w, aghcontract.TaskRunResponse{
				Run: aghcontract.TaskRunPayload{
					ID:        "task-run-1",
					TaskID:    "task-1",
					Status:    taskpkg.TaskRunStatusRunning,
					SessionID: "sess-1",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/agent/tasks/task-run-1/complete":
			if err := json.NewDecoder(r.Body).Decode(&completeRequest); err != nil {
				t.Fatalf("Decode(complete request) error = %v", err)
			}
			if got, want := r.Header.Get(agentidentity.HeaderSessionID), "sess-agent"; got != want {
				t.Fatalf("complete session header = %q, want %q", got, want)
			}
			if got, want := r.Header.Get(agentidentity.HeaderAgent), "worker"; got != want {
				t.Fatalf("complete agent header = %q, want %q", got, want)
			}
			if got, want := r.Header.Get(agentidentity.HeaderWorkspaceID), "ws-1"; got != want {
				t.Fatalf("complete workspace header = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.AgentTaskLeaseResponse{
				Lease: aghcontract.TaskRunLeaseSummaryPayload{
					RunID:     "task-run-1",
					TaskID:    "task-1",
					Status:    taskpkg.TaskRunStatusCompleted,
					SessionID: "sess-1",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/webhooks/global/deploy-hook":
			if got, want := r.Header.Get(
				coreapi.WebhookTimestampHeader,
			), webhookTime.Format(
				time.RFC3339,
			); got != want {
				t.Fatalf("global webhook timestamp = %q, want %q", got, want)
			}
			if got, want := r.Header.Get(coreapi.WebhookSignatureHeader), expectedSignature; got != want {
				t.Fatalf("global webhook signature = %q, want %q", got, want)
			}
			if got, want := r.Header.Get(coreapi.WebhookDeliveryIDHeader), "delivery-1"; got != want {
				t.Fatalf("global webhook delivery id = %q, want %q", got, want)
			}
			if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
				t.Fatalf("global webhook content type = %q, want %q", got, want)
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("io.ReadAll(global webhook body) error = %v", err)
			}
			if got, want := string(body), string(webhookPayload); got != want {
				t.Fatalf("global webhook body = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.WebhookDeliveryResponse{
				Result: aghcontract.WebhookDeliveryPayload{
					Matched: 1,
					Runs: []aghcontract.RunPayload{{
						ID:        "run-1",
						SessionID: "sess-1",
						Status:    automationpkg.RunCompleted,
					}},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/webhooks/workspaces/ws-1/deploy-hook":
			if got, want := r.Header.Get(coreapi.WebhookDeliveryIDHeader), "delivery-2"; got != want {
				t.Fatalf("workspace webhook delivery id = %q, want %q", got, want)
			}
			writeJSON(w, aghcontract.WebhookDeliveryResponse{
				Result: aghcontract.WebhookDeliveryPayload{
					Matched: 1,
					Runs: []aghcontract.RunPayload{{
						ID:        "run-2",
						SessionID: "sess-2",
						Status:    automationpkg.RunCompleted,
					}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	harness := &RuntimeHarness{
		UDSBaseURL:  server.URL,
		UDSClient:   server.Client(),
		HTTPBaseURL: server.URL,
		HTTPClient:  server.Client(),
	}

	run, err := harness.TriggerAutomationJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("TriggerAutomationJob() error = %v", err)
	}
	if got, want := run.TaskRunID, "task-run-1"; got != want {
		t.Fatalf("run.TaskRunID = %q, want %q", got, want)
	}

	runs, err := harness.ListAutomationRuns(context.Background(), url.Values{"status": {"completed"}})
	if err != nil {
		t.Fatalf("ListAutomationRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}

	storedRun, err := harness.GetAutomationRun(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("GetAutomationRun() error = %v", err)
	}
	if got, want := storedRun.SessionID, "sess-1"; got != want {
		t.Fatalf("storedRun.SessionID = %q, want %q", got, want)
	}

	tasks, err := harness.ListTasks(context.Background(), url.Values{"workspace": {"ws-1"}})
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if got, want := len(tasks), 1; got != want {
		t.Fatalf("len(tasks) = %d, want %d", got, want)
	}

	taskDetail, err := harness.GetTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := taskDetail.Task.ID, "task-1"; got != want {
		t.Fatalf("taskDetail.Task.ID = %q, want %q", got, want)
	}

	taskRuns, err := harness.ListTaskRuns(context.Background(), "task-1", url.Values{"status": {"queued"}})
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if got, want := len(taskRuns), 1; got != want {
		t.Fatalf("len(taskRuns) = %d, want %d", got, want)
	}

	claimed, err := harness.ClaimExactTaskRunForSession(context.Background(), "task-run-1", aghcontract.SessionPayload{
		ID:          "sess-agent",
		AgentName:   "worker",
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("ClaimExactTaskRunForSession() error = %v", err)
	}
	if got, want := claimed.Status, taskpkg.TaskRunStatusClaimed; got != want {
		t.Fatalf("claimed.Status = %q, want %q", got, want)
	}

	started, err := harness.StartTaskRun(context.Background(), "task-run-1", aghcontract.StartTaskRunRequest{
		IdempotencyKey: "start-1",
	})
	if err != nil {
		t.Fatalf("StartTaskRun() error = %v", err)
	}
	if got, want := started.SessionID, "sess-1"; got != want {
		t.Fatalf("started.SessionID = %q, want %q", got, want)
	}

	completed, err := harness.CompleteClaimedTaskRunForSession(
		context.Background(),
		"task-run-1",
		aghcontract.SessionPayload{ID: "sess-agent", AgentName: "worker", WorkspaceID: "ws-1"},
		aghcontract.AgentTaskCompleteRequest{
			Result: json.RawMessage(`{"ok":true}`),
		})
	if err != nil {
		t.Fatalf("CompleteClaimedTaskRunForSession() error = %v", err)
	}
	if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("completed.Status = %q, want %q", got, want)
	}

	globalDelivery, err := harness.DeliverGlobalWebhook(
		context.Background(),
		"deploy-hook",
		"shared-secret",
		webhookPayload,
		"delivery-1",
		webhookTime,
	)
	if err != nil {
		t.Fatalf("DeliverGlobalWebhook() error = %v", err)
	}
	if got, want := len(globalDelivery.Runs), 1; got != want {
		t.Fatalf("len(globalDelivery.Runs) = %d, want %d", got, want)
	}

	workspaceDelivery, err := harness.DeliverWorkspaceWebhook(
		context.Background(),
		"ws-1",
		"deploy-hook",
		"shared-secret",
		webhookPayload,
		"delivery-2",
		webhookTime,
	)
	if err != nil {
		t.Fatalf("DeliverWorkspaceWebhook() error = %v", err)
	}
	if got, want := workspaceDelivery.Runs[0].ID, "run-2"; got != want {
		t.Fatalf("workspaceDelivery.Runs[0].ID = %q, want %q", got, want)
	}

	if claimRequest.RunID != "task-run-1" || claimRequest.WorkspaceID != "ws-1" {
		t.Fatalf("claimRequest = %#v, want exact workspace claim", claimRequest)
	}
	if got, want := startRequest.IdempotencyKey, "start-1"; got != want {
		t.Fatalf("startRequest.IdempotencyKey = %q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(string(completeRequest.Result)), `{"ok":true}`; got != want {
		t.Fatalf("completeRequest.Result = %q, want %q", got, want)
	}
}

func TestDeliverWebhookRejectsMissingRequiredFields(t *testing.T) {
	t.Parallel()

	harness := &RuntimeHarness{}
	payload := []byte(`{"payload":"deploy"}`)
	now := time.Now().UTC()

	tests := []struct {
		name       string
		secret     string
		payload    []byte
		deliveryID string
		timestamp  time.Time
		wantErr    string
	}{
		{
			name:       "missing secret",
			payload:    payload,
			deliveryID: "delivery-1",
			timestamp:  now,
			wantErr:    "webhook secret is required",
		},
		{
			name:       "missing payload",
			secret:     "shared-secret",
			deliveryID: "delivery-1",
			timestamp:  now,
			wantErr:    "webhook payload is required",
		},
		{
			name:      "missing delivery id",
			secret:    "shared-secret",
			payload:   payload,
			timestamp: now,
			wantErr:   "webhook delivery id is required",
		},
		{
			name:       "missing timestamp",
			secret:     "shared-secret",
			payload:    payload,
			deliveryID: "delivery-1",
			wantErr:    "webhook timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := harness.deliverWebhook(
				context.Background(),
				"/api/webhooks/global/deploy-hook",
				tt.secret,
				tt.payload,
				tt.deliveryID,
				tt.timestamp,
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("deliverWebhook() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
