package acpmock

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestDriverStreamsStablePermissionAndToolSequence(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "tool_permission_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}

	command := BuildCommand(
		driverPath,
		fixturePath,
		"golden",
		filepath.Join(t.TempDir(), "golden-diagnostics.jsonl"),
	)

	driver := acp.New(acp.WithPermissionTimeout(2 * time.Second))
	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "golden",
		Command:     command,
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveReads,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	defer stopDriverProcess(t, driver, proc)

	eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
		TurnID:  "turn-tool-permission",
		Message: "exercise golden",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	})
	if err != nil {
		t.Fatalf("driver.Prompt() error = %v", err)
	}

	events := collectPromptEvents(t, eventsCh, func(event acp.AgentEvent) {
		if event.Type == acp.EventTypePermission && event.Decision == "" && event.RequestID != "" {
			if err := driver.ApprovePermission(testutil.Context(t), proc, acp.ApproveRequest{
				RequestID: event.RequestID,
				Decision:  "allow-always",
			}); err != nil {
				t.Fatalf("ApprovePermission() error = %v", err)
			}
		}
	})

	got := normalizeEvents(events)
	wantPath := filepath.Join("testdata", "tool_permission_golden.json")
	wantBytes, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", wantPath, err)
	}
	var want []map[string]string
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatalf("json.Unmarshal(golden) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized events = %#v, want %#v", got, want)
	}
}

func TestDriverSupportsNetworkOriginSandboxExpectations(t *testing.T) {
	root := t.TempDir()
	fakeAGH := filepath.Join(root, "agh")
	if err := os.WriteFile(fakeAGH, []byte("#!/bin/sh\nprintf network-ok\n"), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", fakeAGH, err)
	}
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "permission_env_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}

	command := BuildCommand(
		driverPath,
		fixturePath,
		"runner",
		filepath.Join(t.TempDir(), "runner-diagnostics.jsonl"),
	)

	driver := acp.New()
	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "runner",
		Command:     command,
		Cwd:         root,
		Permissions: aghconfig.PermissionModeApproveAll,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	proc.SetTurnSourceProvider(func() string { return "network" })
	defer stopDriverProcess(t, driver, proc)

	eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
		TurnID:  "turn-network-sandbox",
		Message: "run sandbox",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceNetwork},
	})
	if err != nil {
		t.Fatalf("driver.Prompt() error = %v", err)
	}

	events := collectPromptEvents(t, eventsCh, nil)
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type":         acp.EventTypeToolCall,
		"title":        "Run fake network status",
		"tool_call_id": "cmd-1",
	}) {
		t.Fatalf("events = %#v, want tool_call for network status", events)
	}
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type":         acp.EventTypeToolResult,
		"title":        "Run fake network status",
		"tool_call_id": "cmd-1",
	}) {
		t.Fatalf("events = %#v, want tool_result for network status", events)
	}
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type": acp.EventTypeAgentMessage,
		"text": "network-ok",
	}) {
		t.Fatalf("events = %#v, want network-ok assistant output", events)
	}
}

func TestDriverAdvertisesAndSupportsLoadSession(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "browser_session_lifecycle_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}

	command := BuildCommand(
		driverPath,
		fixturePath,
		"browser-lifecycle-agent",
		filepath.Join(t.TempDir(), "browser-lifecycle-diagnostics.jsonl"),
	)

	driver := acp.New()
	cwd := t.TempDir()

	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "browser-lifecycle-agent",
		Command:     command,
		Cwd:         cwd,
		Permissions: aghconfig.PermissionModeDenyAll,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	if !proc.Caps.SupportsLoadSession {
		t.Fatal("driver.Start() SupportsLoadSession = false, want true")
	}

	originalSessionID := proc.SessionID
	stopDriverProcess(t, driver, proc)

	resumed, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:       "browser-lifecycle-agent",
		Command:         command,
		Cwd:             cwd,
		Permissions:     aghconfig.PermissionModeDenyAll,
		ResumeSessionID: originalSessionID,
	})
	if err != nil {
		t.Fatalf("driver.Start(resume) error = %v", err)
	}
	defer stopDriverProcess(t, driver, resumed)

	if !resumed.Caps.SupportsLoadSession {
		t.Fatal("driver.Start(resume) SupportsLoadSession = false, want true")
	}
	if got := resumed.SessionID; got != originalSessionID {
		t.Fatalf("resumed SessionID = %q, want %q", got, originalSessionID)
	}
}

