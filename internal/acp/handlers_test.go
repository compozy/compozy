package acp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/toolruntime"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDriverOptionsAndNormalization(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	driver := New(
		WithLogger(logger),
		WithStopTimeout(2*time.Second),
		WithPromptBufferSize(9),
		WithPromptDrainWait(15*time.Millisecond),
	)
	if driver.logger != logger {
		t.Fatal("WithLogger() did not apply")
	}
	if driver.stopTimeout != 2*time.Second {
		t.Fatalf("WithStopTimeout() = %v, want %v", driver.stopTimeout, 2*time.Second)
	}
	if driver.promptBufferCap != 9 {
		t.Fatalf("WithPromptBufferSize() = %d, want 9", driver.promptBufferCap)
	}
	if driver.promptDrainWait != 15*time.Millisecond {
		t.Fatalf("WithPromptDrainWait() = %v, want %v", driver.promptDrainWait, 15*time.Millisecond)
	}

	root := t.TempDir()
	normalized, err := normalizeStartOpts(StartOpts{
		AgentName:   "helper",
		Command:     "sh -c 'echo ok'",
		Cwd:         root,
		Permissions: "",
	})
	if err != nil {
		t.Fatalf("normalizeStartOpts() error = %v", err)
	}
	if normalized.Permissions != aghconfig.PermissionModeApproveReads {
		t.Fatalf(
			"normalizeStartOpts() permissions = %q, want %q",
			normalized.Permissions,
			aghconfig.PermissionModeApproveReads,
		)
	}

	rootReal := t.TempDir()
	rootLink := filepath.Join(t.TempDir(), "root-link")
	if err := os.Symlink(rootReal, rootLink); err != nil {
		t.Fatalf("os.Symlink(root) error = %v", err)
	}
	additionalReal := t.TempDir()
	additionalLink := filepath.Join(t.TempDir(), "additional-link")
	if err := os.Symlink(additionalReal, additionalLink); err != nil {
		t.Fatalf("os.Symlink(additional) error = %v", err)
	}

	normalizedWithDirs, err := normalizeStartOpts(StartOpts{
		AgentName:      "helper",
		Command:        "sh -c 'echo ok'",
		Cwd:            rootLink,
		AdditionalDirs: []string{additionalLink, rootReal, additionalLink, "   "},
	})
	if err != nil {
		t.Fatalf("normalizeStartOpts(with additional dirs) error = %v", err)
	}
	if got, want := normalizedWithDirs.Cwd, mustCanonicalDir(t, rootReal); got != want {
		t.Fatalf("normalizeStartOpts() cwd = %q, want %q", got, want)
	}
	if got, want := normalizedWithDirs.AdditionalDirs, []string{
		mustCanonicalDir(t, additionalReal),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("normalizeStartOpts() additional dirs = %#v, want %#v", got, want)
	}
}

func TestFallbackPermissionEventRaw(t *testing.T) {
	t.Parallel()

	raw := fallbackPermissionEventRaw("request-1", decisionRejectOnce)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(fallbackPermissionEventRaw()) error = %v", err)
	}
	if got := decoded["request_id"]; got != "request-1" {
		t.Fatalf("request_id = %v, want request-1", got)
	}
	if got := decoded["decision"]; got != string(decisionRejectOnce) {
		t.Fatalf("decision = %v, want %q", got, decisionRejectOnce)
	}

	pending := fallbackPermissionEventRaw("request-2", decisionPending)
	decoded = nil
	if err := json.Unmarshal(pending, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(pending fallbackPermissionEventRaw()) error = %v", err)
	}
	if _, ok := decoded["decision"]; ok {
		t.Fatalf("pending fallback included decision field: %#v", decoded)
	}
}

func TestWatchTerminalShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Should exit when terminal finishes first", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		terminalDone := make(chan struct{})
		shutdownCalls := make(chan struct{}, 1)
		watcherDone := watchTerminalShutdown(ctx, terminalDone, func() {
			shutdownCalls <- struct{}{}
		})

		close(terminalDone)

		select {
		case <-watcherDone:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("watchTerminalShutdown() did not exit after terminal completion")
		}

		cancel()
		select {
		case <-shutdownCalls:
			t.Fatal("watchTerminalShutdown() called shutdown after terminal completion")
		case <-time.After(20 * time.Millisecond):
		}
	})

	t.Run("Should run shutdown when manager context cancels first", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		terminalDone := make(chan struct{})
		shutdownCalls := make(chan struct{}, 1)
		watcherDone := watchTerminalShutdown(ctx, terminalDone, func() {
			shutdownCalls <- struct{}{}
		})

		cancel()

		select {
		case <-shutdownCalls:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("watchTerminalShutdown() did not run shutdown callback after context cancellation")
		}

		select {
		case <-watcherDone:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("watchTerminalShutdown() did not exit after context cancellation")
		}
	})
}

func TestNormalizeStartOptsRejectsInvalidAdditionalDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	missing := filepath.Join(root, "missing")

	if err := (StartOpts{
		AgentName:      "helper",
		Command:        "sh -c 'echo ok'",
		Cwd:            root,
		AdditionalDirs: []string{"relative/path"},
	}).Validate(); err == nil {
		t.Fatal("StartOpts.Validate(relative additional dir) error = nil, want non-nil")
	}

	if _, err := normalizeStartOpts(StartOpts{
		AgentName:      "helper",
		Command:        "sh -c 'echo ok'",
		Cwd:            root,
		AdditionalDirs: []string{missing},
	}); err == nil {
		t.Fatal("normalizeStartOpts(missing additional dir) error = nil, want non-nil")
	}
}

func TestHandleInboundReadWriteFile(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	target := filepath.Join(proc.Cwd, "notes.txt")

	if _, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodFsWriteTextFile,
		mustMarshalJSON(acpsdk.WriteTextFileRequest{
			SessionId: "sess-direct",
			Path:      target,
			Content:   "line1\nline2\nline3",
		}),
	); reqErr != nil {
		t.Fatalf("handleInbound(write) error = %v", reqErr)
	}

	response, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodFsReadTextFile,
		mustMarshalJSON(acpsdk.ReadTextFileRequest{
			SessionId: "sess-direct",
			Path:      target,
			Line:      new(2),
			Limit:     new(1),
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(read) error = %v", reqErr)
	}
	readResponse, ok := response.(acpsdk.ReadTextFileResponse)
	if !ok {
		t.Fatalf("handleInbound(read) type = %T, want ReadTextFileResponse", response)
	}
	if readResponse.Content != "line2" {
		t.Fatalf("handleInbound(read) content = %q, want %q", readResponse.Content, "line2")
	}
}

func TestHandleInboundWriteDenied(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveReads)
	target := filepath.Join(proc.Cwd, "notes.txt")

	if _, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodFsWriteTextFile,
		mustMarshalJSON(acpsdk.WriteTextFileRequest{
			SessionId: "sess-direct",
			Path:      target,
			Content:   "nope",
		}),
	); reqErr == nil {
		t.Fatal("handleInbound(write denied) error = nil, want non-nil")
	}
}

func TestHandleWriteTextFileBlockedForNetworkTurn(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	proc.SetTurnSourceProvider(func() string { return "network" })

	active, err := proc.beginPrompt("turn-network-write", 4)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	target := filepath.Join(proc.Cwd, "network.txt")
	if _, err := proc.handleWriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{
		SessionId: "sess-direct",
		Path:      target,
		Content:   "blocked",
	}); !errors.Is(err, ErrToolBlockedForNetworkTurn) {
		t.Fatalf("handleWriteTextFile(network turn) error = %v, want ErrToolBlockedForNetworkTurn", err)
	}
}

