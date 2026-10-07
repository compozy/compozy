//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	sessionpkg "github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	shellquote "github.com/kballard/go-shellquote"
)

const (
	nightlyCombinedHelperEnvKey   = "COMPOZY_TEST_NIGHTLY_COMBINED_HELPER"
	nightlyCombinedScenarioEnvKey = "COMPOZY_TEST_NIGHTLY_COMBINED_SCENARIO"
	nightlyCombinedTaskScenario   = "task-resume-local"
	nightlyCombinedTaskAgentName  = "nightly-task-runner"
	nightlyTaskResumePrompt       = "Resume the delegated task and write the nightly result."
	nightlyTaskResumeAssistant    = "Nightly delegated task resumed and wrote the result."
	nightlyTaskSideEffectRelative = "toolhost/nightly-task-resume.txt"
)

func TestDaemonNightlyCombinedACPHelperProcess(t *testing.T) {
	if os.Getenv(nightlyCombinedHelperEnvKey) != "1" {
		return
	}

	agent := &daemonNightlyCombinedACPAgent{
		scenario: strings.TrimSpace(os.Getenv(nightlyCombinedScenarioEnvKey)),
	}
	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

func TestDaemonNightlyE2EAutomationTaskResumesAndWritesWorkspace(t *testing.T) {
	t.Run("Should resume an automation task and persist its local tool result", func(t *testing.T) {
		harness := startNightlyCombinedTaskHarness(t)

		ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
		defer cancel()

		var (
			sessionID      string
			taskID         string
			taskRunID      string
			runID          string
			diagnostics    e2etest.ToolHostDiagnosticsArtifact
			combined       e2etest.CombinedFlowArtifact
			taskSideEffect = filepath.Join(harness.WorkspaceRoot, nightlyTaskSideEffectRelative)
		)
		registerNightlyAutomationCombinedArtifacts(
			t,
			harness,
			&sessionID,
			&taskID,
			&taskRunID,
			&runID,
			&diagnostics,
			&combined,
		)

		seeded, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
			Jobs: []compozycontract.CreateJobRequest{{
				Scope:       automationpkg.AutomationScopeWorkspace,
				WorkspaceID: harness.WorkspaceID,
				Name:        "nightly-triage",
				Prompt:      "Investigate nightly regression drift.",
				Schedule: automationpkg.ScheduleSpec{
					Mode:     automationpkg.ScheduleModeEvery,
					Interval: "24h",
				},
				Task: &automationpkg.JobTaskConfig{
					Title:       "Nightly delegated regression follow-up",
					Description: "Resume the delegated regression and write its result to the workspace.",
					Owner: &taskpkg.Ownership{
						Kind: taskpkg.OwnerKindAutomation,
						Ref:  "job:nightly-triage",
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
		runID = run.ID
		taskID = run.TaskID
		taskRunID = run.TaskRunID

		startedRun, err := harness.StartTaskRun(ctx, run.TaskRunID, compozycontract.StartTaskRunRequest{})
		if err != nil {
			t.Fatalf("StartTaskRun(%q) error = %v", run.TaskRunID, err)
		}
		if got, want := startedRun.Status, taskpkg.TaskRunStatusRunning; got != want {
			t.Fatalf("startedRun.Status = %q, want %q", got, want)
		}
		sessionID = startedRun.SessionID
		if strings.TrimSpace(sessionID) == "" {
			t.Fatal("startedRun.SessionID = empty, want delegated task session")
		}

		combined = e2etest.CombinedFlowArtifact{
			Scenario:        nightlyCombinedTaskScenario,
			SessionID:       sessionID,
			AutomationRunID: run.ID,
			JobID:           job.ID,
			TaskID:          run.TaskID,
			TaskRunID:       run.TaskRunID,
			SideEffectPaths: []string{taskSideEffect},
		}

		waitForRuntimeCondition(t, "nightly task session active", 10*time.Second, func() bool {
			current, err := harness.GetSession(ctx, sessionID)
			return err == nil && current.State == sessionpkg.StateActive
		})

		resumed, err := harness.ResumeSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("ResumeSession(%q) error = %v", sessionID, err)
		}
		if got, want := resumed.State, sessionpkg.StateActive; got != want {
			t.Fatalf("resumed.State = %q, want %q", got, want)
		}

		stream, err := harness.PromptSession(ctx, sessionID, nightlyTaskResumePrompt)
		if err != nil {
			t.Fatalf("PromptSession(%q) error = %v", sessionID, err)
		}
		if len(stream) == 0 {
			t.Fatal("prompt stream = empty, want runtime events")
		}
		waitForRuntimeCondition(t, "nightly task reply visible", 15*time.Second, func() bool {
			return sessionTranscriptHasNeedle(ctx, harness, sessionID, nightlyTaskResumeAssistant)
		})

		taskSideEffectBytes, err := os.ReadFile(taskSideEffect)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", taskSideEffect, err)
		}
		if got, want := string(taskSideEffectBytes), "resumed-local"; got != want {
			t.Fatalf("task side effect = %q, want %q", got, want)
		}

		diagnostics = e2etest.ToolHostDiagnosticsArtifact{
			SessionID: sessionID,
			Operations: []e2etest.ToolHostOperationDiagnostic{{
				Operation:        "write_text_file",
				Path:             nightlyTaskSideEffectRelative,
				Outcome:          e2etest.ToolHostOutcomeAllowed,
				SideEffectPath:   taskSideEffect,
				SideEffectExists: true,
			}},
		}

		completedRun, err := harness.CompleteTaskRun(
			ctx,
			run.TaskRunID,
			compozycontract.CompleteTaskRunRequest{
				Result: json.RawMessage(`{"result":"resumed-local"}`),
			},
		)
		if err != nil {
			t.Fatalf("CompleteTaskRun(%q) error = %v", run.TaskRunID, err)
		}
		if got, want := completedRun.Status, taskpkg.TaskRunStatusCompleted; got != want {
			t.Fatalf("completedRun.Status = %q, want %q", got, want)
		}

		taskDetail, err := harness.GetTask(ctx, run.TaskID)
		if err != nil {
			t.Fatalf("GetTask(%q) error = %v", run.TaskID, err)
		}
		if got, want := taskDetail.Task.Status, taskpkg.TaskStatusCompleted; got != want {
			t.Fatalf("taskDetail.Task.Status = %q, want %q", got, want)
		}

		if current, err := harness.GetSession(ctx, sessionID); err == nil && current.State != sessionpkg.StateStopped {
			if err := harness.StopSession(ctx, sessionID); err != nil {
				t.Fatalf("StopSession(%q) after completion error = %v", sessionID, err)
			}
			waitForRuntimeCondition(t, "nightly task session stopped after completion", 10*time.Second, func() bool {
				current, err := harness.GetSession(ctx, sessionID)
				return err == nil && current.State == sessionpkg.StateStopped
			})
		}

		meta := mustReadSessionMeta(t, harness, sessionID)
		if meta.State != string(sessionpkg.StateStopped) {
			t.Fatalf("session meta state = %q, want stopped", meta.State)
		}
	})
}

type daemonNightlyCombinedACPAgent struct {
	conn     *acpsdk.AgentSideConnection
	scenario string
}

func (a *daemonNightlyCombinedACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *daemonNightlyCombinedACPAgent) Logout(
	context.Context,
	acpsdk.LogoutRequest,
) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (a *daemonNightlyCombinedACPAgent) Initialize(
	context.Context,
	acpsdk.InitializeRequest,
) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: true,
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (a *daemonNightlyCombinedACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (a *daemonNightlyCombinedACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (a *daemonNightlyCombinedACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{Sessions: []acpsdk.SessionInfo{}}, nil
}

func (a *daemonNightlyCombinedACPAgent) NewSession(
	context.Context,
	acpsdk.NewSessionRequest,
) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{SessionId: "daemon-nightly-combined-helper"}, nil
}

func (a *daemonNightlyCombinedACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (a *daemonNightlyCombinedACPAgent) SetSessionConfigOption(
	context.Context,
	acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{ConfigOptions: []acpsdk.SessionConfigOption{}}, nil
}

func (a *daemonNightlyCombinedACPAgent) LoadSession(
	context.Context,
	acpsdk.LoadSessionRequest,
) (acpsdk.LoadSessionResponse, error) {
	return acpsdk.LoadSessionResponse{}, nil
}

func (a *daemonNightlyCombinedACPAgent) Prompt(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	switch a.scenario {
	case nightlyCombinedTaskScenario:
		return a.promptTaskResume(ctx, params)
	default:
		return acpsdk.PromptResponse{}, fmt.Errorf("unknown nightly combined scenario %q", a.scenario)
	}
}

func (a *daemonNightlyCombinedACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (a *daemonNightlyCombinedACPAgent) promptTaskResume(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	text := nightlyCombinedPromptText(params.Prompt)
	if !strings.Contains(text, nightlyTaskResumePrompt) {
		return acpsdk.PromptResponse{}, fmt.Errorf("unexpected nightly task prompt %q", text)
	}

	if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
		SessionId: params.SessionId,
		Path:      nightlyTaskSideEffectRelative,
		Content:   "resumed-local",
	}); err != nil {
		return acpsdk.PromptResponse{}, err
	}

	return a.sendMessageAndEndTurn(ctx, params.SessionId, nightlyTaskResumeAssistant)
}

func (a *daemonNightlyCombinedACPAgent) sendMessageAndEndTurn(
	ctx context.Context,
	sessionID acpsdk.SessionId,
	message string,
) (acpsdk.PromptResponse, error) {
	if a.conn != nil {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID,
			Update:    acpsdk.UpdateAgentMessageText(message),
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
	}
	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func nightlyCombinedPromptText(blocks []acpsdk.ContentBlock) string {
	lastText := ""
	for _, block := range blocks {
		switch {
		case block.Text != nil:
			lastText = block.Text.Text
		}
	}
	const userRequestMarker = "User request:"
	if _, request, found := strings.CutLast(lastText, userRequestMarker); found {
		return strings.TrimSpace(request)
	}
	return strings.TrimSpace(lastText)
}

func startNightlyCombinedTaskHarness(t testing.TB) *e2etest.RuntimeHarness {
	t.Helper()

	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: nightlyCombinedConfigSeed(
			t,
			nightlyCombinedTaskAgentName,
			nightlyCombinedTaskScenario,
		),
		Workspace: e2etest.WorkspaceSeedOptions{
			Files: map[string]string{
				"README.md": "nightly combined runtime workspace",
			},
		},
	})

}

func nightlyCombinedConfigSeed(
	t testing.TB,
	agentName string,
	scenario string,
) e2etest.ConfigSeedOptions {
	t.Helper()
	helperCommand := daemonNightlyCombinedHelperCommand(t, scenario)

	return e2etest.ConfigSeedOptions{
		DefaultAgent:   agentName,
		PermissionMode: compozyconfig.PermissionModeApproveAll,
		Providers: map[string]compozyconfig.ProviderConfig{
			acpmock.ProviderName: acpmock.ProviderConfig(helperCommand),
		},
		AgentDefs: []e2etest.AgentSeed{{
			Name:        agentName,
			Provider:    acpmock.ProviderName,
			Command:     helperCommand,
			Permissions: string(compozyconfig.PermissionModeApproveAll),
			Prompt:      "You are a deterministic nightly combined-flow helper.",
		}},
	}
}

func daemonNightlyCombinedHelperCommand(t testing.TB, scenario string) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return shellquote.Join(
		"env",
		nightlyCombinedHelperEnvKey+"=1",
		nightlyCombinedScenarioEnvKey+"="+strings.TrimSpace(scenario),
		bin,
		"-test.run=TestDaemonNightlyCombinedACPHelperProcess",
	)
}

func registerNightlyAutomationCombinedArtifacts(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	sessionID *string,
	taskID *string,
	taskRunID *string,
	runID *string,
	diagnostics *e2etest.ToolHostDiagnosticsArtifact,
	combined *e2etest.CombinedFlowArtifact,
) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if trimmedRunID := strings.TrimSpace(derefStringValue(runID)); trimmedRunID != "" {
			if err := harness.CaptureAutomationRuns(ctx, nil); err != nil {
				t.Logf("CaptureAutomationRuns() error = %v", err)
			}
		}
		if err := harness.CaptureTasks(ctx, urlValues("workspace", harness.WorkspaceID)); err != nil {
			t.Logf("CaptureTasks(workspace=%q) error = %v", harness.WorkspaceID, err)
		}
		if trimmedTaskID := strings.TrimSpace(derefStringValue(taskID)); trimmedTaskID != "" {
			if err := harness.CaptureTaskRuns(ctx, trimmedTaskID, nil); err != nil {
				t.Logf("CaptureTaskRuns(%q) error = %v", trimmedTaskID, err)
			}
		}
		if trimmedSessionID := strings.TrimSpace(derefStringValue(sessionID)); trimmedSessionID != "" {
			if err := harness.CaptureSessionTranscript(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionTranscript(%q) error = %v", trimmedSessionID, err)
			}
			if err := harness.CaptureSessionEvents(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionEvents(%q) error = %v", trimmedSessionID, err)
			}
		}
		if diagnostics != nil && len(diagnostics.Operations) > 0 {
			if err := harness.CaptureToolHostDiagnosticsJSON(*diagnostics); err != nil {
				t.Logf("CaptureToolHostDiagnosticsJSON() error = %v", err)
			}
		}
		if combined != nil && strings.TrimSpace(combined.Scenario) != "" {
			if err := harness.CaptureCombinedFlowJSON(*combined); err != nil {
				t.Logf("CaptureCombinedFlowJSON() error = %v", err)
			}
		}
	})
}

func urlValues(pairs ...string) url.Values {
	values := make(url.Values, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		values[strings.TrimSpace(pairs[i])] = []string{strings.TrimSpace(pairs[i+1])}
	}
	return values
}

func derefStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