func TestDriverDiagnosticsCaptureSessionMCPServers(t *testing.T) {
	t.Parallel()

	t.Run("Should capture the process owner and hosted MCP server", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath, err := filepath.Abs(filepath.Join("testdata", "tool_permission_fixture.json"))
		if err != nil {
			t.Fatalf("filepath.Abs(fixture) error = %v", err)
		}
		diagnosticsPath := filepath.Join(t.TempDir(), "hosted-mcp-diagnostics.jsonl")
		command := BuildCommand(driverPath, fixturePath, "golden", diagnosticsPath)

		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName: "golden",
			Command:   command,
			Cwd:       t.TempDir(),
			Env:       append(testutil.HermeticProcessEnv(nil), "AGH_SESSION_ID=sess-driver-owner"),
			MCPServers: []aghconfig.MCPServer{{
				Name:      "agh-hosted-tools",
				Transport: aghconfig.MCPServerTransportStdio,
				Command:   "/bin/agh",
				Args:      []string{"tool", "mcp", "--session", "sess-1", "--bind-nonce", "nonce"},
			}},
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		records, err := ReadDiagnostics(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics() error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("diagnostics records = %#v, want one session lifecycle record", records)
		}
		if got, want := records[0].AGHSessionID, "sess-driver-owner"; got != want {
			t.Fatalf("AGHSessionID = %q, want %q", got, want)
		}
		if records[0].LifecycleEvent != "session_new" {
			t.Fatalf("LifecycleEvent = %q, want session_new", records[0].LifecycleEvent)
		}
		if got, want := len(records[0].MCPServers), 1; got != want {
			t.Fatalf("MCPServers = %#v, want hosted MCP entry", records[0].MCPServers)
		}
		stdio := records[0].MCPServers[0].Stdio
		if stdio == nil || stdio.Name != "agh-hosted-tools" || stdio.Command != "/bin/agh" {
			t.Fatalf("diagnostic MCP server = %#v, want AGH hosted stdio entry", records[0].MCPServers[0])
		}
	})
}

func TestDriverDiagnosticsIncludePromptMetadataAndMatch(t *testing.T) {
	t.Parallel()

	t.Run("Should include prompt metadata and match in PromptDiagnostics", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath, err := filepath.Abs(filepath.Join("testdata", "multi_agent_fixture.json"))
		if err != nil {
			t.Fatalf("filepath.Abs(fixture) error = %v", err)
		}
		diagnosticsPath := filepath.Join(t.TempDir(), "alpha-diagnostics.jsonl")
		command := BuildCommand(driverPath, fixturePath, "alpha", diagnosticsPath)

		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName:   "alpha",
			Command:     command,
			Cwd:         t.TempDir(),
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
			TurnID:  "turn-alpha",
			Message: "hello alpha",
			Meta: acp.PromptMeta{
				TurnSource: acp.PromptTurnSourceUser,
			},
		})
		if err != nil {
			t.Fatalf("driver.Prompt() error = %v", err)
		}
		_ = collectPromptEvents(t, eventsCh, nil)

		records, err := ReadDiagnostics(diagnosticsPath)
		if err != nil {
			t.Fatalf("ReadDiagnostics() error = %v", err)
		}
		records = PromptDiagnostics(records)
		if got, want := len(records), 1; got != want {
			t.Fatalf("len(records) = %d, want %d", got, want)
		}
		if got, want := records[0].PromptMeta.TurnSource, acp.PromptTurnSourceUser; got != want {
			t.Fatalf("records[0].PromptMeta.TurnSource = %q, want %q", got, want)
		}
		if got, want := records[0].Match.UserText, "hello alpha"; got != want {
			t.Fatalf("records[0].Match.UserText = %q, want %q", got, want)
		}
	})
}

