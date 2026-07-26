//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/compozy/agh/internal/network/participation"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

const automationTaskFixtureAgentName = "mock-automation-runner"

func TestDaemonE2EAutomationPromptTriggerCreatesCompletedSystemSession(t *testing.T) {
	acpmock.RequireDriver(t)

	harness := startAutomationTaskHarness(t, mockFixturePath(t, "automation_task_fixture.json"))
	registration, ok := harness.MockAgentRegistration(automationTaskFixtureAgentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%s) = missing, want present", automationTaskFixtureAgentName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	seeded, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
		Triggers: []aghcontract.CreateTriggerRequest{{
			Scope:              automationpkg.AutomationScopeGlobal,
			Name:               "deploy-review",
			AgentName:          automationTaskFixtureAgentName,
			Prompt:             `Review payload {{ index .Data "payload" }} for {{ index .Data "branch" }}`,
			Event:              "webhook",
			EndpointSlug:       "deploy-review",
			WebhookSecretValue: "shared-secret",
		}},
	})
	if err != nil {
		t.Fatalf("SeedAutomationFixtures(trigger) error = %v", err)
	}
	if got, want := len(seeded.Triggers), 1; got != want {
		t.Fatalf("len(seeded.Triggers) = %d, want %d", got, want)
	}
	trigger := seeded.Triggers[0]

	registerAutomationPromptArtifacts(t, harness, trigger.ID, registration)

	endpoint, err := automationpkg.FormatWebhookEndpoint(trigger.EndpointSlug, trigger.WebhookID)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}

	delivery, err := harness.DeliverGlobalWebhook(
		ctx,
		endpoint,
		"shared-secret",
		[]byte(`{"payload":"deploy","branch":"main"}`),
		"delivery-deploy-review",
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("DeliverGlobalWebhook() error = %v", err)
	}
	if got, want := delivery.Matched, 1; got != want {
		t.Fatalf("delivery.Matched = %d, want %d", got, want)
	}
	if got, want := len(delivery.Runs), 1; got != want {
		t.Fatalf("len(delivery.Runs) = %d, want %d", got, want)
	}

	globalWorkspace, err := harness.ResolveWorkspace(ctx, harness.HomePaths.HomeDir)
	if err != nil {
		t.Fatalf("ResolveWorkspace(%q) error = %v", harness.HomePaths.HomeDir, err)
	}
	if strings.TrimSpace(globalWorkspace.ID) == "" {
		t.Fatalf("global automation workspace ID = empty; workspace=%#v", globalWorkspace)
	}

	runID := delivery.Runs[0].ID
	waitForRuntimeCondition(t, "automation webhook run completion", 10*time.Second, func() bool {
		run, err := harness.GetAutomationRun(ctx, runID)
		if err != nil {
			return false
		}
		if err := requireCompletedSessionAutomationRun(run); err != nil {
			return false
		}
		sessionInfo, err := harness.GetSession(ctx, run.SessionID)
		if err != nil {
			return false
		}
		return sessionInfo.State == sessionpkg.StateStopped && sessionInfo.StopReason == store.StopCompleted
	})

	run, err := harness.GetAutomationRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetAutomationRun(%q) error = %v", runID, err)
	}
	if err := requireCompletedSessionAutomationRun(run); err != nil {
		t.Fatalf("requireCompletedSessionAutomationRun() error = %v", err)
	}
	if got, want := run.TriggerID, trigger.ID; got != want {
		t.Fatalf("run.TriggerID = %q, want %q", got, want)
	}
	if run.EndedAt == nil {
		t.Fatal("run.EndedAt = nil, want completed timestamp")
	}

	runs, err := harness.ListAutomationRuns(ctx, url.Values{"trigger_id": {trigger.ID}})
	if err != nil {
		t.Fatalf("ListAutomationRuns(trigger) error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(ListAutomationRuns(trigger)) = %d, want %d", got, want)
	}

	sessionInfo, err := harness.GetSession(ctx, run.SessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", run.SessionID, err)
	}
	if got, want := sessionInfo.AgentName, automationTaskFixtureAgentName; got != want {
		t.Fatalf("sessionInfo.AgentName = %q, want %q", got, want)
	}
	if got, want := sessionInfo.State, sessionpkg.StateStopped; got != want {
		t.Fatalf("sessionInfo.State = %q, want %q", got, want)
	}
	if got, want := sessionInfo.StopReason, store.StopCompleted; got != want {
		t.Fatalf("sessionInfo.StopReason = %q, want %q", got, want)
	}

	meta := mustReadSessionMeta(t, harness, run.SessionID)
	if got, want := meta.SessionType, string(sessionpkg.SessionTypeSystem); got != want {
		t.Fatalf("session meta type = %q, want %q", got, want)
	}
	if got, want := meta.State, string(sessionpkg.StateStopped); got != want {
		t.Fatalf("session meta state = %q, want %q", got, want)
	}

	transcriptResp := mustSessionTranscript(t, ctx, harness, run.SessionID)
	transcriptContent := joinTranscriptContent(sessionTranscriptMessages(transcriptResp))
	if !strings.Contains(transcriptContent, "Review payload deploy for main") {
		t.Fatalf("transcript = %q, want rendered automation prompt", transcriptContent)
	}
	if !strings.Contains(transcriptContent, "Automation review completed for deploy on main.") {
		t.Fatalf("transcript = %q, want mock agent completion", transcriptContent)
	}
}

