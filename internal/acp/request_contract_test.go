package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
)

func TestACPBehaviorContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate authored identity and typed tool metadata in one optional payload", func(t *testing.T) {
		t.Parallel()

		input := json.RawMessage(`{"path":"README.md"}`)
		event := AgentEvent{}.
			WithClientMessageID(" client-message-1 ").
			WithToolDetail("read_file", input, true, "permission denied")
		input[2] = 'X'

		if got, want := event.ClientMessageIDValue(), "client-message-1"; got != want {
			t.Fatalf("ClientMessageIDValue() = %q, want %q", got, want)
		}
		if !event.HasToolPayload() || event.ToolName() != "read_file" || !event.ToolError() ||
			event.ToolErrorDetail() != "permission denied" {
			t.Fatalf(
				"typed tool payload = (%q, %t, %q), want preserved failure metadata",
				event.ToolName(),
				event.ToolError(),
				event.ToolErrorDetail(),
			)
		}
		if got, want := string(event.ToolInput()), `{"path":"README.md"}`; got != want {
			t.Fatalf("ToolInput() = %s, want %s", got, want)
		}

		clientOnly := event.WithToolDetail("", nil, false, "")
		if clientOnly.HasToolPayload() || clientOnly.ClientMessageIDValue() != "client-message-1" {
			t.Fatalf("cleared tool payload lost authored identity: %#v", clientOnly)
		}
		toolOnly := event.WithClientMessageID("")
		if toolOnly.ClientMessageIDValue() != "" || !toolOnly.HasToolPayload() {
			t.Fatalf("cleared authored identity lost tool payload: %#v", toolOnly)
		}
	})

	t.Run("Should classify load session resource missing from structured request data", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			data any
		}{
			{
				name: "details string",
				data: map[string]any{"details": "Resource not found: sess-dead"},
			},
			{
				name: "nested error message",
				data: map[string]any{"error": map[string]any{"message": "resource not found: sess-dead"}},
			},
		}

		for _, tc := range cases {
			t.Run("Should detect "+tc.name, func(t *testing.T) {
				t.Parallel()

				err := fmt.Errorf(
					"%w: load session %q for %q: %w",
					ErrLoadSessionFailed,
					"sess-dead",
					"helper",
					&acpsdk.RequestError{
						Code:    requestErrorResourceNotFoundCode,
						Message: "Internal error",
						Data:    tc.data,
					},
				)
				if !IsLoadSessionResourceMissing(err) {
					t.Fatalf("IsLoadSessionResourceMissing() = false, want true for data %#v", tc.data)
				}
			})
		}
	})

	t.Run("Should allow file callbacks under additional dirs advertised to the ACP agent", func(t *testing.T) {
		t.Parallel()

		driver := New()
		root := t.TempDir()
		additional := t.TempDir()
		target := filepath.Join(additional, "created.txt")

		proc := startHelperProcess(t, driver, "fs_write_terminal", target, StartOpts{
			Cwd:            root,
			AdditionalDirs: []string{additional},
			Permissions:    aghconfig.PermissionModeApproveAll,
		})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-contract-additional-dirs",
			Message: "exercise additional dir tool host",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		events := collectEvents(t, eventsCh)
		if !contractContainsEventText(events, "from-write") {
			t.Fatalf("Prompt() events = %#v, want additional-dir file content", events)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("os.Stat(%q) error = %v", target, err)
		}
	})

	t.Run("Should allow terminal cwd inside additional dirs advertised to the ACP agent", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		additional := t.TempDir()
		host := newContractLocalToolHost(t, root, additional)
		ctx := testutil.Context(t)

		response, err := host.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: "sess-additional-terminal",
			Command:   "sh",
			Args:      []string{"-c", "printf %s \"$PWD\""},
			Cwd:       new(additional),
		})
		if err != nil {
			t.Fatalf("CreateTerminal(additional cwd) error = %v", err)
		}
		if _, err := host.WaitForTerminalExit(ctx, response.TerminalId); err != nil {
			t.Fatalf("WaitForTerminalExit(additional cwd) error = %v", err)
		}
		output, err := host.TerminalOutput(response.TerminalId)
		if err != nil {
			t.Fatalf("TerminalOutput(additional cwd) error = %v", err)
		}
		wantCwd := mustCanonicalContractDir(t, additional)
		gotCwd := mustCanonicalContractDir(t, output)
		if gotCwd != wantCwd {
			t.Fatalf("TerminalOutput(additional cwd) = %q, want %q", output, additional)
		}
	})

	t.Run("Should resolve terminal command from request PATH", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		binDir := t.TempDir()
		writeExecutableScript(t, binDir, "agh-path-tool", "#!/bin/sh\nprintf path-env-ok")
		host := newContractLocalToolHost(t, root)
		ctx := testutil.Context(t)

		response, err := host.CreateTerminal(ctx, acpsdk.CreateTerminalRequest{
			SessionId: "sess-path",
			Command:   "agh-path-tool",
			Cwd:       new(root),
			Env: []acpsdk.EnvVariable{
				{Name: "PATH", Value: binDir},
			},
		})
		if err != nil {
			t.Fatalf("CreateTerminal() error = %v", err)
		}
		if _, err := host.WaitForTerminalExit(ctx, response.TerminalId); err != nil {
			t.Fatalf("WaitForTerminalExit() error = %v", err)
		}
		output, err := host.TerminalOutput(response.TerminalId)
		if err != nil {
			t.Fatalf("TerminalOutput() error = %v", err)
		}
		if output != "path-env-ok" {
			t.Fatalf("TerminalOutput() = %q, want path-env-ok", output)
		}
	})
}

func newContractLocalToolHost(t *testing.T, root string, additionalRoots ...string) *localToolHost {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	policy, err := newPermissionPolicy(aghconfig.PermissionModeApproveAll, root, additionalRoots...)
	if err != nil {
		t.Fatalf("newPermissionPolicy() error = %v", err)
	}
	return newLocalToolHostFromPolicy(ctx, root, policy, nil)
}

func writeExecutableScript(t *testing.T, dir string, name string, script string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}

func mustCanonicalContractDir(t *testing.T, dir string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", dir, err)
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", resolved, err)
	}
	return filepath.Clean(absolute)
}

func contractContainsEventText(events []AgentEvent, want string) bool {
	for _, event := range events {
		if event.Text == want {
			return true
		}
	}
	return false
}