func TestDriverRoutesConcurrentProcessesByStructuredJudgeMetadata(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath := filepath.Join(t.TempDir(), "concurrent-judge-fixture.json")
	fixture := `{
  "version": 2,
  "agents": [{
    "name": "judge",
    "provider": "claude",
    "turns": [
      {"name":"attempt-1","match":{"turn_source":"user","judge":{"role":"agent-judge","attempt":1}},"steps":[{"kind":"assistant","text":"attempt one"}]},
      {"name":"attempt-2","match":{"turn_source":"user","judge":{"role":"agent-judge","attempt":2}},"steps":[{"kind":"assistant","text":"attempt two"}]}
    ]
  }]
}`
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
		t.Fatalf("os.WriteFile(fixture) error = %v", err)
	}
	diagnosticsPath := filepath.Join(t.TempDir(), "shared-diagnostics.jsonl")
	driver := acp.New()
	processes := make([]*acp.AgentProcess, 0, 2)
	t.Cleanup(func() {
		for _, proc := range processes {
			stopDriverProcess(t, driver, proc)
		}
	})
	for index := 1; index <= 2; index++ {
		proc, startErr := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName:   "judge",
			Command:     BuildCommand(driverPath, fixturePath, "judge", diagnosticsPath),
			Cwd:         t.TempDir(),
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		if startErr != nil {
			t.Fatalf("driver.Start(%d) error = %v", index, startErr)
		}
		processes = append(processes, proc)
	}

	type promptResult struct {
		attempt int
		text    string
		err     error
	}
	results := make(chan promptResult, 2)
	correlationIDs := []string{"goal-judge:attempt-1", "goal-judge:attempt-2"}
	for index, proc := range processes {
		attempt := index + 1
		correlationID := correlationIDs[index]
		go func() {
			meta := acp.PromptJudgeMeta{
				Role: acp.PromptJudgeRoleAgent, Attempt: attempt,
				CorrelationID: correlationID,
				GateID:        "goal:goal-judge", CriterionID: "review",
			}
			events, promptErr := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
				TurnID: "turn-judge", Message: "identical rendered rubric",
				Meta: acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser, Judge: &meta},
			})
			if promptErr != nil {
				results <- promptResult{attempt: attempt, err: promptErr}
				return
			}
			var text strings.Builder
			for event := range events {
				if event.Type == acp.EventTypeAgentMessage {
					text.WriteString(event.Text)
				}
			}
			results <- promptResult{attempt: attempt, text: text.String()}
		}()
	}

	for range processes {
		result := <-results
		if result.err != nil {
			t.Fatalf("driver.Prompt(attempt=%d) error = %v", result.attempt, result.err)
		}
		want := "attempt one"
		if result.attempt == 2 {
			want = "attempt two"
		}
		if result.text != want {
			t.Fatalf("driver.Prompt(attempt=%d) text = %q, want %q", result.attempt, result.text, want)
		}
	}
}

func TestDriverReturnsScriptedPromptUsage(t *testing.T) {
	t.Parallel()

	t.Run("Should return scripted input output and total token usage", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath := filepath.Join(t.TempDir(), "usage-fixture.json")
		fixture := []byte(
			`{"version":2,"agents":[{"name":"usage-agent","provider":"claude","turns":[{` +
				`"match":{"turn_source":"user","user_text":"measure"},` +
				`"usage":{"input_tokens":13,"output_tokens":5},` +
				`"steps":[{"kind":"assistant","text":"measured"}]}]}]}`,
		)
		if err := os.WriteFile(fixturePath, fixture, 0o600); err != nil {
			t.Fatalf("os.WriteFile(usage fixture) error = %v", err)
		}
		diagnosticsPath := filepath.Join(t.TempDir(), "usage-diagnostics.jsonl")
		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName:   "usage-agent",
			Command:     BuildCommand(driverPath, fixturePath, "usage-agent", diagnosticsPath),
			Cwd:         t.TempDir(),
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)
		events, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
			TurnID: "turn-usage", Message: "measure",
			Meta: acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		})
		if err != nil {
			t.Fatalf("driver.Prompt() error = %v", err)
		}
		collected := collectPromptEvents(t, events, nil)
		for _, event := range collected {
			if event.Type != acp.EventTypeDone {
				continue
			}
			if event.Usage == nil || event.Usage.InputTokens == nil || event.Usage.OutputTokens == nil ||
				event.Usage.TotalTokens == nil || *event.Usage.InputTokens != 13 ||
				*event.Usage.OutputTokens != 5 || *event.Usage.TotalTokens != 18 {
				t.Fatalf("done usage = %#v, want input=13 output=5 total=18", event.Usage)
			}
			return
		}
		t.Fatalf("prompt events = %#v, want terminal done usage", collected)
	})
}

