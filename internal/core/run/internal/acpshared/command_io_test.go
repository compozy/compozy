package acpshared

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestBuildSessionExecutionUsesSessionSetupRequest(t *testing.T) {
	t.Parallel()

	outFile, err := os.CreateTemp(t.TempDir(), "session-*.out.log")
	if err != nil {
		t.Fatalf("create out file: %v", err)
	}
	defer outFile.Close()

	errFile, err := os.CreateTemp(t.TempDir(), "session-*.err.log")
	if err != nil {
		t.Fatalf("create err file: %v", err)
	}
	defer errFile.Close()

	var aggregate model.Usage
	aggregateMu := &sync.Mutex{}
	activity := newActivityMonitor()
	job := &job{}
	req := SessionSetupRequest{
		Context: context.Background(),
		Config: &config{
			IDE:          model.IDECodex,
			RunArtifacts: model.RunArtifacts{RunID: "run-123"},
		},
		Job:               job,
		UseUI:             true,
		StreamHumanOutput: true,
		Index:             4,
		AggregateUsage:    &aggregate,
		AggregateMu:       aggregateMu,
		Activity:          activity,
		Logger:            silentLogger(),
	}
	session := fakeSessionExecutionSession{
		id: "sess-123",
		identity: agent.SessionIdentity{
			ACPSessionID:   "sess-123",
			AgentSessionID: "agent-123",
		},
		updates: make(chan model.SessionUpdate),
		done:    make(chan struct{}),
	}

	execution := buildSessionExecution(req, sessionExecutionResources{
		session: session,
		speedResolution: kinds.SpeedResolution{
			Requested: kinds.SpeedFast,
			Status:    kinds.SpeedResolutionStatusApplied,
		},
		outFile: outFile,
		errFile: errFile,
		logger:  silentLogger(),
	})

	if execution == nil {
		t.Fatal("expected session execution")
	}
	if execution.Session.ID() != "sess-123" {
		t.Fatalf("unexpected session id: %s", execution.Session.ID())
	}
	if execution.OutFile != outFile || execution.ErrFile != errFile {
		t.Fatalf("expected execution to retain log files")
	}
	if execution.SpeedResolution != (kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusApplied,
	}) {
		t.Fatalf("unexpected execution speed resolution: %#v", execution.SpeedResolution)
	}
	if execution.Handler == nil {
		t.Fatal("expected session update handler")
	}
	if execution.Handler.index != 4 {
		t.Fatalf("unexpected handler index: %d", execution.Handler.index)
	}
	if execution.Handler.agentID != model.IDECodex {
		t.Fatalf("unexpected handler agent id: %s", execution.Handler.agentID)
	}
	if execution.Handler.runID != "run-123" {
		t.Fatalf("unexpected handler run id: %s", execution.Handler.runID)
	}
	if execution.Handler.jobUsage != &job.Usage {
		t.Fatalf("expected handler to reference job usage")
	}
	if execution.Handler.aggregateUsage != &aggregate || execution.Handler.aggregateMu != aggregateMu {
		t.Fatalf("expected aggregate usage wiring to be preserved")
	}
	if execution.Handler.activity != activity {
		t.Fatalf("expected activity monitor wiring to be preserved")
	}
	if execution.Handler.outWriter != outFile || execution.Handler.errWriter != errFile {
		t.Fatalf("expected UI mode to keep file writers only")
	}
}

func TestHasRuntimeEventSubmitterRejectsTypedNilJournal(t *testing.T) {
	t.Parallel()

	var runJournal *journal.Journal
	if hasRuntimeEventSubmitter(runJournal) {
		t.Fatal("expected typed nil journal to be treated as absent")
	}

	submitter := &stubRuntimeEventSubmitter{}
	if !hasRuntimeEventSubmitter(submitter) {
		t.Fatal("expected concrete submitter to be treated as present")
	}
}

