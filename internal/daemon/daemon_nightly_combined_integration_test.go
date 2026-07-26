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
	aghcontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	extensiontest "github.com/compozy/agh/internal/extensiontest"
	"github.com/compozy/agh/internal/sandbox"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	"github.com/kballard/go-shellquote"
)

const (
	nightlyCombinedHelperEnvKey     = "AGH_TEST_NIGHTLY_COMBINED_HELPER"
	nightlyCombinedScenarioEnvKey   = "AGH_TEST_NIGHTLY_COMBINED_SCENARIO"
	nightlyCombinedTaskScenario     = "task-resume-network"
	nightlyCombinedBridgeScenario   = "bridge-sandbox-delivery"
	nightlyCombinedTaskAgentName    = "nightly-task-network-runner"
	nightlyCombinedBridgeAgentName  = "nightly-bridge-sandbox-runner"
	nightlyCombinedEnvProfileName   = "nightly-local-sandbox"
	nightlyTaskResumePrompt         = "Resume the delegated task and post a nightly network reply."
	nightlyTaskResumeAssistant      = "Nightly delegated task resumed and replied over the network."
	nightlyTaskResumeMessageID      = "msg_task_resume_01"
	nightlyTaskResumeTraceID        = "trace_task_resume_01"
	nightlyTaskSideEffectRelative   = "toolhost/nightly-task-network.txt"
	nightlyBridgeIngressText        = "Need a nightly bridge tool summary"
	nightlyBridgeIngressAssistant   = "Nightly bridge ingress accepted."
	nightlyBridgeToolPrompt         = "Run the nightly bridge sandbox tool summary."
	nightlyBridgeAssistantPrefix    = "Bridge tool summary: "
	nightlyBridgeSideEffectRelative = "toolhost/nightly-bridge-summary.txt"
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

func TestDaemonNightlyE2EAutomationTaskResumesIntoNetworkChannel(t *testing.T) {
	harness := startNightlyCombinedTaskHarness(t)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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
		"ops-nightly",
		&sessionID,
		&taskID,
		&taskRunID,
		&runID,
		&diagnostics,
		&combined,
	)
	if _, err := harness.CreateNetworkChannel(ctx, aghcontract.CreateNetworkChannelRequest{
		Channel:      "ops-nightly",
		WorkspaceID:  harness.WorkspaceID,
		Purpose:      "Nightly delegated task coordination",
		FanoutPolicy: store.NetworkFanoutPolicyAllMembers,
		AgentNames:   []string{nightlyCombinedTaskAgentName},
	}); err != nil {
		t.Fatalf("CreateNetworkChannel(%q) error = %v", "ops-nightly", err)
	}

	seeded, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
		Jobs: []aghcontract.CreateJobRequest{{
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: harness.WorkspaceID,
			Name:        "nightly-triage",
			Prompt:      "Investigate nightly regression drift.",
			Schedule: automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "24h",
			},
			Task: &automationpkg.JobTaskConfig{
				Title:                "Nightly delegated regression follow-up",
				Description:          "Resume the delegated regression and post the result to the shared channel.",
				NetworkParticipation: daemonTestNamedParticipationRequest("ops-nightly"),
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

	worker := createFixtureBackedSession(
		t,
		ctx,
		harness,
		nightlyCombinedTaskAgentName,
		"nightly-task-claim-worker",
	)
	if _, err := harness.ClaimExactTaskRunForSession(ctx, run.TaskRunID, worker); err != nil {
		t.Fatalf("ClaimExactTaskRunForSession(%q) error = %v", run.TaskRunID, err)
	}
	startedRun, err := harness.StartTaskRun(ctx, run.TaskRunID, aghcontract.StartTaskRunRequest{})
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
		Channel:         "ops-nightly",
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
	if got, want := resolvedParticipationChannelID(resumed.ResolvedNetworkParticipation), "ops-nightly"; got != want {
		t.Fatalf("resumed.ResolvedNetworkParticipation.ChannelID = %q, want %q", got, want)
	}
	if resumed.Sandbox == nil {
		t.Fatal("resumed.Sandbox = nil, want sandbox metadata after resume")
	}
	if got, want := resumed.Sandbox.Profile, nightlyCombinedEnvProfileName; got != want {
		t.Fatalf("resumed.Sandbox.Profile = %q, want %q", got, want)
	}

	stream, err := harness.PromptSession(ctx, sessionID, nightlyTaskResumePrompt)
	if err != nil {
		t.Fatalf("PromptSession(%q) error = %v", sessionID, err)
	}
	if len(stream) == 0 {
		t.Fatal("prompt stream = empty, want runtime events")
	}
	sentMessage, err := harness.NetworkSend(ctx, aghcontract.NetworkSendRequest{
		SessionID: sessionID,
		Channel:   "ops-nightly",
		Surface:   "thread",
		ThreadID:  "thread_ops_nightly",
		Kind:      "say",
		ID:        nightlyTaskResumeMessageID,
		TraceID:   nightlyTaskResumeTraceID,
		Body:      json.RawMessage(`{"text":"` + nightlyTaskResumeAssistant + `","intent":"status","artifacts":[]}`),
	})
	if err != nil {
		t.Fatalf("NetworkSend(%q) error = %v", sessionID, err)
	}
	if got, want := sentMessage.ID, nightlyTaskResumeMessageID; got != want {
		t.Fatalf("sentMessage.ID = %q, want %q", got, want)
	}

	waitForRuntimeCondition(t, "nightly network reply visible", 15*time.Second, func() bool {
		return channelHasMessageID(ctx, harness, "ops-nightly", nightlyTaskResumeMessageID) &&
			sessionTranscriptHasNeedle(ctx, harness, sessionID, nightlyTaskResumeAssistant)
	})

	channelMessages := mustHTTPNetworkChannelMessages(t, ctx, harness, "ops-nightly")
	requireChannelMessage(t, channelMessages, nightlyTaskResumeMessageID, nightlyTaskResumeAssistant)
	combined.NetworkMessageIDs = []string{nightlyTaskResumeMessageID}

	audit := mustNetworkAuditSnapshot(t, harness)
	if err := validateNetworkAuditEntry(audit, networkAuditExpectation{
		MessageID: nightlyTaskResumeMessageID,
		Direction: "sent",
		Kind:      "say",
	}); err != nil {
		t.Fatalf("validateNetworkAuditEntry(nightly task reply) error = %v", err)
	}

	taskSideEffectBytes, err := os.ReadFile(taskSideEffect)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", taskSideEffect, err)
	}
	if got, want := string(taskSideEffectBytes), "resumed-network"; got != want {
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

	taskSession, err := harness.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) before task completion error = %v", sessionID, err)
	}
	completedRun, err := harness.CompleteClaimedTaskRunForSession(ctx, run.TaskRunID, taskSession, aghcontract.AgentTaskCompleteRequest{
		Result: json.RawMessage(`{"network_reply":"` + nightlyTaskResumeMessageID + `"}`),
	})
	if err != nil {
		t.Fatalf("CompleteClaimedTaskRunForSession(%q) error = %v", run.TaskRunID, err)
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
	if got, want := meta.NetworkParticipation.ChannelID, "ops-nightly"; got != want {
		t.Fatalf("session meta channel = %q, want %q", got, want)
	}
	if meta.Sandbox == nil {
		t.Fatal("session meta sandbox = nil, want persisted sandbox metadata")
	}
	if got, want := meta.Sandbox.Profile, nightlyCombinedEnvProfileName; got != want {
		t.Fatalf("session meta sandbox profile = %q, want %q", got, want)
	}
}

func TestDaemonNightlyE2EBridgeIngressDeliversThenUserSandboxTool(t *testing.T) {
	repoRoot := daemonBridgeRuntimeRepoRoot(t)
	extensionDir := prepareDaemonTelegramReferenceExtension(t, repoRoot)

	markers := extensiontest.NewTempMarkerPaths(t)
	env := markers.Env()
	delete(env, extensiontest.EnvCrashOncePath)
	env["AGH_TEST_TELEGRAM_TOKEN"] = "telegram-bot-token"

	configSeed := nightlyCombinedConfigSeed(
		t,
		nightlyCombinedBridgeAgentName,
		nightlyCombinedBridgeScenario,
	)
	configSeed.Mutate = allowUnverifiedBridgeExtensionInstalls
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: configSeed,
		Env:        env,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	var (
		bridgeID       string
		sessionID      string
		diagnostics    e2etest.ToolHostDiagnosticsArtifact
		combined       e2etest.CombinedFlowArtifact
		sideEffectPath = filepath.Join(harness.WorkspaceRoot, nightlyBridgeSideEffectRelative)
	)
	registerNightlyBridgeCombinedArtifacts(
		t,
		harness,
		markers,
		&bridgeID,
		&sessionID,
		&diagnostics,
		&combined,
	)

	checksum, err := extensionpkg.ComputeDirectoryChecksum(extensionDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", extensionDir, err)
	}

	if _, err := harness.InstallExtension(ctx, aghcontract.InstallExtensionRequest{
		Path:            extensionDir,
		Checksum:        checksum,
		AllowUnverified: true,
	}); err != nil {
		t.Fatalf("InstallExtension(%q) error = %v", extensionDir, err)
	}

	waitForRuntimeCondition(t, "nightly bridge extension registered", 10*time.Second, func() bool {
		ext, err := harness.GetExtension(ctx, "telegram-reference")
		return err == nil && ext.Enabled
	})

	createdBridge, err := harness.CreateBridge(ctx, aghcontract.CreateBridgeRequest{
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   harness.WorkspaceID,
		Platform:      "telegram",
		ExtensionName: "telegram-reference",
		DisplayName:   "Nightly Bridge Runtime E2E",
		Enabled:       false,
		RoutingPolicy: bridgepkg.RoutingPolicy{
			IncludePeer:   true,
			IncludeThread: true,
		},
	})
	if err != nil {
		t.Fatalf("CreateBridge() error = %v", err)
	}
	bridgeID = createdBridge.Bridge.ID
	combined = e2etest.CombinedFlowArtifact{
		Scenario:        nightlyCombinedBridgeScenario,
		BridgeID:        bridgeID,
		SideEffectPaths: []string{sideEffectPath},
	}

	secretValue := "telegram-bot-token"
	if _, err := harness.PutBridgeSecretBinding(
		ctx,
		bridgeID,
		"bot_token",
		aghcontract.PutBridgeSecretBindingRequest{
			SecretRef:   "vault:bridges/" + bridgeID + "/bot_token",
			Kind:        "token",
			SecretValue: &secretValue,
		},
	); err != nil {
		t.Fatalf("PutBridgeSecretBinding() error = %v", err)
	}

	if _, err := harness.EnableBridge(ctx, bridgeID); err != nil {
		t.Fatalf("EnableBridge(%q) error = %v", bridgeID, err)
	}

	waitForRuntimeCondition(t, "nightly bridge ready", 10*time.Second, func() bool {
		bridge, err := harness.GetBridge(ctx, bridgeID)
		return err == nil &&
			bridge.Health.Status.Normalize() == bridgepkg.BridgeStatusReady &&
			bridge.Health.RouteCount == 0
	})

	extensiontest.AppendInboundUpdateMarker(
		t,
		markers,
		telegramRuntimeInboundUpdate(time.Now().UTC(), 9901, 777, nightlyBridgeIngressText),
	)

	ingests := extensiontest.WaitForIngestMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.IngestRecord) bool {
			return len(records) >= 1 && strings.TrimSpace(records[0].Result.SessionID) != ""
		},
	)
	sessionID = ingests[0].Result.SessionID
	combined.SessionID = sessionID

	waitForRuntimeCondition(t, "nightly bridge transcript", 15*time.Second, func() bool {
		return sessionTranscriptHasNeedle(ctx, harness, sessionID, nightlyBridgeIngressText) &&
			sessionTranscriptHasNeedle(ctx, harness, sessionID, nightlyBridgeIngressAssistant)
	})

	deliveries := extensiontest.WaitForDeliveryMarkers(
		t,
		markers,
		15*time.Second,
		func(records []extensiontest.DeliveryRecord) bool {
			return countDeliveryEvents(records, bridgepkg.DeliveryEventTypeFinal) >= 1
		},
	)
	if !hasDeliveryEventType(deliveries, bridgepkg.DeliveryEventTypeStart) {
		t.Fatalf("deliveries = %#v, want start event", deliveries)
	}
	if !hasDeliveryEventType(deliveries, bridgepkg.DeliveryEventTypeFinal) {
		t.Fatalf("deliveries = %#v, want final event", deliveries)
	}

	routes, err := harness.ListBridgeRoutes(ctx, bridgeID)
	if err != nil {
		t.Fatalf("ListBridgeRoutes(%q) error = %v", bridgeID, err)
	}
	if got, want := len(routes), 1; got != want {
		t.Fatalf("len(routes) = %d, want %d", got, want)
	}
	if got, want := routes[0].SessionID, sessionID; got != want {
		t.Fatalf("routes[0].SessionID = %q, want %q", got, want)
	}

	stream, err := harness.PromptSession(ctx, sessionID, nightlyBridgeToolPrompt)
	if err != nil {
		t.Fatalf("PromptSession(%q) error = %v", sessionID, err)
	}
	if len(stream) == 0 {
		t.Log("PromptSession returned no immediate SSE records; waiting for queued dispatch")
	}

	waitForRuntimeCondition(t, "nightly bridge sandbox tool transcript", 15*time.Second, func() bool {
		return sessionTranscriptHasNeedle(ctx, harness, sessionID, nightlyBridgeAssistantPrefix+"bridge-nightly")
	})

	sideEffectBytes, err := os.ReadFile(sideEffectPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", sideEffectPath, err)
	}
	if got, want := string(sideEffectBytes), "bridge-nightly"; got != want {
		t.Fatalf("bridge side effect = %q, want %q", got, want)
	}

	diagnostics = e2etest.ToolHostDiagnosticsArtifact{
		SessionID: sessionID,
		Operations: []e2etest.ToolHostOperationDiagnostic{{
			Operation:        "write_text_file",
			Path:             nightlyBridgeSideEffectRelative,
			Outcome:          e2etest.ToolHostOutcomeAllowed,
			SideEffectPath:   sideEffectPath,
			SideEffectExists: true,
		}},
	}

	bridge, err := harness.GetBridge(ctx, bridgeID)
	if err != nil {
		t.Fatalf("GetBridge(%q) error = %v", bridgeID, err)
	}
	if got, want := bridge.Health.RouteCount, 1; got != want {
		t.Fatalf("bridge.Health.RouteCount = %d, want %d", got, want)
	}
	if bridge.Health.LastSuccessAt == nil {
		t.Fatal("bridge.Health.LastSuccessAt = nil, want successful bridge delivery")
	}

	if err := harness.StopSession(ctx, sessionID); err != nil {
		t.Fatalf("StopSession(%q) error = %v", sessionID, err)
	}
	waitForRuntimeCondition(t, "nightly bridge session stopped", 10*time.Second, func() bool {
		current, err := harness.GetSession(ctx, sessionID)
		return err == nil && current.State == sessionpkg.StateStopped
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
		return a.promptTaskResumeNetwork(ctx, params)
	case nightlyCombinedBridgeScenario:
		return a.promptBridgeSandboxDelivery(ctx, params)
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

func (a *daemonNightlyCombinedACPAgent) promptTaskResumeNetwork(
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
		Content:   "resumed-network",
	}); err != nil {
		return acpsdk.PromptResponse{}, err
	}

	return a.sendMessageAndEndTurn(ctx, params.SessionId, nightlyTaskResumeAssistant)
}