func TestHandleCreateTerminalBlocksNonAllowlistedCommandsForNetworkTurn(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	proc.SetTurnSourceProvider(func() string { return "network" })

	active, err := proc.beginPrompt("turn-network-create", 4)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	tests := []struct {
		name    string
		request acpsdk.CreateTerminalRequest
	}{
		{
			name: "shell wrapper",
			request: acpsdk.CreateTerminalRequest{
				SessionId: "sess-direct",
				Command:   "sh",
				Args:      []string{"-c", "printf nope"},
				Cwd:       new(proc.Cwd),
			},
		},
		{
			name: "non-network agh subcommand",
			request: acpsdk.CreateTerminalRequest{
				SessionId: "sess-direct",
				Command:   "agh",
				Args:      []string{"version"},
				Cwd:       new(proc.Cwd),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := proc.handleCreateTerminal(
				context.Background(),
				tt.request,
			); !errors.Is(
				err,
				ErrToolBlockedForNetworkTurn,
			) {
				t.Fatalf("handleCreateTerminal(%s) error = %v, want ErrToolBlockedForNetworkTurn", tt.name, err)
			}
		})
	}
}

func TestHandleWriteTextFileDeniedByToolGatewayPreventsSideEffect(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	proc.toolGateway = toolExecutionGatewayFunc(
		func(context.Context, ToolExecutionRequest) (ToolExecutionRequest, error) {
			return ToolExecutionRequest{}, ErrPermissionDenied
		},
	)

	target := filepath.Join(proc.Cwd, "denied.txt")
	if _, err := proc.handleWriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{
		SessionId: "sess-direct",
		Path:      target,
		Content:   "blocked",
	}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("handleWriteTextFile() error = %v, want ErrPermissionDenied", err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", target, err)
	}
}

func TestHandleWriteTextFileAppliesToolGatewayPatch(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	patchedPath := filepath.Join(proc.Cwd, "patched.txt")
	proc.toolGateway = toolExecutionGatewayFunc(
		func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
			req.Input = mustMarshalJSON(writeTextFileToolInput{
				Path:    patchedPath,
				Content: "patched",
			})
			return req, nil
		},
	)

	originalPath := filepath.Join(proc.Cwd, "original.txt")
	if _, err := proc.handleWriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{
		SessionId: "sess-direct",
		Path:      originalPath,
		Content:   "original",
	}); err != nil {
		t.Fatalf("handleWriteTextFile() error = %v", err)
	}

	if _, err := os.Stat(originalPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(%q) error = %v, want os.ErrNotExist", originalPath, err)
	}
	got, err := os.ReadFile(patchedPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", patchedPath, err)
	}
	if string(got) != "patched" {
		t.Fatalf("patched file content = %q, want %q", string(got), "patched")
	}
}

func TestHandleInboundPermissionRequest(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveReads)
	proc.permissionTimeout = time.Second
	active, err := proc.beginPrompt("turn-permission", 8)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	title := "permission request"
	path := filepath.Join(proc.Cwd, "secret.txt")
	kind := acpsdk.ToolKindEdit
	request := acpsdk.RequestPermissionRequest{
		SessionId: "sess-direct",
		Options: []acpsdk.PermissionOption{
			{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
			{OptionId: "allow-always", Name: "allow always", Kind: acpsdk.PermissionOptionKindAllowAlways},
			{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
			{OptionId: "reject-always", Name: "reject always", Kind: acpsdk.PermissionOptionKindRejectAlways},
		},
		ToolCall: acpsdk.ToolCallUpdate{
			ToolCallId: "tool-1",
			Title:      &title,
			Kind:       &kind,
			RawInput: map[string]any{
				"command": "rm -rf /tmp/demo",
			},
			Locations: []acpsdk.ToolCallLocation{{Path: path}},
		},
	}
	resultCh := make(chan acpsdk.RequestPermissionResponse, 1)
	errCh := make(chan *acpsdk.RequestError, 1)
	go func() {
		response, reqErr := proc.handleInbound(
			context.Background(),
			acpsdk.ClientMethodSessionRequestPermission,
			mustMarshalJSON(request),
		)
		if reqErr != nil {
			errCh <- reqErr
			return
		}
		permissionResponse, ok := response.(acpsdk.RequestPermissionResponse)
		if !ok {
			errCh <- requestError(errors.New("unexpected permission response type"))
			return
		}
		resultCh <- permissionResponse
	}()

	initialEvents := collectEventsUntilCount(t, active.events, 1)
	if len(initialEvents) != 1 || initialEvents[0].Type != EventTypePermission {
		t.Fatalf("initial permission events = %#v, want one permission event", initialEvents)
	}
	if initialEvents[0].Decision != "" {
		t.Fatalf("initial permission decision = %q, want empty", initialEvents[0].Decision)
	}
	if initialEvents[0].RequestID == "" {
		t.Fatal("initial permission request_id = empty, want non-empty")
	}

	raw := decodePermissionEventRaw(t, initialEvents[0].Raw)
	if raw.RequestID != initialEvents[0].RequestID {
		t.Fatalf("raw.request_id = %q, want %q", raw.RequestID, initialEvents[0].RequestID)
	}
	if len(raw.Options) != 4 {
		t.Fatalf("raw.options = %#v, want 4 permission options", raw.Options)
	}
	if got := raw.ToolInput["command"]; got != "rm -rf /tmp/demo" {
		t.Fatalf("raw.tool_input.command = %#v, want %q", got, "rm -rf /tmp/demo")
	}

	if err := proc.ResolvePermission(ApproveRequest{
		RequestID: initialEvents[0].RequestID,
		Decision:  string(decisionAllowAlways),
	}); err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}

	select {
	case reqErr := <-errCh:
		t.Fatalf("handleInbound(permission) error = %v", reqErr)
	case permissionResponse := <-resultCh:
		if permissionResponse.Outcome.Selected == nil ||
			permissionResponse.Outcome.Selected.OptionId != "allow-always" {
			t.Fatalf("permission outcome = %#v, want allow-always option", permissionResponse.Outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for permission response")
	}

	finalEvents := collectEventsUntilCount(t, active.events, 1)
	if len(finalEvents) != 1 || finalEvents[0].Decision != string(decisionAllowAlways) {
		t.Fatalf("final permission events = %#v, want allow-always decision", finalEvents)
	}
}

func TestResolvePermissionUnknownRequest(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
	err := proc.ResolvePermission(ApproveRequest{
		RequestID: "missing",
		Decision:  string(decisionAllowOnce),
	})
	if !errors.Is(err, ErrPendingPermissionNotFound) {
		t.Fatalf("ResolvePermission(missing) error = %v, want ErrPendingPermissionNotFound", err)
	}
}

func TestResolvePermissionRejectsUnsupportedPersistentDecision(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsupported persistent decision and keep pending request", func(t *testing.T) {
		t.Parallel()

		proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
		requestID, pending := proc.registerPendingPermission("turn-unsupported", acpsdk.RequestPermissionRequest{
			Options: []acpsdk.PermissionOption{
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{ToolCallId: "tool-unsupported"},
		})
		t.Cleanup(func() {
			proc.clearPendingPermission(requestID)
		})

		err := proc.ResolvePermission(ApproveRequest{
			RequestID: requestID,
			Decision:  string(decisionRejectAlways),
		})
		if !errors.Is(err, ErrPermissionDecisionUnsupported) {
			t.Fatalf("ResolvePermission() error = %v, want ErrPermissionDecisionUnsupported", err)
		}
		select {
		case decision := <-pending.response:
			t.Fatalf("pending response = %q, want none", decision)
		default:
		}
		if _, ok := proc.pendingPermissions[requestID]; !ok {
			t.Fatal("pending permission was removed after unsupported decision")
		}
	})
}

func TestHandleInboundPermissionRequestTimeout(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveReads)
	proc.permissionTimeout = 25 * time.Millisecond
	active, err := proc.beginPrompt("turn-timeout", 8)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	title := "permission request"
	kind := acpsdk.ToolKindEdit
	response, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodSessionRequestPermission,
		mustMarshalJSON(acpsdk.RequestPermissionRequest{
			SessionId: "sess-direct",
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: "tool-timeout",
				Title:      &title,
				Kind:       &kind,
			},
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(permission timeout) error = %v", reqErr)
	}

	permissionResponse, ok := response.(acpsdk.RequestPermissionResponse)
	if !ok {
		t.Fatalf("handleInbound(permission timeout) type = %T, want RequestPermissionResponse", response)
	}
	if permissionResponse.Outcome.Selected == nil || permissionResponse.Outcome.Selected.OptionId != "reject-once" {
		t.Fatalf("permission timeout outcome = %#v, want reject-once option", permissionResponse.Outcome)
	}

	events := collectEventsUntilCount(t, active.events, 2)
	if events[0].Decision != "" {
		t.Fatalf("initial timeout decision = %q, want empty", events[0].Decision)
	}
	if events[1].Decision != string(decisionRejectOnce) {
		t.Fatalf("final timeout decision = %q, want %q", events[1].Decision, decisionRejectOnce)
	}
}