func TestCreateACPSessionForwardsMCPServersOnNewSession(t *testing.T) {
	t.Parallel()

	wantResolution := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusApplied,
	}
	client := &capturingCommandIOClient{createResolution: wantResolution}
	servers := []model.MCPServer{{
		Stdio: &model.MCPServerStdio{
			Name:    "compozy",
			Command: "/tmp/compozy-test",
			Args:    []string{"mcp-serve", "--server", "compozy"},
		},
	}}

	testJob := &job{
		Prompt:       []byte("solve it"),
		SystemPrompt: "system framing",
		MCPServers:   servers,
	}
	session, err := createACPSession(
		context.Background(),
		context.Background(),
		client,
		&config{Model: "model-1", Speed: kinds.SpeedFast},
		testJob,
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("create ACP session: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if len(client.createReq.MCPServers) != 1 {
		t.Fatalf("expected one forwarded MCP server, got %#v", client.createReq.MCPServers)
	}
	if client.createReq.MCPServers[0].Stdio == nil ||
		client.createReq.MCPServers[0].Stdio.Name != "compozy" {
		t.Fatalf("unexpected forwarded MCP servers: %#v", client.createReq.MCPServers)
	}
	if client.createReq.Model != "model-1" || client.createReq.Speed != kinds.SpeedFast {
		t.Fatalf("unexpected atomic create request: %#v", client.createReq)
	}
	if testJob.SpeedResolution != wantResolution {
		t.Fatalf("job speed resolution = %#v, want %#v", testJob.SpeedResolution, wantResolution)
	}
	if client.createAtomicCalls != 1 || client.createLegacyCalls != 0 || client.setModelCalls != 0 {
		t.Fatalf(
			"unexpected lifecycle calls: atomic=%d legacy=%d set_model=%d",
			client.createAtomicCalls,
			client.createLegacyCalls,
			client.setModelCalls,
		)
	}
}

func TestCreateACPSessionForwardsMCPServersOnResume(t *testing.T) {
	t.Parallel()

	wantResolution := kinds.SpeedResolution{
		Requested: kinds.SpeedNormal,
		Status:    kinds.SpeedResolutionStatusUnsupported,
		Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
	}
	client := &capturingCommandIOClient{resumeResolution: wantResolution}
	servers := []model.MCPServer{{
		Stdio: &model.MCPServerStdio{
			Name:    "filesystem",
			Command: "/tmp/fs-mcp",
			Args:    []string{"--serve"},
		},
	}}

	testJob := &job{
		Prompt:        []byte("solve it"),
		ResumeSession: "sess-existing",
		MCPServers:    servers,
	}
	session, err := createACPSession(
		context.Background(),
		context.Background(),
		client,
		&config{Model: "model-1", Speed: kinds.SpeedNormal},
		testJob,
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("resume ACP session: %v", err)
	}
	if session == nil {
		t.Fatal("expected session")
	}
	if client.resumeReq.SessionID != "sess-existing" {
		t.Fatalf("unexpected resumed session id: %#v", client.resumeReq)
	}
	if len(client.resumeReq.MCPServers) != 1 {
		t.Fatalf("expected one forwarded MCP server, got %#v", client.resumeReq.MCPServers)
	}
	if client.resumeReq.MCPServers[0].Stdio == nil ||
		client.resumeReq.MCPServers[0].Stdio.Name != "filesystem" {
		t.Fatalf("unexpected forwarded MCP servers: %#v", client.resumeReq.MCPServers)
	}
	if client.resumeReq.Model != "model-1" || client.resumeReq.Speed != kinds.SpeedNormal {
		t.Fatalf("unexpected atomic resume request: %#v", client.resumeReq)
	}
	if testJob.SpeedResolution != wantResolution {
		t.Fatalf("job speed resolution = %#v, want %#v", testJob.SpeedResolution, wantResolution)
	}
	if client.resumeAtomicCalls != 1 || client.resumeLegacyCalls != 0 || client.setModelCalls != 0 {
		t.Fatalf(
			"unexpected lifecycle calls: atomic=%d legacy=%d set_model=%d",
			client.resumeAtomicCalls,
			client.resumeLegacyCalls,
			client.setModelCalls,
		)
	}
}

