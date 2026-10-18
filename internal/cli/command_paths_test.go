package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/spf13/cobra"
)

type stubRunner struct {
	ran   bool
	runFn func(context.Context) error
}

func (s *stubRunner) Run(ctx context.Context) error {
	s.ran = true
	if s.runFn != nil {
		return s.runFn(ctx)
	}
	return nil
}

func TestCommandPathsAndHelpers(t *testing.T) {
	t.Parallel()

	statusSession := SessionRecord{
		ID:            "sess-1",
		AgentName:     "coder",
		WorkspaceID:   "ws-1",
		WorkspacePath: "/workspace/project",
		State:         "active",
		CreatedAt:     fixedTestNow,
		UpdatedAt:     fixedTestNow,
	}
	statusSessionHealth := SessionHealthRecord{
		SessionID:       "sess-1",
		AgentName:       "coder",
		WorkspaceID:     "ws-1",
		State:           "idle",
		Health:          "healthy",
		Attachable:      true,
		EligibleForWake: true,
		UpdatedAt:       fixedTestNow,
	}
	tempDir := t.TempDir()
	soulBodyPath := filepath.Join(tempDir, "SOUL.md")
	if err := os.WriteFile(soulBodyPath, []byte("# Soul\n\nStay precise.\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
	}
	heartbeatBodyPath := filepath.Join(tempDir, "HEARTBEAT.md")
	if err := os.WriteFile(heartbeatBodyPath, []byte("# Heartbeat\n\nCheck in.\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(HEARTBEAT.md) error = %v", err)
	}
	attachmentPath := filepath.Join(tempDir, "frame.png")
	if err := os.WriteFile(attachmentPath, []byte("attachment"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(frame.png) error = %v", err)
	}

	getCalls := 0
	getAgentSoulCalled := false
	putAgentSoulCalled := false
	deleteAgentSoulCalled := false
	rollbackAgentSoulCalled := false
	refreshSessionSoulCalled := false
	getAgentHeartbeatCalled := false
	putAgentHeartbeatCalled := false
	deleteAgentHeartbeatCalled := false
	rollbackAgentHeartbeatCalled := false
	getAgentHeartbeatStatusCalled := false
	waitSessionCalled := false
	client := &stubClient{
		statusFn: func(context.Context) (StatusRecord, error) {
			return StatusRecord{
				SchemaVersion: "2026-05-20",
				GeneratedAt:   fixedTestNow,
				Daemon: DaemonStatus{
					Status:    "running",
					PID:       10,
					StartedAt: fixedTestNow.Add(-time.Minute),
					Socket:    "/tmp/compozy.sock",
					HTTPHost:  "localhost",
					HTTPPort:  2123,
				},
				Health: contract.ObserveHealthPayload{Status: "ok"},
				Sessions: contract.SessionAggregatePayload{
					Active:   1,
					Total:    1,
					ByStatus: map[string]int{"active": 1},
				},
				Config:  contract.ConfigRuntimeStatusPayload{Status: "current"},
				LogTail: contract.LogTailStatusPayload{Status: "available"},
			}, nil
		},
		doctorFn: func(context.Context, DoctorQuery) (DoctorRecord, error) {
			return DoctorRecord{
				SchemaVersion: "2026-05-20",
				GeneratedAt:   fixedTestNow,
				Status:        "ok",
				Summary:       contract.DoctorSummaryPayload{Total: 0, CountsBySeverity: map[string]int{}},
				Items:         []contract.DiagnosticItem{},
			}, nil
		},
		getAgentFn: func(context.Context, string, AgentQuery) (AgentRecord, error) {
			return AgentRecord{Name: "coder", Provider: "fake", Prompt: "hi"}, nil
		},
		getAgentSoulFn: func(context.Context, string, AgentQuery) (AgentSoulRecord, error) {
			getAgentSoulCalled = true
			return AgentSoulRecord{AgentName: "coder", Enabled: true, Valid: true, ValidationStatus: "valid"}, nil
		},
		putAgentSoulFn: func(_ context.Context, _ string, request AgentSoulPutRequest) (AgentSoulMutationRecord, error) {
			putAgentSoulCalled = true
			return AgentSoulMutationRecord{
				Soul: AgentSoulRecord{
					AgentName:        "coder",
					Valid:            true,
					ValidationStatus: "valid",
					Digest:           request.ExpectedDigest,
				},
			}, nil
		},
		deleteAgentSoulFn: func(_ context.Context, _ string, request AgentSoulDeleteRequest) (AgentSoulMutationRecord, error) {
			deleteAgentSoulCalled = true
			return AgentSoulMutationRecord{
				Soul: AgentSoulRecord{
					AgentName:        "coder",
					Valid:            true,
					ValidationStatus: "valid",
					Digest:           request.ExpectedDigest,
				},
			}, nil
		},
		rollbackAgentSoulFn: func(
			_ context.Context,
			_ string,
			request AgentSoulRollbackRequest,
		) (AgentSoulMutationRecord, error) {
			rollbackAgentSoulCalled = true
			return AgentSoulMutationRecord{
				Soul: AgentSoulRecord{
					AgentName:        "coder",
					Valid:            true,
					ValidationStatus: "valid",
					Digest:           request.ExpectedDigest,
				},
			}, nil
		},
		refreshSessionSoulFn: func(context.Context, string, SessionSoulRefreshRequest) (AgentSoulRecord, error) {
			refreshSessionSoulCalled = true
			return AgentSoulRecord{AgentName: "coder", Enabled: true, Valid: true, ValidationStatus: "valid"}, nil
		},
		getAgentHeartbeatFn: func(context.Context, string, AgentQuery) (AgentHeartbeatRecord, error) {
			getAgentHeartbeatCalled = true
			return AgentHeartbeatRecord{AgentName: "coder", Enabled: true, Valid: true, ValidationStatus: "valid"}, nil
		},
		putAgentHeartbeatFn: func(
			_ context.Context,
			_ string,
			request AgentHeartbeatPutRequest,
		) (AgentHeartbeatMutationRecord, error) {
			putAgentHeartbeatCalled = true
			return AgentHeartbeatMutationRecord{
				Heartbeat: AgentHeartbeatRecord{
					AgentName: "coder", Valid: true, ValidationStatus: "valid", Digest: request.ExpectedDigest,
				},
			}, nil
		},
		deleteAgentHeartbeatFn: func(
			_ context.Context,
			_ string,
			request AgentHeartbeatDeleteRequest,
		) (AgentHeartbeatMutationRecord, error) {
			deleteAgentHeartbeatCalled = true
			return AgentHeartbeatMutationRecord{
				Heartbeat: AgentHeartbeatRecord{
					AgentName: "coder", Valid: true, ValidationStatus: "valid", Digest: request.ExpectedDigest,
				},
			}, nil
		},
		rollbackAgentHeartbeatFn: func(
			_ context.Context,
			_ string,
			request AgentHeartbeatRollbackRequest,
		) (AgentHeartbeatMutationRecord, error) {
			rollbackAgentHeartbeatCalled = true
			return AgentHeartbeatMutationRecord{
				Heartbeat: AgentHeartbeatRecord{
					AgentName: "coder", Valid: true, ValidationStatus: "valid", Digest: request.ExpectedDigest,
				},
			}, nil
		},
		getAgentHeartbeatStatusFn: func(
			context.Context,
			string,
			AgentHeartbeatStatusRequest,
		) (AgentHeartbeatStatusRecord, error) {
			getAgentHeartbeatStatusCalled = true
			return AgentHeartbeatStatusRecord{
				AgentName:        "coder",
				Enabled:          true,
				Valid:            true,
				ValidationStatus: "valid",
			}, nil
		},
		listLogsFn: func(_ context.Context, query LogsListQuery) ([]LogEventRecord, error) {
			if query.WorkspaceRef != "ws-1" {
				t.Fatalf("ListLogs() workspaceRef = %q, want ws-1", query.WorkspaceRef)
			}
			return []LogEventRecord{
				{ID: "sum-1", SessionID: "sess-1", Type: "done", AgentName: "coder", Timestamp: fixedTestNow},
			}, nil
		},
		streamLogsFn: func(_ context.Context, query LogsListQuery, _ string, handler SSEHandler) error {
			if query.WorkspaceRef != "ws-1" {
				t.Fatalf("StreamLogs() workspaceRef = %q, want ws-1", query.WorkspaceRef)
			}
			return handler(
				SSEEvent{
					Event: "done",
					Data: mustJSON(
						t,
						LogEventRecord{
							ID:        "sum-1",
							SessionID: "sess-1",
							Type:      "done",
							AgentName: "coder",
							Timestamp: fixedTestNow,
						},
					),
				},
			)
		},
		getSessionFn: func(context.Context, string) (SessionRecord, error) {
			getCalls++
			if getCalls == 1 {
				return statusSession, nil
			}
			stopped := statusSession
			stopped.State = "stopped"
			return stopped, nil
		},
		getSessionHealthFn: func(context.Context, string) (SessionHealthRecord, error) {
			return statusSessionHealth, nil
		},
		getSessionStatusFn: func(context.Context, string) (SessionStatusRecord, error) {
			return SessionStatusRecord{
				SessionID:       "sess-1",
				AgentName:       "coder",
				WorkspaceID:     "ws-1",
				State:           "idle",
				Health:          "healthy",
				Attachable:      true,
				EligibleForWake: true,
				UpdatedAt:       fixedTestNow,
			}, nil
		},
		getSessionUsageFn: func(context.Context, string) (SessionUsageRecord, error) {
			return SessionUsageRecord{CostStatus: "included", CostSource: "none"}, nil
		},
		waitSessionFn: func(
			_ context.Context,
			workspaceID string,
			sessionID string,
			request SessionWaitRequest,
		) (SessionWaitRecord, error) {
			waitSessionCalled = true
			if workspaceID != "ws-1" || sessionID != "sess-1" {
				t.Fatalf("WaitSession() scope = %q/%q, want ws-1/sess-1", workspaceID, sessionID)
			}
			if request.TimeoutMS != 300_000 {
				t.Fatalf("WaitSession() timeout = %d, want 300000", request.TimeoutMS)
			}
			return SessionWaitRecord{SessionID: sessionID, Outcome: "state-reached", State: "idle"}, nil
		},
		inspectSessionFn: func(context.Context, string, SessionInspectQuery) (SessionInspectRecord, error) {
			return SessionInspectRecord{SessionID: "sess-1", Health: statusSessionHealth}, nil
		},
		resumeSessionFn: func(context.Context, string) (SessionRecord, error) {
			return statusSession, nil
		},
		uploadSessionAttachmentFn: func(_ context.Context, id string, path string) (SessionAttachmentRecord, error) {
			if id != "sess-1" || path != attachmentPath {
				t.Fatalf("UploadSessionAttachment() = (%q, %q), want (%q, %q)", id, path, "sess-1", attachmentPath)
			}
			return SessionAttachmentRecord{ID: "att-1", Name: "frame.png", CreatedAt: fixedTestNow}, nil
		},
		streamSessionFn: func(_ context.Context, _ string, _ SessionEventQuery, _ string, handler SSEHandler) error {
			return handler(SSEEvent{Event: "session_stopped"})
		},
		sessionHistoryFn: func(context.Context, string, SessionEventQuery) ([]TurnHistoryRecord, error) {
			return []TurnHistoryRecord{{TurnID: "turn-1"}}, nil
		},
		daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{Status: "running", PID: 10, StartedAt: fixedTestNow}, nil
		},
	}
	deps := newWorkspaceTestDeps(t, client)
	runner := &stubRunner{}
	deps.newDaemon = func() (daemonRunner, error) { return runner, nil }

	tests := [][]string{
		{"agent", "info", "coder", "-o", "json"},
		{"agent", "soul", "inspect", "coder", "-o", "json"},
		{
			"agent",
			"soul",
			"write",
			"coder",
			"--file",
			soulBodyPath,
			"--expected-digest",
			"sha256:soul-old",
			"-o",
			"json",
		},
		{"agent", "soul", "delete", "coder", "--expected-digest", "sha256:soul-old", "-o", "json"},
		{
			"agent",
			"soul",
			"rollback",
			"coder",
			"--revision-id",
			"rev-soul-1",
			"--expected-digest",
			"sha256:soul-old",
			"-o",
			"json",
		},
		{"agent", "heartbeat", "inspect", "coder", "-o", "json"},
		{
			"agent",
			"heartbeat",
			"write",
			"coder",
			"--file",
			heartbeatBodyPath,
			"--expected-digest",
			"sha256:hb-old",
			"-o",
			"json",
		},
		{"agent", "heartbeat", "delete", "coder", "--expected-digest", "sha256:hb-old", "-o", "json"},
		{
			"agent",
			"heartbeat",
			"rollback",
			"coder",
			"--revision-id",
			"rev-hb-1",
			"--expected-digest",
			"sha256:hb-old",
			"-o",
			"json",
		},
		{"agent", "heartbeat", "status", "coder", "-o", "json"},
		{"logs", "--workspace", "ws-1", "-o", "json"},
		{"logs", "--workspace", "ws-1", "--follow", "-o", "json"},
		{"status", "-o", "json"},
		{"doctor", "-o", "json"},
		{"session", "soul", "refresh", "sess-1", "--expected-digest", "sha256:old", "-o", "json"},
		{"session", "health", "sess-1", "-o", "json"},
		{"session", "status", "sess-1", "-o", "json"},
		{"session", "usage", "sess-1", "-o", "json"},
		{"session", "inspect", "sess-1", "-o", "json"},
		{"session", "resume", "sess-1", "-o", "json"},
		{"session", "attachments", "upload", "sess-1", attachmentPath, "-o", "json"},
		{"session", "wait", "sess-1", "-o", "json"},
		{"session", "history", "sess-1", "-o", "json"},
	}

	for _, args := range tests {
		if _, _, err := executeRootCommand(t, deps, args...); err != nil {
			t.Fatalf("executeRootCommand(%v) error = %v", args, err)
		}
	}

	if _, _, err := executeRootCommand(t, deps, "daemon", "start", "--foreground"); err != nil {
		t.Fatalf("daemon start --foreground error = %v", err)
	}
	if !getAgentSoulCalled || !putAgentSoulCalled || !deleteAgentSoulCalled || !rollbackAgentSoulCalled {
		t.Fatalf(
			"soul command routing flags = inspect:%v write:%v delete:%v rollback:%v, want all true",
			getAgentSoulCalled,
			putAgentSoulCalled,
			deleteAgentSoulCalled,
			rollbackAgentSoulCalled,
		)
	}
	if !getAgentHeartbeatCalled || !putAgentHeartbeatCalled || !deleteAgentHeartbeatCalled ||
		!rollbackAgentHeartbeatCalled || !getAgentHeartbeatStatusCalled {
		t.Fatalf(
			"heartbeat command routing flags = inspect:%v write:%v delete:%v rollback:%v status:%v, want all true",
			getAgentHeartbeatCalled,
			putAgentHeartbeatCalled,
			deleteAgentHeartbeatCalled,
			rollbackAgentHeartbeatCalled,
			getAgentHeartbeatStatusCalled,
		)
	}
	if !waitSessionCalled {
		t.Fatal("WaitSession() was not called")
	}
	if !refreshSessionSoulCalled {
		t.Fatal("RefreshSessionSoul() was not called")
	}
	if !runner.ran {
		t.Fatal("daemon runner did not execute")
	}

	if wd, err := currentWorkingDirectory(deps); err != nil || wd != "/workspace/project" {
		t.Fatalf("currentWorkingDirectory() = %q, %v", wd, err)
	}

	if err := procutil.Signal(os.Getpid(), syscall.Signal(0)); err != nil {
		t.Fatalf("procutil.Signal(os.Getpid(), 0) error = %v", err)
	}
	if !procutil.Alive(os.Getpid()) {
		t.Fatal("procutil.Alive(os.Getpid()) = false, want true")
	}
}

func TestLogsFollowIncludesWorkspaceResolution(t *testing.T) {
	t.Parallel()

	for _, format := range []string{string(OutputJSON), string(OutputToon)} {
		t.Run("Should annotate followed logs in "+format, func(t *testing.T) {
			t.Parallel()

			deps := newWorkspaceTestDeps(t, &stubClient{
				getWorkspaceFn: func(context.Context, string) (WorkspaceDetailRecord, error) {
					return WorkspaceDetailRecord{
						Workspace: WorkspaceRecord{ID: "ws-project", RootDir: "/workspace/project"},
					}, nil
				},
				streamLogsFn: func(_ context.Context, _ LogsListQuery, _ string, handler SSEHandler) error {
					return handler(SSEEvent{
						Event: "done",
						Data:  mustJSON(t, LogEventRecord{ID: "log-1", Type: "done"}),
					})
				},
			})
			deps.getwd = func() (string, error) { return "/workspace/project/nested", nil }
			stdout, _, err := executeRootCommand(
				t,
				deps,
				"logs",
				"--follow",
				"-o",
				format,
			)
			if err != nil {
				t.Fatalf("executeRootCommand(logs --follow -o %s) error = %v", format, err)
			}
			if !strings.Contains(stdout, "resolution_source") ||
				!strings.Contains(stdout, workspaceResolutionCWD) {
				t.Fatalf("logs --follow %s output = %q, want cwd provenance", format, stdout)
			}
		})
	}
}

func TestExecuteContextVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	code := ExecuteContext(t.Context(), []string{"version", "-o", "json"}, &stdout, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("ExecuteContext(version) code = %d, want 0", code)
	}

	var payload map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(version) error = %v", err)
	}
	if _, ok := payload["Version"]; !ok {
		t.Fatalf("version payload = %#v, want Version field", payload)
	}
}

