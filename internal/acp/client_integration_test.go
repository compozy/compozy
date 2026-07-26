//go:build integration

package acp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestACPIntegrationRoundTrip(t *testing.T) {
	t.Run("Should return prompt events through helper ACP process", func(t *testing.T) {
		t.Parallel()

		driver := New()
		proc := startHelperProcess(t, driver, "stream_updates", "", StartOpts{})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-roundtrip",
			Message: "run roundtrip",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if len(events) == 0 {
			t.Fatal("Prompt() returned no events")
		}
		if events[len(events)-1].Type != EventTypeDone {
			t.Fatalf("Prompt() last event = %#v, want done", events[len(events)-1])
		}
	})
}

func TestACPIntegrationReadTextFileRequest(t *testing.T) {
	t.Run("Should read file content through ACP file callback", func(t *testing.T) {
		t.Parallel()

		driver := New()

		root := t.TempDir()
		target := filepath.Join(root, "notes.txt")
		if err := os.WriteFile(target, []byte("from-disk"), 0o644); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}

		proc := startHelperProcess(t, driver, "fs_read", target, StartOpts{
			Cwd:         root,
			Permissions: aghconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-fs",
			Message: "read file",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "from-disk") {
			t.Fatalf("Prompt() events = %#v, want file content", events)
		}
	})
}

func TestACPIntegrationToolHostFileWriteReadAndTerminal(t *testing.T) {
	t.Run("Should write read and run terminal commands through the local tool host", func(t *testing.T) {
		t.Parallel()

		driver := New()

		root := t.TempDir()
		target := filepath.Join(root, "created.txt")
		proc := startHelperProcess(t, driver, "fs_write_terminal", target, StartOpts{
			Cwd:         root,
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-toolhost",
			Message: "exercise tool host",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "from-write") {
			t.Fatalf("Prompt() events = %#v, want written file content", events)
		}
		if !containsEventText(events, "terminal-ok") {
			t.Fatalf("Prompt() events = %#v, want terminal output", events)
		}
		content, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", target, err)
		}
		if string(content) != "from-write" {
			t.Fatalf("written file content = %q, want %q", content, "from-write")
		}
	})
}

func TestACPIntegrationRequestPermissionPolicy(t *testing.T) {
	t.Run("Should allow an interactive edit permission request", func(t *testing.T) {
		t.Parallel()

		driver := New()

		root := t.TempDir()
		target := filepath.Join(root, "danger.txt")
		proc := startHelperProcess(t, driver, "permission", target, StartOpts{
			Cwd:         root,
			Permissions: aghconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-permission",
			Message: "request permission",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := make([]AgentEvent, 0, 8)
		var pendingRequestID string
		timeout := time.NewTimer(5 * time.Second)
		defer timeout.Stop()

		for {
			select {
			case event, ok := <-eventsCh:
				if !ok {
					goto done
				}
				events = append(events, event)
				if event.Type == EventTypePermission && event.Decision == "" && pendingRequestID == "" {
					pendingRequestID = event.RequestID
					if pendingRequestID == "" {
						t.Fatal("permission request_id = empty, want non-empty")
					}
					if err := driver.ApprovePermission(testutil.Context(t), proc, ApproveRequest{
						RequestID: pendingRequestID,
						Decision:  string(decisionAllowAlways),
					}); err != nil {
						t.Fatalf("ApprovePermission() error = %v", err)
					}
				}
			case <-timeout.C:
				t.Fatalf("timed out waiting for prompt events; collected %#v", events)
			}
		}
	done:

		permissionEvents := make([]AgentEvent, 0, len(events))
		for _, event := range events {
			if event.Type == EventTypePermission {
				permissionEvents = append(permissionEvents, event)
			}
		}
		if len(permissionEvents) == 0 {
			t.Fatalf("Prompt() events = %#v, want permission event", events)
		}
		if permissionEvents[0].Decision != "" {
			t.Fatalf("initial permission event decision = %q, want empty", permissionEvents[0].Decision)
		}
		if permissionEvents[len(permissionEvents)-1].Decision != string(decisionAllowAlways) {
			t.Fatalf(
				"final permission event decision = %q, want %q",
				permissionEvents[len(permissionEvents)-1].Decision,
				decisionAllowAlways,
			)
		}
		if !containsEventText(events, "allow-always") {
			t.Fatalf("Prompt() events = %#v, want approved permission outcome", events)
		}
	})
}

func TestACPIntegrationRequestPermissionTimeout(t *testing.T) {
	t.Run("Should reject an interactive edit permission request after timeout", func(t *testing.T) {
		t.Parallel()

		driver := New(WithPermissionTimeout(25 * time.Millisecond))

		root := t.TempDir()
		target := filepath.Join(root, "danger.txt")
		proc := startHelperProcess(t, driver, "permission", target, StartOpts{
			Cwd:         root,
			Permissions: aghconfig.PermissionModeApproveReads,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-timeout",
			Message: "request permission",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "reject-once") {
			t.Fatalf("Prompt() events = %#v, want reject-once permission outcome", events)
		}

		permissionEvents := make([]AgentEvent, 0, len(events))
		for _, event := range events {
			if event.Type == EventTypePermission {
				permissionEvents = append(permissionEvents, event)
			}
		}
		if len(permissionEvents) < 2 {
			t.Fatalf("Prompt() permission events = %#v, want initial and final permission events", permissionEvents)
		}
		if permissionEvents[len(permissionEvents)-1].Decision != string(decisionRejectOnce) {
			t.Fatalf(
				"final permission event decision = %q, want %q",
				permissionEvents[len(permissionEvents)-1].Decision,
				decisionRejectOnce,
			)
		}
	})
}

func TestACPIntegrationNetworkTurnGuardrails(t *testing.T) {
	t.Run("Should allow only network-safe tools during network turns", func(t *testing.T) {
		driver := New()

		root := t.TempDir()
		target := filepath.Join(root, "network.txt")
		fakeAGH := filepath.Join(root, "agh")
		if err := os.WriteFile(fakeAGH, []byte("#!/bin/sh\nprintf network-ok\n"), 0o755); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", fakeAGH, err)
		}
		// not parallel: t.Setenv mutates process-wide PATH for the helper executable lookup.
		t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))

		proc := startHelperProcess(t, driver, "network_guardrails", target, StartOpts{
			Cwd:         root,
			Permissions: aghconfig.PermissionModeApproveAll,
		})
		proc.SetTurnSourceProvider(func() string { return "network" })
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-integration-network-guardrails",
			Message: "exercise network guardrails",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		if !containsEventText(events, "write_blocked") {
			t.Fatalf("Prompt() events = %#v, want blocked file write result", events)
		}
		if !containsEventText(events, "shell_blocked") {
			t.Fatalf("Prompt() events = %#v, want blocked shell-wrapper result", events)
		}
		if !containsEventText(events, "network-ok") {
			t.Fatalf("Prompt() events = %#v, want allowlisted agh network output", events)
		}
	})
}

func containsEventText(events []AgentEvent, want string) bool {
	for _, event := range events {
		if event.Text == want {
			return true
		}
	}
	return false
}