func TestCreateACPClientUsesPerJobRuntimeWhenPresent(t *testing.T) {
	var captured agent.ClientConfig
	restore := SwapNewAgentClientForTest(func(_ context.Context, cfg agent.ClientConfig) (agent.Client, error) {
		captured = cfg
		return &capturingCommandIOClient{}, nil
	})
	defer restore()

	client, err := createACPClient(
		context.Background(),
		&config{
			IDE:             model.IDECodex,
			Model:           "base-model",
			ReasoningEffort: "medium",
			AddDirs:         []string{"../shared"},
			AccessMode:      model.AccessModeFull,
		},
		&job{
			IDE:             model.IDEClaude,
			Model:           "job-model",
			ReasoningEffort: "high",
		},
		silentLogger(),
	)
	if err != nil {
		t.Fatalf("create ACP client: %v", err)
	}
	if client == nil {
		t.Fatal("expected client")
	}
	if captured.IDE != model.IDEClaude {
		t.Fatalf("expected job IDE override, got %q", captured.IDE)
	}
	if captured.Model != "job-model" {
		t.Fatalf("expected job model override, got %q", captured.Model)
	}
	if captured.ReasoningEffort != "high" {
		t.Fatalf("expected job reasoning override, got %q", captured.ReasoningEffort)
	}
	if captured.AccessMode != model.AccessModeFull {
		t.Fatalf("expected access mode to stay global, got %q", captured.AccessMode)
	}
}

func TestCreateACPClientRejectsLegacyContractAndClosesClient(t *testing.T) {
	legacyClient := &legacyOnlyCommandIOClient{}
	restore := SwapNewAgentClientForTest(
		func(context.Context, agent.ClientConfig) (agent.Client, error) {
			return legacyClient, nil
		},
	)
	defer restore()

	client, err := createACPClient(
		context.Background(),
		&config{IDE: model.IDECodex},
		&job{},
		silentLogger(),
	)
	if err == nil || !strings.Contains(err.Error(), "does not support atomic session setup") {
		t.Fatalf("createACPClient() error = %v, want atomic contract failure", err)
	}
	if client != nil {
		t.Fatalf("createACPClient() client = %#v, want nil", client)
	}
	if legacyClient.closeCalls != 1 {
		t.Fatalf("legacy client Close calls = %d, want 1", legacyClient.closeCalls)
	}
}

