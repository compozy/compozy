package run

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/core/model"
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
		})
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
		})
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
		})
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
	installACPHelperOnPath(
		t,
		[]runACPHelperScenario{{SupportsLoadSession: true}},
		[]runACPHelperScenario{{
			SessionID:              "sess-1",
			ExpectedPromptContains: "first turn",
			SupportsLoadSession:    true,
			SessionMeta:            map[string]any{"agentSessionId": "agent-1"},
			Updates: []acp.SessionUpdate{
				acp.UpdateAgentMessageText("first response"),
			},
		}},
		[]runACPHelperScenario{{SupportsLoadSession: true}},
		[]runACPHelperScenario{{
			SessionID:              "sess-1",
			ExpectedLoadSessionID:  "sess-1",
			ExpectedPromptContains: "second turn",
			SupportsLoadSession:    true,
			SessionMeta:            map[string]any{"agentSessionId": "agent-1"},
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
	}); err != nil {
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
	}); err != nil {
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
		})
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
		})
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
