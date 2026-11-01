package acp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestPermissionPolicyModes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policies := map[string]struct {
		mode       compozyconfig.PermissionMode
		readOK     bool
		writeOK    bool
		terminalOK bool
	}{
		"deny-all": {
			mode:       compozyconfig.PermissionModeDenyAll,
			readOK:     false,
			writeOK:    false,
			terminalOK: false,
		},
		"approve-reads": {
			mode:       compozyconfig.PermissionModeApproveReads,
			readOK:     true,
			writeOK:    false,
			terminalOK: false,
		},
		"approve-all": {
			mode:       compozyconfig.PermissionModeApproveAll,
			readOK:     true,
			writeOK:    true,
			terminalOK: true,
		},
	}

	for name, tc := range policies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			policy, err := newPermissionPolicy(tc.mode, root)
			if err != nil {
				t.Fatalf("newPermissionPolicy() error = %v", err)
			}

			assertPermissionResult(t, policy.authorize(permissionReadTextFile), tc.readOK)
			assertPermissionResult(t, policy.authorize(permissionWriteTextFile), tc.writeOK)
			assertPermissionResult(t, policy.authorize(permissionCreateTerminal), tc.terminalOK)
		})
	}
}

func TestPermissionPolicyResolvePathContainment(t *testing.T) {
	t.Parallel()
	t.Run("Should confine file access to the allowed workspace", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveAll, root)
		if err != nil {
			t.Fatalf("newPermissionPolicy() error = %v", err)
		}

		insideFile := filepath.Join(root, "nested", "file.txt")
		resolvedInside, err := policy.resolvePath(insideFile)
		if err != nil {
			t.Fatalf("resolvePath(%q) error = %v", insideFile, err)
		}
		if !strings.HasSuffix(resolvedInside, filepath.Join("nested", "file.txt")) {
			t.Fatalf(
				"resolvePath(%q) = %q, want suffix %q",
				insideFile,
				resolvedInside,
				filepath.Join("nested", "file.txt"),
			)
		}

		if _, err := policy.resolvePath(
			filepath.Join(root, "..", "escape.txt"),
		); !errors.Is(
			err,
			ErrPathOutsideWorkspace,
		) {
			t.Fatalf("resolvePath(outside) error = %v, want ErrPathOutsideWorkspace", err)
		}
	})
}

func TestDriverApprovePermissionValidationAndForwarding(t *testing.T) {
	t.Parallel()

	driver := New(WithPermissionTimeout(123 * time.Millisecond))
	if driver.permissionWait != 123*time.Millisecond {
		t.Fatalf("permissionWait = %v, want %v", driver.permissionWait, 123*time.Millisecond)
	}

	proc := newDirectProcess(t, compozyconfig.PermissionModeDenyAll)
	requestID, pending, registerErr := proc.registerPendingPermission("turn-1", acpsdk.RequestPermissionRequest{
		ToolCall: acpsdk.ToolCallUpdate{ToolCallId: "tool-1"},
	})
	if registerErr != nil {
		t.Fatal(registerErr)
	}

	if err := driver.ApprovePermission(t.Context(), proc, ApproveRequest{
		RequestID: requestID,
		Decision:  string(decisionAllowOnce),
	}); err != nil {
		t.Fatalf("ApprovePermission() error = %v", err)
	}
	select {
	case decision := <-pending.response:
		if decision != decisionAllowOnce {
			t.Fatalf("pending response = %q, want %q", decision, decisionAllowOnce)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending permission response")
	}

	if err := driver.ApprovePermission(t.Context(), nil, ApproveRequest{
		RequestID: "req-1",
		Decision:  string(decisionAllowOnce),
	}); err == nil {
		t.Fatal("ApprovePermission(nil proc) error = nil, want non-nil")
	}

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := driver.ApprovePermission(canceledCtx, proc, ApproveRequest{
		RequestID: "req-1",
		Decision:  string(decisionAllowOnce),
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ApprovePermission(canceled ctx) error = %v, want context.Canceled", err)
	}
}

func TestPermissionWaitResolution(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve each request using the daemon session identity", func(t *testing.T) {
		t.Parallel()
		wait := 12 * time.Hour
		proc := &AgentProcess{compozySessionID: "daemon-session", permissionTimeout: time.Minute,
			permissionWaitResolver: func(id string) time.Duration {
				if id != "daemon-session" {
					t.Fatalf("session = %q", id)
				}
				return wait
			},
		}
		if got := proc.permissionTimeoutOrDefault(); got != wait {
			t.Fatalf("wait = %v", got)
		}
		wait = 24 * time.Hour
		if got := proc.permissionTimeoutOrDefault(); got != wait {
			t.Fatalf("updated wait = %v", got)
		}
		wait = 0
		if got := proc.permissionTimeoutOrDefault(); got != time.Minute {
			t.Fatalf("fallback = %v", got)
		}
	})
}

