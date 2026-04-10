package exec

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/agent"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestExecuteExecTextModePrintsOnlyFinalAssistantResponse(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("final answer"),
		},
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatText,
			PromptText:             "finish the task",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec text: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "final answer" {
		t.Fatalf("expected final assistant response only, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".compozy", "runs")); !os.IsNotExist(err) {
		t.Fatalf("expected no persisted run artifacts, got stat err=%v", err)
	}
}

func TestExecuteExecHeadlessDefaultDoesNotEmitOperationalLogs(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("final answer"),
		},
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		previousLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		defer slog.SetDefault(previousLogger)

		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatText,
			PromptText:             "finish the task",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec default logging: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "final answer" {
		t.Fatalf("unexpected exec stdout: %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no operational stderr by default, got %q", stderr)
	}
}

func TestExecuteExecVerboseEmitsOperationalLogsToStderr(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "finish the task",
		Updates: []acp.SessionUpdate{
			acp.UpdateAgentMessageText("final answer"),
		},
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		previousLogger := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		defer slog.SetDefault(previousLogger)

		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatText,
			Verbose:                true,
			PromptText:             "finish the task",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec verbose logging: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "final answer" {
		t.Fatalf("unexpected verbose exec stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "acp session created") {
		t.Fatalf("expected verbose stderr to include ACP lifecycle logs, got %q", stderr)
	}
}

func TestExecuteExecPersistedRunCanResumeSameSession(t *testing.T) {
	tmpDir := t.TempDir()
	plannerDir := filepath.Join(tmpDir, model.WorkflowRootDirName, "agents", "planner")
	if err := os.MkdirAll(plannerDir, 0o755); err != nil {
		t.Fatalf("mkdir planner agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Planner",
			"description: Plans the work",
			"ide: codex",
			"---",
			"",
			"Plan the work carefully.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write planner agent: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "mcp.json"),
		[]byte(`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`),
		0o600,
	); err != nil {
		t.Fatalf("write planner mcp.json: %v", err)
	}
	installACPHelperOnPath(
		t,
		[]runACPHelperScenario{{
			SupportsLoadSession:              true,
			ExpectedNewSessionMCPServerNames: []string{"compozy", "filesystem"},
		}},
		[]runACPHelperScenario{{
			SessionID:                        "sess-1",
			ExpectedPromptContains:           "first turn",
			ExpectedNewSessionMCPServerNames: []string{"compozy", "filesystem"},
			SupportsLoadSession:              true,
			SessionMeta:                      map[string]any{"agentSessionId": "agent-1"},
			Updates: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("first response"),
			},
		}},
		[]runACPHelperScenario{{SupportsLoadSession: true}},
		[]runACPHelperScenario{{
			SessionID:                         "sess-1",
			ExpectedLoadSessionID:             "sess-1",
			ExpectedPromptContains:            "second turn",
			ExpectedLoadSessionMCPServerNames: []string{"compozy", "filesystem"},
			SupportsLoadSession:               true,
			SessionMeta:                       map[string]any{"agentSessionId": "agent-1"},
			ReplayUpdatesOnLoad: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("replayed response"),
			},
			Updates: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("second response"),
			},
		}},
	)

	if err := ExecuteExec(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot:          tmpDir,
		IDE:                    model.IDECodex,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "first turn",
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
		AgentName:              "planner",
	}, nil); err != nil {
		t.Fatalf("execute first persisted exec: %v", err)
	}

	runID := latestPersistedExecRunID(t, tmpDir)
	if err := ExecuteExec(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot:          tmpDir,
		IDE:                    model.IDECodex,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "second turn",
		ReasoningEffort:        "medium",
		RetryBackoffMultiplier: 1.5,
		Persist:                true,
		RunID:                  runID,
		AgentName:              "planner",
	}, nil); err != nil {
		t.Fatalf("execute resumed exec: %v", err)
	}

	runRecord, err := LoadPersistedExecRun(tmpDir, runID)
	if err != nil {
		t.Fatalf("load persisted exec run: %v", err)
	}
	if runRecord.TurnCount != 2 {
		t.Fatalf("expected two turns after resume, got %d", runRecord.TurnCount)
	}
	if runRecord.ACPSessionID != "sess-1" {
		t.Fatalf("unexpected persisted acp session id: %q", runRecord.ACPSessionID)
	}
	if runRecord.AgentSessionID != "agent-1" {
		t.Fatalf("unexpected persisted agent session id: %q", runRecord.AgentSessionID)
	}
	responseBytes, err := os.ReadFile(filepath.Join(tmpDir, ".compozy", "runs", runID, "turns", "0002", "response.txt"))
	if err != nil {
		t.Fatalf("read resumed response: %v", err)
	}
	if strings.TrimSpace(string(responseBytes)) != "second response" {
		t.Fatalf("unexpected resumed response: %q", string(responseBytes))
	}
}