func TestDaemonE2EAutomationTaskBackedJobDelegatesTaskRun(t *testing.T) {
	acpmock.RequireDriver(t)

	harness := startAutomationTaskHarness(t, mockFixturePath(t, "automation_task_fixture.json"))
	registration, ok := harness.MockAgentRegistration(automationTaskFixtureAgentName)
	if !ok {
		t.Fatalf("MockAgentRegistration(%s) = missing, want present", automationTaskFixtureAgentName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	live := participation.ModeLive
	named := participation.StrategyNamed
	channelID := "ops-automation"
	if _, err := harness.CreateNetworkChannel(ctx, aghcontract.CreateNetworkChannelRequest{
		Channel:      channelID,
		WorkspaceID:  harness.WorkspaceID,
		Purpose:      "Automation task coordination",
		FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
		AgentNames:   []string{automationTaskFixtureAgentName},
	}); err != nil {
		t.Fatalf("CreateNetworkChannel(%q) error = %v", channelID, err)
	}

	seeded, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
		Jobs: []aghcontract.CreateJobRequest{{
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: harness.WorkspaceID,
			Name:        "triage-deploy",
			Prompt:      "Investigate deployment drift.",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
			Task: &automationpkg.JobTaskConfig{
				Title:       "Investigate deploy drift",
				Description: "Review the latest deployment discrepancy.",
				NetworkParticipation: &participation.Request{
					Mode:            &live,
					ChannelStrategy: &named,
					ChannelID:       &channelID,
				},
				Owner: &taskpkg.Ownership{
					Kind: taskpkg.OwnerKindAutomation,
					Ref:  "job:triage-deploy",
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("SeedAutomationFixtures(job) error = %v", err)
	}
	if got, want := len(seeded.Jobs), 1; got != want {
		t.Fatalf("len(seeded.Jobs) = %d, want %d", got, want)
	}
	job := seeded.Jobs[0]

	run, err := harness.TriggerAutomationJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("TriggerAutomationJob(%q) error = %v", job.ID, err)
	}
	if err := requireDelegatedTaskAutomationRun(run); err != nil {
		t.Fatalf("requireDelegatedTaskAutomationRun() error = %v", err)
	}
	if got, want := run.JobID, job.ID; got != want {
		t.Fatalf("run.JobID = %q, want %q", got, want)
	}

	taskDetail, err := harness.GetTask(ctx, run.TaskID)
	if err != nil {
		t.Fatalf("GetTask(%q) error = %v", run.TaskID, err)
	}
	if got, want := taskDetail.Task.Scope, taskpkg.ScopeWorkspace; got != want {
		t.Fatalf("taskDetail.Task.Scope = %q, want %q", got, want)
	}
	if got, want := taskDetail.Task.WorkspaceID, harness.WorkspaceID; got != want {
		t.Fatalf("taskDetail.Task.WorkspaceID = %q, want %q", got, want)
	}
	if got, want := taskDetail.Task.Status, taskpkg.TaskStatusReady; got != want {
		t.Fatalf("taskDetail.Task.Status = %q, want %q before task runtime start", got, want)
	}
	if got := resolvedParticipationChannelID(taskDetail.Task.ResolvedNetworkParticipation); got != "" {
		t.Fatalf(
			"taskDetail.Task.ResolvedNetworkParticipation.ChannelID = %q, want empty task ingress policy",
			got,
		)
	}
	if taskDetail.Task.Owner == nil || taskDetail.Task.Owner.Kind != taskpkg.OwnerKindAutomation {
		t.Fatalf("taskDetail.Task.Owner = %#v, want automation ownership", taskDetail.Task.Owner)
	}
	if got, want := taskDetail.Task.CreatedBy.Kind, taskpkg.ActorKindAutomation; got != want {
		t.Fatalf("taskDetail.Task.CreatedBy.Kind = %q, want %q", got, want)
	}
	if got, want := taskDetail.Task.CreatedBy.Ref, job.ID; got != want {
		t.Fatalf("taskDetail.Task.CreatedBy.Ref = %q, want %q", got, want)
	}
	if got, want := taskDetail.Task.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("taskDetail.Task.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := taskDetail.Task.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("taskDetail.Task.Origin.Ref = %q, want %q", got, want)
	}

	taskRuns, err := harness.ListTaskRuns(ctx, run.TaskID, nil)
	if err != nil {
		t.Fatalf("ListTaskRuns(%q) error = %v", run.TaskID, err)
	}
	delegatedTaskRun, ok := findTaskRunPayload(taskRuns, run.TaskRunID)
	if !ok {
		t.Fatalf("ListTaskRuns() missing %q in %#v", run.TaskRunID, taskRuns)
	}
	if got, want := delegatedTaskRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("delegatedTaskRun.Status = %q, want %q", got, want)
	}
	if got, want := delegatedTaskRun.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("delegatedTaskRun.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := delegatedTaskRun.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("delegatedTaskRun.Origin.Ref = %q, want %q", got, want)
	}
	if got, want := delegatedTaskRun.IdempotencyKey, "automation-run:"+run.ID; got != want {
		t.Fatalf("delegatedTaskRun.IdempotencyKey = %q, want %q", got, want)
	}
	if got, want := resolvedParticipationChannelID(
		delegatedTaskRun.ResolvedNetworkParticipation,
	), channelID; got != want {
		t.Fatalf(
			"delegatedTaskRun.ResolvedNetworkParticipation.ChannelID = %q, want resolved %q",
			got,
			want,
		)
	}
	if got := delegatedTaskRun.SessionID; got != "" {
		t.Fatalf("delegatedTaskRun.SessionID = %q, want empty before start", got)
	}

	worker := createFixtureBackedSession(
		t,
		ctx,
		harness,
		automationTaskFixtureAgentName,
		"automation-task-claim-worker",
	)
	claimedRun, err := harness.ClaimExactTaskRunForSession(ctx, run.TaskRunID, worker)
	if err != nil {
		t.Fatalf("ClaimExactTaskRunForSession(%q) error = %v", run.TaskRunID, err)
	}
	if got, want := claimedRun.Status, taskpkg.TaskRunStatusClaimed; got != want {
		t.Fatalf("claimedRun.Status = %q, want %q", got, want)
	}

	startedRun, err := harness.StartTaskRun(ctx, run.TaskRunID, aghcontract.StartTaskRunRequest{})
	if err != nil {
		t.Fatalf("StartTaskRun(%q) error = %v", run.TaskRunID, err)
	}
	if got, want := startedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("startedRun.Status = %q, want %q", got, want)
	}
	if strings.TrimSpace(startedRun.SessionID) == "" {
		t.Fatal("startedRun.SessionID = empty, want linked task session")
	}

	registerAutomationTaskArtifacts(t, harness, job.ID, run.TaskID, startedRun.SessionID, registration)

	taskSessionInfo, err := harness.GetSession(ctx, startedRun.SessionID)
	if err != nil {
		t.Fatalf("GetSession(task session) error = %v", err)
	}
	if got, want := taskSessionInfo.AgentName, automationTaskFixtureAgentName; got != want {
		t.Fatalf("taskSessionInfo.AgentName = %q, want %q", got, want)
	}
	if got, want := taskSessionInfo.State, sessionpkg.StateActive; got != want {
		t.Fatalf("taskSessionInfo.State = %q, want %q", got, want)
	}

	taskMeta := mustReadSessionMeta(t, harness, startedRun.SessionID)
	if got, want := taskMeta.SessionType, string(sessionpkg.SessionTypeSystem); got != want {
		t.Fatalf("task session meta type = %q, want %q", got, want)
	}
	if got, want := taskMeta.AgentName, automationTaskFixtureAgentName; got != want {
		t.Fatalf("task session meta agent = %q, want %q", got, want)
	}

	if _, err := harness.PromptSession(ctx, startedRun.SessionID, "Continue delegated task run"); err != nil {
		t.Fatalf("PromptSession(task session) error = %v", err)
	}

	taskTranscript := mustSessionTranscript(t, ctx, harness, startedRun.SessionID)
	taskTranscriptContent := joinTranscriptContent(sessionTranscriptMessages(taskTranscript))
	if !strings.Contains(taskTranscriptContent, "Continue delegated task run") {
		t.Fatalf("task transcript = %q, want task-session prompt", taskTranscriptContent)
	}
	if !strings.Contains(taskTranscriptContent, "Delegated task session responded.") {
		t.Fatalf("task transcript = %q, want mock task-session response", taskTranscriptContent)
	}

	var completedLease aghcontract.AgentTaskLeaseResponse
	agentUDSJSON(
		t,
		ctx,
		harness,
		taskSessionInfo,
		http.MethodPost,
		"/api/agent/tasks/"+url.PathEscape(run.TaskRunID)+"/complete",
		aghcontract.AgentTaskCompleteRequest{Result: json.RawMessage(`{"result":"ok"}`)},
		&completedLease,
	)
	if got, want := completedLease.Lease.Status.Normalize(), taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("completedLease.Lease.Status = %q, want %q", got, want)
	}

	taskDetail, err = harness.GetTask(ctx, run.TaskID)
	if err != nil {
		t.Fatalf("GetTask(after complete) error = %v", err)
	}
	if got, want := taskDetail.Task.Status, taskpkg.TaskStatusCompleted; got != want {
		t.Fatalf("taskDetail.Task.Status after complete = %q, want %q", got, want)
	}
	detailRun, ok := findTaskRunInDetail(&taskDetail, run.TaskRunID)
	if !ok {
		t.Fatalf("task detail runs missing %q in %#v", run.TaskRunID, taskDetail.Runs)
	}
	if got, want := detailRun.Status, taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("detailRun.Status = %q, want %q", got, want)
	}
	if got, want := detailRun.SessionID, startedRun.SessionID; got != want {
		t.Fatalf("detailRun.SessionID = %q, want %q", got, want)
	}

	stillDelegated, err := harness.GetAutomationRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetAutomationRun(after task start) error = %v", err)
	}
	if err := requireDelegatedTaskAutomationRun(stillDelegated); err != nil {
		t.Fatalf("requireDelegatedTaskAutomationRun(after task start) error = %v", err)
	}
}

