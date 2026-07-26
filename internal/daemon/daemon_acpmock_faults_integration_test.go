//go:build integration && !windows

package daemon

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
)

const faultyMockAgentName = "mock-faulty"

func TestDaemonE2EACPmockCrashMidStreamProjectsRuntimeFailure(t *testing.T) {
	acpmock.RequireDriver(t)

	t.Run("Should project runtime failure when the ACP mock crashes mid-stream", func(t *testing.T) {
		t.Parallel()

		harness, session := startFaultyMockSession(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		stream, err := harness.PromptSessionHTTP(ctx, session.ID, "trigger crash mid-stream")
		if err != nil {
			t.Fatalf("PromptSessionHTTP() error = %v", err)
		}
		assertFaultPromptProjection(
			t,
			ctx,
			harness,
			session.ID,
			stream,
			"partial before crash",
			false,
		)
	})
}

func TestDaemonE2EACPmockCrashEscalatesBoundTaskRun(t *testing.T) {
	acpmock.RequireDriver(t)

	t.Run("Should expose a crashed task session as needs attention across status surfaces", func(t *testing.T) {
		t.Parallel()

		harness := startFaultyMockHarness(t)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		taskRecord := createFaultyProfiledTask(t, ctx, harness)
		run := enqueueTaskRunViaUDS(t, ctx, harness, taskRecord.ID, "")
		claimant := createFixtureBackedSession(t, ctx, harness, faultyMockAgentName, "faulty-claimant")
		var claimResponse aghcontract.AgentTaskClaimResponse
		agentUDSJSON(
			t,
			ctx,
			harness,
			claimant,
			http.MethodPost,
			"/api/agent/tasks/claim-next",
			aghcontract.AgentTaskClaimNextRequest{
				RunID:        run.ID,
				WorkspaceID:  harness.WorkspaceID,
				LeaseSeconds: 30,
			},
			&claimResponse,
		)
		claimed := claimResponse.Claim.Run
		if got, want := claimed.Status.Normalize(), taskpkg.TaskRunStatusClaimed; got != want {
			t.Fatalf("claimed run status = %q, want %q", got, want)
		}
		started, err := harness.StartTaskRun(ctx, run.ID, aghcontract.StartTaskRunRequest{})
		if err != nil {
			t.Fatalf("StartTaskRun(%q) error = %v", run.ID, err)
		}
		if got, want := started.Status.Normalize(), taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("started run status = %q, want %q", got, want)
		}
		if strings.TrimSpace(started.SessionID) == "" {
			t.Fatal("started run session_id = empty, want task-bound session")
		}

		stream, err := harness.PromptSessionHTTP(ctx, started.SessionID, "trigger crash mid-stream")
		if err != nil {
			t.Fatalf("PromptSessionHTTP() error = %v", err)
		}
		if !sseStreamContainsEvent(stream, "error") {
			t.Fatalf("prompt stream = %#v, want error event after process exit", stream)
		}

		waitForRuntimeCondition(t, "crashed task run needs attention", 10*time.Second, func() bool {
			runs, listErr := harness.ListTaskRuns(ctx, taskRecord.ID, nil)
			if listErr != nil {
				return false
			}
			current, ok := findTaskRunPayload(runs, run.ID)
			return ok && current.Status.Normalize() == taskpkg.TaskRunStatusNeedsAttention
		})

		sessionInfo, err := harness.GetSession(ctx, started.SessionID)
		if err != nil {
			t.Fatalf("GetSession(%q) error = %v", started.SessionID, err)
		}
		if got, want := sessionInfo.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("task session stop reason = %q, want %q", got, want)
		}

		detail, err := harness.GetTask(ctx, taskRecord.ID)
		if err != nil {
			t.Fatalf("GetTask(%q) error = %v", taskRecord.ID, err)
		}
		if got := countTaskEvents(detail.Events, eventspkg.TaskRunNeedsAttention, run.ID); got != 1 {
			t.Fatalf("needs-attention event count = %d, want 1; events=%#v", got, detail.Events)
		}

		statusPath := "/api/status?workspace=" + url.QueryEscape(harness.WorkspaceRoot)
		var httpStatus aghcontract.StatusPayload
		if err := harness.HTTPJSON(ctx, http.MethodGet, statusPath, nil, &httpStatus); err != nil {
			t.Fatalf("HTTP status error = %v", err)
		}
		var udsStatus aghcontract.StatusPayload
		if err := harness.UDSJSON(ctx, http.MethodGet, statusPath, nil, &udsStatus); err != nil {
			t.Fatalf("UDS status error = %v", err)
		}
		var cliStatus aghcontract.StatusPayload
		if err := harness.CLI.RunJSON(ctx, &cliStatus, "status", "-o", "json"); err != nil {
			t.Fatalf("CLI JSON status error = %v", err)
		}
		for surface, status := range map[string]aghcontract.StatusPayload{
			"HTTP": httpStatus,
			"UDS":  udsStatus,
			"CLI":  cliStatus,
		} {
			if got := taskRunTotal(status.Tasks, taskpkg.TaskRunStatusNeedsAttention); got != 1 {
				t.Fatalf("%s needs_attention total = %d, want 1; totals=%#v", surface, got, status.Tasks.RunTotals)
			}
		}

		stdout, stderr, err := harness.CLI.Run(ctx, "status")
		if err != nil {
			t.Fatalf("CLI human status error = %v; stderr=%s", err, strings.TrimSpace(stderr))
		}
		if !strings.Contains(stdout, "Subprocess Health") ||
			!strings.Contains(stdout, "Needs Attention") ||
			!strings.Contains(stdout, "1 task runs") {
			t.Fatalf("CLI human status = %q, want subprocess and needs-attention summaries", stdout)
		}
	})
}