func TestExecuteExecJSONModeEmitsLeanJSONLAndPersistsRawEvents(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "stream the session",
		Updates:                execJSONProjectionScenarioUpdates(),
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatJSON,
			PromptText:             "stream the session",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
			Persist:                true,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec json projection: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	stdoutEvents := decodeExecJSONLEventsForRunTest(t, stdout)
	assertSessionUpdateKindsPresent(t, stdoutEvents,
		string(model.UpdateKindUserMessageChunk),
		string(model.UpdateKindAgentMessageChunk),
		string(model.UpdateKindToolCallStarted),
		string(model.UpdateKindToolCallUpdated),
	)
	assertSessionUpdateKindsAbsent(t, stdoutEvents,
		string(model.UpdateKindAgentThoughtChunk),
		string(model.UpdateKindPlanUpdated),
		string(model.UpdateKindAvailableCommandsUpdated),
		string(model.UpdateKindCurrentModeUpdated),
	)

	runID := latestPersistedExecRunID(t, tmpDir)
	rawEvents := readRuntimeEventFile(t, filepath.Join(tmpDir, ".compozy", "runs", runID, "events.jsonl"))
	assertRuntimeSessionUpdateKindsPresent(t, rawEvents,
		string(model.UpdateKindUserMessageChunk),
		string(model.UpdateKindAgentThoughtChunk),
		string(model.UpdateKindAgentMessageChunk),
		string(model.UpdateKindPlanUpdated),
		string(model.UpdateKindAvailableCommandsUpdated),
		string(model.UpdateKindCurrentModeUpdated),
		string(model.UpdateKindToolCallStarted),
		string(model.UpdateKindToolCallUpdated),
	)
}

func TestExecuteExecRawJSONModeEmitsFullJSONL(t *testing.T) {
	tmpDir := t.TempDir()
	installACPHelperOnPath(t, []runACPHelperScenario{{
		ExpectedPromptContains: "stream everything",
		Updates:                execJSONProjectionScenarioUpdates(),
	}})

	stdout, stderr, execErr := captureExecuteStreams(t, func() error {
		return ExecuteExec(context.Background(), &model.RuntimeConfig{
			WorkspaceRoot:          tmpDir,
			IDE:                    model.IDECodex,
			Mode:                   model.ExecutionModeExec,
			OutputFormat:           model.OutputFormatRawJSON,
			PromptText:             "stream everything",
			ReasoningEffort:        "medium",
			RetryBackoffMultiplier: 1.5,
		}, nil)
	})
	if execErr != nil {
		t.Fatalf("execute exec raw-json: %v\nstdout:\n%s\nstderr:\n%s", execErr, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected no stderr, got %q", stderr)
	}

	events := decodeExecJSONLEventsForRunTest(t, stdout)
	assertSessionUpdateKindsPresent(t, events,
		string(model.UpdateKindUserMessageChunk),
		string(model.UpdateKindAgentThoughtChunk),
		string(model.UpdateKindAgentMessageChunk),
		string(model.UpdateKindPlanUpdated),
		string(model.UpdateKindAvailableCommandsUpdated),
		string(model.UpdateKindCurrentModeUpdated),
		string(model.UpdateKindToolCallStarted),
		string(model.UpdateKindToolCallUpdated),
	)
}

func TestShouldEmitLeanSessionUpdateKeepsOnlyUserFacingAndTerminalUpdates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		update model.SessionUpdate
		want   bool
	}{
		{
			name:   "Should keep agent message chunks",
			update: model.SessionUpdate{Kind: model.UpdateKindAgentMessageChunk, Status: model.StatusRunning},
			want:   true,
		},
		{
			name:   "Should drop plan updates",
			update: model.SessionUpdate{Kind: model.UpdateKindPlanUpdated, Status: model.StatusRunning},
			want:   false,
		},
		{
			name:   "Should keep unknown completed updates",
			update: model.SessionUpdate{Status: model.StatusCompleted},
			want:   true,
		},
		{
			name:   "Should drop unknown running updates",
			update: model.SessionUpdate{Status: model.StatusRunning},
			want:   false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldEmitLeanSessionUpdate(&tc.update); got != tc.want {
				t.Fatalf("shouldEmitLeanSessionUpdate() = %v, want %v for %#v", got, tc.want, tc.update)
			}
		})
	}
}