func TestSetupSessionExecutionEmitsReusableAgentLifecycleSetupEventsOnNewAndResume(t *testing.T) {
	tests := []struct {
		name           string
		resumed        bool
		speed          kinds.Speed
		wantResolution kinds.SpeedResolution
	}{
		{
			name:  "new session",
			speed: kinds.SpeedFast,
			wantResolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusApplied,
			},
		},
		{
			name:    "resume session",
			resumed: true,
			speed:   kinds.SpeedNormal,
			wantResolution: kinds.SpeedResolution{
				Requested: kinds.SpeedNormal,
				Status:    kinds.SpeedResolutionStatusUnsupported,
				Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runID, runJournal, eventsCh, cleanup := openRuntimeEventCapture(t)
			defer cleanup()

			restore := SwapNewAgentClientForTest(
				func(context.Context, agent.ClientConfig) (agent.Client, error) {
					return &lifecycleCommandIOClient{
						session: fakeSessionExecutionSession{
							id: "sess-lifecycle",
							identity: agent.SessionIdentity{
								ACPSessionID: "sess-lifecycle",
								Resumed:      tt.resumed,
							},
							updates: make(chan model.SessionUpdate),
							done:    make(chan struct{}),
						},
						resolution: tt.wantResolution,
					}, nil
				},
			)
			defer restore()

			tmpDir := t.TempDir()
			testJob := &job{
				SafeName:     "exec",
				Prompt:       []byte("finish the task"),
				SystemPrompt: "workflow memory\n\n<agent_metadata>\nname: planner\n</agent_metadata>",
				ReusableAgent: &reusableAgentExecution{
					Name:                "planner",
					Source:              "workspace",
					AvailableAgentCount: 2,
				},
				MCPServers: []model.MCPServer{
					{Stdio: &model.MCPServerStdio{Name: "compozy", Command: "/tmp/compozy-test"}},
					{Stdio: &model.MCPServerStdio{Name: "filesystem", Command: "/tmp/fs-mcp"}},
				},
				ResumeSession: map[bool]string{true: "sess-existing", false: ""}[tt.resumed],
				OutLog:        filepath.Join(tmpDir, "exec.out.log"),
				ErrLog:        filepath.Join(tmpDir, "exec.err.log"),
			}
			execution, err := SetupSessionExecution(SessionSetupRequest{
				Context: context.Background(),
				Config: &config{
					IDE:          model.IDECodex,
					Speed:        tt.speed,
					RunArtifacts: model.RunArtifacts{RunID: runID},
				},
				Job:        testJob,
				CWD:        tmpDir,
				RunJournal: runJournal,
				Logger:     silentLogger(),
			})
			if err != nil {
				t.Fatalf("setup session execution: %v", err)
			}
			if testJob.SpeedResolution != tt.wantResolution {
				t.Fatalf(
					"job speed resolution = %#v, want %#v",
					testJob.SpeedResolution,
					tt.wantResolution,
				)
			}
			if execution.SpeedResolution != tt.wantResolution {
				t.Fatalf(
					"execution speed resolution = %#v, want %#v",
					execution.SpeedResolution,
					tt.wantResolution,
				)
			}
			execution.Close()

			events := collectRuntimeEvents(t, eventsCh, 4)
			gotKinds := []eventspkg.EventKind{events[0].Kind, events[1].Kind, events[2].Kind, events[3].Kind}
			wantKinds := []eventspkg.EventKind{
				eventspkg.EventKindReusableAgentLifecycle,
				eventspkg.EventKindReusableAgentLifecycle,
				eventspkg.EventKindReusableAgentLifecycle,
				eventspkg.EventKindSessionStarted,
			}
			if !slices.Equal(gotKinds, wantKinds) {
				t.Fatalf("unexpected runtime event kinds: got %v want %v", gotKinds, wantKinds)
			}

			var resolved kinds.ReusableAgentLifecyclePayload
			decodeRuntimeEventPayload(t, events[0], &resolved)
			if resolved.Stage != kinds.ReusableAgentLifecycleStageResolved || resolved.AgentName != "planner" {
				t.Fatalf("unexpected resolved payload: %#v", resolved)
			}

			var prompt kinds.ReusableAgentLifecyclePayload
			decodeRuntimeEventPayload(t, events[1], &prompt)
			if prompt.Stage != kinds.ReusableAgentLifecycleStagePromptAssembled || prompt.AvailableAgents != 2 {
				t.Fatalf("unexpected prompt payload: %#v", prompt)
			}

			var mcpMerged kinds.ReusableAgentLifecyclePayload
			decodeRuntimeEventPayload(t, events[2], &mcpMerged)
			if mcpMerged.Stage != kinds.ReusableAgentLifecycleStageMCPMerged {
				t.Fatalf("unexpected mcp payload: %#v", mcpMerged)
			}
			if mcpMerged.Resumed != tt.resumed {
				t.Fatalf("unexpected resumed flag: %#v", mcpMerged)
			}
			if got, want := mcpMerged.MCPServers, []string{"compozy", "filesystem"}; !slices.Equal(got, want) {
				t.Fatalf("unexpected mcp server names: got %v want %v", got, want)
			}
		})
	}
}