func startAutomationTaskHarness(
	t testing.TB,
	fixturePath string,
) *e2etest.RuntimeHarness {
	t.Helper()

	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent: automationTaskFixtureAgentName,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  fixturePath,
			FixtureAgent: "automation-runner",
			AgentName:    automationTaskFixtureAgentName,
		}},
	})

}

func mustReadSessionMeta(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) store.SessionMeta {
	t.Helper()

	metaPath := store.SessionMetaFile(filepath.Join(harness.HomePaths.SessionsDir, strings.TrimSpace(sessionID)))
	meta, err := store.ReadSessionMeta(metaPath)
	if err != nil {
		t.Fatalf("ReadSessionMeta(%q) error = %v", metaPath, err)
	}
	return meta
}

func registerAutomationPromptArtifacts(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	triggerID string,
	registration acpmock.Registration,
) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		runs, err := harness.ListAutomationRuns(ctx, url.Values{"trigger_id": {triggerID}})
		if err != nil {
			t.Logf("ListAutomationRuns(trigger=%q) artifact error = %v", triggerID, err)
			return
		}
		if err := harness.Artifacts.CaptureJSON(e2etest.ArtifactKindAutomationRuns, runs); err != nil {
			t.Logf("CaptureJSON(automation_runs) error = %v", err)
		}
		if len(runs) == 0 || strings.TrimSpace(runs[0].SessionID) == "" {
			return
		}
		if err := harness.CaptureSessionTranscript(ctx, runs[0].SessionID); err != nil {
			t.Logf("CaptureSessionTranscript(%q) error = %v", runs[0].SessionID, err)
		}
		if err := harness.CaptureSessionEvents(ctx, runs[0].SessionID); err != nil {
			t.Logf("CaptureSessionEvents(%q) error = %v", runs[0].SessionID, err)
		}
		if err := harness.CaptureSessionSandbox(ctx, runs[0].SessionID); err != nil {
			t.Logf("CaptureSessionSandbox(%q) error = %v", runs[0].SessionID, err)
		}
		if err := harness.CaptureMockAgentDiagnostics(registration); err != nil {
			t.Logf("CaptureMockAgentDiagnostics() error = %v", err)
		}
	})
}

