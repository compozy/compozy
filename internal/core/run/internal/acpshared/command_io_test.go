package acpshared

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
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