func TestSetupSessionExecutionWarnsButContinuesWhenReusableAgentSetupLifecycleSubmitFails(t *testing.T) {
	var logs bytes.Buffer
	submitter := &stubRuntimeEventSubmitter{
		submitFn: func(ev eventspkg.Event) error {
			if ev.Kind == eventspkg.EventKindReusableAgentLifecycle {
				return errors.New("journal unavailable")
			}
			return nil
		},
	}

	restore := SwapNewAgentClientForTest(
		func(context.Context, agent.ClientConfig) (agent.Client, error) {
			return &lifecycleCommandIOClient{
				session: fakeSessionExecutionSession{
					id: "sess-lifecycle",
					identity: agent.SessionIdentity{
						ACPSessionID: "sess-lifecycle",
					},
					updates: make(chan model.SessionUpdate),
					done:    make(chan struct{}),
				},
			}, nil
		},
	)
	defer restore()

	tmpDir := t.TempDir()
	execution, err := SetupSessionExecution(SessionSetupRequest{
		Context: context.Background(),
		Config: &config{
			IDE:          model.IDECodex,
			RunArtifacts: model.RunArtifacts{RunID: "run-lifecycle"},
		},
		Job: &job{
			SafeName:     "exec",
			Prompt:       []byte("finish the task"),
			SystemPrompt: "workflow memory",
			ReusableAgent: &reusableAgentExecution{
				Name:                "planner",
				Source:              "workspace",
				AvailableAgentCount: 2,
			},
			OutLog: filepath.Join(tmpDir, "exec.out.log"),
			ErrLog: filepath.Join(tmpDir, "exec.err.log"),
		},
		CWD:        tmpDir,
		RunJournal: submitter,
		Logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelWarn,
		})),
	})
	if err != nil {
		t.Fatalf("setup session execution: %v", err)
	}
	execution.Close()

	if !strings.Contains(logs.String(), "failed to emit reusable agent setup lifecycle; continuing") {
		t.Fatalf("expected reusable-agent lifecycle warning, got %q", logs.String())
	}
	if got := submitter.countKind(eventspkg.EventKindSessionStarted); got != 1 {
		t.Fatalf("expected session started event to still be submitted, got %d", got)
	}
}

func TestSetupSessionExecutionWritesSessionCreateFailureToErrLog(t *testing.T) {
	t.Run("Should write authentication error to err.log when session creation fails", func(t *testing.T) {
		authErr := &agent.SessionSetupError{
			Stage: agent.SessionSetupStageNewSession,
			Err: &agent.AuthenticationRequiredError{
				Err: &agent.SessionError{Code: -32000, Message: "Authentication required"},
			},
		}
		restore := SwapNewAgentClientForTest(
			func(context.Context, agent.ClientConfig) (agent.Client, error) {
				return &failingCommandIOClient{createErr: authErr}, nil
			},
		)
		defer restore()

		tmpDir := t.TempDir()
		_, err := SetupSessionExecution(SessionSetupRequest{
			Context: context.Background(),
			Config: &config{
				IDE:          model.IDECursor,
				RunArtifacts: model.RunArtifacts{RunID: "run-auth"},
			},
			Job: &job{
				SafeName: "exec",
				Prompt:   []byte("finish the task"),
				OutLog:   filepath.Join(tmpDir, "exec.out.log"),
				ErrLog:   filepath.Join(tmpDir, "exec.err.log"),
			},
			CWD:    tmpDir,
			Logger: silentLogger(),
		})
		if err == nil {
			t.Fatal("expected setup error")
		}
		for _, want := range []string{
			"cursor-agent is not authenticated",
			"Run 'cursor-agent login' and retry",
			"create ACP session",
			"Authentication required",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("setup error %q does not contain %q", err, want)
			}
		}

		errLog, readErr := os.ReadFile(filepath.Join(tmpDir, "exec.err.log"))
		if readErr != nil {
			t.Fatalf("read err log: %v", readErr)
		}
		for _, want := range []string{
			"ACP session setup error:",
			"cursor-agent is not authenticated",
			"Run 'cursor-agent login' and retry",
			"Authentication required",
		} {
			if !strings.Contains(string(errLog), want) {
				t.Fatalf("err log %q does not contain %q", string(errLog), want)
			}
		}
	})
}