func TestDaemonE2EACPmockInvalidFrameProjectsRuntimeFailure(t *testing.T) {
	acpmock.RequireDriver(t)

	t.Run("Should project runtime failure when the ACP mock emits an invalid frame", func(t *testing.T) {
		t.Parallel()

		harness, session := startFaultyMockSession(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		stream, err := harness.PromptSessionHTTP(ctx, session.ID, "trigger invalid frame")
		if err != nil {
			t.Fatalf("PromptSessionHTTP() error = %v", err)
		}
		assertFaultPromptProjection(
			t,
			ctx,
			harness,
			session.ID,
			stream,
			"partial before invalid frame",
			false,
		)
	})
}

func TestDaemonE2EACPmockPermissionDisconnectProjectsRuntimeFailure(t *testing.T) {
	acpmock.RequireDriver(t)

	t.Run("Should project runtime failure when the ACP mock disconnects during permission", func(t *testing.T) {
		t.Parallel()

		harness, session := startFaultyMockSession(t)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		stream, err := harness.PromptSessionHTTP(ctx, session.ID, "trigger permission disconnect")
		if err != nil {
			t.Fatalf("PromptSessionHTTP() error = %v", err)
		}
		assertFaultPromptProjection(
			t,
			ctx,
			harness,
			session.ID,
			stream,
			"",
			true,
		)
	})
}

func TestDaemonE2EACPmockBlockedCancelStopsPromptWithoutOrphaning(t *testing.T) {
	acpmock.RequireDriver(t)

	t.Run("Should stop a blocked prompt without orphaning runtime liveness", func(t *testing.T) {
		t.Parallel()

		harness, session := startFaultyMockSession(t, func(cfg *aghconfig.Config) {
			cfg.Session.Supervision.ActivityHeartbeatInterval = 20 * time.Millisecond
			cfg.Session.Supervision.ProgressNotifyInterval = 20 * time.Millisecond
			cfg.Session.Supervision.InactivityWarningAfter = 0
			cfg.Session.Supervision.InactivityTimeout = 0
		})

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		stream, err := harness.PromptSessionHTTPUntil(
			ctx,
			session.ID,
			"block until canceled",
			func(event e2etest.SSEEvent) bool {
				return event.Event == "runtime_progress"
			},
		)
		if err != nil {
			t.Fatalf("PromptSessionHTTPUntil(blocked progress) error = %v", err)
		}
		if !sseStreamContainsEvent(stream, "runtime_progress") {
			t.Fatalf("prompt stream = %#v, want runtime_progress before explicit stop", stream)
		}

		sessionStream, err := harness.StreamSessionRawHTTPUntil(ctx, session.ID, func(event e2etest.SSEEvent) bool {
			return event.Event == "runtime_progress"
		})
		if err != nil {
			t.Fatalf("StreamSessionRawHTTPUntil(runtime_progress) error = %v", err)
		}
		if !sseStreamContainsEvent(sessionStream, "runtime_progress") {
			t.Fatalf("session stream = %#v, want runtime_progress replay", sessionStream)
		}

		if err := harness.StopSession(ctx, session.ID); err != nil {
			t.Fatalf("StopSession(%q) error = %v", session.ID, err)
		}

		waitForRuntimeCondition(t, "blocked ACP session stopped", 10*time.Second, func() bool {
			current, err := harness.GetSession(ctx, session.ID)
			return err == nil && string(current.State) == "stopped"
		})

		sessionInfo, err := harness.GetSession(ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSession(%q) error = %v", session.ID, err)
		}
		if got, want := string(sessionInfo.State), "stopped"; got != want {
			t.Fatalf("sessionInfo.State = %q, want %q", got, want)
		}
		if got, want := sessionInfo.StopReason, store.StopUserCanceled; got != want {
			t.Fatalf("sessionInfo.StopReason = %q, want %q", got, want)
		}

		meta := mustReadSessionMeta(t, harness, session.ID)
		if meta.Liveness == nil {
			t.Fatal("meta.Liveness = nil, want persisted liveness metadata")
		}
		if got := meta.Liveness.SubprocessPID; got != 0 {
			t.Fatalf("meta.Liveness.SubprocessPID = %d, want 0 after clean stop", got)
		}
		if got := strings.TrimSpace(meta.Liveness.StallState); got != "" {
			t.Fatalf("meta.Liveness.StallState = %q, want empty after clean stop", got)
		}
		if got := strings.TrimSpace(meta.Liveness.StallReason); got != "" {
			t.Fatalf("meta.Liveness.StallReason = %q, want empty after clean stop", got)
		}

		eventsResp := mustSessionEvents(t, ctx, harness, session.ID)
		events := decodeAgentEvents(t, eventsResp.Events)
		if !containsAgentEvent(events, aghcontract.AgentEventPayload{Type: "runtime_progress"}) {
			t.Fatalf("events = %#v, want runtime_progress before blocked peer stop", events)
		}
		assertNoFatalBlockedCancelError(t, events)

		if err := harness.CaptureSessionTranscript(ctx, session.ID); err != nil {
			t.Fatalf("CaptureSessionTranscript() error = %v", err)
		}
		if err := harness.CaptureSessionEvents(ctx, session.ID); err != nil {
			t.Fatalf("CaptureSessionEvents() error = %v", err)
		}
		if err := harness.CaptureSessionSandbox(ctx, session.ID); err != nil {
			t.Fatalf("CaptureSessionSandbox() error = %v", err)
		}
	})
}

func startFaultyMockSession(
	t testing.TB,
	mutateConfig ...func(*aghconfig.Config),
) (*e2etest.RuntimeHarness, aghcontract.SessionPayload) {
	t.Helper()

	harness := startFaultyMockHarness(t, mutateConfig...)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session := createFixtureBackedSession(t, ctx, harness, faultyMockAgentName, "faulty-session")
	return harness, session
}

func startFaultyMockHarness(
	t testing.TB,
	mutateConfig ...func(*aghconfig.Config),
) *e2etest.RuntimeHarness {
	t.Helper()

	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			Mutate: func(cfg *aghconfig.Config) {
				for _, mutate := range mutateConfig {
					if mutate != nil {
						mutate(cfg)
					}
				}
			},
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "driver_fault_fixture.json"),
			FixtureAgent: "faulty",
			AgentName:    faultyMockAgentName,
		}},
	})

}