func TestHandleInboundPermissionRequestHonorsDenyAllWithToolGateway(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
	proc.toolGateway = toolExecutionGatewayFunc(
		func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
			return req, nil
		},
	)
	active, err := proc.beginPrompt("turn-deny-all", 8)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	title := "Write"
	kind := acpsdk.ToolKindEdit
	response, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodSessionRequestPermission,
		mustMarshalJSON(acpsdk.RequestPermissionRequest{
			SessionId: "sess-direct",
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: "tool-deny-all",
				Title:      &title,
				Kind:       &kind,
				RawInput: map[string]any{
					"path":    filepath.Join(proc.Cwd, "deny-all.txt"),
					"content": "blocked",
				},
				Locations: []acpsdk.ToolCallLocation{{Path: filepath.Join(proc.Cwd, "deny-all.txt")}},
			},
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(permission deny-all) error = %v", reqErr)
	}

	permissionResponse, ok := response.(acpsdk.RequestPermissionResponse)
	if !ok {
		t.Fatalf("handleInbound(permission deny-all) type = %T, want RequestPermissionResponse", response)
	}
	if permissionResponse.Outcome.Selected == nil || permissionResponse.Outcome.Selected.OptionId != "reject-once" {
		t.Fatalf("permission deny-all outcome = %#v, want reject-once option", permissionResponse.Outcome)
	}

	events := collectEventsUntilCount(t, active.events, 1)
	if len(events) != 1 || events[0].Decision != string(decisionRejectOnce) {
		t.Fatalf("permission deny-all events = %#v, want one reject-once event", events)
	}
}

func TestHandleInboundPermissionRequestHonorsApproveAllWithToolGateway(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	proc.toolGateway = toolExecutionGatewayFunc(
		func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
			return req, nil
		},
	)
	active, err := proc.beginPrompt("turn-approve-all", 8)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	title := "Write"
	kind := acpsdk.ToolKindEdit
	response, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodSessionRequestPermission,
		mustMarshalJSON(acpsdk.RequestPermissionRequest{
			SessionId: "sess-direct",
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: "tool-approve-all",
				Title:      &title,
				Kind:       &kind,
				RawInput: map[string]any{
					"path":    filepath.Join(proc.Cwd, "approve-all.txt"),
					"content": "allowed",
				},
				Locations: []acpsdk.ToolCallLocation{{Path: filepath.Join(proc.Cwd, "approve-all.txt")}},
			},
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(permission approve-all) error = %v", reqErr)
	}

	permissionResponse, ok := response.(acpsdk.RequestPermissionResponse)
	if !ok {
		t.Fatalf("handleInbound(permission approve-all) type = %T, want RequestPermissionResponse", response)
	}
	if permissionResponse.Outcome.Selected == nil || permissionResponse.Outcome.Selected.OptionId != "allow-once" {
		t.Fatalf("permission approve-all outcome = %#v, want allow-once option", permissionResponse.Outcome)
	}

	events := collectEventsUntilCount(t, active.events, 1)
	if len(events) != 1 || events[0].Decision != string(decisionAllowOnce) {
		t.Fatalf("permission approve-all events = %#v, want one allow-once event", events)
	}
}

func TestEmitPermissionEvent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		decision permissionDecision
	}{
		{name: "ShouldHandleInteractivePending", decision: ""},
		{name: "ShouldAllowOnceAutomatically", decision: decisionAllowOnce},
		{name: "ShouldRejectOnceOnTimeout", decision: decisionRejectOnce},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
			active, err := proc.beginPrompt("turn-permission-event", 4)
			if err != nil {
				t.Fatalf("beginPrompt() error = %v", err)
			}
			defer proc.endPrompt(active)

			raw := mustMarshalJSON(map[string]any{"decision": string(tt.decision), "value": "original"})
			wantRaw := append(json.RawMessage(nil), raw...)

			proc.emitPermissionEvent(
				"sess-emit",
				"turn-permission-event",
				"req-1",
				"permission request",
				"tool-1",
				"/tmp/demo.txt",
				tt.decision,
				raw,
			)
			event := collectEventsUntilCount(t, active.events, 1)[0]

			raw[0] = '['

			if event.Type != EventTypePermission {
				t.Fatalf("event.Type = %q, want %q", event.Type, EventTypePermission)
			}
			if event.SessionID != "sess-emit" || event.TurnID != "turn-permission-event" || event.RequestID != "req-1" {
				t.Fatalf("event ids = %#v, want session/turn/request populated", event)
			}
			if event.Title != "permission request" || event.ToolCallID != "tool-1" {
				t.Fatalf("event title/tool = %#v, want copied fields", event)
			}
			if event.Action != string(permissionRequestToolGrant) || event.Resource != "/tmp/demo.txt" {
				t.Fatalf("event action/resource = %#v, want permission action/resource", event)
			}
			if event.Decision != string(tt.decision) {
				t.Fatalf("event.Decision = %q, want %q", event.Decision, tt.decision)
			}
			if event.Timestamp.IsZero() {
				t.Fatal("event.Timestamp = zero, want populated")
			}
			if string(event.Raw) != string(wantRaw) {
				t.Fatalf("event.Raw = %s, want %s", string(event.Raw), string(wantRaw))
			}
		})
	}
}

func TestResolvePermissionByTurnIDConflictsWhenMultipleRequestsPending(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
	turnID := "turn-conflict"
	_, first := proc.registerPendingPermission(turnID, acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{ToolCallId: "tool-1"},
	})
	_, second := proc.registerPendingPermission(turnID, acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{ToolCallId: "tool-2"},
	})
	t.Cleanup(func() {
		proc.clearPendingPermission(first.requestID)
		proc.clearPendingPermission(second.requestID)
	})

	err := proc.ResolvePermission(ApproveRequest{
		TurnID:   turnID,
		Decision: string(decisionRejectOnce),
	})
	if !errors.Is(err, ErrPendingPermissionConflict) {
		t.Fatalf("ResolvePermission(turn conflict) error = %v, want ErrPendingPermissionConflict", err)
	}
}

func TestResolvePermissionConcurrentSafety(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)

	const total = 8
	type registered struct {
		requestID string
		response  chan permissionDecision
	}

	registeredPending := make([]registered, 0, total)
	for i := range total {
		requestID, pending := proc.registerPendingPermission(
			fmt.Sprintf("turn-%d", i),
			acpsdk.RequestPermissionRequest{
				ToolCall: acpsdk.ToolCallUpdate{ToolCallId: acpsdk.ToolCallId(fmt.Sprintf("tool-%d", i))},
			},
		)
		registeredPending = append(registeredPending, registered{
			requestID: requestID,
			response:  pending.response,
		})
	}

	var wg sync.WaitGroup
	for _, pending := range registeredPending {
		wg.Go(func() {
			if err := proc.ResolvePermission(ApproveRequest{
				RequestID: pending.requestID,
				Decision:  string(decisionAllowOnce),
			}); err != nil {
				t.Errorf("ResolvePermission(%q) error = %v", pending.requestID, err)
			}
		})
	}
	wg.Wait()

	for _, pending := range registeredPending {
		select {
		case decision := <-pending.response:
			if decision != decisionAllowOnce {
				t.Fatalf("pending response = %q, want %q", decision, decisionAllowOnce)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for pending response %q", pending.requestID)
		}
	}
	if len(proc.pendingPermissions) != 0 {
		t.Fatalf("pendingPermissions = %#v, want empty", proc.pendingPermissions)
	}
}