func TestSetupSessionExecutionPreservesRejectedResolutionAndClosesResources(t *testing.T) {
	rejected := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusRejected,
		Reason:    kinds.SpeedResolutionReasonProviderRejected,
	}
	client := &failingCommandIOClient{
		createErr: &agent.SessionSetupError{
			Stage: agent.SessionSetupStageSetSpeed,
			Err:   errors.New("speed rejected"),
		},
		resolution: rejected,
	}
	restore := SwapNewAgentClientForTest(
		func(context.Context, agent.ClientConfig) (agent.Client, error) {
			return client, nil
		},
	)
	defer restore()

	tmpDir := t.TempDir()
	outLog := filepath.Join(tmpDir, "exec.out.log")
	errLog := filepath.Join(tmpDir, "exec.err.log")
	testJob := &job{
		SafeName: "exec",
		Prompt:   []byte("finish the task"),
		OutLog:   outLog,
		ErrLog:   errLog,
	}
	releaseCalls := 0
	_, err := SetupSessionExecution(SessionSetupRequest{
		Context: context.Background(),
		Config: &config{
			IDE:          model.IDECodex,
			Model:        "model-1",
			Speed:        kinds.SpeedFast,
			RunArtifacts: model.RunArtifacts{RunID: "run-rejected"},
		},
		Job:    testJob,
		CWD:    tmpDir,
		Logger: silentLogger(),
		TrackClient: func(agent.Client) func() {
			return func() {
				releaseCalls++
			}
		},
	})
	if err == nil || !strings.Contains(err.Error(), "speed rejected") {
		t.Fatalf("setup error = %v, want rejected speed failure", err)
	}
	if testJob.SpeedResolution != rejected {
		t.Fatalf("job speed resolution = %#v, want %#v", testJob.SpeedResolution, rejected)
	}
	if client.closeCalls != 1 {
		t.Fatalf("client Close calls = %d, want 1", client.closeCalls)
	}
	if releaseCalls != 1 {
		t.Fatalf("client release calls = %d, want 1", releaseCalls)
	}

	errLogContents, readErr := os.ReadFile(errLog)
	if readErr != nil {
		t.Fatalf("read err log: %v", readErr)
	}
	if !strings.Contains(string(errLogContents), "speed rejected") {
		t.Fatalf("err log = %q, want rejected speed failure", string(errLogContents))
	}
	for _, path := range []string{outLog, errLog} {
		if removeErr := os.Remove(path); removeErr != nil {
			t.Fatalf("remove closed setup log %s: %v", path, removeErr)
		}
	}
}

func TestSetupSessionExecutionWritesSessionStartedEventFailureToErrLog(t *testing.T) {
	t.Run("Should write session started event failure to err.log", func(t *testing.T) {
		restore := SwapNewAgentClientForTest(
			func(context.Context, agent.ClientConfig) (agent.Client, error) {
				return &lifecycleCommandIOClient{
					session: fakeSessionExecutionSession{
						id: "sess-event-failure",
						identity: agent.SessionIdentity{
							ACPSessionID: "sess-event-failure",
						},
						updates: make(chan model.SessionUpdate),
						done:    make(chan struct{}),
					},
				}, nil
			},
		)
		defer restore()

		submitter := &stubRuntimeEventSubmitter{
			submitFn: func(ev eventspkg.Event) error {
				if ev.Kind == eventspkg.EventKindSessionStarted {
					return errors.New("journal unavailable")
				}
				return nil
			},
		}
		tmpDir := t.TempDir()
		_, err := SetupSessionExecution(SessionSetupRequest{
			Context: context.Background(),
			Config: &config{
				IDE:          model.IDECodex,
				RunArtifacts: model.RunArtifacts{RunID: "run-event-failure"},
			},
			Job: &job{
				SafeName: "exec",
				Prompt:   []byte("finish the task"),
				OutLog:   filepath.Join(tmpDir, "exec.out.log"),
				ErrLog:   filepath.Join(tmpDir, "exec.err.log"),
			},
			CWD:        tmpDir,
			RunJournal: submitter,
			Logger:     silentLogger(),
		})
		if err == nil {
			t.Fatal("expected setup error")
		}
		for _, want := range []string{
			"submit session started event",
			"journal unavailable",
		} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("setup error %q does not contain %q", err, want)
			}
		}

		errLog, readErr := os.ReadFile(filepath.Join(tmpDir, "exec.err.log"))
		if readErr != nil {
			t.Fatalf("read err log: %v", readErr)
		}
		for _, want := range []string{
			"ACP session setup error:",
			"submit session started event",
			"journal unavailable",
		} {
			if !strings.Contains(string(errLog), want) {
				t.Fatalf("err log %q does not contain %q", string(errLog), want)
			}
		}
	})
}

type fakeSessionExecutionSession struct {
	id       string
	identity agent.SessionIdentity
	updates  chan model.SessionUpdate
	done     chan struct{}
}

func (s fakeSessionExecutionSession) ID() string {
	return s.id
}

func (s fakeSessionExecutionSession) Identity() agent.SessionIdentity {
	return s.identity
}