func (a *daemonNightlyCombinedACPAgent) promptBridgeSandboxDelivery(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	text := nightlyCombinedPromptText(params.Prompt)
	if strings.Contains(text, nightlyBridgeIngressText) {
		return a.sendMessageAndEndTurn(ctx, params.SessionId, nightlyBridgeIngressAssistant)
	}
	if !strings.Contains(text, nightlyBridgeToolPrompt) {
		return acpsdk.PromptResponse{}, fmt.Errorf("unexpected nightly bridge prompt %q", text)
	}

	if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
		SessionId: params.SessionId,
		Path:      nightlyBridgeSideEffectRelative,
		Content:   "bridge-nightly",
	}); err != nil {
		return acpsdk.PromptResponse{}, err
	}
	readResp, err := a.conn.ReadTextFile(ctx, acpsdk.ReadTextFileRequest{
		SessionId: params.SessionId,
		Path:      nightlyBridgeSideEffectRelative,
	})
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}

	return a.sendMessageAndEndTurn(ctx, params.SessionId, nightlyBridgeAssistantPrefix+readResp.Content)
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
	if idx := strings.LastIndex(lastText, userRequestMarker); idx >= 0 {
		return strings.TrimSpace(lastText[idx+len(userRequestMarker):])
	}
	return strings.TrimSpace(lastText)
}

