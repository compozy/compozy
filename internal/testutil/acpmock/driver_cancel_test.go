package acpmock

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/toolruntime"
)

func TestDriverSandboxCancellationCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should release terminal after canceled sandbox exec", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("POSIX shell process lifecycle test")
		}

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath := writeDriverCancelFixture(t, "sandbox-cancel-fixture.json", `{
			"version": 2,
			"agents": [
				{
					"name": "sandboxer",
					"provider": "claude",
					"turns": [
						{
							"name": "long-sandbox",
							"match": {
								"turn_source": "user",
								"user_text": "run long sandbox"
							},
							"steps": [
								{
									"kind": "sandbox_exec",
									"tool_call_id": "cmd-sleep",
									"title": "Run sleepy terminal",
									"command": "/bin/sh",
									"args": ["-c", "sleep 30"]
								}
							]
						}
					]
				}
			]
		}`)
		store := toolruntime.NewMemoryStore()
		registry := toolruntime.NewRegistry(store)
		driver := acp.New(
			acp.WithProcessRegistry(registry),
			acp.WithTerminalManager(newDriverTerminalManager(t, registry)),
		)
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName: "sandboxer",
			Command: BuildCommand(
				driverPath,
				fixturePath,
				"sandboxer",
				filepath.Join(t.TempDir(), "sandbox-diagnostics.jsonl"),
			),
			Cwd:         t.TempDir(),
			Permissions: compozyconfig.PermissionModeApproveAll,
			WorkspaceID: "workspace-acpmock",
			ProfileID:   "profile-acpmock",
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		ctx, cancel := context.WithCancel(testutil.Context(t))
		eventsCh, err := driver.Prompt(ctx, proc, acp.PromptRequest{
			TurnID:     "turn-sandbox-cancel",
			RunID:      "run-sandbox-cancel",
			Generation: 1,
			Message:    "run long sandbox",
			Meta:       acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		})
		if err != nil {
			t.Fatalf("driver.Prompt() error = %v", err)
		}

		waitForTerminalRecords(t, store, func(records []toolruntime.ProcessRecord) bool {
			return len(records) == 1 && records[0].State == toolruntime.ProcessStateRunning
		})
		cancel()
		events := collectPromptEvents(t, eventsCh, nil)
		if !containsNormalizedEvent(normalizeEvents(events), map[string]string{
			"type": acp.EventTypeError,
		}) {
			t.Fatalf("events = %#v, want prompt error after sandbox cancellation", events)
		}

		records := waitForTerminalRecords(t, store, func(records []toolruntime.ProcessRecord) bool {
			if len(records) != 1 {
				return false
			}
			return records[0].State != toolruntime.ProcessStateRunning &&
				records[0].State != toolruntime.ProcessStateInterrupting
		})
		if got := records[0].State; got != toolruntime.ProcessStateFailed &&
			got != toolruntime.ProcessStateCompleted {
			t.Fatalf("terminal process state = %q, want inactive terminal", got)
		}
	})
}

