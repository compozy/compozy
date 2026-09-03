package acp

// Suite: ACP terminal behavioral conformance.
// Invariant: terminal adapters preserve output bounds, request cancellation,
// independent cleanup budgets, polling, permissions, and network-turn ownership.
// Boundary IN: ACP terminal requests. Boundary OUT: shared terminal core.

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
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
	t.Run("Should bound detached terminal lifecycle work", assertBoundedTerminalLifecycleContext)
	t.Run("Should reject canceled kill output and release before host access", assertCanceledTerminalRequests)
	t.Run("Should isolate every terminal cleanup and core shutdown budget", assertIndependentCloseAllBudgets)
}

func assertBoundedTerminalLifecycleContext(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithDeadline(
		t.Context(),
		time.Now().Add(defaultStopTimeout/2),
	)
	child, cancelChild := withoutCancelPreservingDeadline(parent)
	defer cancelParent()
	defer cancelChild()
	parentDeadline, parentBounded := parent.Deadline()
	childDeadline, childBounded := child.Deadline()
	if !parentBounded || !childBounded || !childDeadline.Equal(parentDeadline) {
		t.Fatalf(
			"bounded child deadline = %v/%t, want %v/%t",
			childDeadline,
			childBounded,
			parentDeadline,
			parentBounded,
		)
	}
	cancelParent()
	select {
	case <-child.Done():
		t.Fatalf("child context canceled by parent: %v", child.Err())
	default:
	}

	// Invariant: detached cleanup still receives a finite deadline when its caller supplies none.
	unboundedChild, cancelUnboundedChild := withoutCancelPreservingDeadline(context.WithoutCancel(t.Context()))
	defer cancelUnboundedChild()
	if _, ok := unboundedChild.Deadline(); !ok {
		t.Fatal("detached terminal lifecycle context has no deadline")
	}
}

func assertTerminalOutputWindow(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, compozyconfig.PermissionModeApproveAll)
	beginACPTestRun(t, proc, "turn-output-window")
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
		output, err := proc.handleTerminalOutput(t.Context(), acpsdk.TerminalOutputRequest{
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

func assertCanceledTerminalRequests(t *testing.T) {
	t.Parallel()

	probe := &canceledTerminalHostProbe{}
	proc := newDirectProcess(t, compozyconfig.PermissionModeApproveAll)
	proc.toolHost = probe
	cause := errors.New("RPC canceled")
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(cause)
	for _, request := range []struct {
		name   string
		method string
		params any
	}{
		{
			name: "Should reject kill after cancellation", method: acpsdk.ClientMethodTerminalKill,
			params: acpsdk.KillTerminalRequest{SessionId: "sess-direct", TerminalId: "term-canceled"},
		},
		{
			name: "Should reject output after cancellation", method: acpsdk.ClientMethodTerminalOutput,
			params: acpsdk.TerminalOutputRequest{SessionId: "sess-direct", TerminalId: "term-canceled"},
		},
		{
			name: "Should reject release after cancellation", method: acpsdk.ClientMethodTerminalRelease,
			params: acpsdk.ReleaseTerminalRequest{SessionId: "sess-direct", TerminalId: "term-canceled"},
		},
	} {
		t.Run(request.name, func(t *testing.T) {
			result, requestErr := proc.handleInbound(ctx, request.method, mustMarshalJSON(request.params))
			if result != nil || requestErr == nil {
				t.Fatalf("handleInbound(%s, canceled) = %#v error=%v", request.name, result, requestErr)
			}
		})
	}
	if probe.killCalls.Load() != 0 || probe.outputCalls.Load() != 0 || probe.releaseCalls.Load() != 0 {
		t.Fatalf(
			"canceled host calls = kill %d output %d release %d, want zero",
			probe.killCalls.Load(), probe.outputCalls.Load(), probe.releaseCalls.Load(),
		)
	}
}

func assertIndependentCloseAllBudgets(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		core := &closeAllTerminalCore{observed: make(chan cleanupContextState, 2)}
		lifecycle := context.WithValue(t.Context(), acpCleanupContextKey{}, "lifecycle-value")
		manager := &terminalManager{
			lifecycle: lifecycle,
			core:      core,
			terminals: map[string]*managedTerminal{
				"term-a": {
					handle: &terminalInfoHandle{info: terminalpkg.Info{
						ID: "term-a", WS: "workspace-a", ProfileID: "profile-a",
					}},
				},
				"term-b": {
					handle: &terminalInfoHandle{info: terminalpkg.Info{
						ID: "term-b", WS: "workspace-a", ProfileID: "profile-a",
					}},
				},
			},
		}
		manager.closeAll()
		first := <-core.observed
		second := <-core.observed
		if !errors.Is(first.err, context.DeadlineExceeded) || !first.hasDeadline {
			t.Fatalf("first terminal cleanup context = %#v, want exhausted bounded context", first)
		}
		for name, state := range map[string]cleanupContextState{"second terminal": second} {
			if state.err != nil || !state.hasDeadline || state.value != "lifecycle-value" {
				t.Fatalf("%s context = %#v, want active independent bounded context", name, state)
			}
		}
	})
}

type canceledTerminalHostProbe struct {
	ToolHost
	killCalls    atomic.Int32
	outputCalls  atomic.Int32
	releaseCalls atomic.Int32
}

func (p *canceledTerminalHostProbe) KillTerminal(string) error {
	p.killCalls.Add(1)
	return nil
}

func (p *canceledTerminalHostProbe) TerminalOutput(string) (string, error) {
	p.outputCalls.Add(1)
	return "unexpected", nil
}

func (p *canceledTerminalHostProbe) ReleaseTerminal(string) error {
	p.releaseCalls.Add(1)
	return nil
}

type cleanupContextState struct {
	err         error
	value       any
	hasDeadline bool
}

type acpCleanupContextKey struct{}

func observeACPContext(ctx context.Context) cleanupContextState {
	_, hasDeadline := ctx.Deadline()
	return cleanupContextState{
		err: ctx.Err(), value: ctx.Value(acpCleanupContextKey{}), hasDeadline: hasDeadline,
	}
}

type closeAllTerminalCore struct {
	TerminalHost
	calls    atomic.Int32
	observed chan cleanupContextState
}

func (c *closeAllTerminalCore) Release(
	ctx context.Context,
	_ string,
	_ string,
	_ terminalpkg.ID,
	_ terminalpkg.Actor,
) error {
	if c.calls.Add(1) == 1 {
		<-ctx.Done()
	}
	c.observed <- observeACPContext(ctx)
	return ctx.Err()
}

type terminalInfoHandle struct {
	terminalpkg.Handle
	info terminalpkg.Info
}

func (h *terminalInfoHandle) Info() terminalpkg.Info { return h.info }

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
