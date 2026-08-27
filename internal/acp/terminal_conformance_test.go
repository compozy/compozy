package acp

// Suite: ACP terminal behavioral conformance.
// Invariant: the terminal adapters preserve the 64 KiB UTF-8-safe output window,
// polling lifecycle, permission gate, and network-turn ownership lockdown.
// Boundary IN: ACP terminal requests. Boundary OUT: shared terminal core.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestTerminalBehavioralConformance(t *testing.T) {
	t.Run("Should preserve the polled terminal lifecycle [IT-011]", assertTerminalLifecycleHandlers)
	t.Run("Should retain a UTF-8-safe 64 KiB output window [IT-011]", assertTerminalOutputWindow)
	t.Run("Should apply the terminal permission gate [IT-011]", assertTerminalPermissionGate)
	t.Run(
		"Should reject non-allowlisted commands during network turns [IT-011]",
		assertTerminalNetworkTurnRejectsNonAllowlistedCommands,
	)
	t.Run("Should enforce network-turn terminal ownership [IT-011]", assertNetworkTurnTerminalOwnershipGuards)
}

func assertTerminalOutputWindow(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, compozyconfig.PermissionModeApproveAll)
	run := func(payload string, limit *int) acpsdk.TerminalOutputResponse {
		create, err := proc.handleCreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
			SessionId:       "sess-direct",
			Command:         "printf",
			Args:            []string{"%s", payload},
			Cwd:             new(proc.Cwd),
			OutputByteLimit: limit,
		})
		if err != nil {
			t.Fatalf("handleCreateTerminal() error = %v", err)
		}
		if _, err := proc.handleWaitForTerminalExit(context.Background(), acpsdk.WaitForTerminalExitRequest{
			SessionId: "sess-direct", TerminalId: create.TerminalId,
		}); err != nil {
			t.Fatalf("handleWaitForTerminalExit() error = %v", err)
		}
		output, err := proc.handleTerminalOutput(acpsdk.TerminalOutputRequest{
			SessionId: "sess-direct", TerminalId: create.TerminalId,
		})
		if err != nil {
			t.Fatalf("handleTerminalOutput() error = %v", err)
		}
		return output
	}

	payload := "a€b" + strings.Repeat("x", defaultTerminalOutputLimit-3)
	output := run(payload, nil)
	if !output.Truncated {
		t.Fatal("TerminalOutput.Truncated = false, want true")
	}
	if !utf8.ValidString(output.Output) {
		t.Fatalf("TerminalOutput.Output begins inside a UTF-8 rune: %q", output.Output[:min(len(output.Output), 8)])
	}
	if got, want := len(output.Output), defaultTerminalOutputLimit-2; got != want {
		t.Fatalf("len(TerminalOutput.Output) = %d, want %d", got, want)
	}
	if !strings.HasPrefix(output.Output, "b") || !strings.HasSuffix(output.Output, "x") {
		t.Fatalf(
			"TerminalOutput.Output boundaries = %q…%q, want b…x",
			output.Output[:1],
			output.Output[len(output.Output)-1:],
		)
	}

	configured := run("a€b", new(3))
	if configured.Output != "b" || !configured.Truncated {
		t.Fatalf("configured TerminalOutput = %#v, want UTF-8-safe truncated b", configured)
	}
	zero := run("output", new(0))
	if zero.Output != "" || !zero.Truncated {
		t.Fatalf("zero-limit TerminalOutput = %#v, want empty truncated output", zero)
	}
}

func assertTerminalPermissionGate(t *testing.T) {
	t.Parallel()

	host, root := newTestLocalToolHost(t, compozyconfig.PermissionModeDenyAll)
	_, err := host.CreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
		SessionId: "sess-denied", Command: "printf", Args: []string{"denied"}, Cwd: new(root),
	})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("CreateTerminal(deny-all) error = %v, want ErrPermissionDenied", err)
	}
	host.terminals.mu.RLock()
	terminalCount := len(host.terminals.terminals)
	host.terminals.mu.RUnlock()
	if terminalCount != 0 {
		t.Fatalf("terminal count after permission rejection = %d, want 0", terminalCount)
	}
}
