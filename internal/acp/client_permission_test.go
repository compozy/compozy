package acp

import (
	"context"
	"errors"
	"os"
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
		"approve-edits": {mode: compozyconfig.PermissionModeApproveEdits, readOK: true, writeOK: true},
		"auto":          {mode: compozyconfig.PermissionModeAuto, readOK: true, writeOK: true},
		"approve-all": {
			mode:       compozyconfig.PermissionModeApproveAll,
			readOK:     true,
			writeOK:    true,
			terminalOK: true,
		},
	}

	for name, tc := range policies {
		t.Run("Should enforce "+name, func(t *testing.T) {
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
	// Invariant: a read-only policy violation fences future prompts even while idle.
	// Owner: ACP prompt admission; canonical suite: TestTriagePolicy.
	t.Run("Should reject a later prompt after an idle mode violation", func(t *testing.T) {
		t.Parallel()
		p := &AgentProcess{SessionID: "provider", executionPolicy: ExecutionPolicyTriage}
		err := p.handleSessionUpdate(
			mustMarshalJSON(
				map[string]any{
					"sessionId": "provider",
					"update":    map[string]any{"sessionUpdate": "current_mode_update", "currentModeId": "full-access"},
				},
			),
		)
		if !errors.Is(err, ErrTriageUnsupported) {
			t.Fatal(err)
		}
		if active, err := p.beginPrompt("later", 16); !errors.Is(err, ErrTriageUnsupported) || active != nil {
			t.Fatalf("later prompt=%v error=%v", active, err)
		}
	})
	// Invariant: allowed reads still pass the authoritative pre-call gateway, with raw input.
	// Owner: ACP tool execution boundary; canonical suite: TestTriagePolicy.
	t.Run("Should apply gateway denial to a triage read", func(t *testing.T) {
		t.Parallel()
		policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveReads, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		p := &AgentProcess{
			executionPolicy: ExecutionPolicyTriage,
			permissions:     policy,
			toolGateway: toolExecutionGatewayFunc(
				func(_ context.Context, req ToolExecutionRequest) (ToolExecutionRequest, error) {
					calls++
					if req.ToolID != "Read" || !req.ReadOnly || !strings.Contains(string(req.Input), "notes.txt") {
						t.Fatalf("gateway=%+v", req)
					}
					return ToolExecutionRequest{}, ErrPermissionDenied
				},
			),
		}
		active := beginObservedPrompt(t, p)
		response, err := p.resolvePermissionRequest(
			t.Context(),
			acpsdk.RequestPermissionRequest{
				ToolCall: acpsdk.ToolCallUpdate{
					Kind:     new(acpsdk.ToolKindRead),
					RawInput: map[string]any{"path": "notes.txt"},
				},
			},
			"provider",
		)
		if err != nil || response.Outcome.Cancelled == nil || calls != 1 { //nolint:misspell // ACP SDK field.
			t.Fatalf("response=%+v calls=%d error=%v", response, calls, err)
		}
		drainObservedPrompt(p, active)
	})
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
			if response.Outcome.Cancelled == nil { //nolint:misspell // ACP SDK field uses British spelling.
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
			if event.RequestIDValue() == "" || event.Type != EventTypePermission ||
				event.Decision != string(decisionRejectOnce) {
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
			StartOpts{PermissionPolicy: &StartPermissionPolicy{ExecutionPolicy: ExecutionPolicyTriage}},
		); !errors.Is(
			err,
			ErrTriageUnsupported,
		) {
			t.Fatalf("unknown provider err=%v", err)
		}
		got, err := constrainTriageStart(
			StartOpts{
				PermissionPolicy:   &StartPermissionPolicy{ExecutionPolicy: ExecutionPolicyTriage},
				ProviderName:       "test",
				Permissions:        compozyconfig.PermissionModeApproveAll,
				MCPServers:         []compozyconfig.MCPServer{{}},
				ActivateMCPServers: func(context.Context) error { t.Fatal("sidecars activated"); return nil },
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if got.Permissions != compozyconfig.PermissionModeApproveReads || len(got.MCPServers) != 0 ||
			got.ActivateMCPServers != nil {
			t.Fatalf("opts=%+v", got)
		}
	})
}

func TestEditLevelPermissionRequests(t *testing.T) {
	t.Parallel()
	// Invariant (rule 12, SEC-4, SEC-7): approve-edits and auto approve reads and contained edits,
	// and every other request, including an edit without locations or an unknown kind, asks.
	// Owning layer: ACP permission level decision; canonical suite: TestEditLevelPermissionRequests.
	for _, mode := range []compozyconfig.PermissionMode{
		compozyconfig.PermissionModeApproveEdits,
		compozyconfig.PermissionModeAuto,
	} {
		for _, tc := range []struct {
			name     string
			kind     *acpsdk.ToolKind
			location string
			decision permissionDecision
			pending  bool
		}{
			{"Should allow read once", new(acpsdk.ToolKindRead), "docs/a.md", decisionAllowOnce, false},
			{"Should allow a contained edit once", new(acpsdk.ToolKindEdit), "docs/a.md", decisionAllowOnce, false},
			{"Should ask for an edit without locations", new(acpsdk.ToolKindEdit), "", decisionPending, true},
			{"Should ask for command execution", new(acpsdk.ToolKindExecute), "", decisionPending, true},
			{"Should ask for kind other", new(acpsdk.ToolKindOther), "", decisionPending, true},
			{"Should ask for unknown operations", new(acpsdk.ToolKind("unknown")), "", decisionPending, true},
			{"Should ask for a missing kind", nil, "docs/a.md", decisionPending, true},
		} {
			t.Run(string(mode)+" "+tc.name, func(t *testing.T) {
				t.Parallel()
				policy, err := newPermissionPolicy(mode, t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				request := acpsdk.RequestPermissionRequest{ToolCall: acpsdk.ToolCallUpdate{Kind: tc.kind}}
				if tc.location != "" {
					request.ToolCall.Locations = []acpsdk.ToolCallLocation{{Path: tc.location}}
				}
				decision, pending := policy.permissionDecision(request)
				if decision != tc.decision || pending != tc.pending {
					t.Fatalf("decision=%s pending=%v, want %s %v", decision, pending, tc.decision, tc.pending)
				}
			})
		}
	}
}

func TestPermissionRuleDecisions(t *testing.T) {
	t.Parallel()
	// Invariant (UT-091/095): one allowed path cannot authorize another path or a symlink escape.
	// Owning layer: ACP request permission; canonical suite: TestPermissionRuleDecisions.
	t.Run("Should require allow coverage for every location and resolve symlinks", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o700); err != nil {
			t.Fatal(err)
		}
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "docs", "escape")); err != nil {
			t.Fatal(err)
		}
		policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveReads, root)
		if err != nil {
			t.Fatal(err)
		}
		policy.rules = []compozyconfig.PermissionRule{{Kind: "edit", Path: "docs/**", Decision: "allow"}}
		for _, tc := range []struct {
			name    string
			paths   []string
			want    permissionDecision
			pending bool
		}{
			{"all allowed", []string{"docs/a.md", "docs/b.md"}, decisionAllowOnce, false},
			{"mixed locations", []string{"docs/a.md", "src/main.go"}, decisionPending, true},
			{"symlink escape", []string{"docs/escape/secret.txt"}, decisionRejectOnce, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				request := acpsdk.RequestPermissionRequest{
					ToolCall: acpsdk.ToolCallUpdate{Kind: new(acpsdk.ToolKindEdit)},
				}
				for _, path := range tc.paths {
					request.ToolCall.Locations = append(request.ToolCall.Locations, acpsdk.ToolCallLocation{Path: path})
				}
				decision, pending := policy.permissionDecision(request)
				if decision != tc.want || pending != tc.pending {
					t.Fatalf("decision=%s pending=%v", decision, pending)
				}
			})
		}
	})
	for _, tc := range []struct {
		name     string
		mode     compozyconfig.PermissionMode
		rules    []compozyconfig.PermissionRule
		location string
		want     permissionDecision
		pending  bool
	}{
		{"Should allow an edit under docs once", compozyconfig.PermissionModeApproveReads,
			[]compozyconfig.PermissionRule{{Kind: "edit", Path: "docs/**", Decision: "allow"}},
			"docs/guide.md", decisionAllowOnce, false},
		{"Should let deny beat allow", compozyconfig.PermissionModeApproveAll,
			[]compozyconfig.PermissionRule{{Kind: "edit", Decision: "allow"}, {Path: "docs/**", Decision: "deny"}},
			"docs/guide.md", decisionRejectOnce, false},
		{"Should ignore path rules without locations", compozyconfig.PermissionModeApproveReads,
			[]compozyconfig.PermissionRule{{Path: "**", Decision: "allow"}},
			"", decisionPending, true},
		{"Should preserve deny-all before allow rules", compozyconfig.PermissionModeDenyAll,
			[]compozyconfig.PermissionRule{{Kind: "edit", Decision: "allow"}},
			"docs/guide.md", decisionRejectOnce, false},
		{"Should preserve allowed-root checks before allow rules", compozyconfig.PermissionModeApproveAll,
			[]compozyconfig.PermissionRule{{Kind: "edit", Decision: "allow"}},
			"../outside.txt", decisionRejectOnce, false},
		{"Should deny a path rule regardless of letter case", compozyconfig.PermissionModeApproveAll,
			[]compozyconfig.PermissionRule{{Kind: "edit", Path: "secrets/**", Decision: "deny"}},
			"SECRETS/key", decisionRejectOnce, false},
		{"Should not widen an allow path rule by letter case", compozyconfig.PermissionModeApproveReads,
			[]compozyconfig.PermissionRule{{Kind: "edit", Path: "docs/**", Decision: "allow"}},
			"DOCS/guide.md", decisionPending, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy, err := newPermissionPolicy(tc.mode, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := compozyconfig.ValidatePermissionRules(tc.rules); err != nil {
				t.Fatal(err)
			}
			policy.rules = tc.rules
			request := acpsdk.RequestPermissionRequest{
				ToolCall: acpsdk.ToolCallUpdate{Kind: new(acpsdk.ToolKindEdit)},
			}
			if tc.location != "" {
				request.ToolCall.Locations = []acpsdk.ToolCallLocation{{Path: tc.location}}
			}
			decision, pending := policy.permissionDecision(request)
			if decision != tc.want || pending != tc.pending {
				t.Fatalf("decision=%s pending=%v, want %s %v", decision, pending, tc.want, tc.pending)
			}
		})
	}
	// Invariant (rule 49, SEC-3): rules cover what a provider asks; daemon-originated Compozy tool
	// approvals are decided by the level alone, identified by call path rather than forgeable metadata.
	t.Run("Should not let a kind-only allow rule approve a Compozy tool", func(t *testing.T) {
		t.Parallel()
		policy, err := newPermissionPolicy(compozyconfig.PermissionModeApproveEdits, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		policy.rules = []compozyconfig.PermissionRule{{Kind: "execute", Decision: "allow"}}
		request := acpsdk.RequestPermissionRequest{
			Meta:     map[string]any{PermissionToolIDMetaKey: "compozy__agent_create"},
			ToolCall: acpsdk.ToolCallUpdate{Kind: new(acpsdk.ToolKindExecute)},
		}
		if decision, pending, source := policy.permissionDecisionSource(request); decision != decisionAllowOnce ||
			pending || source != permissionSourceRule {
			t.Fatalf("provider request: decision=%s pending=%v source=%s", decision, pending, source)
		}
		decision, pending, source := policy.daemonPermissionDecisionSource(request)
		if decision != decisionPending || !pending || source != permissionSourcePolicy {
			t.Fatalf("daemon request: decision=%s pending=%v source=%s", decision, pending, source)
		}
	})
	t.Run(
		"Should route daemon-originated requests around rules and provider requests through them",
		func(t *testing.T) {
			t.Parallel()
			proc := newDirectProcess(t, compozyconfig.PermissionModeApproveAll)
			proc.permissions.rules = []compozyconfig.PermissionRule{{Kind: "delete", Decision: "deny"}}
			request := acpsdk.RequestPermissionRequest{
				SessionId: "sess-direct",
				ToolCall:  acpsdk.ToolCallUpdate{ToolCallId: "tool-delete", Kind: new(acpsdk.ToolKindDelete)},
				Options: []acpsdk.PermissionOption{
					{OptionId: "allow", Name: "Allow", Kind: acpsdk.PermissionOptionKindAllowOnce},
					{OptionId: "reject", Name: "Reject", Kind: acpsdk.PermissionOptionKindRejectOnce},
				},
			}
			provider, err := proc.handleRequestPermission(t.Context(), request)
			if err != nil || provider.Outcome.Selected == nil || provider.Outcome.Selected.OptionId != "reject" {
				t.Fatalf("provider outcome=%+v err=%v, want the deny rule", provider.Outcome, err)
			}
			request.Meta = map[string]any{PermissionToolIDMetaKey: "compozy__session_stop"}
			request.ToolCall.ToolCallId = "tool-approval-delete"
			daemon, err := proc.RequestPermission(t.Context(), request)
			if err != nil || daemon.Outcome.Selected == nil || daemon.Outcome.Selected.OptionId != "allow" {
				t.Fatalf("daemon outcome=%+v err=%v, want the level decision", daemon.Outcome, err)
			}
		},
	)
}