func TestHandleInboundPermissionRequestAutoApprovesReadRequests(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveReads)
	active, err := proc.beginPrompt("turn-read", 8)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	title := "read file"
	kind := acpsdk.ToolKindRead
	response, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodSessionRequestPermission,
		mustMarshalJSON(acpsdk.RequestPermissionRequest{
			SessionId: "sess-direct",
			Options: []acpsdk.PermissionOption{
				{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
				{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
			},
			ToolCall: acpsdk.ToolCallUpdate{
				ToolCallId: "tool-read",
				Title:      &title,
				Kind:       &kind,
				Locations:  []acpsdk.ToolCallLocation{{Path: filepath.Join(proc.Cwd, "notes.txt")}},
			},
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(read permission) error = %v", reqErr)
	}

	permissionResponse, ok := response.(acpsdk.RequestPermissionResponse)
	if !ok {
		t.Fatalf("handleInbound(read permission) type = %T, want RequestPermissionResponse", response)
	}
	if permissionResponse.Outcome.Selected == nil || permissionResponse.Outcome.Selected.OptionId != "allow-once" {
		t.Fatalf("permission outcome = %#v, want reject option", permissionResponse.Outcome)
	}

	events := collectEventsUntilCount(t, active.events, 1)
	if len(events) != 1 || events[0].Decision != string(decisionAllowOnce) {
		t.Fatalf("permission events = %#v, want allow-once permission event", events)
	}
}

func TestTerminalLifecycleHandlers(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)

	createResult, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodTerminalCreate,
		mustMarshalJSON(acpsdk.CreateTerminalRequest{
			SessionId: "sess-direct",
			Command:   "sh",
			Args:      []string{"-c", "printf hi"},
			Cwd:       new(proc.Cwd),
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(create terminal) error = %v", reqErr)
	}
	createResponse, ok := createResult.(acpsdk.CreateTerminalResponse)
	if !ok {
		t.Fatalf("handleInbound(create terminal) type = %T, want CreateTerminalResponse", createResult)
	}

	waitResult, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodTerminalWaitForExit,
		mustMarshalJSON(acpsdk.WaitForTerminalExitRequest{
			SessionId:  "sess-direct",
			TerminalId: createResponse.TerminalId,
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(wait terminal) error = %v", reqErr)
	}
	waitResponse, ok := waitResult.(acpsdk.WaitForTerminalExitResponse)
	if !ok {
		t.Fatalf("handleInbound(wait terminal) type = %T, want WaitForTerminalExitResponse", waitResult)
	}
	if waitResponse.ExitCode == nil || *waitResponse.ExitCode != 0 {
		t.Fatalf("handleInbound(wait terminal) exit code = %#v, want 0", waitResponse.ExitCode)
	}

	outputResult, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodTerminalOutput,
		mustMarshalJSON(acpsdk.TerminalOutputRequest{
			SessionId:  "sess-direct",
			TerminalId: createResponse.TerminalId,
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(output terminal) error = %v", reqErr)
	}
	outputResponse, ok := outputResult.(acpsdk.TerminalOutputResponse)
	if !ok {
		t.Fatalf("handleInbound(output terminal) type = %T, want TerminalOutputResponse", outputResult)
	}
	if outputResponse.Output != "hi" {
		t.Fatalf("handleInbound(output terminal) output = %q, want %q", outputResponse.Output, "hi")
	}

	if _, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodTerminalKill,
		mustMarshalJSON(acpsdk.KillTerminalRequest{
			SessionId:  "sess-direct",
			TerminalId: createResponse.TerminalId,
		}),
	); reqErr != nil {
		t.Fatalf("handleInbound(kill terminal) error = %v", reqErr)
	}

	if _, reqErr := proc.handleInbound(
		context.Background(),
		acpsdk.ClientMethodTerminalRelease,
		mustMarshalJSON(acpsdk.ReleaseTerminalRequest{
			SessionId:  "sess-direct",
			TerminalId: createResponse.TerminalId,
		}),
	); reqErr != nil {
		t.Fatalf("handleInbound(release terminal) error = %v", reqErr)
	}

	if _, _, _, err := proc.terminals.output(createResponse.TerminalId); err == nil {
		t.Fatal("output(released terminal) error = nil, want terminal not found")
	}
}

func TestNetworkTurnTerminalOwnershipGuards(t *testing.T) {
	// This test mutates PATH with t.Setenv, so it must stay process-serial.
	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	turnSource := ""
	proc.SetTurnSourceProvider(func() string { return turnSource })

	aghDir := t.TempDir()
	writeFakeAGHBinary(t, aghDir, "printf network-ok")
	t.Setenv("PATH", aghDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("OPENAI_API_KEY", "sk-network-secret")

	turnSource = "network"
	firstTurn, err := proc.beginPrompt("turn-network-1", 4)
	if err != nil {
		t.Fatalf("beginPrompt(first network) error = %v", err)
	}

	networkCreate, err := proc.handleCreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
		SessionId: "sess-direct",
		Command:   "agh",
		Args:      []string{"network", "status"},
		Cwd:       new(proc.Cwd),
		Env: []acpsdk.EnvVariable{
			{Name: "AGH_HOME", Value: "/tmp/redirected"},
			{Name: "SAFE_NETWORK_OVERRIDE", Value: "blocked"},
		},
	})
	if err != nil {
		t.Fatalf("handleCreateTerminal(allowlisted network command) error = %v", err)
	}

	networkTerm, err := proc.terminals.lookup(networkCreate.TerminalId)
	if err != nil {
		t.Fatalf("lookup(network terminal) error = %v", err)
	}
	if !networkTerm.networkOwned {
		t.Fatal("network terminal networkOwned = false, want true")
	}
	if networkTerm.ownerTurnID != "turn-network-1" {
		t.Fatalf("network terminal ownerTurnID = %q, want %q", networkTerm.ownerTurnID, "turn-network-1")
	}
	if got, ok := envValue(networkTerm.cmd.Env, "OPENAI_API_KEY"); ok {
		t.Fatalf("network terminal OPENAI_API_KEY = %q, want filtered", got)
	}
	if got, ok := envValue(networkTerm.cmd.Env, "AGH_HOME"); ok {
		t.Fatalf("network terminal AGH_HOME = %q, want request env ignored", got)
	}
	if got, ok := envValue(networkTerm.cmd.Env, "SAFE_NETWORK_OVERRIDE"); ok {
		t.Fatalf("network terminal SAFE_NETWORK_OVERRIDE = %q, want request env ignored", got)
	}

	if _, err := proc.handleWaitForTerminalExit(context.Background(), acpsdk.WaitForTerminalExitRequest{
		SessionId:  "sess-direct",
		TerminalId: networkCreate.TerminalId,
	}); err != nil {
		t.Fatalf("handleWaitForTerminalExit(same network turn) error = %v", err)
	}

	networkOutput, err := proc.handleTerminalOutput(acpsdk.TerminalOutputRequest{
		SessionId:  "sess-direct",
		TerminalId: networkCreate.TerminalId,
	})
	if err != nil {
		t.Fatalf("handleTerminalOutput(same network turn) error = %v", err)
	}
	if networkOutput.Output != "network-ok" {
		t.Fatalf("network terminal output = %q, want %q", networkOutput.Output, "network-ok")
	}

	proc.endPrompt(firstTurn)

	turnSource = "user"
	userTurn, err := proc.beginPrompt("turn-user-1", 4)
	if err != nil {
		t.Fatalf("beginPrompt(user) error = %v", err)
	}

	userCreate, err := proc.handleCreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
		SessionId: "sess-direct",
		Command:   "sh",
		Args:      []string{"-c", "sleep 5"},
		Cwd:       new(proc.Cwd),
	})
	if err != nil {
		t.Fatalf("handleCreateTerminal(user shell) error = %v", err)
	}

	proc.endPrompt(userTurn)

	turnSource = "network"
	secondTurn, err := proc.beginPrompt("turn-network-2", 4)
	if err != nil {
		t.Fatalf("beginPrompt(second network) error = %v", err)
	}
	defer proc.endPrompt(secondTurn)

	if _, err := proc.handleTerminalOutput(acpsdk.TerminalOutputRequest{
		SessionId:  "sess-direct",
		TerminalId: networkCreate.TerminalId,
	}); !errors.Is(err, ErrToolBlockedForNetworkTurn) {
		t.Fatalf("handleTerminalOutput(previous network turn) error = %v, want ErrToolBlockedForNetworkTurn", err)
	}

	if _, err := proc.handleWaitForTerminalExit(context.Background(), acpsdk.WaitForTerminalExitRequest{
		SessionId:  "sess-direct",
		TerminalId: networkCreate.TerminalId,
	}); !errors.Is(err, ErrToolBlockedForNetworkTurn) {
		t.Fatalf("handleWaitForTerminalExit(previous network turn) error = %v, want ErrToolBlockedForNetworkTurn", err)
	}

	if _, err := proc.handleKillTerminal(acpsdk.KillTerminalRequest{
		SessionId:  "sess-direct",
		TerminalId: networkCreate.TerminalId,
	}); err != nil {
		t.Fatalf("handleKillTerminal(network-owned) error = %v", err)
	}

	if _, err := proc.handleReleaseTerminal(acpsdk.ReleaseTerminalRequest{
		SessionId:  "sess-direct",
		TerminalId: networkCreate.TerminalId,
	}); err != nil {
		t.Fatalf("handleReleaseTerminal(network-owned) error = %v", err)
	}

	checks := []struct {
		name string
		run  func() error
	}{
		{
			name: "output user terminal",
			run: func() error {
				_, err := proc.handleTerminalOutput(acpsdk.TerminalOutputRequest{
					SessionId:  "sess-direct",
					TerminalId: userCreate.TerminalId,
				})
				return err
			},
		},
		{
			name: "wait user terminal",
			run: func() error {
				_, err := proc.handleWaitForTerminalExit(context.Background(), acpsdk.WaitForTerminalExitRequest{
					SessionId:  "sess-direct",
					TerminalId: userCreate.TerminalId,
				})
				return err
			},
		},
		{
			name: "kill user terminal",
			run: func() error {
				_, err := proc.handleKillTerminal(acpsdk.KillTerminalRequest{
					SessionId:  "sess-direct",
					TerminalId: userCreate.TerminalId,
				})
				return err
			},
		},
		{
			name: "release user terminal",
			run: func() error {
				_, err := proc.handleReleaseTerminal(acpsdk.ReleaseTerminalRequest{
					SessionId:  "sess-direct",
					TerminalId: userCreate.TerminalId,
				})
				return err
			},
		},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); !errors.Is(err, ErrToolBlockedForNetworkTurn) {
				t.Fatalf("%s error = %v, want ErrToolBlockedForNetworkTurn", check.name, err)
			}
		})
	}
}