func TestDriverControlDisconnectSurfacesPromptFailure(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "driver_fault_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}
	command := BuildCommand(
		driverPath,
		fixturePath,
		"faulty",
		filepath.Join(t.TempDir(), "faulty-diagnostics.jsonl"),
	)

	driver := acp.New()
	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "faulty",
		Command:     command,
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveReads,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	defer stopDriverProcess(t, driver, proc)

	eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
		TurnID:  "turn-crash",
		Message: "trigger crash mid-stream",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	})
	if err != nil {
		t.Fatalf("driver.Prompt() error = %v", err)
	}

	events := collectPromptEvents(t, eventsCh, nil)
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type": acp.EventTypeAgentMessage,
		"text": "partial before crash",
	}) {
		t.Fatalf("events = %#v, want partial assistant output before crash", events)
	}
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type": acp.EventTypeError,
	}) {
		t.Fatalf("events = %#v, want error event after driver disconnect", events)
	}
}

func TestDriverControlBlockUntilCancelReturnsCanceledStopReason(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "driver_fault_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}
	command := BuildCommand(
		driverPath,
		fixturePath,
		"faulty",
		filepath.Join(t.TempDir(), "block-diagnostics.jsonl"),
	)

	driver := acp.New()
	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "faulty",
		Command:     command,
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveReads,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	defer stopDriverProcess(t, driver, proc)

	ctx, cancel := context.WithCancel(testutil.Context(t))
	eventsCh, err := driver.Prompt(ctx, proc, acp.PromptRequest{
		TurnID:  "turn-cancel",
		Message: "block until canceled",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	})
	if err != nil {
		t.Fatalf("driver.Prompt() error = %v", err)
	}

	time.AfterFunc(100*time.Millisecond, cancel)
	events := collectPromptEvents(t, eventsCh, nil)
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type": acp.EventTypeError,
	}) {
		t.Fatalf("events = %#v, want error event after prompt cancellation", events)
	}
}

func TestDriverSurfacesPromptTokenUsage(t *testing.T) {
	t.Parallel()

	t.Run("Should surface fixture prompt-response token usage as a done-event payload", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath, err := filepath.Abs(filepath.Join("testdata", "cost_provenance_fixture.json"))
		if err != nil {
			t.Fatalf("filepath.Abs(fixture) error = %v", err)
		}
		command := BuildCommand(
			driverPath,
			fixturePath,
			"cost-provenance-agent",
			filepath.Join(t.TempDir(), "cost-diagnostics.jsonl"),
		)

		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName:   "cost-provenance-agent",
			Command:     command,
			Cwd:         t.TempDir(),
			Permissions: aghconfig.PermissionModeApproveReads,
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
			TurnID:  "turn-cost-provenance",
			Message: "Summarize the cost provenance run",
			Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		})
		if err != nil {
			t.Fatalf("driver.Prompt() error = %v", err)
		}

		events := collectPromptEvents(t, eventsCh, nil)
		var usage *acp.TokenUsage
		for i := range events {
			if events[i].Type == acp.EventTypeDone && events[i].Usage != nil {
				usage = events[i].Usage
			}
		}
		if usage == nil {
			t.Fatalf("no done event carried usage; events = %#v", events)
		}
		if usage.InputTokens == nil || *usage.InputTokens != 128400 {
			t.Fatalf("Usage.InputTokens = %v, want 128400", usage.InputTokens)
		}
		if usage.OutputTokens == nil || *usage.OutputTokens != 24900 {
			t.Fatalf("Usage.OutputTokens = %v, want 24900", usage.OutputTokens)
		}
		if usage.TotalTokens == nil || *usage.TotalTokens != 153300 {
			t.Fatalf("Usage.TotalTokens = %v, want 153300", usage.TotalTokens)
		}
	})
}

