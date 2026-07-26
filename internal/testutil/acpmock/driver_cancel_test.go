package acpmock

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
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
		driver := acp.New(acp.WithProcessRegistry(registry))
		proc, err := driver.Start(testutil.Context(t), acp.StartOpts{
			AgentName: "sandboxer",
			Command: BuildCommand(
				driverPath,
				fixturePath,
				"sandboxer",
				filepath.Join(t.TempDir(), "sandbox-diagnostics.jsonl"),
			),
			Cwd:         t.TempDir(),
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		if err != nil {
			t.Fatalf("driver.Start() error = %v", err)
		}
		defer stopDriverProcess(t, driver, proc)

		ctx, cancel := context.WithCancel(testutil.Context(t))
		eventsCh, err := driver.Prompt(ctx, proc, acp.PromptRequest{
			TurnID:  "turn-sandbox-cancel",
			Message: "run long sandbox",
			Meta:    acp.PromptMeta{TurnSource: acp.PromptTurnSourceUser},
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
			Scope: toolruntime.InterruptScope{Source: toolruntime.ProcessSourceACPTerminal},
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