func TestHelperUtilities(t *testing.T) {
	t.Parallel()

	if got := attachStderr(errors.New("boom"), " stderr "); got == nil || got.Error() == "boom" {
		t.Fatalf("attachStderr() = %v, want wrapped stderr", got)
	}
	if got := attachStderr(errors.New("boom"), ""); got == nil || got.Error() != "boom" {
		t.Fatalf("attachStderr(empty) = %v, want original error", got)
	}

	env := mergeCommandEnv(
		[]string{"A=1", "B=2"},
		[]acpsdk.EnvVariable{{Name: "B", Value: "3"}, {Name: "C", Value: "4"}},
	)
	if len(env) != 3 || env[1] != "B=3" || env[2] != "C=4" {
		t.Fatalf("mergeCommandEnv() = %#v, want overridden env", env)
	}

	servers := toSDKMCPServers([]aghconfig.MCPServer{{
		Name:    "github",
		Command: "npx",
		Args:    []string{"-y", "mcp-server"},
		Env: map[string]string{
			"Z_VAR": "z",
			"A_VAR": "a",
		},
	}})
	if len(servers) != 1 || servers[0].Stdio == nil || len(servers[0].Stdio.Env) != 2 {
		t.Fatalf("toSDKMCPServers() = %#v, want one stdio server with env", servers)
	}
	if servers[0].Stdio.Env[0].Name != "A_VAR" {
		t.Fatalf("toSDKMCPServers() env order = %#v, want sorted env keys", servers[0].Stdio.Env)
	}

	if got := extractContentText(
		acpsdk.ResourceLinkBlock("doc", "file:///tmp/demo.txt"),
	); got != "file:///tmp/demo.txt" {
		t.Fatalf("extractContentText(resource_link) = %q", got)
	}

	trimmed := trimUTF8LeadingBytes([]byte{0xff, 'h', 'i'})
	if string(trimmed) != "hi" {
		t.Fatalf("trimUTF8LeadingBytes() = %q, want %q", string(trimmed), "hi")
	}

	if sliceLines("a\nb\nc", new(2), new(2)) != "b\nc" {
		t.Fatalf("sliceLines() returned unexpected content")
	}

	raw := mustMarshalJSON(map[string]string{"hello": "world"})
	if string(CloneRawMessage(raw)) != string(raw) {
		t.Fatalf("CloneRawMessage() = %q, want %q", string(CloneRawMessage(raw)), string(raw))
	}

	if requestError(ErrPermissionDenied) == nil {
		t.Fatal("requestError(permission) = nil, want request error")
	}
	if requestError(errors.New("boom")) == nil {
		t.Fatal("requestError(internal) = nil, want request error")
	}
	t.Run("Should preserve MCP auth reason codes in request error data", func(t *testing.T) {
		t.Parallel()

		toolReqErr := requestError(toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			toolspkg.ToolID("mcp/github/search"),
			"MCP login required",
			errors.New("auth unavailable"),
			toolspkg.ReasonMCPAuthRequired,
		))
		var reasonData struct {
			ReasonCodes []string `json:"reason_codes"`
		}
		encoded, err := json.Marshal(toolReqErr.Data)
		if err != nil {
			t.Fatalf("json.Marshal(tool request error data) error = %v", err)
		}
		if err := json.Unmarshal(encoded, &reasonData); err != nil {
			t.Fatalf("json.Unmarshal(tool request error data) error = %v", err)
		}
		if !slices.Contains(reasonData.ReasonCodes, string(toolspkg.ReasonMCPAuthRequired)) {
			t.Fatalf(
				"tool request error reason codes = %#v, want %q",
				reasonData.ReasonCodes,
				toolspkg.ReasonMCPAuthRequired,
			)
		}
	})

	buffer := &lockedBuffer{}
	if _, err := buffer.Write([]byte("stderr")); err != nil {
		t.Fatalf("lockedBuffer.Write() error = %v", err)
	}
	if buffer.String() != "stderr" {
		t.Fatalf("lockedBuffer.String() = %q, want %q", buffer.String(), "stderr")
	}

	if chooseFloat64(nil, new(1.2)) == nil {
		t.Fatal("chooseFloat64(nil, fallback) = nil, want fallback")
	}
	if chooseString(nil, new("usd")) == nil {
		t.Fatal("chooseString(nil, fallback) = nil, want fallback")
	}
}

func TestTrimUTF8LeadingBytesPreservesTrailingPartialRune(t *testing.T) {
	t.Parallel()

	input := append([]byte("hello"), 0xE2)
	got := trimUTF8LeadingBytes(append([]byte(nil), input...))
	if !bytes.Equal(got, input) {
		t.Fatalf("trimUTF8LeadingBytes() = %v, want %v", got, input)
	}
}

func TestManagedTerminalAppendOutputOverflowPreservesTrailingPartialRune(t *testing.T) {
	t.Parallel()

	term := &managedTerminal{
		output:      bytes.Repeat([]byte("a"), defaultTerminalOutputLimit),
		outputLimit: defaultTerminalOutputLimit,
	}

	term.appendOutput([]byte{0xE2})

	if !term.truncated {
		t.Fatal("appendOutput() did not mark output as truncated")
	}
	if len(term.output) != defaultTerminalOutputLimit {
		t.Fatalf("len(term.output) = %d, want %d", len(term.output), defaultTerminalOutputLimit)
	}
	if cap(term.output) > defaultTerminalOutputLimit {
		t.Fatalf("cap(term.output) = %d, want <= %d", cap(term.output), defaultTerminalOutputLimit)
	}
	if term.output[len(term.output)-1] != 0xE2 {
		t.Fatalf("last output byte = 0x%x, want 0xE2", term.output[len(term.output)-1])
	}
	if term.output[0] != 'a' {
		t.Fatalf("first output byte = 0x%x, want 'a'", term.output[0])
	}
}