func TestDriverCancelNotification(t *testing.T) {
	t.Parallel()

	t.Run("Should unblock active prompt controls with a canceled stop reason", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath := filepath.Join(t.TempDir(), "cancel-notification-fixture.json")
		fixture := `{
			"version": 2,
			"agents": [
				{
					"name": "faulty",
					"provider": "claude",
					"turns": [
						{
							"name": "cancel-after-visible-progress",
							"match": {
								"turn_source": "user",
								"user_text": "block after progress"
							},
							"steps": [
								{
									"kind": "assistant",
									"text": "cancel ready"
								},
								{
									"kind": "driver_control",
									"driver_control": {
										"action": "block_until_cancel"
									}
								}
							]
						}
					]
				}
			]
		}`
		if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", fixturePath, err)
		}
		command := BuildCommand(
			driverPath,
			fixturePath,
			"faulty",
			filepath.Join(t.TempDir(), "cancel-notification-diagnostics.jsonl"),
		)

		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName:   "faulty",
			Command:     command,
			Cwd:         t.TempDir(),
			Permissions: aghconfig.PermissionModeApproveReads,
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
			TurnID:  "turn-cancel-notification",
			Message: "block after progress",
			Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		})
		if err != nil {
			t.Fatalf("driver.Prompt() error = %v", err)
		}

		var cancelOnce sync.Once
		events := collectPromptEvents(t, eventsCh, func(event acp.AgentEvent) {
			if event.Type != acp.EventTypeAgentMessage {
				return
			}
			cancelOnce.Do(func() {
				if err := driver.Cancel(testutil.Context(t), proc); err != nil {
					t.Fatalf("driver.Cancel() error = %v", err)
				}
			})
		})
		normalized := normalizeEvents(events)
		if !containsNormalizedEvent(normalized, map[string]string{
			"type": acp.EventTypeAgentMessage,
			"text": "cancel ready",
		}) {
			t.Fatalf("events = %#v, want visible prompt progress before cancel", events)
		}
		sawCanceledTerminal := false
		for _, event := range events {
			if event.Type == acp.EventTypeDone && event.StopReason == "canceled" {
				sawCanceledTerminal = true
			}
			if event.Type == acp.EventTypeError && event.Failure != nil {
				if event.Failure.Kind == store.FailureCanceled {
					sawCanceledTerminal = true
				}
				if event.Failure.Kind == store.FailureProcess {
					t.Fatalf("events = %#v, did not expect process failure after cancel notification", events)
				}
			}
		}
		if !sawCanceledTerminal {
			t.Fatalf("events = %#v, want canceled terminal event after cancel notification", events)
		}
	})
}

func TestDriverControlAsyncDisconnectDuringPermissionRequestSurfacesPromptFailure(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}
	fixturePath, err := filepath.Abs(filepath.Join("testdata", "driver_fault_fixture.json"))
	if err != nil {
		t.Fatalf("filepath.Abs(fixture) error = %v", err)
	}
	command := BuildCommand(
		driverPath,
		fixturePath,
		"faulty",
		filepath.Join(t.TempDir(), "permission-disconnect-diagnostics.jsonl"),
	)

	driver := acp.New()
	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "faulty",
		Command:     command,
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeApproveReads,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	defer stopDriverProcess(t, driver, proc)

	ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
	defer cancel()
	eventsCh, err := driver.Prompt(ctx, proc, acp.PromptRequest{
		TurnID:  "turn-permission-disconnect",
		Message: "trigger permission disconnect",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	})
	if err != nil {
		t.Fatalf("driver.Prompt() error = %v", err)
	}

	events := collectPromptEvents(t, eventsCh, nil)
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type": acp.EventTypePermission,
	}) {
		t.Fatalf("events = %#v, want permission event before disconnect", events)
	}
	if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
		"type": acp.EventTypeError,
	}) {
		t.Fatalf("events = %#v, want error event after disconnect", events)
	}
}