func registerAutomationTaskArtifacts(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	jobID string,
	taskID string,
	sessionID string,
	registration acpmock.Registration,
) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := harness.CaptureAutomationRuns(ctx, url.Values{"job_id": {jobID}}); err != nil {
			t.Logf("CaptureAutomationRuns(job=%q) error = %v", jobID, err)
		}
		if err := harness.CaptureTasks(ctx, url.Values{"workspace": {harness.WorkspaceID}}); err != nil {
			t.Logf("CaptureTasks(workspace=%q) error = %v", harness.WorkspaceID, err)
		}
		if err := harness.CaptureTaskRuns(ctx, taskID, nil); err != nil {
			t.Logf("CaptureTaskRuns(task=%q) error = %v", taskID, err)
		}
		if strings.TrimSpace(sessionID) != "" {
			if err := harness.CaptureSessionTranscript(ctx, sessionID); err != nil {
				t.Logf("CaptureSessionTranscript(%q) error = %v", sessionID, err)
			}
			if err := harness.CaptureSessionEvents(ctx, sessionID); err != nil {
				t.Logf("CaptureSessionEvents(%q) error = %v", sessionID, err)
			}
			if err := harness.CaptureSessionSandbox(ctx, sessionID); err != nil {
				t.Logf("CaptureSessionSandbox(%q) error = %v", sessionID, err)
			}
		}
		if err := harness.CaptureMockAgentDiagnostics(registration); err != nil {
			t.Logf("CaptureMockAgentDiagnostics() error = %v", err)
		}
	})
}