func (s fakeSessionExecutionSession) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s fakeSessionExecutionSession) Done() <-chan struct{} {
	return s.done
}

func (s fakeSessionExecutionSession) Err() error {
	return nil
}

func (s fakeSessionExecutionSession) SlowPublishes() uint64 {
	return 0
}

func (s fakeSessionExecutionSession) DroppedUpdates() uint64 {
	return 0
}

type capturingCommandIOClient struct {
	createReq         agent.SessionRequest
	resumeReq         agent.ResumeSessionRequest
	createResolution  kinds.SpeedResolution
	resumeResolution  kinds.SpeedResolution
	createAtomicCalls int
	resumeAtomicCalls int
	createLegacyCalls int
	resumeLegacyCalls int
	setModelCalls     int
}

type lifecycleCommandIOClient struct {
	session    agent.Session
	resolution kinds.SpeedResolution
}

type failingCommandIOClient struct {
	createErr  error
	resolution kinds.SpeedResolution
	closeCalls int
}

type legacyOnlyCommandIOClient struct {
	closeCalls int
}

type stubRuntimeEventSubmitter struct {
	mu       sync.Mutex
	events   []eventspkg.Event
	submitFn func(eventspkg.Event) error
}

func (s *stubRuntimeEventSubmitter) Submit(_ context.Context, ev eventspkg.Event) error {
	s.mu.Lock()
	s.events = append(s.events, ev)
	submitFn := s.submitFn
	s.mu.Unlock()
	if submitFn != nil {
		return submitFn(ev)
	}
	return nil
}

func (s *stubRuntimeEventSubmitter) countKind(kind eventspkg.EventKind) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	total := 0
	for _, ev := range s.events {
		if ev.Kind == kind {
			total++
		}
	}
	return total
}

func (s *stubRuntimeEventSubmitter) snapshot() []eventspkg.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]eventspkg.Event(nil), s.events...)
}

func (c *capturingCommandIOClient) CreateSession(
	_ context.Context,
	req agent.SessionRequest,
) (agent.Session, error) {
	c.createLegacyCalls++
	c.createReq = req
	return nil, errors.New("legacy CreateSession must not be called")
}

func (c *capturingCommandIOClient) ResumeSession(
	_ context.Context,
	req agent.ResumeSessionRequest,
) (agent.Session, error) {
	c.resumeLegacyCalls++
	c.resumeReq = req
	return nil, errors.New("legacy ResumeSession must not be called")
}

func (c *capturingCommandIOClient) CreateSessionAtomic(
	_ context.Context,
	req agent.SessionRequest,
) (agent.SessionStart, error) {
	c.createAtomicCalls++
	c.createReq = req
	return agent.SessionStart{
		Session: fakeSessionExecutionSession{
			id:      "sess-create",
			updates: make(chan model.SessionUpdate),
			done:    make(chan struct{}),
		},
		Speed: c.createResolution,
	}, nil
}

func (c *capturingCommandIOClient) ResumeSessionAtomic(
	_ context.Context,
	req agent.ResumeSessionRequest,
) (agent.SessionStart, error) {
	c.resumeAtomicCalls++
	c.resumeReq = req
	return agent.SessionStart{
		Session: fakeSessionExecutionSession{
			id:      "sess-resume",
			updates: make(chan model.SessionUpdate),
			done:    make(chan struct{}),
		},
		Speed: c.resumeResolution,
	}, nil
}

func (*capturingCommandIOClient) SupportsLoadSession() bool { return true }
func (*capturingCommandIOClient) Close() error              { return nil }
func (*capturingCommandIOClient) Kill() error               { return nil }

func (*capturingCommandIOClient) CancelSession(context.Context, string) error {
	return nil
}

func (c *capturingCommandIOClient) SetSessionModel(context.Context, string, string) error {
	c.setModelCalls++
	return errors.New("caller-owned model setup must not be called")
}

func (*capturingCommandIOClient) PromptSession(
	_ context.Context,
	req agent.PromptSessionRequest,
) (agent.Session, error) {
	return fakeSessionExecutionSession{
		id:      req.SessionID,
		updates: make(chan model.SessionUpdate),
		done:    make(chan struct{}),
	}, nil
}