func TestExecuteExecWithSelectedAgentResolvesRuntimeAndCanonicalSystemPrompt(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	plannerDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "planner")
	if err := os.MkdirAll(plannerDir, 0o755); err != nil {
		t.Fatalf("mkdir planner agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Planner",
			"description: Plans the work",
			"ide: claude",
			"model: agent-model",
			"access_mode: default",
			"---",
			"",
			"Plan the work carefully.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write planner agent: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(plannerDir, "mcp.json"),
		[]byte(`{"mcpServers":{"filesystem":{"command":"/tmp/fs-mcp","args":["--serve"]}}}`),
		0o600,
	); err != nil {
		t.Fatalf("write planner mcp.json: %v", err)
	}

	reviewerDir := filepath.Join(workspaceRoot, model.WorkflowRootDirName, "agents", "reviewer")
	if err := os.MkdirAll(reviewerDir, 0o755); err != nil {
		t.Fatalf("mkdir reviewer agent dir: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(reviewerDir, "AGENT.md"),
		[]byte(strings.Join([]string{
			"---",
			"title: Reviewer",
			"description: Reviews code",
			"ide: codex",
			"---",
			"",
			"Review the code.",
			"",
		}, "\n")),
		0o600,
	); err != nil {
		t.Fatalf("write reviewer agent: %v", err)
	}

	var (
		gotClientCfg agent.ClientConfig
		gotReq       agent.SessionRequest
	)
	restore := acpshared.SwapNewAgentClientForTest(
		func(_ context.Context, cfg agent.ClientConfig) (agent.Client, error) {
			gotClientCfg = cfg
			return &capturingExecACPClient{
				createSessionFn: func(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
					gotReq = req
					session := newCapturingExecSession("sess-agent")
					go session.finish(nil)
					return session, nil
				},
			}, nil
		},
	)
	t.Cleanup(restore)

	err := ExecuteExec(context.Background(), &model.RuntimeConfig{
		WorkspaceRoot:          workspaceRoot,
		IDE:                    model.IDECodex,
		Model:                  "cli-model",
		ReasoningEffort:        "workspace-reasoning",
		AccessMode:             model.AccessModeFull,
		Mode:                   model.ExecutionModeExec,
		OutputFormat:           model.OutputFormatText,
		PromptText:             "finish the task",
		RetryBackoffMultiplier: 1.5,
		AgentName:              "planner",
		ExplicitRuntime: model.ExplicitRuntimeFlags{
			Model: true,
		},
	})
	if err != nil {
		t.Fatalf("execute exec with selected agent: %v", err)
	}

	if gotClientCfg.IDE != model.IDEClaude {
		t.Fatalf("expected agent ide to reach ACP client config, got %q", gotClientCfg.IDE)
	}
	if gotClientCfg.Model != "cli-model" {
		t.Fatalf("expected explicit model to reach ACP client config, got %q", gotClientCfg.Model)
	}
	if gotClientCfg.ReasoningEffort != "workspace-reasoning" {
		t.Fatalf("expected existing reasoning effort to be preserved, got %q", gotClientCfg.ReasoningEffort)
	}
	if gotClientCfg.AccessMode != model.AccessModeDefault {
		t.Fatalf("expected agent access mode to reach ACP client config, got %q", gotClientCfg.AccessMode)
	}
	if gotReq.Model != "cli-model" {
		t.Fatalf("expected session request model to use explicit override, got %q", gotReq.Model)
	}
	if len(gotReq.MCPServers) != 2 {
		t.Fatalf("expected reserved plus agent-local MCP servers, got %#v", gotReq.MCPServers)
	}
	if gotReq.MCPServers[0].Stdio == nil || gotReq.MCPServers[0].Stdio.Name != reusableagents.ReservedMCPServerName {
		t.Fatalf("unexpected reserved MCP server wiring: %#v", gotReq.MCPServers)
	}
	if gotReq.MCPServers[1].Stdio == nil || gotReq.MCPServers[1].Stdio.Name != "filesystem" {
		t.Fatalf("unexpected agent-local MCP server wiring: %#v", gotReq.MCPServers)
	}

	promptText := string(gotReq.Prompt)
	requiredSnippets := []string{
		"<agent_metadata>",
		"name: planner",
		"title: Planner",
		"description: Plans the work",
		"source: workspace",
		"<available_agents>",
		"- reviewer: Reviews code (workspace)",
		"Plan the work carefully.",
		"finish the task",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(promptText, snippet) {
			t.Fatalf("expected composed ACP prompt to include %q, got:\n%s", snippet, promptText)
		}
	}
	if strings.Count(promptText, "<agent_metadata>") != 1 ||
		strings.Count(promptText, "Plan the work carefully.") != 1 {
		t.Fatalf("expected selected agent metadata and prompt body exactly once, got:\n%s", promptText)
	}
	if strings.Contains(promptText, "- planner:") {
		t.Fatalf("expected discovery catalog to exclude selected agent, got:\n%s", promptText)
	}
}

func latestPersistedExecRunID(t *testing.T, workspaceRoot string) string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(workspaceRoot, ".compozy", "runs"))
	if err != nil {
		t.Fatalf("read persisted exec runs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one persisted exec run, got %d", len(entries))
	}
	return entries[0].Name()
}