func TestTriagePolicy(t *testing.T) {
	t.Run("Should fail the active triage run when the provider leaves read only mode", func(t *testing.T) {
		t.Parallel()
		for _, mode := range []string{"default", "plan", "full-access", ""} {
			p := &AgentProcess{SessionID: "provider", executionPolicy: ExecutionPolicyTriage}
			canceled := false
			active, err := p.beginPrompt("triage", 16, func() { canceled = true })
			if err != nil {
				t.Fatal(err)
			}
			err = p.handleSessionUpdate(mustMarshalJSON(map[string]any{
				"sessionId": "provider", "update": map[string]any{
					"sessionUpdate": "current_mode_update", "currentModeId": mode,
				},
			}))
			if !errors.Is(err, ErrTriageUnsupported) || !canceled {
				t.Fatalf("mode %q: error=%v canceled=%v", mode, err, canceled)
			}
			events := drainObservedPrompt(p, active)
			if len(events) != 1 || events[0].Type != EventTypeError {
				t.Fatalf("mode %q: events=%+v", mode, events)
			}
		}
	})

	t.Run("Should reject writes and unknown calls before any approval gateway", func(t *testing.T) {
		t.Parallel()
		policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveAll, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		p := &AgentProcess{
			executionPolicy: ExecutionPolicyTriage,
			permissions:     policy,
			toolGateway: toolExecutionGatewayFunc(
				func(context.Context, ToolExecutionRequest) (ToolExecutionRequest, error) {
					t.Fatal("triage write reached gateway")
					return ToolExecutionRequest{}, nil
				},
			),
		}
		active := beginObservedPrompt(t, p)
		for _, kind := range []*acpsdk.ToolKind{nil, new(acpsdk.ToolKindEdit), new(acpsdk.ToolKindExecute)} {
			response, err := p.resolvePermissionRequest(t.Context(), acpsdk.RequestPermissionRequest{
				ToolCall: acpsdk.ToolCallUpdate{Kind: kind},
			}, "provider")
			if err != nil {
				t.Fatal(err)
			}
			if response.Outcome.Cancelled == nil {
				t.Fatalf("write outcome=%+v", response.Outcome)
			}
		}
		if _, err := p.interceptWriteTextFileRequest(
			t.Context(),
			acpsdk.WriteTextFileRequest{},
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("write err=%v", err)
		}
		if _, err := p.interceptCreateTerminalRequest(
			t.Context(),
			acpsdk.CreateTerminalRequest{},
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("terminal err=%v", err)
		}
		if _, err := p.interceptProviderNativeTool(
			t.Context(),
			ToolExecutionRequest{ToolID: "unknown"},
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("unknown err=%v", err)
		}
		events := drainObservedPrompt(p, active)
		if len(events) != 6 {
			t.Fatalf("denial events=%+v", events)
		}
		for _, event := range events {
			if event.RequestIDValue() == "" || event.Type != EventTypePermission || event.Decision != string(decisionRejectOnce) {
				t.Fatalf("denial=%+v", event)
			}
		}
		if err := New().ConfigureRuntime(t.Context(), p, RuntimeConfig{}); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("runtime err=%v", err)
		}
	})
	t.Run("Should require explicit read only modes and discard sidecars", func(t *testing.T) {
		t.Parallel()
		if triageReadOnlyMode("unknown", []string{"ask", "plan", "default", "full-access"}) != "" {
			t.Fatal("unsafe mode accepted")
		}
		if triageReadOnlyMode("codex", []string{"default", "read-only"}) != "read-only" {
			t.Fatal("read only mode missing")
		}
		if _, err := constrainTriageStart(
			StartOpts{ExecutionPolicy: ExecutionPolicyTriage},
		); !errors.Is(
			err,
			ErrTriageUnsupported,
		) {
			t.Fatalf("unknown provider err=%v", err)
		}
		got, err := constrainTriageStart(StartOpts{ExecutionPolicy: ExecutionPolicyTriage, ProviderName: "test",
			Permissions: compozyconfig.PermissionModeApproveAll, MCPServers: []compozyconfig.MCPServer{{}},
			ActivateMCPServers: func(context.Context) error { t.Fatal("sidecars activated"); return nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Permissions != compozyconfig.PermissionModeApproveReads || len(got.MCPServers) != 0 ||
			got.ActivateMCPServers != nil {
			t.Fatalf("opts=%+v", got)
		}
	})
}