func (c *lifecycleCommandIOClient) CreateSession(
	context.Context,
	agent.SessionRequest,
) (agent.Session, error) {
	return nil, errors.New("legacy CreateSession must not be called")
}

func (c *lifecycleCommandIOClient) ResumeSession(
	context.Context,
	agent.ResumeSessionRequest,
) (agent.Session, error) {
	return nil, errors.New("legacy ResumeSession must not be called")
}

func (c *lifecycleCommandIOClient) CreateSessionAtomic(
	context.Context,
	agent.SessionRequest,
) (agent.SessionStart, error) {
	return agent.SessionStart{Session: c.session, Speed: c.resolution}, nil
}

func (c *lifecycleCommandIOClient) ResumeSessionAtomic(
	context.Context,
	agent.ResumeSessionRequest,
) (agent.SessionStart, error) {
	return agent.SessionStart{Session: c.session, Speed: c.resolution}, nil
}

func (*lifecycleCommandIOClient) SupportsLoadSession() bool { return true }
func (*lifecycleCommandIOClient) Close() error              { return nil }
func (*lifecycleCommandIOClient) Kill() error               { return nil }

func (*lifecycleCommandIOClient) CancelSession(context.Context, string) error {
	return nil
}

func (*lifecycleCommandIOClient) SetSessionModel(context.Context, string, string) error {
	return errors.New("caller-owned model setup must not be called")
}

func (c *lifecycleCommandIOClient) PromptSession(
	context.Context,
	agent.PromptSessionRequest,
) (agent.Session, error) {
	return c.session, nil
}

func (c *failingCommandIOClient) CreateSession(
	context.Context,
	agent.SessionRequest,
) (agent.Session, error) {
	return nil, errors.New("legacy CreateSession must not be called")
}

func (c *failingCommandIOClient) ResumeSession(
	context.Context,
	agent.ResumeSessionRequest,
) (agent.Session, error) {
	return nil, errors.New("legacy ResumeSession must not be called")
}

func (c *failingCommandIOClient) CreateSessionAtomic(
	context.Context,
	agent.SessionRequest,
) (agent.SessionStart, error) {
	return agent.SessionStart{Speed: c.resolution}, c.createErr
}

func (c *failingCommandIOClient) ResumeSessionAtomic(
	context.Context,
	agent.ResumeSessionRequest,
) (agent.SessionStart, error) {
	return agent.SessionStart{Speed: c.resolution}, c.createErr
}

func (*failingCommandIOClient) SupportsLoadSession() bool { return true }
func (c *failingCommandIOClient) Close() error {
	c.closeCalls++
	return nil
}
func (*failingCommandIOClient) Kill() error { return nil }

func (*failingCommandIOClient) CancelSession(context.Context, string) error {
	return nil
}

func (*failingCommandIOClient) SetSessionModel(context.Context, string, string) error {
	return errors.New("caller-owned model setup must not be called")
}

func (c *failingCommandIOClient) PromptSession(
	context.Context,
	agent.PromptSessionRequest,
) (agent.Session, error) {
	return nil, c.createErr
}

func (*legacyOnlyCommandIOClient) CreateSession(
	context.Context,
	agent.SessionRequest,
) (agent.Session, error) {
	return nil, errors.New("legacy CreateSession must not be called")
}

func (*legacyOnlyCommandIOClient) ResumeSession(
	context.Context,
	agent.ResumeSessionRequest,
) (agent.Session, error) {
	return nil, errors.New("legacy ResumeSession must not be called")
}

func (*legacyOnlyCommandIOClient) CancelSession(context.Context, string) error {
	return nil
}

func (*legacyOnlyCommandIOClient) PromptSession(
	context.Context,
	agent.PromptSessionRequest,
) (agent.Session, error) {
	return nil, nil
}

func (*legacyOnlyCommandIOClient) SetSessionModel(context.Context, string, string) error {
	return errors.New("caller-owned model setup must not be called")
}

func (*legacyOnlyCommandIOClient) SupportsLoadSession() bool { return false }

func (c *legacyOnlyCommandIOClient) Close() error {
	c.closeCalls++
	return nil
}

func (*legacyOnlyCommandIOClient) Kill() error { return nil }