func execJSONProjectionScenarioUpdates() []acp.SessionUpdate {
	return []acp.SessionUpdate{
		acp.UpdateUserMessageText("user says hello"),
		acp.UpdateAgentThoughtText("thinking"),
		acp.UpdateAgentMessageText("visible answer"),
		acp.UpdatePlan(acp.PlanEntry{
			Content:  "Inspect repo",
			Status:   acp.PlanEntryStatusInProgress,
			Priority: acp.PlanEntryPriorityHigh,
		}),
		{
			AvailableCommandsUpdate: &acp.SessionAvailableCommandsUpdate{
				AvailableCommands: []acp.AvailableCommand{{
					Name:        "run",
					Description: "Run the task",
					Input: &acp.AvailableCommandInput{
						UnstructuredCommandInput: &acp.AvailableCommandUnstructuredCommandInput{Hint: "--fast"},
					},
				}},
			},
		},
		{
			CurrentModeUpdate: &acp.SessionCurrentModeUpdate{CurrentModeId: "review"},
		},
		acp.StartReadToolCall(acp.ToolCallId("tool-1"), "Read README.md", "README.md"),
		acp.UpdateToolCall(
			acp.ToolCallId("tool-1"),
			acp.WithUpdateStatus(acp.ToolCallStatusCompleted),
			acp.WithUpdateContent([]acp.ToolCallContent{
				acp.ToolContent(acp.TextBlock("README contents")),
			}),
		),
	}
}