func TestDaemonStatusFallbackStartingAndStopped(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{
		daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{}, os.ErrNotExist
		},
	})
	info := compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) { return info, nil }
	deps.processAlive = func(pid int) bool { return pid == 42 }

	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		t.Fatalf("loadRuntimeContext() error = %v", err)
	}

	status, err := daemonStatusFromDeps(t.Context(), deps, runtime)
	if err != nil {
		t.Fatalf("daemonStatusFromDeps(starting) error = %v", err)
	}
	if status.Status != "starting" {
		t.Fatalf("starting status = %q, want %q", status.Status, "starting")
	}

	deps.processAlive = func(int) bool { return false }
	status, err = daemonStatusFromDeps(t.Context(), deps, runtime)
	if err != nil {
		t.Fatalf("daemonStatusFromDeps(stopped) error = %v", err)
	}
	if status.Status != "stopped" {
		t.Fatalf("stopped status = %q, want %q", status.Status, "stopped")
	}
}

func TestWriteCommandOutputErrors(t *testing.T) {
	t.Parallel()

	if _, _, err := executeRootCommand(
		t,
		newWorkspaceTestDeps(t, &stubClient{}),
		"version",
		"-o",
		"bogus",
	); err == nil ||
		!strings.Contains(err.Error(), "invalid output format") {
		t.Fatalf("invalid output error = %v, want invalid output format", err)
	}
}