func TestDriverHoldIgnoringCancelKeepsTurnOpen(t *testing.T) {
	t.Parallel()

	t.Run("Should finish the held turn after the delay despite a session cancel", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath := writeDriverCancelFixture(t, "hold-ignoring-cancel-fixture.json", `{
			"version": 2,
			"agents": [
				{
					"name": "holder",
					"provider": "claude",
					"turns": [
						{
							"name": "hold",
							"match": {
								"turn_source": "user",
								"user_text": "hold ignoring cancel"
							},
							"steps": [
								{
									"kind": "assistant",
									"text": "holding"
								},
								{
									"kind": "driver_control",
									"driver_control": {
										"action": "hold_ignoring_cancel",
										"delay_ms": 1500
									}
								}
							]
						}
					]
				}
			]
		}`)
		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName: "holder",
			Command: BuildCommand(
				driverPath,
				fixturePath,
				"holder",
				filepath.Join(t.TempDir(), "hold-diagnostics.jsonl"),
			),
			Cwd:         t.TempDir(),
			Permissions: compozyconfig.PermissionModeApproveAll,
			WorkspaceID: "workspace-acpmock",
			ProfileID:   "profile-acpmock",
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
			TurnID:     "turn-hold-ignoring-cancel",
			RunID:      "run-hold-ignoring-cancel",
			Generation: 1,
			Message:    "hold ignoring cancel",
			Meta:       acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		})
		if err != nil {
			t.Fatalf("driver.Prompt() error = %v", err)
		}
		var canceledAt time.Time
		events := normalizeEvents(collectPromptEvents(t, eventsCh, func(event acp.AgentEvent) {
			if event.Type != acp.EventTypeAgentMessage || event.Text != "holding" || !canceledAt.IsZero() {
				return
			}
			// Let the holding step settle so the cancel lands inside the hold, as a real stop ladder would.
			time.Sleep(300 * time.Millisecond)
			canceledAt = time.Now()
			if cancelErr := driver.CancelCooperatively(testutil.Context(t), proc); cancelErr != nil {
				t.Errorf("driver.CancelCooperatively() error = %v", cancelErr)
			}
		}))
		if canceledAt.IsZero() {
			t.Fatalf("events = %#v, want the holding message before cancellation", events)
		}
		if held := time.Since(canceledAt); held < time.Second {
			t.Fatalf("turn ended %s after cancellation, want the full hold despite cancel; events = %#v", held, events)
		}
		if !containsNormalizedEvent(events, map[string]string{"type": acp.EventTypeDone}) {
			t.Fatalf("events = %#v, want the held turn to settle after the delay", events)
		}
	})
}

func TestDriverLateCancelDoesNotPoisonNextPrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should ignore cancel after prompt completion", func(t *testing.T) {
		t.Parallel()

		driverPath, err := DefaultDriverPath()
		if err != nil {
			t.Fatalf("DefaultDriverPath() error = %v", err)
		}
		fixturePath := writeDriverCancelFixture(t, "late-cancel-fixture.json", `{
			"version": 2,
			"agents": [
				{
					"name": "late-cancel",
					"provider": "claude",
					"turns": [
						{
							"name": "first-completes",
							"match": {
								"turn_source": "user",
								"user_text": "first prompt"
							},
							"steps": [
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
		}`)
		driver := acp.New()
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName: "late-cancel",
			Command: BuildCommand(
				driverPath,
				fixturePath,
				"late-cancel",
				filepath.Join(t.TempDir(), "late-cancel-diagnostics.jsonl"),
			),
			Cwd:         t.TempDir(),
			Permissions: compozyconfig.PermissionModeDenyAll,
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

		if err := driver.Cancel(testutil.Context(t), proc); err != nil {
			t.Fatalf("driver.Cancel(late) error = %v", err)
		}

		secondEventsCh, err := driver.Prompt(testutil.Context(t), proc, acp.PromptRequest{
			TurnID:  "turn-second",
			Message: "second prompt",
			Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
		})
		if err != nil {
			t.Fatalf("driver.Prompt(second) error = %v", err)
		}
		secondEvents := collectPromptEvents(t, secondEventsCh, nil)
		if !containsNormalizedEvent(normalizeEvents(secondEvents), map[string]string{
			"type": acp.EventTypeAgentMessage,
			"text": "second ok",
		}) {
			t.Fatalf("second events = %#v, want prompt output after late cancel", secondEvents)
		}
	})
}

func waitForTerminalRecords(
	t testing.TB,
	store *toolruntime.MemoryStore,
	accept func([]toolruntime.ProcessRecord) bool,
) []toolruntime.ProcessRecord {
	t.Helper()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		records, err := store.ListProcessRecords(context.Background(), toolruntime.ProcessQuery{
			Scope: toolruntime.InterruptScope{Source: toolruntime.ProcessSourceTerminal},
		})
		if err != nil {
			t.Fatalf("ListProcessRecords() error = %v", err)
		}
		if accept(records) {
			return records
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("timed out waiting for terminal records; last records %#v", records)
		}
	}
}

func writeDriverCancelFixture(t testing.TB, name string, fixture string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}