func readRuntimeEventFile(t *testing.T, path string) []eventspkg.Event {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read runtime event file %s: %v", path, err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	events := make([]eventspkg.Event, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event eventspkg.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode runtime event: %v\nline:\n%s", err, line)
		}
		events = append(events, event)
	}
	return events
}

func decodeExecJSONLEventsForRunTest(t *testing.T, data string) []map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(data), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatalf("decode exec event: %v\nline:\n%s", err, line)
		}
		events = append(events, payload)
	}
	return events
}

func assertSessionUpdateKindsPresent(t *testing.T, events []map[string]any, want ...string) {
	t.Helper()

	kinds := collectedSessionUpdateKinds(events)
	for _, kind := range want {
		if !slices.Contains(kinds, kind) {
			t.Fatalf("expected session.update kind %q in %v", kind, kinds)
		}
	}
}

func assertSessionUpdateKindsAbsent(t *testing.T, events []map[string]any, want ...string) {
	t.Helper()

	kinds := collectedSessionUpdateKinds(events)
	for _, kind := range want {
		if slices.Contains(kinds, kind) {
			t.Fatalf("expected session.update kind %q to be absent from %v", kind, kinds)
		}
	}
}

func assertRuntimeSessionUpdateKindsPresent(t *testing.T, events []eventspkg.Event, want ...string) {
	t.Helper()

	kinds := collectedRuntimeSessionUpdateKinds(t, events)
	for _, kind := range want {
		if !slices.Contains(kinds, kind) {
			t.Fatalf("expected runtime session.update kind %q in %v", kind, kinds)
		}
	}
}

func collectedSessionUpdateKinds(events []map[string]any) []string {
	kinds := make([]string, 0, len(events))
	for _, event := range events {
		eventType, _ := event["type"].(string)
		if eventType != "session.update" {
			continue
		}
		update, _ := event["update"].(map[string]any)
		kind, _ := update["kind"].(string)
		kinds = append(kinds, kind)
	}
	return kinds
}

func collectedRuntimeSessionUpdateKinds(t *testing.T, events []eventspkg.Event) []string {
	t.Helper()

	updateKinds := make([]string, 0, len(events))
	for _, event := range events {
		if event.Kind != eventspkg.EventKindSessionUpdate {
			continue
		}
		var payload kinds.SessionUpdatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("decode runtime session.update payload: %v", err)
		}
		updateKinds = append(updateKinds, string(payload.Update.Kind))
	}
	return updateKinds
}

type capturingExecACPClient struct {
	createSessionFn func(context.Context, agent.SessionRequest) (agent.Session, error)
}

func (c *capturingExecACPClient) CreateSession(ctx context.Context, req agent.SessionRequest) (agent.Session, error) {
	return c.createSessionFn(ctx, req)
}

func (*capturingExecACPClient) ResumeSession(context.Context, agent.ResumeSessionRequest) (agent.Session, error) {
	return nil, nil
}

func (*capturingExecACPClient) SupportsLoadSession() bool {
	return false
}

func (*capturingExecACPClient) Close() error {
	return nil
}

func (*capturingExecACPClient) Kill() error {
	return nil
}

type capturingExecSession struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}

	mu       sync.RWMutex
	err      error
	finished bool
}

func newCapturingExecSession(id string) *capturingExecSession {
	return &capturingExecSession{
		id:      id,
		updates: make(chan model.SessionUpdate, 1),
		done:    make(chan struct{}),
	}
}

func (s *capturingExecSession) ID() string {
	return s.id
}

func (s *capturingExecSession) Identity() agent.SessionIdentity {
	return agent.SessionIdentity{ACPSessionID: s.id}
}

func (s *capturingExecSession) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *capturingExecSession) Done() <-chan struct{} {
	return s.done
}

func (s *capturingExecSession) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

func (*capturingExecSession) SlowPublishes() uint64 {
	return 0
}

func (*capturingExecSession) DroppedUpdates() uint64 {
	return 0
}

func (s *capturingExecSession) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	s.err = err
	close(s.updates)
	close(s.done)
}