func TestManagedTerminalAppendOutputUsesConfiguredLimit(t *testing.T) {
	t.Parallel()

	term := &managedTerminal{
		outputLimit: 4,
	}

	term.appendOutput([]byte("abcdef"))

	if got, want := string(term.output), "cdef"; got != want {
		t.Fatalf("string(term.output) = %q, want %q", got, want)
	}
	if !term.truncated {
		t.Fatal("appendOutput() truncated = false, want true")
	}
}

func TestWithoutCancelPreservingDeadline(t *testing.T) {
	t.Parallel()

	parent, cancelParent := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	child, cancelChild := withoutCancelPreservingDeadline(parent)
	defer cancelParent()
	defer cancelChild()

	parentDeadline, ok := parent.Deadline()
	if !ok {
		t.Fatal("parent.Deadline() = missing, want deadline")
	}
	childDeadline, ok := child.Deadline()
	if !ok {
		t.Fatal("child.Deadline() = missing, want preserved deadline")
	}
	if !childDeadline.Equal(parentDeadline) {
		t.Fatalf("child deadline = %v, want %v", childDeadline, parentDeadline)
	}

	cancelParent()

	select {
	case <-child.Done():
		t.Fatalf("child context canceled by parent: %v", child.Err())
	default:
	}
}

func TestHandleCreateTerminalRemovesOwnershipOnRegistrationFailure(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	proc.processRegistry = toolruntime.NewRegistry(failingProcessStore{upsertErr: errors.New("boom")})
	proc.SetTurnSourceProvider(func() string { return "network" })

	var killedTerminalID string
	proc.toolHost = contextAwareToolHost{
		createTerminalFn: func(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
			return acpsdk.CreateTerminalResponse{TerminalId: "term-external"}, nil
		},
		killTerminalFn: func(id string) error {
			killedTerminalID = id
			return nil
		},
	}

	active, err := proc.beginPrompt("turn-network-create-failure", 4)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	if _, err := proc.handleCreateTerminal(context.Background(), acpsdk.CreateTerminalRequest{
		SessionId: "sess-direct",
		Command:   "agh",
		Args:      []string{"network", "status"},
	}); err == nil {
		t.Fatal("handleCreateTerminal() error = nil, want registration failure")
	}

	if killedTerminalID != "term-external" {
		t.Fatalf("cleanup killed terminal = %q, want %q", killedTerminalID, "term-external")
	}
	if _, ok := proc.terminalOwnership["term-external"]; ok {
		t.Fatalf("terminalOwnership = %#v, want failed terminal removed", proc.terminalOwnership)
	}
}

func TestHandleReleaseTerminalRemovesExternalOwnership(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	proc.SetTurnSourceProvider(func() string { return "network" })
	proc.terminalOwnership = map[string]terminalOwnership{
		"term-external": {
			networkOwned:   true,
			ownerSessionID: "sess-direct",
			ownerTurnID:    "turn-network-release",
		},
	}

	var releasedTerminalID string
	proc.toolHost = contextAwareToolHost{
		releaseTerminalFn: func(id string) error {
			releasedTerminalID = id
			return nil
		},
	}

	active, err := proc.beginPrompt("turn-network-release", 4)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	if _, err := proc.handleReleaseTerminal(acpsdk.ReleaseTerminalRequest{
		SessionId:  "sess-direct",
		TerminalId: "term-external",
	}); err != nil {
		t.Fatalf("handleReleaseTerminal() error = %v", err)
	}

	if releasedTerminalID != "term-external" {
		t.Fatalf("released terminal = %q, want %q", releasedTerminalID, "term-external")
	}
	if _, ok := proc.terminalOwnership["term-external"]; ok {
		t.Fatalf("terminalOwnership = %#v, want released terminal removed", proc.terminalOwnership)
	}
}

func TestHandleSessionUpdateVariants(t *testing.T) {
	t.Run("Should translate updates and retain the current ACP mode", func(t *testing.T) {
		t.Parallel()

		proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
		proc.setCaps(Caps{ConfigOptions: []SessionConfigOption{{
			ID:      "mode",
			Kind:    SessionConfigOptionKindSelect,
			Current: "ask",
			Values: []SessionConfigOptionValue{
				{Value: "agent"},
				{Value: "plan"},
				{Value: "ask"},
			},
		}}})
		active, err := proc.beginPrompt("turn-update", 16)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}
		defer proc.endPrompt(active)

		title := "permission"
		agentMessage := mustMarshalJSON(wireSessionNotification{
			SessionID: "sess-direct",
			Update: mustMarshalJSON(map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       map[string]any{"type": "text", "text": "hello"},
			}),
		})
		if err := proc.handleSessionUpdate(agentMessage); err != nil {
			t.Fatalf("handleSessionUpdate(agent_message_chunk) error = %v", err)
		}

		usageUpdate := mustMarshalJSON(wireSessionNotification{
			SessionID: "sess-direct",
			Update: mustMarshalJSON(map[string]any{
				"sessionUpdate": "usage_update",
				"used":          10,
				"size":          20,
				"cost": map[string]any{
					"amount":   1.5,
					"currency": "USD",
				},
			}),
		})
		if err := proc.handleSessionUpdate(usageUpdate); err != nil {
			t.Fatalf("handleSessionUpdate(usage_update) error = %v", err)
		}

		toolCall := mustMarshalJSON(wireSessionNotification{
			SessionID: "sess-direct",
			Update: mustMarshalJSON(map[string]any{
				"sessionUpdate": "tool_call",
				"toolCallId":    "tool-1",
				"title":         title,
				"kind":          "edit",
				"rawInput":      map[string]any{"path": "retry.go"},
				"status":        "in_progress",
			}),
		})
		if err := proc.handleSessionUpdate(toolCall); err != nil {
			t.Fatalf("handleSessionUpdate(tool_call) error = %v", err)
		}

		modeUpdate := mustMarshalJSON(wireSessionNotification{
			SessionID: "sess-direct",
			Update: mustMarshalJSON(map[string]any{
				"sessionUpdate": "current_mode_update",
				"currentModeId": "code",
			}),
		})
		if err := proc.handleSessionUpdate(modeUpdate); err != nil {
			t.Fatalf("handleSessionUpdate(current_mode_update) error = %v", err)
		}
		failedToolResult := mustMarshalJSON(acpsdk.SessionNotification{
			SessionId: "sess-direct",
			Update: acpsdk.UpdateToolCall(
				"tool-1",
				acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusFailed),
			),
		})
		if err := proc.handleSessionUpdate(failedToolResult); err != nil {
			t.Fatalf("handleSessionUpdate(failed tool result) error = %v", err)
		}

		events := collectEventsUntilCount(t, active.events, 5)
		if events[0].Type != EventTypeAgentMessage {
			t.Fatalf("agent message event = %#v, want agent message", events[0])
		}
		if events[1].Type != EventTypeUsage || events[1].Usage == nil || events[1].Usage.ContextUsed == nil ||
			*events[1].Usage.ContextUsed != 10 {
			t.Fatalf("usage event = %#v, want usage metadata", events[1])
		}
		if events[2].Type != EventTypeToolCall || events[2].ToolKind() != "edit" ||
			string(events[2].ToolInput()) != `{"path":"retry.go"}` {
			t.Fatalf("tool call event = %#v, want tool call", events[2])
		}
		if events[3].Type != EventTypeSystem {
			t.Fatalf("system event = %#v, want system", events[3])
		}
		if events[4].Type != EventTypeToolResult || !events[4].ToolError() {
			t.Fatalf("failed tool result event = %#v, want typed failure", events[4])
		}
		assertConfigOption(t, proc.CapsSnapshot().ConfigOptions, "mode", "code", "agent", "plan", "ask")
	})
}

