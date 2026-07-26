//go:build integration && !windows

package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
	sessionpkg "github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	"github.com/kballard/go-shellquote"
)

const (
	daemonSandboxHelperEnvKey         = "AGH_TEST_DAEMON_ENV_HELPER"
	daemonSandboxHelperScenarioEnvKey = "AGH_TEST_DAEMON_ENV_SCENARIO"
	daemonSandboxFixtureAgentName     = "sandbox-helper"
	daemonSandboxProfileName          = "local-sandbox"
	daemonSandboxScenarioAllowed      = "allowed"
	daemonSandboxScenarioBlocked      = "blocked"
)

func TestDaemonSandboxACPHelperProcess(t *testing.T) {
	if os.Getenv(daemonSandboxHelperEnvKey) != "1" {
		return
	}

	agent := &daemonSandboxACPAgent{
		scenario: strings.TrimSpace(os.Getenv(daemonSandboxHelperScenarioEnvKey)),
	}
	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

func TestDaemonE2ELocalSandboxAllowsToolExecutionAndPersistsMetadata(t *testing.T) {
	harness := startSandboxRuntimeHarness(t, daemonSandboxScenarioAllowed, aghconfig.PermissionModeApproveAll)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		sessionID      string
		diagnostics    e2etest.ToolHostDiagnosticsArtifact
		sideEffectPath = filepath.Join(harness.WorkspaceRoot, "toolhost", "allowed.txt")
	)
	registerSandboxRuntimeArtifacts(t, harness, &sessionID, &diagnostics)

	sessionInfo, stream := mustRunSandboxScenarioSession(
		t,
		ctx,
		harness,
		"allowed-local-runtime",
		"write an allowed runtime file",
	)
	sessionID = sessionInfo.ID
	if len(stream) == 0 {
		t.Fatal("prompt stream = empty, want runtime events")
	}
	if err := harness.StopSession(ctx, sessionID); err != nil {
		t.Fatalf("StopSession(%q) error = %v", sessionID, err)
	}

	waitForRuntimeCondition(t, "allowed sandbox session stopped", 10*time.Second, func() bool {
		current, err := harness.GetSession(ctx, sessionID)
		return err == nil && current.State == sessionpkg.StateStopped
	})

	sessionInfo, err := harness.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", sessionID, err)
	}
	if got, want := sessionInfo.State, sessionpkg.StateStopped; got != want {
		t.Fatalf("sessionInfo.State = %q, want %q", got, want)
	}
	if got, want := sessionInfo.StopReason, store.StopUserCanceled; got != want {
		t.Fatalf("sessionInfo.StopReason = %q, want %q", got, want)
	}
	if sessionInfo.Sandbox == nil {
		t.Fatal("sessionInfo.Sandbox = nil, want local sandbox metadata")
	}
	if got, want := sessionInfo.Sandbox.Backend, string(sandbox.BackendLocal); got != want {
		t.Fatalf("sessionInfo.Sandbox.Backend = %q, want %q", got, want)
	}
	if got, want := sessionInfo.Sandbox.Profile, daemonSandboxProfileName; got != want {
		t.Fatalf("sessionInfo.Sandbox.Profile = %q, want %q", got, want)
	}
	if got, want := sessionInfo.Sandbox.State, "stopped"; got != want {
		t.Fatalf("sessionInfo.Sandbox.State = %q, want %q", got, want)
	}

	content, err := os.ReadFile(sideEffectPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", sideEffectPath, err)
	}
	if got, want := string(content), "allowed-runtime"; got != want {
		t.Fatalf("allowed side effect content = %q, want %q", got, want)
	}

	meta := mustReadSessionMeta(t, harness, sessionID)
	if meta.Sandbox == nil {
		t.Fatal("meta.Sandbox = nil, want persisted sandbox metadata")
	}
	if got, want := meta.Sandbox.RuntimeRootDir, harness.WorkspaceRoot; got != want {
		t.Fatalf("meta.Sandbox.RuntimeRootDir = %q, want %q", got, want)
	}
	if got, want := meta.Sandbox.Profile, daemonSandboxProfileName; got != want {
		t.Fatalf("meta.Sandbox.Profile = %q, want %q", got, want)
	}
	if got, want := meta.Sandbox.State, "stopped"; got != want {
		t.Fatalf("meta.Sandbox.State = %q, want %q", got, want)
	}

	sandboxArtifact, err := harness.SessionSandboxArtifact(ctx, sessionID)
	if err != nil {
		t.Fatalf("SessionSandboxArtifact(%q) error = %v", sessionID, err)
	}
	if sandboxArtifact.Persisted == nil {
		t.Fatal("sandboxArtifact.Persisted = nil, want persisted metadata in artifact helper")
	}
	diagnostics = e2etest.ToolHostDiagnosticsArtifact{
		SessionID: sessionID,
		Operations: []e2etest.ToolHostOperationDiagnostic{{
			Operation:        "write_text_file",
			Path:             "toolhost/allowed.txt",
			Outcome:          e2etest.ToolHostOutcomeAllowed,
			SideEffectPath:   sideEffectPath,
			SideEffectExists: true,
		}},
	}
}