func TestDaemonStartRejectsNilDetachedProcess(t *testing.T) {
	t.Parallel()

	deps := newWorkspaceTestDeps(t, &stubClient{})
	deps.spawnDetached = func(context.Context, compozyconfig.HomePaths) (daemonProcess, error) {
		return nil, nil
	}

	if _, _, err := executeRootCommand(
		t,
		deps,
		"daemon",
		"start",
		"-o",
		"json",
	); err == nil ||
		!strings.Contains(err.Error(), "detached daemon process is required") {
		t.Fatalf("daemon start nil detached process error = %v, want detached daemon process is required", err)
	}
}

func TestTerminalCommandsShouldKeepProfileContracts(t *testing.T) { // IT-037
	t.Parallel()
	deps := commandDeps{}
	testCases := []struct {
		commandName         string
		command             *cobra.Command
		wantAllProfilesFlag bool
	}{
		{commandName: "attach", command: newTerminalAttachCommand(deps)},
		{commandName: "exec", command: newTerminalExecCommand(deps)},
		{commandName: "get", command: newTerminalGetCommand(deps)},
		{
			commandName:         "input-requests",
			command:             newTerminalInputRequestsCommand(deps),
			wantAllProfilesFlag: true,
		},
		{
			commandName:         "journal",
			command:             newTerminalJournalCommand(deps),
			wantAllProfilesFlag: true,
		},
		{commandName: "kill", command: newTerminalKillCommand(deps)},
		{
			commandName:         "list",
			command:             newTerminalListCommand(deps),
			wantAllProfilesFlag: true,
		},
		{commandName: "open", command: newTerminalOpenCommand(deps)},
		{commandName: "quote", command: newTerminalQuoteCommand(deps)},
		{commandName: "record", command: newTerminalRecordCommand(deps)},
		{commandName: "respond", command: newTerminalRespondCommand(deps)},
		{commandName: "signal", command: newTerminalSignalCommand(deps)},
	}

	wantNames := make([]string, 0, len(testCases))
	for _, testCase := range testCases {
		wantNames = append(wantNames, testCase.commandName)
		t.Run("Should configure "+testCase.commandName, func(t *testing.T) {
			t.Parallel()
			flag := testCase.command.Flags().Lookup(allProfilesFlagName)
			if got := flag != nil; got != testCase.wantAllProfilesFlag {
				t.Fatalf("--all-profiles present = %t, want %t", got, testCase.wantAllProfilesFlag)
			}
		})
	}

	commands := newTerminalCommand(deps).Commands()
	gotNames := make([]string, 0, len(commands))
	for _, command := range commands {
		gotNames = append(gotNames, command.Name())
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("terminal commands = %#v, want %#v", gotNames, wantNames)
	}
}