func TestHandleSessionUpdateAvailableCommands(t *testing.T) {
	t.Run("Should retain the complete typed ACP command replacement set", func(t *testing.T) {
		t.Parallel()

		proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
		active, err := proc.beginPrompt("turn-commands", 4)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}
		defer proc.endPrompt(active)

		update := mustMarshalJSON(wireSessionNotification{
			SessionID: "sess-direct",
			Update: mustMarshalJSON(map[string]any{
				"sessionUpdate": "available_commands_update",
				"availableCommands": []map[string]any{
					{
						"name":        " compact ",
						"description": " Compact context ",
						"input":       map[string]any{"hint": " optional focus "},
					},
					{"name": "", "description": "Ignored"},
				},
			}),
		})
		if err := proc.handleSessionUpdate(update); err != nil {
			t.Fatalf("handleSessionUpdate(available_commands_update) error = %v", err)
		}

		events := collectEventsUntilCount(t, active.events, 1)
		if got, want := events[0].Type, EventTypeAvailableCommands; got != want {
			t.Fatalf("event type = %q, want %q", got, want)
		}
		if got, want := events[0].Title, SystemEventTitleAvailableCommandsUpdate; got != want {
			t.Fatalf("event title = %q, want %q", got, want)
		}
		if got, want := events[0].AvailableCommands.Values(), []store.SessionAdvertisedCommand{{
			Name:        "compact",
			Description: "Compact context",
			Input:       &store.SessionAdvertisedCommandInput{Hint: "optional focus"},
		}}; !slices.EqualFunc(got, want, store.SessionAdvertisedCommandsEqual) {
			t.Fatalf("available commands = %#v, want %#v", got, want)
		}
	})
}

func TestHandleSessionUpdateMarksToolCallAsPrecheckedAfterGatewayIntercept(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	active, err := proc.beginPrompt("turn-prechecked", 4)
	if err != nil {
		t.Fatalf("beginPrompt() error = %v", err)
	}
	defer proc.endPrompt(active)

	rawInput := mustMarshalJSON(writeTextFileToolInput{
		Path:    filepath.Join(proc.Cwd, "prechecked.txt"),
		Content: "patched",
	})
	proc.recordProviderNativeToolPrecheck(providerNativeToolIDWrite, rawInput)

	toolCall := mustMarshalJSON(wireSessionNotification{
		SessionID: "sess-direct",
		Update: mustMarshalJSON(map[string]any{
			"sessionUpdate": "tool_call",
			"title":         providerNativeToolIDWrite,
			"rawInput": map[string]any{
				"path":    filepath.Join(proc.Cwd, "prechecked.txt"),
				"content": "patched",
			},
		}),
	})
	if err := proc.handleSessionUpdate(toolCall); err != nil {
		t.Fatalf("handleSessionUpdate(tool_call) error = %v", err)
	}

	events := collectEventsUntilCount(t, active.events, 1)
	if len(events) != 1 || events[0].Type != EventTypeToolCall {
		t.Fatalf("events = %#v, want one tool_call", events)
	}
	if !events[0].ToolPrechecked {
		t.Fatalf("tool call event = %#v, want ToolPrechecked true", events[0])
	}
}

func TestAccessorsAndValidationHelpers(t *testing.T) {
	t.Parallel()

	if err := (StartOpts{}).Validate(); err == nil {
		t.Fatal("StartOpts.Validate() error = nil, want validation error")
	}
	if err := (PromptRequest{}).Validate(); err == nil {
		t.Fatal("PromptRequest.Validate() error = nil, want validation error")
	}
	t.Run("ShouldRejectNegativeHeartbeatInterval", func(t *testing.T) {
		t.Parallel()

		err := (PromptRequest{
			TurnID:                    "turn-negative-heartbeat",
			Message:                   "hello",
			ActivityHeartbeatInterval: -time.Second,
		}).Validate()
		if err == nil {
			t.Fatal("PromptRequest.Validate(negative heartbeat) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "heartbeat") {
			t.Fatalf("PromptRequest.Validate(negative heartbeat) error = %v, want heartbeat context", err)
		}
	})

	proc := &AgentProcess{stderr: &lockedBuffer{}}
	if _, err := proc.stderr.Write([]byte("boom")); err != nil {
		t.Fatalf("stderr.Write() error = %v", err)
	}
	if proc.Stderr() != "boom" {
		t.Fatalf("Stderr() = %q, want %q", proc.Stderr(), "boom")
	}
}

func TestPermissionHelperBranches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policy, err := newPermissionPolicy(aghconfig.PermissionModeApproveAll, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy() error = %v", err)
	}

	resolved, err := policy.resolvePathList([]acpsdk.ToolCallLocation{{Path: filepath.Join(root, "inside.txt")}})
	if err != nil {
		t.Fatalf("resolvePathList() error = %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("resolvePathList() = %#v, want one path", resolved)
	}

	allowOutcome, allowDecision := selectPermissionOutcome([]acpsdk.PermissionOption{
		{OptionId: "allow-once", Name: "allow once", Kind: acpsdk.PermissionOptionKindAllowOnce},
		{OptionId: "allow-always", Name: "allow", Kind: acpsdk.PermissionOptionKindAllowAlways},
	}, decisionAllowOnce)
	if allowOutcome.Selected == nil || allowOutcome.Selected.OptionId != "allow-once" ||
		allowDecision != decisionAllowOnce {
		t.Fatalf("selectPermissionOutcome(allow-once) = %#v, %q", allowOutcome, allowDecision)
	}

	rejectOutcome, rejectDecision := selectPermissionOutcome([]acpsdk.PermissionOption{
		{OptionId: "reject-once", Name: "reject once", Kind: acpsdk.PermissionOptionKindRejectOnce},
		{OptionId: "reject-always", Name: "reject always", Kind: acpsdk.PermissionOptionKindRejectAlways},
	}, decisionRejectAlways)
	if rejectOutcome.Selected == nil || rejectOutcome.Selected.OptionId != "reject-always" ||
		rejectDecision != decisionRejectAlways {
		t.Fatalf("selectPermissionOutcome(reject-always) = %#v, %q", rejectOutcome, rejectDecision)
	}

	cancelOutcome, cancelDecision := selectPermissionOutcome(nil, decisionRejectOnce)
	if err := cancelOutcome.Validate(); err != nil {
		t.Fatalf("selectPermissionOutcome(cancel).Validate() error = %v", err)
	}
	if cancelOutcome.Selected != nil {
		t.Fatalf("selectPermissionOutcome(cancel) = %#v, want no selected option", cancelOutcome)
	}
	if cancelDecision != "" {
		t.Fatalf("selectPermissionOutcome(cancel) decision = %q, want empty", cancelDecision)
	}

	if _, err := parsePermissionDecision("maybe"); err == nil {
		t.Fatal("parsePermissionDecision(invalid) error = nil, want non-nil")
	}
	if err := (ApproveRequest{Decision: string(decisionAllowOnce)}).Validate(); err == nil {
		t.Fatal("ApproveRequest.Validate(missing request id and turn id) error = nil, want non-nil")
	}

	readKind := acpsdk.ToolKindRead
	readDecision, interactive := policy.permissionDecision(acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{
			Kind:      &readKind,
			Locations: []acpsdk.ToolCallLocation{{Path: filepath.Join(root, "inside.txt")}},
		},
	})
	if readDecision != decisionAllowOnce || interactive {
		t.Fatalf("permissionDecision(read) = %q, %v, want %q, false", readDecision, interactive, decisionAllowOnce)
	}

	approveReadsPolicy, err := newPermissionPolicy(aghconfig.PermissionModeApproveReads, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy(approve-reads) error = %v", err)
	}
	editKind := acpsdk.ToolKindEdit
	editDecision, interactive := approveReadsPolicy.permissionDecision(acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{Kind: &editKind},
	})
	if editDecision != decisionPending || !interactive {
		t.Fatalf("permissionDecision(edit) = %q, %v, want %q, true", editDecision, interactive, decisionPending)
	}

	denyAllPolicy, err := newPermissionPolicy(aghconfig.PermissionModeDenyAll, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy(deny-all) error = %v", err)
	}
	denyDecision, interactive := denyAllPolicy.permissionDecision(acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{Kind: &editKind},
	})
	if denyDecision != decisionRejectOnce || interactive {
		t.Fatalf("permissionDecision(deny-all) = %q, %v, want %q, false", denyDecision, interactive, decisionRejectOnce)
	}

	if got := permissionRequestIDFromMeta(map[string]any{"request_id": "req-meta"}); got != "req-meta" {
		t.Fatalf("permissionRequestIDFromMeta() = %q, want %q", got, "req-meta")
	}
	title := "Write file"
	if got := permissionRequestName("turn-1", acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{
			Title: &title,
			Kind:  &editKind,
		},
	}); got != "turn-1:Write file" {
		t.Fatalf("permissionRequestName() = %q, want %q", got, "turn-1:Write file")
	}

	proc := newDirectProcess(t, aghconfig.PermissionModeDenyAll)
	if got := proc.nextPermissionRequestID("turn-1", acpsdk.RequestPermissionRequest{
		Meta: map[string]any{"request_id": "req-from-meta"},
	}); got != "req-from-meta" {
		t.Fatalf("nextPermissionRequestID(meta) = %q, want %q", got, "req-from-meta")
	}
	if got := (&AgentProcess{}).permissionTimeoutOrDefault(); got != 5*time.Minute {
		t.Fatalf("permissionTimeoutOrDefault() = %v, want %v", got, 5*time.Minute)
	}
}