func createFaultyProfiledTask(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) aghcontract.TaskPayload {
	t.Helper()

	var response aghcontract.TaskResponse
	request := aghcontract.CreateTaskRequest{
		Scope:     taskpkg.ScopeWorkspace,
		Workspace: harness.WorkspaceID,
		Title:     "Escalate crashed task subprocess",
	}
	if err := harness.UDSJSON(ctx, http.MethodPost, "/api/tasks", request, &response); err != nil {
		t.Fatalf("UDS create faulty-profiled task error = %v", err)
	}

	profileRequest := aghcontract.SetTaskExecutionProfileRequest{
		Worker: taskpkg.WorkerProfile{
			Mode:      taskpkg.WorkerModeSelect,
			AgentName: faultyMockAgentName,
		},
	}
	var profileResponse aghcontract.TaskExecutionProfileResponse
	profilePath := "/api/tasks/" + url.PathEscape(response.Task.ID) + "/execution-profile"
	if err := harness.UDSJSON(ctx, http.MethodPut, profilePath, profileRequest, &profileResponse); err != nil {
		t.Fatalf("UDS set faulty task execution profile error = %v", err)
	}
	if got, want := profileResponse.Profile.Worker.AgentName, faultyMockAgentName; got != want {
		t.Fatalf("stored worker agent = %q, want %q", got, want)
	}
	return response.Task
}

func countTaskEvents(events []aghcontract.TaskEventPayload, eventType string, runID string) int {
	count := 0
	for _, event := range events {
		if event.EventType == eventType && event.RunID == runID {
			count++
		}
	}
	return count
}

func taskRunTotal(health aghcontract.TaskHealthPayload, status taskpkg.RunStatus) int {
	total := 0
	for _, row := range health.RunTotals {
		if row.Status == status.String() {
			total += row.Count
		}
	}
	return total
}