func startNightlyCombinedTaskHarness(t testing.TB) *e2etest.RuntimeHarness {
	t.Helper()

	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		EnableNetwork: true,
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
		DefaultSandbox: nightlyCombinedEnvProfileName,
		PermissionMode: aghconfig.PermissionModeApproveAll,
		Providers: map[string]aghconfig.ProviderConfig{
			acpmock.ProviderName: acpmock.ProviderConfig(helperCommand),
		},
		Sandboxes: map[string]aghconfig.SandboxProfile{
			nightlyCombinedEnvProfileName: {
				Backend:     string(sandbox.BackendLocal),
				Persistence: string(sandbox.PersistenceReuse),
			},
		},
		AgentDefs: []e2etest.AgentSeed{{
			Name:        agentName,
			Provider:    acpmock.ProviderName,
			Command:     helperCommand,
			Permissions: string(aghconfig.PermissionModeApproveAll),
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
	channel string,
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
			if err := harness.CaptureSessionSandbox(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionSandbox(%q) error = %v", trimmedSessionID, err)
			}
		}
		if err := harness.CaptureNetworkArtifacts(ctx, strings.TrimSpace(channel)); err != nil {
			t.Logf("CaptureNetworkArtifacts(%q) error = %v", channel, err)
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

func registerNightlyBridgeCombinedArtifacts(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	markers extensiontest.MarkerPaths,
	bridgeID *string,
	sessionID *string,
	diagnostics *e2etest.ToolHostDiagnosticsArtifact,
	combined *e2etest.CombinedFlowArtifact,
) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if trimmedBridgeID := strings.TrimSpace(derefStringValue(bridgeID)); trimmedBridgeID != "" {
			if err := harness.CaptureBridgeHealth(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeHealth(%q) error = %v", trimmedBridgeID, err)
			}
			if err := harness.CaptureBridgeRoutes(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeRoutes(%q) error = %v", trimmedBridgeID, err)
			}
			if err := harness.CaptureBridgeDeliveryState(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeDeliveryState(%q) error = %v", trimmedBridgeID, err)
			}
			if err := harness.CaptureBridgeSecretBindings(ctx, trimmedBridgeID); err != nil {
				t.Logf("CaptureBridgeSecretBindings(%q) error = %v", trimmedBridgeID, err)
			}
		}
		if trimmedSessionID := strings.TrimSpace(derefStringValue(sessionID)); trimmedSessionID != "" {
			if err := harness.CaptureSessionTranscript(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionTranscript(%q) error = %v", trimmedSessionID, err)
			}
			if err := harness.CaptureSessionEvents(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionEvents(%q) error = %v", trimmedSessionID, err)
			}
			if err := harness.CaptureSessionSandbox(ctx, trimmedSessionID); err != nil {
				t.Logf("CaptureSessionSandbox(%q) error = %v", trimmedSessionID, err)
			}
		}
		payload := map[string]any{
			"provider_markers": extensiontest.ReportFromMarkers(t, markers),
		}
		if err := harness.CaptureProviderCallsJSON(payload); err != nil {
			t.Logf("CaptureProviderCallsJSON() error = %v", err)
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
