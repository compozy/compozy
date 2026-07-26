//go:build integration && !windows

package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghcontract "github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil/acpmock"
	"github.com/kballard/go-shellquote"
)

const e2eACPHelperEnvKey = "AGH_TEST_E2E_ACP_HELPER"

type runtimeMigrationExpectation struct {
	stream       string
	version      int64
	appliedCount int
}

func runtimeMigrationExpectations() []runtimeMigrationExpectation {
	return []runtimeMigrationExpectation{
		{stream: "global", version: 25, appliedCount: 25},
		{stream: "memory", version: 1, appliedCount: 1},
	}
}

type e2eACPAgent struct {
	conn *acpsdk.AgentSideConnection
}

func TestE2EACPHelperProcess(t *testing.T) {
	// This test body is only executed by the subprocess-backed ACP helper command.
	if os.Getenv(e2eACPHelperEnvKey) != "1" {
		return
	}

	agent := &e2eACPAgent{}
	conn := acpsdk.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
	os.Exit(0)
}

func TestStartRuntimeHarnessBootsRealDaemonAndExposesClients(t *testing.T) {
	t.Parallel()

	harness := StartRuntimeHarness(t, &RuntimeHarnessOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var httpStatus aghcontract.StatusPayload
	if err := harness.HTTPJSON(ctx, "GET", "/api/status", nil, &httpStatus); err != nil {
		t.Fatalf("HTTP runtime status error = %v", err)
	}
	if got, want := httpStatus.Daemon.Status, "running"; got != want {
		t.Fatalf("httpStatus.Daemon.Status = %q, want %q", got, want)
	}
	assertSchemaStreamStatuses(t, httpStatus.Daemon.SchemaStreams)

	var udsStatus aghcontract.StatusPayload
	if err := harness.UDSJSON(ctx, "GET", "/api/status", nil, &udsStatus); err != nil {
		t.Fatalf("UDS runtime status error = %v", err)
	}
	if got, want := udsStatus.Daemon.HTTPPort, harness.Config.HTTP.Port; got != want {
		t.Fatalf("udsStatus.Daemon.HTTPPort = %d, want %d", got, want)
	}
	if !slices.Equal(udsStatus.Daemon.SchemaStreams, httpStatus.Daemon.SchemaStreams) {
		t.Fatalf(
			"uds schema streams = %#v, want HTTP schema streams %#v",
			udsStatus.Daemon.SchemaStreams,
			httpStatus.Daemon.SchemaStreams,
		)
	}

	var cliStatus aghcontract.StatusPayload
	if err := harness.CLI.RunJSON(ctx, &cliStatus, "status", "-o", "json"); err != nil {
		t.Fatalf("CLI runtime status error = %v", err)
	}
	if got, want := cliStatus.Daemon.Socket, harness.Config.Daemon.Socket; got != want {
		t.Fatalf("cliStatus.Daemon.Socket = %q, want %q", got, want)
	}
	if !slices.Equal(cliStatus.Daemon.SchemaStreams, httpStatus.Daemon.SchemaStreams) {
		t.Fatalf(
			"CLI schema streams = %#v, want HTTP schema streams %#v",
			cliStatus.Daemon.SchemaStreams,
			httpStatus.Daemon.SchemaStreams,
		)
	}

	if err := harness.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	processLog, err := harness.readProcessLog()
	if err != nil {
		t.Fatalf("readProcessLog() error = %v", err)
	}
	assertMigrationAppliedLogs(t, processLog)
	if _, err := os.Stat(harness.HomePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("daemon info stat error = %v, want os.ErrNotExist", err)
	}
	if _, err := os.Stat(harness.Config.Daemon.Socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("daemon socket stat error = %v, want os.ErrNotExist", err)
	}
}

func assertSchemaStreamStatuses(t *testing.T, statuses []aghcontract.SchemaStreamStatus) {
	t.Helper()
	expectations := runtimeMigrationExpectations()
	if len(statuses) != len(expectations) {
		t.Fatalf("schema streams = %#v, want global and memory", statuses)
	}
	for index, expectation := range expectations {
		status := statuses[index]
		if status.Stream != expectation.stream || status.Version != expectation.version ||
			status.AppliedCount != expectation.appliedCount ||
			strings.TrimSpace(status.SumDigest) == "" {
			t.Fatalf(
				"schema stream[%d] = %#v, want stream=%q version=%d applied_count=%d and digest",
				index,
				status,
				expectation.stream,
				expectation.version,
				expectation.appliedCount,
			)
		}
	}
}

func assertMigrationAppliedLogs(t *testing.T, processLog string) {
	t.Helper()
	type migrationLog struct {
		Message      string `json:"msg"`
		Stream       string `json:"stream"`
		Version      int64  `json:"version"`
		AppliedCount int    `json:"applied_count"`
	}
	found := make(map[string]migrationLog, 2)
	for _, line := range strings.Split(processLog, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var record migrationLog
		if err := json.Unmarshal([]byte(line), &record); err == nil {
			if record.Message == "store.migrations.applied" {
				found[record.Stream] = record
			}
			continue
		}
		for _, expectation := range runtimeMigrationExpectations() {
			needle := fmt.Sprintf(
				"store.migrations.applied stream=%s version=%d applied_count=%d",
				expectation.stream,
				expectation.version,
				expectation.appliedCount,
			)
			if strings.Contains(line, needle) {
				found[expectation.stream] = migrationLog{
					Message:      "store.migrations.applied",
					Stream:       expectation.stream,
					Version:      expectation.version,
					AppliedCount: expectation.appliedCount,
				}
			}
		}
	}
	for _, expectation := range runtimeMigrationExpectations() {
		record, ok := found[expectation.stream]
		if !ok || record.Version != expectation.version || record.AppliedCount != expectation.appliedCount {
			t.Fatalf(
				"migration applied logs = %#v, want %q version=%d applied_count=%d; process log=%s",
				found,
				expectation.stream,
				expectation.version,
				expectation.appliedCount,
				processLog,
			)
		}
	}
}

func TestStartRuntimeHarnessRefusesLegacyDatabaseBeforeReadiness(t *testing.T) {
	t.Run("Should exit non-zero before readiness and leave the legacy database untouched", func(t *testing.T) {
		t.Parallel()

		layout := prepareRuntimeLayout(t, &RuntimeHarnessOptions{})
		seedLegacyRuntimeDatabase(t, layout.HomePaths.DatabaseFile)
		before, err := os.ReadFile(layout.HomePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("ReadFile(legacy before boot) error = %v", err)
		}
		canonicalPath, err := filepath.EvalSymlinks(layout.HomePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("EvalSymlinks(legacy database) error = %v", err)
		}

		binaryPath := buildAGHBinary(t)
		env, err := withRuntimeCLIEnv(layout.HomePaths, layout.Env, binaryPath)
		if err != nil {
			t.Fatalf("withRuntimeCLIEnv() error = %v", err)
		}
		harness := newRuntimeHarness(t, &layout, binaryPath)
		startDaemonProcess(t, harness, env)

		waitCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		exitErr := harness.waitForExit(waitCtx)
		if errors.Is(exitErr, context.DeadlineExceeded) {
			if killErr := harness.forceKillDaemonProcess(); killErr != nil {
				t.Fatalf("legacy daemon did not exit before readiness; force kill error = %v", killErr)
			}
			t.Fatal("legacy daemon did not exit before readiness")
		}
		if exitErr == nil {
			t.Fatal("legacy daemon exit error = nil, want non-zero exit")
		}
		if harness.process == nil || harness.process.ProcessState == nil ||
			harness.process.ProcessState.ExitCode() == 0 {
			t.Fatalf("legacy daemon process state = %#v, want non-zero exit", harness.process)
		}

		logBytes, err := os.ReadFile(harness.processLogPath)
		if err != nil {
			t.Fatalf("ReadFile(daemon process log) error = %v", err)
		}
		logText := string(logBytes)
		if !strings.Contains(logText, store.ErrLegacyDatabase.Error()) ||
			!strings.Contains(logText, canonicalPath) ||
			!strings.Contains(logText, "complete AGH_HOME or workspace .agh directory") {
			t.Fatalf("daemon process log = %q, want legacy sentinel, path, and whole-family remediation", logText)
		}
		if _, err := os.Stat(layout.HomePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("daemon info stat error = %v, want no readiness artifact", err)
		}
		after, err := os.ReadFile(layout.HomePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("ReadFile(legacy after boot) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatal("legacy database bytes changed after refused daemon boot")
		}
	})
}

func seedLegacyRuntimeDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open(legacy runtime database) error = %v", err)
	}
	if _, err := db.ExecContext(t.Context(), `CREATE TABLE schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy schema_migrations error = %v", err)
	}
	if _, err := db.ExecContext(t.Context(), `WITH RECURSIVE versions(version) AS (
		SELECT 1 UNION ALL SELECT version + 1 FROM versions WHERE version < 61
	) INSERT INTO schema_migrations (version, name, checksum, applied_at)
	SELECT version, printf('legacy_%02d', version), printf('checksum-%02d', version), '2026-07-10T00:00:00Z'
	FROM versions`); err != nil {
		t.Fatalf("seed legacy schema_migrations error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close(legacy runtime database) error = %v", err)
	}
}

func TestStartRuntimeHarnessRetriesHTTPPortConflicts(t *testing.T) {
	t.Parallel()

	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer func() {
		_ = blocker.Close()
	}()

	conflictPort := blocker.Addr().(*net.TCPAddr).Port
	harness := StartRuntimeHarness(t, &RuntimeHarnessOptions{
		ConfigSeed: ConfigSeedOptions{
			HTTPPort: conflictPort,
		},
	})

	if got := harness.Config.HTTP.Port; got == conflictPort {
		t.Fatalf("harness.Config.HTTP.Port = %d, want retry onto a new port", got)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var status aghcontract.StatusPayload
	if err := harness.HTTPJSON(ctx, "GET", "/api/status", nil, &status); err != nil {
		t.Fatalf("HTTP runtime status after retry error = %v", err)
	}
	if got, want := status.Daemon.HTTPPort, harness.Config.HTTP.Port; got != want {
		t.Fatalf("status.Daemon.HTTPPort = %d, want %d", got, want)
	}
}

func TestStartRuntimeHarnessResolvesSeededWorkspaceThroughPublicSurface(t *testing.T) {
	t.Parallel()

	harness := StartRuntimeHarness(t, &RuntimeHarnessOptions{
		Workspace: WorkspaceSeedOptions{
			Files: map[string]string{
				"README.md": "shared harness workspace",
			},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if harness.WorkspaceID == "" {
		t.Fatal("harness.WorkspaceID = empty, want resolved workspace id")
	}

	workspace, err := harness.GetWorkspace(ctx, harness.WorkspaceID)
	if err != nil {
		t.Fatalf("GetWorkspace(%q) error = %v", harness.WorkspaceID, err)
	}
	if got, want := workspace.RootDir, harness.WorkspaceRoot; got != want {
		t.Fatalf("workspace.RootDir = %q, want %q", got, want)
	}
	if got, want := workspace.ID, harness.WorkspaceID; got != want {
		t.Fatalf("workspace.ID = %q, want %q", got, want)
	}
}

func TestStartRuntimeHarnessCapturesTranscriptAndEventsArtifacts(t *testing.T) {
	t.Parallel()

	helperCommand := e2eACPHelperCommand(t)
	harness := StartRuntimeHarness(t, &RuntimeHarnessOptions{
		Env: map[string]string{
			e2eACPHelperEnvKey: "1",
		},
		ConfigSeed: ConfigSeedOptions{
			Providers: map[string]aghconfig.ProviderConfig{
				acpmock.ProviderName: acpmock.ProviderConfig(helperCommand),
			},
			AgentDefs: []AgentSeed{{
				Name:        "coder",
				Provider:    acpmock.ProviderName,
				Command:     helperCommand,
				Permissions: string(aghconfig.PermissionModeApproveReads),
				Prompt:      "You are a deterministic E2E helper.",
			}},
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	created, err := harness.CreateSession(ctx, aghcontract.CreateSessionRequest{
		AgentName:     "coder",
		Name:          "artifact-demo",
		WorkspacePath: harness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	created, err = harness.WaitForSessionActive(ctx, created.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v", created.ID, err)
	}

	stream, err := harness.PromptSession(ctx, created.ID, "hello harness")
	if err != nil {
		t.Fatalf("PromptSession() error = %v", err)
	}
	if len(stream) == 0 {
		t.Fatal("prompt stream = empty, want agent_message/done events")
	}
	if stream[0].Event == "" {
		t.Fatalf("stream[0].Event = empty, want populated SSE event")
	}

	if err := harness.CaptureSessionTranscript(ctx, created.ID); err != nil {
		t.Fatalf("CaptureSessionTranscript() error = %v", err)
	}
	if err := harness.CaptureSessionEvents(ctx, created.ID); err != nil {
		t.Fatalf("CaptureSessionEvents() error = %v", err)
	}

	manifest := harness.Artifacts.Manifest()
	if got, want := len(manifest.Artifacts), 2; got != want {
		t.Fatalf("len(manifest.Artifacts) = %d, want %d", got, want)
	}

	transcriptPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindTranscript)
	if !ok {
		t.Fatal("ArtifactPath(transcript) = missing, want present")
	}
	transcriptBytes, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", transcriptPath, err)
	}
	transcript := string(transcriptBytes)
	if !strings.Contains(transcript, "echo:") || !strings.Contains(transcript, "hello harness") {
		t.Fatalf("transcript artifact = %s, want echoed assistant content", string(transcriptBytes))
	}

	eventsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindEvents)
	if !ok {
		t.Fatal("ArtifactPath(events) = missing, want present")
	}
	eventsBytes, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", eventsPath, err)
	}
	if !strings.Contains(string(eventsBytes), "agent_message") {
		t.Fatalf("events artifact = %s, want agent_message event", string(eventsBytes))
	}
}

func TestStartRuntimeHarnessRepeatedCyclesLeaveNoStaleDaemonArtifacts(t *testing.T) {
	for cycle := 0; cycle < 3; cycle++ {
		harness := StartRuntimeHarness(t, &RuntimeHarnessOptions{})
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		var httpStatus aghcontract.StatusPayload
		if err := harness.HTTPJSON(ctx, "GET", "/api/status", nil, &httpStatus); err != nil {
			cancel()
			t.Fatalf("cycle %d HTTP runtime status error = %v", cycle, err)
		}

		if err := harness.Stop(ctx); err != nil {
			cancel()
			t.Fatalf("cycle %d Stop() error = %v", cycle, err)
		}
		cancel()

		if _, err := os.Stat(harness.HomePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cycle %d daemon info stat error = %v, want os.ErrNotExist", cycle, err)
		}
		if _, err := os.Stat(harness.Config.Daemon.Socket); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("cycle %d socket stat error = %v, want os.ErrNotExist", cycle, err)
		}
	}
}

func TestStartRuntimeHarnessCLIStatusCanBeCapturedInRuntimeManifest(t *testing.T) {
	harness := StartRuntimeHarness(t, &RuntimeHarnessOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdout, stderr, err := harness.CLI.Run(ctx, "status", "-o", "json")
	if err != nil {
		t.Fatalf("CLI.Run(runtime status) error = %v; stderr=%s", err, strings.TrimSpace(stderr))
	}

	var cliStatus aghcontract.StatusPayload
	if err := json.Unmarshal([]byte(stdout), &cliStatus); err != nil {
		t.Fatalf("json.Unmarshal(cli status) error = %v; stdout=%s", err, strings.TrimSpace(stdout))
	}
	if got, want := cliStatus.Daemon.Socket, harness.Config.Daemon.Socket; got != want {
		t.Fatalf("cliStatus.Daemon.Socket = %q, want %q", got, want)
	}
	if got, want := cliStatus.Daemon.HTTPPort, harness.Config.HTTP.Port; got != want {
		t.Fatalf("cliStatus.Daemon.HTTPPort = %d, want %d", got, want)
	}

	outputPath, err := harness.CaptureCLIOutput(
		"runtime status",
		[]string{"status", "-o", "json"},
		stdout,
		stderr,
		nil,
	)
	if err != nil {
		t.Fatalf("CaptureCLIOutput() error = %v", err)
	}
	outputBytes, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", outputPath, err)
	}
	if !strings.Contains(string(outputBytes), `"transport": "cli"`) {
		t.Fatalf("CLI output artifact = %s, want CLI transport record", string(outputBytes))
	}

	manifestBytes, err := os.ReadFile(harness.RuntimeManifestPath())
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", harness.RuntimeManifestPath(), err)
	}
	var manifest RuntimeArtifactManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(runtime manifest) error = %v", err)
	}
	if got, want := manifest.Transport.SocketPath, harness.Config.Daemon.Socket; got != want {
		t.Fatalf("manifest.Transport.SocketPath = %q, want %q", got, want)
	}
	if !runtimeManifestHasArtifact(manifest.CapturedArtifacts, ArtifactKindTransportOutputs, "transport_outputs") {
		t.Fatalf("manifest.CapturedArtifacts = %#v, want transport_outputs entry", manifest.CapturedArtifacts.Artifacts)
	}
}

func (a *e2eACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (a *e2eACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (a *e2eACPAgent) Initialize(
	context.Context,
	acpsdk.InitializeRequest,
) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AuthMethods:     []acpsdk.AuthMethod{},
	}, nil
}

func (a *e2eACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (a *e2eACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (a *e2eACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{Sessions: []acpsdk.SessionInfo{}}, nil
}

func (a *e2eACPAgent) NewSession(
	context.Context,
	acpsdk.NewSessionRequest,
) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{SessionId: "e2e-helper-session"}, nil
}

func (a *e2eACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (a *e2eACPAgent) SetSessionConfigOption(
	context.Context,
	acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{ConfigOptions: []acpsdk.SessionConfigOption{}}, nil
}

func (a *e2eACPAgent) LoadSession(
	context.Context,
	acpsdk.LoadSessionRequest,
) (acpsdk.LoadSessionResponse, error) {
	return acpsdk.LoadSessionResponse{}, nil
}

func (a *e2eACPAgent) Prompt(
	ctx context.Context,
	params acpsdk.PromptRequest,
) (acpsdk.PromptResponse, error) {
	if a.conn != nil {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update:    acpsdk.UpdateAgentMessageText("echo: " + promptText(params.Prompt)),
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
	}
	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func (a *e2eACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func e2eACPHelperCommand(t testing.TB) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return shellquote.Join(bin, "-test.run=TestE2EACPHelperProcess")
}

func promptText(blocks []acpsdk.ContentBlock) string {
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
	return lastText
}

func runtimeManifestHasArtifact(manifest ArtifactManifest, kind ArtifactKind, path string) bool {
	for _, artifact := range manifest.Artifacts {
		if artifact.Kind == kind && artifact.Path == path {
			return true
		}
	}
	return false
}