func TestHandleInboundCreateTerminalUsesRequestContext(t *testing.T) {
	t.Parallel()

	proc := newDirectProcess(t, aghconfig.PermissionModeApproveAll)
	type contextKey string
	const ctxKey contextKey = "terminal-create"
	proc.toolHost = contextAwareToolHost{
		createTerminalFn: func(ctx context.Context, _ acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
			if got, want := ctx.Value(ctxKey), "present"; got != want {
				return acpsdk.CreateTerminalResponse{}, fmt.Errorf("ctx value = %v, want %q", got, want)
			}
			return acpsdk.CreateTerminalResponse{TerminalId: "term-ctx"}, nil
		},
	}

	ctx := context.WithValue(context.Background(), ctxKey, "present")
	result, reqErr := proc.handleInbound(
		ctx,
		acpsdk.ClientMethodTerminalCreate,
		mustMarshalJSON(acpsdk.CreateTerminalRequest{
			SessionId: "sess-direct",
			Command:   "sh",
			Args:      []string{"-c", "printf ready"},
			Cwd:       new(proc.Cwd),
		}),
	)
	if reqErr != nil {
		t.Fatalf("handleInbound(create terminal) error = %v", reqErr)
	}
	response, ok := result.(acpsdk.CreateTerminalResponse)
	if !ok {
		t.Fatalf("handleInbound(create terminal) type = %T, want CreateTerminalResponse", result)
	}
	if response.TerminalId != "term-ctx" {
		t.Fatalf("terminal id = %q, want %q", response.TerminalId, "term-ctx")
	}
}

func TestToolHostOrDefaultUsesProcessLifecycleContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	policy, err := newPermissionPolicy(aghconfig.PermissionModeApproveAll, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy() error = %v", err)
	}

	proc := &AgentProcess{
		Cwd:           root,
		processCtx:    ctx,
		cancelProcess: cancel,
		permissions:   policy,
		stderr:        &lockedBuffer{},
		done:          make(chan struct{}),
		StartedAt:     timeNowUTC(),
		SessionID:     "sess-direct",
		AgentName:     "direct",
	}

	host, ok := proc.toolHostOrDefault().(*localToolHost)
	if !ok {
		t.Fatalf("toolHostOrDefault() type = %T, want *localToolHost", proc.toolHostOrDefault())
	}

	cancel()
	select {
	case <-host.terminals.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("terminal manager context did not close with process lifecycle")
	}
}

func newDirectProcess(t *testing.T, mode aghconfig.PermissionMode) *AgentProcess {
	t.Helper()

	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	policy, err := newPermissionPolicy(mode, root)
	if err != nil {
		t.Fatalf("newPermissionPolicy() error = %v", err)
	}

	proc := &AgentProcess{
		AgentName:         "direct",
		Cwd:               root,
		SessionID:         "sess-direct",
		StartedAt:         timeNowUTC(),
		processCtx:        ctx,
		permissions:       policy,
		terminals:         newTerminalManager(ctx, slog.Default()),
		done:              make(chan struct{}),
		cancelProcess:     cancel,
		stderr:            &lockedBuffer{},
		permissionTimeout: time.Second,
	}
	t.Cleanup(proc.terminals.closeAll)
	return proc
}

type contextAwareToolHost struct {
	createTerminalFn  func(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error)
	killTerminalFn    func(string) error
	releaseTerminalFn func(string) error
}

type toolExecutionGatewayFunc func(context.Context, ToolExecutionRequest) (ToolExecutionRequest, error)

func (fn toolExecutionGatewayFunc) Intercept(
	ctx context.Context,
	req ToolExecutionRequest,
) (ToolExecutionRequest, error) {
	return fn(ctx, req)
}

func (h contextAwareToolHost) ReadTextFile(context.Context, string) (string, error) {
	return "", nil
}

func (h contextAwareToolHost) WriteTextFile(context.Context, string, string) error {
	return nil
}

func (h contextAwareToolHost) ResolvePath(path string) (string, error) {
	return path, nil
}

func (h contextAwareToolHost) Authorize(permissionOperation) error {
	return nil
}

func (h contextAwareToolHost) PermissionDecision(acpsdk.RequestPermissionRequest) (permissionDecision, bool) {
	return decisionAllowOnce, false
}

func (h contextAwareToolHost) CreateTerminal(
	ctx context.Context,
	req acpsdk.CreateTerminalRequest,
) (acpsdk.CreateTerminalResponse, error) {
	if h.createTerminalFn == nil {
		return acpsdk.CreateTerminalResponse{}, nil
	}
	return h.createTerminalFn(ctx, req)
}

func (h contextAwareToolHost) KillTerminal(id string) error {
	if h.killTerminalFn != nil {
		return h.killTerminalFn(id)
	}
	return nil
}

func (h contextAwareToolHost) TerminalOutput(string) (string, error) {
	return "", nil
}

func (h contextAwareToolHost) WaitForTerminalExit(context.Context, string) (int, error) {
	return 0, nil
}

func (h contextAwareToolHost) ReleaseTerminal(id string) error {
	if h.releaseTerminalFn != nil {
		return h.releaseTerminalFn(id)
	}
	return nil
}

type failingProcessStore struct {
	upsertErr error
}

func (s failingProcessStore) UpsertProcessRecord(context.Context, toolruntime.ProcessRecord) error {
	return s.upsertErr
}

func (failingProcessStore) UpdateProcessRecordState(context.Context, toolruntime.ProcessStateUpdate) error {
	return nil
}

func (failingProcessStore) ListProcessRecords(
	context.Context,
	toolruntime.ProcessQuery,
) ([]toolruntime.ProcessRecord, error) {
	return nil, nil
}

func writeFakeAGHBinary(t *testing.T, dir string, body string) {
	t.Helper()

	path := filepath.Join(dir, "agh")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func decodePermissionEventRaw(t *testing.T, raw json.RawMessage) struct {
	RequestID string                  `json:"request_id"`
	ToolInput map[string]any          `json:"tool_input"`
	Options   []permissionEventOption `json:"options"`
} {
	t.Helper()

	var payload struct {
		RequestID string                  `json:"request_id"`
		ToolInput map[string]any          `json:"tool_input"`
		Options   []permissionEventOption `json:"options"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(permission raw) error = %v; raw=%s", err, string(raw))
	}
	return payload
}

func collectEventsUntilCount(t *testing.T, eventsCh <-chan AgentEvent, want int) []AgentEvent {
	t.Helper()

	events := make([]AgentEvent, 0, want)
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	for len(events) < want {
		select {
		case event := <-eventsCh:
			events = append(events, event)
		case <-timeout.C:
			t.Fatalf("timeout collecting %d events; got %#v", want, events)
		}
	}
	return events
}
