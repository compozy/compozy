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

func TestCreateACPSessionForwardsMCPServersOnNewSession(t *testing.T) {
	t.Parallel()

	client := &capturingCommandIOClient{}
	servers := []model.MCPServer{{
		Stdio: &model.MCPServerStdio{
			Name:    "compozy",
			Command: "/tmp/compozy-test",
			Args:    []string{"mcp-serve", "--server", "compozy"},
		},
	}}

	session, err := createACPSession(
		context.Background(),
		client,
		&config{Model: "model-1"},
		&job{
			Prompt:       []byte("solve it"),
			SystemPrompt: "system framing",
			MCPServers:   servers,
		},
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
}

func TestCreateACPSessionForwardsMCPServersOnResume(t *testing.T) {
	t.Parallel()

	client := &capturingCommandIOClient{}
	servers := []model.MCPServer{{
		Stdio: &model.MCPServerStdio{
			Name:    "filesystem",
			Command: "/tmp/fs-mcp",
			Args:    []string{"--serve"},
		},
	}}

	session, err := createACPSession(
		context.Background(),
		client,
		&config{Model: "model-1"},
		&job{
			Prompt:        []byte("solve it"),
			ResumeSession: "sess-existing",
			MCPServers:    servers,
		},
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
}

func TestSetupSessionExecutionEmitsReusableAgentLifecycleSetupEventsOnNewAndResume(t *testing.T) {
	tests := []struct {
		name    string
		resumed bool
	}{
		{name: "new session", resumed: false},
		{name: "resume session", resumed: true},
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
					}, nil
				},
			)
			defer restore()

			tmpDir := t.TempDir()
			execution, err := SetupSessionExecution(SessionSetupRequest{
				Context: context.Background(),
				Config: &config{
					IDE:          model.IDECodex,
					RunArtifacts: model.RunArtifacts{RunID: runID},
				},
				Job: &job{
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
				},
				CWD:        tmpDir,
				RunJournal: runJournal,
				Logger:     silentLogger(),
			})
			if err != nil {
				t.Fatalf("setup session execution: %v", err)
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
	createReq agent.SessionRequest
	resumeReq agent.ResumeSessionRequest
}

type lifecycleCommandIOClient struct {
	session agent.Session
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

func (c *capturingCommandIOClient) CreateSession(
	_ context.Context,
	req agent.SessionRequest,
) (agent.Session, error) {
	c.createReq = req
	return fakeSessionExecutionSession{
		id:      "sess-create",
		updates: make(chan model.SessionUpdate),
		done:    make(chan struct{}),
	}, nil
}

func (c *capturingCommandIOClient) ResumeSession(
	_ context.Context,
	req agent.ResumeSessionRequest,
) (agent.Session, error) {
	c.resumeReq = req
	return fakeSessionExecutionSession{
		id:      "sess-resume",
		updates: make(chan model.SessionUpdate),
		done:    make(chan struct{}),
	}, nil
}

func (*capturingCommandIOClient) SupportsLoadSession() bool { return true }
func (*capturingCommandIOClient) Close() error              { return nil }
func (*capturingCommandIOClient) Kill() error               { return nil }

func (c *lifecycleCommandIOClient) CreateSession(
	context.Context,
	agent.SessionRequest,
) (agent.Session, error) {
	return c.session, nil
}

func (c *lifecycleCommandIOClient) ResumeSession(
	context.Context,
	agent.ResumeSessionRequest,
) (agent.Session, error) {
	return c.session, nil
}

func (*lifecycleCommandIOClient) SupportsLoadSession() bool { return true }
func (*lifecycleCommandIOClient) Close() error              { return nil }
func (*lifecycleCommandIOClient) Kill() error               { return nil }