func TestDaemonE2ELocalSandboxBlockedOperationLeavesFailureDiagnostics(t *testing.T) {
	harness := startSandboxRuntimeHarness(t, daemonSandboxScenarioBlocked, aghconfig.PermissionModeApproveReads)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		sessionID      string
		diagnostics    e2etest.ToolHostDiagnosticsArtifact
		sideEffectPath = filepath.Join(harness.WorkspaceRoot, "toolhost", "blocked.txt")
	)
	registerSandboxRuntimeArtifacts(t, harness, &sessionID, &diagnostics)

	sessionInfo, stream := mustRunSandboxScenarioSession(
		t,
		ctx,
		harness,
		"blocked-local-runtime",
		"attempt a blocked runtime write",
	)
	sessionID = sessionInfo.ID
	if len(stream) == 0 {
		t.Fatal("prompt stream = empty, want runtime events")
	}
	if err := harness.StopSession(ctx, sessionID); err != nil {
		t.Fatalf("StopSession(%q) error = %v", sessionID, err)
	}

	waitForRuntimeCondition(t, "blocked sandbox session stopped", 10*time.Second, func() bool {
		current, err := harness.GetSession(ctx, sessionID)
		return err == nil && current.State == sessionpkg.StateStopped
	})

	sessionInfo, err := harness.GetSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetSession(%q) error = %v", sessionID, err)
	}
	if got, want := sessionInfo.State, sessionpkg.StateStopped; got != want {
		t.Fatalf("sessionInfo.State = %q, want %q", got, want)
	}
	if sessionInfo.Sandbox == nil {
		t.Fatal("sessionInfo.Sandbox = nil, want stopped sandbox metadata")
	}
	if got, want := sessionInfo.Sandbox.Backend, string(sandbox.BackendLocal); got != want {
		t.Fatalf("sessionInfo.Sandbox.Backend = %q, want %q", got, want)
	}
	if got, want := sessionInfo.Sandbox.Profile, daemonSandboxProfileName; got != want {
		t.Fatalf("sessionInfo.Sandbox.Profile = %q, want %q", got, want)
	}

	if _, err := os.Stat(sideEffectPath); !os.IsNotExist(err) {
		t.Fatalf("os.Stat(%q) error = %v, want not exist", sideEffectPath, err)
	}

	eventsResp := mustSessionEvents(t, ctx, harness, sessionID)
	events := decodeAgentEvents(t, eventsResp.Events)
	blockedMessage := findAgentMessageContaining(events, "blocked by approve-reads")
	if blockedMessage == "" {
		t.Fatalf("events = %#v, want blocked-operation failure signal", events)
	}

	meta := mustReadSessionMeta(t, harness, sessionID)
	if meta.Sandbox == nil {
		t.Fatal("meta.Sandbox = nil, want persisted metadata after blocked operation")
	}
	if got, want := meta.Sandbox.State, "stopped"; got != want {
		t.Fatalf("meta.Sandbox.State = %q, want %q", got, want)
	}

	sandboxArtifact, err := harness.SessionSandboxArtifact(ctx, sessionID)
	if err != nil {
		t.Fatalf("SessionSandboxArtifact(%q) error = %v", sessionID, err)
	}
	if sandboxArtifact.API == nil {
		t.Fatal("sandboxArtifact.API = nil, want public session sandbox projection")
	}

	diagnostics = e2etest.ToolHostDiagnosticsArtifact{
		SessionID: sessionID,
		Operations: []e2etest.ToolHostOperationDiagnostic{{
			Operation:        "write_text_file",
			Path:             "toolhost/blocked.txt",
			Outcome:          e2etest.ToolHostOutcomeBlocked,
			Error:            blockedMessage,
			SideEffectPath:   sideEffectPath,
			SideEffectExists: false,
		}},
	}
}