func TestAsyncDriverControlIsCanceledWhenPromptCompletes(t *testing.T) {
	t.Parallel()

	driverPath, err := DefaultDriverPath()
	if err != nil {
		t.Fatalf("DefaultDriverPath() error = %v", err)
	}

	fixturePath := filepath.Join(t.TempDir(), "async-cancel-fixture.json")
	fixture := `{
		"version": 2,
		"agents": [
			{
				"name": "faulty",
				"provider": "claude",
				"turns": [
					{
						"name": "first-completes-before-disconnect",
						"match": {
							"turn_source": "user",
							"user_text": "first prompt"
						},
						"steps": [
							{
								"kind": "driver_control",
								"driver_control": {
									"action": "disconnect",
									"async": true,
									"delay_ms": 200
								}
							},
							{
								"kind": "assistant",
								"text": "first ok"
							}
						]
					},
					{
						"name": "second-still-runs",
						"match": {
							"turn_source": "user",
							"user_text": "second prompt"
						},
						"steps": [
							{
								"kind": "assistant",
								"text": "second ok"
							}
						]
					}
				]
			}
		]
	}`
	if err := os.WriteFile(fixturePath, []byte(fixture), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", fixturePath, err)
	}

	command := BuildCommand(
		driverPath,
		fixturePath,
		"faulty",
		filepath.Join(t.TempDir(), "async-cancel-diagnostics.jsonl"),
	)

	driver := acp.New()
	proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName:   "faulty",
		Command:     command,
		Cwd:         t.TempDir(),
		Permissions: aghconfig.PermissionModeDenyAll,
	})
	if err != nil {
		t.Fatalf("driver.Start() error = %v", err)
	}
	defer stopDriverProcess(t, driver, proc)

	firstEventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
		TurnID:  "turn-first",
		Message: "first prompt",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	})
	if err != nil {
		t.Fatalf("driver.Prompt(first) error = %v", err)
	}
	firstEvents := collectPromptEvents(t, firstEventsCh, nil)
	if !containsNormalizedEvent(normalizeEvents(firstEvents), map[string]string{
		"type": acp.EventTypeAgentMessage,
		"text": "first ok",
	}) {
		t.Fatalf("first events = %#v, want completed first prompt output", firstEvents)
	}

	select {
	case <-proc.Done():
		t.Fatal("process exited after completed prompt, want async driver control cancellation")
	case <-time.After(300 * time.Millisecond):
	}

	secondEventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
		TurnID:  "turn-second",
		Message: "second prompt",
		Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
	})
	if err != nil {
		t.Fatalf("driver.Prompt(second) error = %v, want process to remain alive", err)
	}
	secondEvents := collectPromptEvents(t, secondEventsCh, nil)
	if !containsNormalizedEvent(normalizeEvents(secondEvents), map[string]string{
		"type": acp.EventTypeAgentMessage,
		"text": "second ok",
	}) {
		t.Fatalf("second events = %#v, want second prompt output", secondEvents)
	}
}

func collectPromptEvents(
	t testing.TB,
	eventsCh <-chan acp.AgentEvent,
	onEvent func(acp.AgentEvent),
) []acp.AgentEvent {
	t.Helper()

	events := make([]acp.AgentEvent, 0, 8)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				return events
			}
			events = append(events, event)
			if onEvent != nil {
				onEvent(event)
			}
		case <-timeout.C:
			t.Fatalf("timed out waiting for prompt events; collected %#v", events)
		}
	}
}

func containsNormalizedEvent(events []map[string]string, want map[string]string) bool {
	for _, event := range events {
		match := true
		for key, value := range want {
			if event[key] != value {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func stopDriverProcess(t testing.TB, driver *acp.Driver, proc *acp.AgentProcess) {
	t.Helper()
	if proc == nil {
		return
	}
	select {
	case <-proc.Done():
		_ = proc.Wait()
		return
	default:
	}
	timer := time.NewTimer(150 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-proc.Done():
		_ = proc.Wait()
		return
	case <-timer.C:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := driver.Stop(ctx, proc); err != nil {
		t.Fatalf("driver.Stop() error = %v", err)
	}
}
func normalizeEvents(events []acp.AgentEvent) []map[string]string {
	normalized := make([]map[string]string, 0, len(events))
	for _, event := range events {
		item := map[string]string{
			"type": event.Type,
		}
		if event.Text != "" {
			item["text"] = event.Text
		}
		if event.Title != "" {
			item["title"] = event.Title
		}
		if event.ToolCallID != "" {
			item["tool_call_id"] = event.ToolCallID
		}
		if event.Resource != "" {
			item["resource"] = event.Resource
		}
		if event.Decision != "" {
			item["decision"] = event.Decision
		}
		if event.StopReason != "" {
			item["stop_reason"] = event.StopReason
		}
		normalized = append(normalized, item)
	}
	return normalized
}