func assertFaultPromptProjection(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	stream []e2etest.SSEEvent,
	wantTranscriptFragment string,
	wantPermission bool,
) {
	t.Helper()

	if !sseStreamContainsEvent(stream, "error") {
		t.Fatalf("prompt stream = %#v, want error event", stream)
	}
	if wantTranscriptFragment != "" && !e2etest.RecordsContainTextDelta(stream, wantTranscriptFragment) {
		t.Fatalf("prompt stream = %#v, want assistant text delta %q", stream, wantTranscriptFragment)
	}
	if wantPermission && !sseStreamContainsEvent(stream, "permission") {
		t.Fatalf("prompt stream = %#v, want permission event before failure", stream)
	}

	httpSession := mustHTTPSession(t, ctx, harness, sessionID)
	udsSession, err := harness.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", sessionID, err)
	}
	if got, want := httpSession.ID, sessionID; got != want {
		t.Fatalf("HTTP session ID = %q, want %q", got, want)
	}
	if got, want := udsSession.ID, sessionID; got != want {
		t.Fatalf("UDS session ID = %q, want %q", got, want)
	}

	transcript := mustSessionTranscript(t, ctx, harness, sessionID)
	content := joinTranscriptContent(sessionTranscriptMessages(transcript))
	if wantTranscriptFragment != "" && !strings.Contains(content, wantTranscriptFragment) {
		t.Fatalf("transcript = %q, want fragment %q", content, wantTranscriptFragment)
	}
	if wantPermission && strings.Contains(content, "allow-always") {
		t.Fatalf("transcript = %q, want no approval decision after disconnect", content)
	}

	eventsResp := mustSessionEvents(t, ctx, harness, sessionID)
	events := decodeAgentEvents(t, eventsResp.Events)
	if wantTranscriptFragment != "" && !containsAgentEvent(events, aghcontract.AgentEventPayload{
		Type: "agent_message",
		Text: wantTranscriptFragment,
	}) {
		t.Fatalf("events = %#v, want agent_message %q", events, wantTranscriptFragment)
	}
	if !containsAgentEvent(events, aghcontract.AgentEventPayload{Type: "error"}) {
		t.Fatalf("events = %#v, want error event", events)
	}
	if containsAgentEvent(events, aghcontract.AgentEventPayload{Type: "done"}) {
		t.Fatalf("events = %#v, want no done event after runtime failure", events)
	}
	if wantPermission && !containsAgentEvent(events, aghcontract.AgentEventPayload{Type: "permission"}) {
		t.Fatalf("events = %#v, want permission event", events)
	}

	if err := harness.CaptureSessionTranscript(ctx, sessionID); err != nil {
		t.Fatalf("CaptureSessionTranscript() error = %v", err)
	}
	if err := harness.CaptureSessionEvents(ctx, sessionID); err != nil {
		t.Fatalf("CaptureSessionEvents() error = %v", err)
	}
	if err := harness.CaptureSessionSandbox(ctx, sessionID); err != nil {
		t.Fatalf("CaptureSessionSandbox() error = %v", err)
	}

	assertArtifactExists(t, harness, e2etest.ArtifactKindTranscript)
	assertArtifactExists(t, harness, e2etest.ArtifactKindEvents)
	assertArtifactExists(t, harness, e2etest.ArtifactKindSessionSandbox)
}

func assertNoFatalBlockedCancelError(t testing.TB, events []aghcontract.AgentEventPayload) {
	t.Helper()

	for _, event := range events {
		if event.Type != "error" {
			continue
		}
		if event.Failure != nil && event.Failure.Kind == store.FailureCanceled {
			continue
		}
		t.Fatalf("events = %#v, want no fatal error after explicit blocked prompt cancellation", events)
	}
}

func mustHTTPSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) aghcontract.SessionPayload {
	t.Helper()

	var response aghcontract.SessionResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodGet,
		"/api/workspaces/"+url.PathEscape(harness.WorkspaceID)+"/sessions/"+url.PathEscape(sessionID),
		nil,
		&response,
	); err != nil {
		t.Fatalf("HTTP session %q error = %v", sessionID, err)
	}
	return response.Session
}

func sseStreamContainsEvent(records []e2etest.SSEEvent, want string) bool {
	for _, record := range records {
		if record.Event == want {
			return true
		}
	}
	return false
}

func assertArtifactExists(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	kind e2etest.ArtifactKind,
) {
	t.Helper()

	path, ok := harness.Artifacts.ArtifactPath(kind)
	if !ok {
		t.Fatalf("ArtifactPath(%s) = missing, want present", kind)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}
}