type daemonSandboxACPAgent struct {
	conn     *acpsdk.AgentSideConnection
	scenario string
}

func (a *daemonSandboxACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *daemonSandboxACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (a *daemonSandboxACPAgent) Initialize(
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

func (a *daemonSandboxACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (a *daemonSandboxACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (a *daemonSandboxACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{Sessions: []acpsdk.SessionInfo{}}, nil
}

func (a *daemonSandboxACPAgent) NewSession(
	context.Context,
	acpsdk.NewSessionRequest,
) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{SessionId: "daemon-sandbox-helper"}, nil
}

func (a *daemonSandboxACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (a *daemonSandboxACPAgent) SetSessionConfigOption(
	context.Context,
	acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{ConfigOptions: []acpsdk.SessionConfigOption{}}, nil
}

func (a *daemonSandboxACPAgent) LoadSession(
	context.Context,
	acpsdk.LoadSessionRequest,
) (acpsdk.LoadSessionResponse, error) {
	return acpsdk.LoadSessionResponse{}, nil
}

func (a *daemonSandboxACPAgent) Prompt(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	switch a.scenario {
	case daemonSandboxScenarioAllowed:
		return a.promptAllowed(ctx, params)
	case daemonSandboxScenarioBlocked:
		return a.promptBlocked(ctx, params)
	default:
		return acpsdk.PromptResponse{}, fmt.Errorf("unknown sandbox helper scenario %q", a.scenario)
	}
}

func (a *daemonSandboxACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (a *daemonSandboxACPAgent) promptAllowed(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	const relativePath = "toolhost/allowed.txt"

	if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
		SessionId: params.SessionId,
		Path:      relativePath,
		Content:   "allowed-runtime",
	}); err != nil {
		return acpsdk.PromptResponse{}, err
	}

	readResp, err := a.conn.ReadTextFile(ctx, acpsdk.ReadTextFileRequest{
		SessionId: params.SessionId,
		Path:      relativePath,
	})
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}

	return a.sendMessageAndEndTurn(ctx, params.SessionId, "allowed:"+readResp.Content)
}

func (a *daemonSandboxACPAgent) promptBlocked(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	const relativePath = "toolhost/blocked.txt"

	message := "blocked:write unexpectedly succeeded"
	if _, err := a.conn.WriteTextFile(ctx, acpsdk.WriteTextFileRequest{
		SessionId: params.SessionId,
		Path:      relativePath,
		Content:   "should-not-write",
	}); err != nil {
		message = "blocked:" + err.Error()
	}

	return a.sendMessageAndEndTurn(ctx, params.SessionId, message)
}

func (a *daemonSandboxACPAgent) sendMessageAndEndTurn(
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

func startSandboxRuntimeHarness(
	t testing.TB,
	scenario string,
	permissions aghconfig.PermissionMode,
) *e2etest.RuntimeHarness {
	t.Helper()
	helperCommand := daemonSandboxHelperCommand(t)

	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		Env: map[string]string{
			daemonSandboxHelperEnvKey:         "1",
			daemonSandboxHelperScenarioEnvKey: scenario,
		},
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent:   daemonSandboxFixtureAgentName,
			DefaultSandbox: daemonSandboxProfileName,
			Providers: map[string]aghconfig.ProviderConfig{
				acpmock.ProviderName: acpmock.ProviderConfig(helperCommand),
			},
			Sandboxes: map[string]aghconfig.SandboxProfile{
				daemonSandboxProfileName: {
					Backend:     string(sandbox.BackendLocal),
					Persistence: string(sandbox.PersistenceReuse),
				},
			},
			AgentDefs: []e2etest.AgentSeed{{
				Name:        daemonSandboxFixtureAgentName,
				Provider:    acpmock.ProviderName,
				Command:     helperCommand,
				Permissions: string(permissions),
				Prompt:      "You are a deterministic sandbox runtime helper.",
			}},
		},
		Workspace: e2etest.WorkspaceSeedOptions{
			Files: map[string]string{
				"README.md": "sandbox runtime workspace",
			},
		},
	})

}

func mustRunSandboxScenarioSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	name string,
	message string,
) (aghcontract.SessionPayload, []e2etest.SSEEvent) {
	t.Helper()

	created, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
		AgentName:     daemonSandboxFixtureAgentName,
		Name:          name,
		WorkspacePath: harness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession(%q) error = %v", name, err)
	}
	current, err := harness.WaitForSessionActive(ctx, created.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v", created.ID, err)
	}

	stream, err := harness.PromptSession(ctx, current.ID, message)
	if err != nil {
		t.Fatalf("PromptSession(%q) error = %v", current.ID, err)
	}
	return current, stream
}

func registerSandboxRuntimeArtifacts(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	sessionID *string,
	diagnostics *e2etest.ToolHostDiagnosticsArtifact,
) {
	t.Helper()

	t.Cleanup(func() {
		trimmedSessionID := strings.TrimSpace(derefStringValue(sessionID))
		if trimmedSessionID == "" {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := harness.CaptureSessionTranscript(ctx, trimmedSessionID); err != nil {
			t.Logf("CaptureSessionTranscript(%q) error = %v", trimmedSessionID, err)
		}
		if err := harness.CaptureSessionEvents(ctx, trimmedSessionID); err != nil {
			t.Logf("CaptureSessionEvents(%q) error = %v", trimmedSessionID, err)
		}
		if err := harness.CaptureSessionSandbox(ctx, trimmedSessionID); err != nil {
			t.Logf("CaptureSessionSandbox(%q) error = %v", trimmedSessionID, err)
		}
		if diagnostics != nil && len(diagnostics.Operations) > 0 {
			if err := harness.CaptureToolHostDiagnosticsJSON(*diagnostics); err != nil {
				t.Logf("CaptureToolHostDiagnosticsJSON(%q) error = %v", trimmedSessionID, err)
			}
		}
	})
}

func daemonSandboxHelperCommand(t testing.TB) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return shellquote.Join(bin, "-test.run=TestDaemonSandboxACPHelperProcess")
}

func mustSessionEvents(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) aghcontract.SessionEventsResponse {
	t.Helper()

	events, err := harness.SessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("SessionEvents(%q) error = %v", sessionID, err)
	}
	return events
}

func findAgentMessageContaining(events []aghcontract.AgentEventPayload, fragment string) string {
	want := strings.TrimSpace(fragment)
	for _, event := range events {
		if event.Type != "agent_message" {
			continue
		}
		if strings.Contains(event.Text, want) {
			return event.Text
		}
	}
	return ""
}
