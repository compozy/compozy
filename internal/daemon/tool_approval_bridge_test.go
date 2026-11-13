package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	core "github.com/compozy/compozy/internal/api/core"
	apitest "github.com/compozy/compozy/internal/api/testutil"
	sessionpkg "github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/jonboulle/clockwork"
)

func TestToolApprovalBridgeDeterministicErrors(t *testing.T) {
	t.Parallel()

	view := toolApprovalTestView()

	t.Run("Should preserve run identity across terminal approval calls", func(t *testing.T) {
		t.Parallel()

		capture := &terminalApprovalCapture{}
		bridge := newTerminalPermissionBridge()
		bridge.bind(capture)
		actor := terminalpkg.Actor{
			Kind: terminalpkg.ActorKindAgent, ID: "codex", ProfileID: "profile-a",
			SessionID: "sess-a", RunID: "run-a", Generation: 7,
		}
		if _, err := bridge.AuthorizeTerminalExec(t.Context(), terminalpkg.ExecRequest{
			WS: "workspace-a", Command: "printf", Actor: actor,
		}, terminalpkg.CommandClassification{}); err != nil {
			t.Fatalf("AuthorizeTerminalExec() error = %v", err)
		}
		if len(capture.calls) != 1 {
			t.Fatalf("approval calls = %#v, want one exec call", capture.calls)
		}
		for _, call := range capture.calls {
			if call.scope.RunID != "run-a" || call.scope.Generation != 7 ||
				call.request.RunID != "run-a" || call.request.Generation != 7 {
				t.Fatalf("approval identity = %#v, want run-a generation 7", call)
			}
			if call.view.Descriptor.ID != toolspkg.ToolIDTerminalExec ||
				call.view.Descriptor.Risk != toolspkg.RiskDestructive ||
				!slices.Equal(call.view.Descriptor.Backend.RequiresCapabilities, []string{"terminal.exec"}) {
				t.Fatalf(
					"terminal approval descriptor = %#v, want terminal exec approval metadata",
					call.view.Descriptor,
				)
			}
		}
	})

	t.Run("Should type explicit terminal exec approval rejections", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalRejectOnceID)
		approval := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		terminalApproval := newTerminalPermissionBridge()
		terminalApproval.bind(approval)
		actor := terminalpkg.Actor{
			Kind: terminalpkg.ActorKindAgent, ID: "codex", ProfileID: "profile-a",
			SessionID: "sess-a", RunID: "run-a", Generation: 7,
		}
		_, err := terminalApproval.AuthorizeTerminalExec(t.Context(), terminalpkg.ExecRequest{
			WS: "workspace-a", Command: "printf", Actor: actor,
		}, terminalpkg.CommandClassification{})
		execErr, execTyped := errors.AsType[*terminalpkg.Error](err)
		if !execTyped || execErr.Code != terminalpkg.ErrorCodeApprovalRejected ||
			!errors.Is(err, terminalpkg.ErrApprovalRejected) {
			t.Fatalf("AuthorizeTerminalExec(rejected) error = %v, want approval_rejected", err)
		}
	})

	t.Run("Should return approval_unreachable without a permission channel", func(t *testing.T) {
		t.Parallel()

		bridge := newToolApprovalBridge(nil, time.Second, nil, nil, nil)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalUnreachable)
	})

	t.Run("Should return approval_timed_out when ACP permission request exceeds timeout", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			fn: func(ctx context.Context, _ string, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
				<-ctx.Done()
				return acp.RequestPermissionResponse{}, ctx.Err()
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Nanosecond,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalTimedOut)
		for _, text := range []string{"nobody answered", "Elapsed time is not approval", "Do not repeat", "unaffected work"} {
			if !strings.Contains(err.Error(), text) {
				t.Fatalf("timeout error %q does not explain %q", err, text)
			}
		}
	})

	t.Run(
		"Should keep terminal approval pending beyond four minutes and stop at an authorized end",
		func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name       string
				toolID     toolspkg.ToolID
				finish     func(*clockwork.FakeClock, context.CancelFunc)
				wantReason toolspkg.ReasonCode
			}{
				{
					name:   "Should expire terminal exec at the terminal approval limit",
					toolID: toolspkg.ToolIDTerminalExec,
					finish: func(clock *clockwork.FakeClock, _ context.CancelFunc) {
						clock.Advance(11 * time.Minute)
					},
					wantReason: toolspkg.ReasonApprovalTimedOut,
				},
				{
					name:   "Should stop another terminal approval when the caller cancels",
					toolID: toolspkg.ToolIDTerminalOpen,
					finish: func(_ *clockwork.FakeClock, cancel context.CancelFunc) {
						cancel()
					},
					wantReason: toolspkg.ReasonApprovalCanceled,
				},
			}
			for _, test := range tests {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()

					clock := clockwork.NewFakeClock()
					started := make(chan struct{})
					stopped := make(chan struct{})
					requester := permissionRequesterFunc(func(
						ctx context.Context,
						_ string,
						_ acp.RequestPermissionRequest,
					) (acp.RequestPermissionResponse, error) {
						close(started)
						<-ctx.Done()
						close(stopped)
						return acp.RequestPermissionResponse{}, ctx.Err()
					})
					bridge := newToolApprovalBridge(
						func() sessionPermissionRequester { return requester },
						2*time.Minute,
						nil,
						nil,
						nil,
					)
					bridge.clock = clock
					view := toolApprovalTestView()
					view.Descriptor.ID = test.toolID
					call := toolApprovalTestCall(test.toolID, "ws-1")
					if test.toolID == toolspkg.ToolIDTerminalExec {
						call.Input = json.RawMessage(`{"command":"bun","args":["test"]}`)
					}
					ctx, cancel := context.WithCancel(t.Context())
					t.Cleanup(cancel)
					result := make(chan error, 1)
					go func() {
						result <- bridge.RequestToolApproval(ctx, toolspkg.Scope{}, &call, &view)
					}()
					<-started

					clock.Advance(4 * time.Minute)
					select {
					case err := <-result:
						t.Fatalf("terminal approval ended after four minutes: %v", err)
					default:
					}

					test.finish(clock, cancel)
					err := <-result
					requireToolApprovalReason(t, err, test.wantReason)
					select {
					case <-stopped:
					default:
						t.Fatal("permission requester remained blocked after approval ended")
					}
				})
			}
		},
	)

	t.Run("Should return approval_timed_out when the caller deadline has expired", func(t *testing.T) {
		t.Parallel()

		requester := permissionRequesterFunc(func(
			ctx context.Context,
			_ string,
			_ acp.RequestPermissionRequest,
		) (acp.RequestPermissionResponse, error) {
			return acp.RequestPermissionResponse{}, ctx.Err()
		})
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		ctx, cancel := context.WithDeadline(t.Context(), time.Unix(1, 0))
		t.Cleanup(cancel)
		err := bridge.RequestToolApproval(
			ctx,
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalTimedOut)
	})

	// Owner: native approval bridge; a suppressed expired request retains its timeout reason.
	t.Run("Should distinguish an expired retry from an explicit denial", func(t *testing.T) {
		t.Parallel()
		requester := permissionRequesterFunc(func(
			context.Context, string, acp.RequestPermissionRequest,
		) (acp.RequestPermissionResponse, error) {
			return acp.RequestPermissionResponse{}, acp.ErrPermissionExpired
		})
		bridge := newToolApprovalBridge(func() sessionPermissionRequester { return requester }, time.Second, nil, nil, nil)
		err := bridge.RequestToolApproval(t.Context(), toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}), &view)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalTimedOut)
		if !strings.Contains(err.Error(), "operator to prompt the session") {
			t.Fatalf("expired retry guidance = %v", err)
		}
	})

	t.Run("Should return approval_canceled when caller context is canceled", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			fn: func(ctx context.Context, _ string, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
				return acp.RequestPermissionResponse{}, ctx.Err()
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		err := bridge.RequestToolApproval(
			ctx,
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalCanceled)
	})
	t.Run("Should retain the timeout reason when ACP returns its canceled outcome", func(t *testing.T) {
		t.Parallel()
		requester := permissionRequesterFunc(func(
			ctx context.Context,
			_ string,
			_ acp.RequestPermissionRequest,
		) (acp.RequestPermissionResponse, error) {
			<-ctx.Done()
			return acp.RequestPermissionResponse{Outcome: acpsdk.NewRequestPermissionOutcomeCancelled()}, nil
		})
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester }, time.Second, nil, nil, nil,
		)
		ctx, cancel := context.WithDeadline(t.Context(), time.Unix(1, 0))
		defer cancel()
		err := bridge.RequestToolApproval(
			ctx,
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalTimedOut)
	})

	t.Run("Should return approval_canceled when ACP returns canceled outcome", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			response: acp.RequestPermissionResponse{Outcome: acpsdk.NewRequestPermissionOutcomeCancelled()},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalCanceled)
	})
}

type terminalApprovalCaptureCall struct {
	scope   toolspkg.Scope
	request toolspkg.CallRequest
	view    toolspkg.ToolView
}

type terminalApprovalCapture struct {
	calls []terminalApprovalCaptureCall
	err   error
}

func (c *terminalApprovalCapture) RequestToolApproval(
	_ context.Context,
	scope toolspkg.Scope,
	request *toolspkg.CallRequest,
	view *toolspkg.ToolView,
) error {
	request.ApprovalLabel = toolApprovalApprovedOnceLabel
	c.calls = append(c.calls, terminalApprovalCaptureCall{scope: scope, request: *request, view: *view})
	return c.err
}

func BenchmarkTerminalApprovalDescriptor(b *testing.B) {
	for _, id := range []toolspkg.ToolID{toolspkg.ToolIDTerminalExec, toolspkg.ToolIDTerminalRequestInput} {
		b.Run(string(id), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				descriptor := terminalToolDescriptor(id)
				if descriptor.ID != id {
					b.Fatalf("terminal descriptor ID = %q, want %q", descriptor.ID, id)
				}
			}
		})
	}
}

func TestWorkspaceAccessPromptBridgeSessionConsent(t *testing.T) {
	t.Parallel()

	descriptor := toolApprovalTestView().Descriptor
	call := toolApprovalTestCall(descriptor.ID, "ws-a")
	scope := toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-a", AgentName: "codex"}

	t.Run("Should allow once without caching and prompt again", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(workspaceAccessAllowOnceID)
		cache := newWorkspaceAccessConsentCache()
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			cache,
			nil,
			nil,
		)
		for attempt := range 2 {
			allowed, err := bridge.RequestWorkspaceAccess(
				t.Context(),
				scope,
				call,
				descriptor,
				workspaceAccessPrompt{label: "ws-b"},
			)
			if err != nil || !allowed {
				t.Fatalf("RequestWorkspaceAccess(%d) = %v, %v, want allowed", attempt, allowed, err)
			}
		}
		if _, ok := cache.ConsentFor(t.Context(), "sess-1"); ok {
			t.Fatal("allow_once wrote session consent")
		}
		if len(requester.requests) != 2 {
			t.Fatalf("permission requests = %d, want 2", len(requester.requests))
		}
	})

	for _, test := range []struct {
		name        string
		option      acpsdk.PermissionOptionId
		wantAllowed bool
		wantConsent workspaceaccess.Consent
	}{
		{name: "Should cache allow_session", option: workspaceAccessAllowSessionID, wantAllowed: true, wantConsent: workspaceaccess.ConsentAllow},
		{name: "Should cache reject_session", option: workspaceAccessRejectSessionID, wantConsent: workspaceaccess.ConsentReject},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			requester := selectedPermissionRequester(test.option)
			cache := newWorkspaceAccessConsentCache()
			bridge := newWorkspaceAccessPromptBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				cache,
				nil,
				nil,
			)
			allowed, err := bridge.RequestWorkspaceAccess(
				t.Context(),
				scope,
				call,
				descriptor,
				workspaceAccessPrompt{label: "ws-b"},
			)
			if err != nil {
				t.Fatalf("RequestWorkspaceAccess() error = %v", err)
			}
			if allowed != test.wantAllowed {
				t.Fatalf("allowed = %v, want %v", allowed, test.wantAllowed)
			}
			consent, ok := cache.ConsentFor(t.Context(), "sess-1")
			if !ok || consent != test.wantConsent {
				t.Fatalf("consent = %q, %v, want %q", consent, ok, test.wantConsent)
			}
			request := requester.lastRequest(t)
			if request.ToolCall.Kind != nil {
				t.Fatalf(
					"workspace permission tool kind = %q, want nil to force the interactive path",
					*request.ToolCall.Kind,
				)
			}
			gotIDs := make([]acpsdk.PermissionOptionId, 0, len(request.Options))
			for _, option := range request.Options {
				gotIDs = append(gotIDs, option.OptionId)
			}
			wantIDs := []acpsdk.PermissionOptionId{
				workspaceAccessAllowOnceID,
				workspaceAccessAllowSessionID,
				workspaceAccessRejectOnceID,
				workspaceAccessRejectSessionID,
			}
			if !slices.Equal(gotIDs, wantIDs) {
				t.Fatalf("permission option ids = %#v, want %#v", gotIDs, wantIDs)
			}
		})
	}
}

func TestWorkspaceAccessPromptBridgeFailurePaths(t *testing.T) {
	t.Parallel()

	descriptor := toolApprovalTestView().Descriptor
	call := toolApprovalTestCall(descriptor.ID, "ws-a")
	scope := toolspkg.Scope{SessionID: "sess-1", WorkspaceID: "ws-a"}

	t.Run("Should retain session consent when a resolved read has no durable owner", func(t *testing.T) {
		t.Parallel()
		requester := selectedPermissionRequester(workspaceAccessAllowSessionID)
		cache := newWorkspaceAccessConsentCache()
		grants := &recordingApprovalGrantStore{}
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester }, time.Second, cache, grants, nil,
		)
		readDescriptor := descriptor
		readDescriptor.ReadOnly = true
		allowed, err := bridge.RequestWorkspaceAccess(t.Context(), scope, call, readDescriptor,
			workspaceAccessPrompt{label: "ws-b", remember: true})
		if err != nil || !allowed {
			t.Fatalf("RequestWorkspaceAccess() = %t, %v, want session consent", allowed, err)
		}
		if option := requester.requests[0].Options[1]; option.OptionId != workspaceAccessAllowSessionID {
			t.Fatalf("read option = %q, want temporary consent without a durable owner", option.OptionId)
		}
		if consent, ok := cache.ConsentFor(
			t.Context(),
			scope.SessionID,
		); !ok ||
			consent != workspaceaccess.ConsentAllow {
			t.Fatalf("cached consent = %q, %t, want session allow", consent, ok)
		}
		if len(grants.grants) != 0 {
			t.Fatalf("stored grants = %#v, want none without an owner", grants.grants)
		}
	})

	t.Run("Should identify the workspace and profile covered by read consent", func(t *testing.T) {
		t.Parallel()
		requester := selectedPermissionRequester(workspaceAccessAllowOnceID)
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester }, time.Second,
			newWorkspaceAccessConsentCache(), nil, nil,
		)
		readDescriptor := descriptor
		readDescriptor.ReadOnly = true
		allowed, err := bridge.RequestWorkspaceAccess(t.Context(), scope, call, readDescriptor,
			workspaceAccessPrompt{label: nativeAccessPromptProfilePrefix + "p-target", targetWorkspace: "ws-target"})
		if err != nil || !allowed {
			t.Fatalf("RequestWorkspaceAccess() = %t, %v", allowed, err)
		}
		request := requester.requests[0]
		if request.ToolCall.Title == nil || *request.ToolCall.Title != "Read profile p-target in workspace ws-target" {
			t.Fatalf("prompt title = %v, want both target workspace and profile", request.ToolCall.Title)
		}
		input, ok := request.ToolCall.RawInput.(map[string]any)
		if !ok || input["target_profile"] != "p-target" || input["target_workspace_id"] != "ws-target" {
			t.Fatalf("prompt input = %#v, want both target workspace and profile", request.ToolCall.RawInput)
		}
	})
	t.Run("Should identify the workspace and profile covered by User read consent", func(t *testing.T) {
		t.Parallel()
		requester := selectedPermissionRequester(workspaceAccessAllowOnceID)
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester }, time.Second,
			newWorkspaceAccessConsentCache(), nil, nil,
		)
		readDescriptor := descriptor
		readDescriptor.ReadOnly = true
		allowed, err := bridge.RequestWorkspaceAccess(
			t.Context(),
			scope,
			call,
			readDescriptor,
			workspaceAccessPrompt{
				label:           nativeAccessPromptProfilePrefix + "p-target",
				targetWorkspace: workspaceaccess.UserSelector,
			},
		)
		if err != nil || !allowed {
			t.Fatalf("RequestWorkspaceAccess() = %t, %v", allowed, err)
		}
		request := requester.requests[0]
		if request.ToolCall.Title == nil || *request.ToolCall.Title != "Read profile p-target in User scope" {
			t.Fatalf("prompt title = %v, want both target workspace and profile", request.ToolCall.Title)
		}
		input, ok := request.ToolCall.RawInput.(map[string]any)
		if !ok || input["target_profile"] != "p-target" ||
			input["target_workspace_id"] != workspaceaccess.UserSelector {
			t.Fatalf("prompt input = %#v, want both target workspace and profile", request.ToolCall.RawInput)
		}
	})

	t.Run("Should survive originating request cancellation within its own deadline", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(workspaceAccessAllowOnceID)
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			newWorkspaceAccessConsentCache(),
			nil,
			nil,
		)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		allowed, err := bridge.RequestWorkspaceAccess(
			ctx, scope, call, descriptor, workspaceAccessPrompt{label: "ws-b"},
		)
		if err != nil || !allowed {
			t.Fatalf("RequestWorkspaceAccess(canceled origin) = %v, %v, want allowed", allowed, err)
		}
	})

	t.Run("Should deny an Always allow that cannot be remembered", func(t *testing.T) {
		t.Parallel()

		key := toolspkg.ApprovalGrantKey{
			ProfileID: "p-1", WorkspaceID: "ws-a", AgentName: "codex", ToolID: descriptor.ID,
			InputDigest: "sha256:00", AccessScope: "read:workspace:ws-b@p-1",
		}
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return selectedPermissionRequester(workspaceAccessAllowAlwaysID) },
			time.Second,
			newWorkspaceAccessConsentCache(),
			&recordingApprovalGrantStore{putErr: errors.New("disk full")},
			nil,
		)
		readDescriptor := descriptor
		readDescriptor.ReadOnly = true
		allowed, err := bridge.RequestWorkspaceAccess(
			t.Context(), scope, call, readDescriptor,
			workspaceAccessPrompt{label: "ws-b", remember: true, grant: &key},
		)
		if err == nil || allowed || !strings.Contains(err.Error(), "remember workspace access grant") {
			t.Fatalf("RequestWorkspaceAccess() = %v, %v, want a remember failure denial", allowed, err)
		}
	})

	t.Run("Should deny timeout and unknown answers without caching", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name      string
			requester *recordingPermissionRequester
			timeout   time.Duration
			wantError string
		}{
			{
				name: "Should report a timeout",
				requester: &recordingPermissionRequester{fn: func(
					ctx context.Context,
					_ string,
					_ acp.RequestPermissionRequest,
				) (acp.RequestPermissionResponse, error) {
					<-ctx.Done()
					return acp.RequestPermissionResponse{}, ctx.Err()
				}},
				timeout:   time.Nanosecond,
				wantError: "workspace access prompt timed out",
			},
			{
				name:      "Should reject an unknown answer",
				requester: selectedPermissionRequester("unknown"),
				timeout:   time.Second,
				wantError: "selected an unknown option",
			},
			{
				name: "Should preserve a malformed ACP outcome",
				requester: &recordingPermissionRequester{response: acp.RequestPermissionResponse{
					Outcome: acpsdk.RequestPermissionOutcome{},
				}},
				timeout:   time.Second,
				wantError: "must have exactly one variant set",
			},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				cache := newWorkspaceAccessConsentCache()
				bridge := newWorkspaceAccessPromptBridge(
					func() sessionPermissionRequester { return test.requester },
					test.timeout,
					cache,
					nil,
					nil,
				)
				allowed, err := bridge.RequestWorkspaceAccess(
					t.Context(),
					scope,
					call,
					descriptor,
					workspaceAccessPrompt{label: "ws-b"},
				)
				if err == nil || allowed {
					t.Fatalf("RequestWorkspaceAccess() = %v, %v, want denial error", allowed, err)
				}
				if !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("RequestWorkspaceAccess() error = %v, want %q", err, test.wantError)
				}
				if _, ok := cache.ConsentFor(t.Context(), "sess-1"); ok {
					t.Fatal("failed prompt wrote session consent")
				}
			})
		}
	})
}

func TestToolApprovalBridgeRoutesAllowAndRejectOutcomes(t *testing.T) {
	t.Parallel()

	view := toolApprovalTestView()

	t.Run("Should allow selected allow option", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			response: acp.RequestPermissionResponse{
				Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalAllowOnceID),
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{
				ToolID:      view.Descriptor.ID,
				ToolCallID:  "call-1",
				SessionID:   "sess-1",
				WorkspaceID: "ws-1",
				AgentName:   "codex",
				Input:       []byte(`{"message":"hello"}`),
			}),
			&view,
		)
		if err != nil {
			t.Fatalf("RequestToolApproval() error = %v, want nil", err)
		}
		request := requester.lastRequest(t)
		if request.ToolCall.ToolCallId != "call-1" || request.SessionId != "sess-1" {
			t.Fatalf("permission request = %#v, want hosted call context", request)
		}
		if got := request.Meta[acp.PermissionRequestIDMetaKey]; got != "call-1" {
			t.Fatalf("permission request ID metadata = %#v, want call-1", got)
		}
		if got, want := len(request.Options), 4; got != want {
			t.Fatalf("permission options = %#v, want %d options", request.Options, want)
		}
	})

	t.Run("Should isolate repeated fallback approvals in one session", func(t *testing.T) {
		t.Parallel()

		pending := make(chan acp.RequestPermissionRequest, 2)
		var waitersMu sync.Mutex
		waiters := make(map[string][]chan acp.RequestPermissionResponse)
		requester := permissionRequesterFunc(func(
			ctx context.Context,
			_ string,
			request acp.RequestPermissionRequest,
		) (acp.RequestPermissionResponse, error) {
			requestID, _ := request.Meta[acp.PermissionRequestIDMetaKey].(string)
			response := make(chan acp.RequestPermissionResponse, 1)
			waitersMu.Lock()
			waiters[requestID] = append(waiters[requestID], response)
			waitersMu.Unlock()
			pending <- request
			select {
			case result := <-response:
				return result, nil
			case <-ctx.Done():
				return acp.RequestPermissionResponse{}, ctx.Err()
			}
		})
		resolve := func(requestID string, response acp.RequestPermissionResponse) {
			waitersMu.Lock()
			requestWaiters := waiters[requestID]
			delete(waiters, requestID)
			waitersMu.Unlock()
			for _, waiter := range requestWaiters {
				waiter <- response
			}
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		call.ToolCallID = ""
		call.CorrelationID = ""
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		t.Cleanup(cancel)

		firstCall := call
		secondCall := call
		firstResult := make(chan error, 1)
		go func() {
			firstResult <- bridge.RequestToolApproval(ctx, toolspkg.Scope{}, &firstCall, &view)
		}()
		first := <-pending

		secondResult := make(chan error, 1)
		go func() {
			secondResult <- bridge.RequestToolApproval(ctx, toolspkg.Scope{}, &secondCall, &view)
		}()
		second := <-pending

		firstID, _ := first.Meta[acp.PermissionRequestIDMetaKey].(string)
		secondID, _ := second.Meta[acp.PermissionRequestIDMetaKey].(string)
		if firstID == "" || secondID == "" || firstID == secondID {
			t.Errorf("fallback permission request IDs = %q, %q, want distinct non-empty IDs", firstID, secondID)
		}
		if got := string(first.ToolCall.ToolCallId); got != firstID {
			t.Errorf("first tool call ID = %q, want request ID %q", got, firstID)
		}
		if got := string(second.ToolCall.ToolCallId); got != secondID {
			t.Errorf("second tool call ID = %q, want request ID %q", got, secondID)
		}

		resolve(firstID, acp.RequestPermissionResponse{
			Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalAllowOnceID),
		})
		if err := <-firstResult; err != nil {
			t.Fatalf("RequestToolApproval(first) error = %v, want nil", err)
		}
		select {
		case err := <-secondResult:
			t.Fatalf("second approval completed after resolving the first: %v", err)
		default:
		}

		resolve(secondID, acp.RequestPermissionResponse{
			Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalRejectOnceID),
		})
		requireToolApprovalReason(t, <-secondResult, toolspkg.ReasonApprovalRequired)
	})

	t.Run("Should reject selected reject option", func(t *testing.T) {
		t.Parallel()

		requester := &recordingPermissionRequester{
			response: acp.RequestPermissionResponse{
				Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalRejectOnceID),
			},
		}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			nil,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{SessionID: "sess-1"},
			new(toolspkg.CallRequest{ToolID: view.Descriptor.ID, Input: []byte(`{}`)}),
			&view,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalRequired)
	})
}

func TestToolApprovalBridgePersistsDurableOutcomes(t *testing.T) {
	t.Parallel()

	view := toolApprovalTestView()

	t.Run("Should prompt again for shell command strings despite a durable allow", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input json.RawMessage
		}{
			{
				name:  "Should ignore a grant for combined POSIX options through sudo",
				input: json.RawMessage(`{"command":"sudo","args":["bash","-ec","echo hidden"]}`),
			},
			{
				name:  "Should ignore a grant for encoded PowerShell through env",
				input: json.RawMessage(`{"command":"env","args":["CI=1","pwsh","-ENC","SQBFAFgA"]}`),
			},
			{
				name:  "Should ignore a grant for mixed case cmd options through env",
				input: json.RawMessage(`{"command":"env","args":["ComSpec=cmd.exe","cmd.exe","/C","echo hidden"]}`),
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				terminalView := toolApprovalTestView()
				terminalView.Descriptor.ID = toolspkg.ToolIDTerminalExec
				call := toolApprovalTestCall(toolspkg.ToolIDTerminalExec, "ws-1")
				call.Input = test.input
				scope := toolspkg.Scope{ProfileID: store.DefaultProfileID}
				key, err := toolApprovalGrantKey(scope, call, toolspkg.ToolIDTerminalExec)
				if err != nil {
					t.Fatalf("toolApprovalGrantKey() error = %v", err)
				}
				grants := &recordingApprovalGrantStore{grants: []toolspkg.ApprovalGrant{
					materializedApprovalGrant("grant-terminal", key, toolspkg.ApprovalGrantAllow),
				}}
				requester := selectedPermissionRequester(toolApprovalAllowOnceID)
				bridge := newToolApprovalBridge(
					func() sessionPermissionRequester { return requester },
					time.Second,
					nil,
					grants,
					nil,
				)
				if err := bridge.RequestToolApproval(t.Context(), scope, &call, &terminalView); err != nil {
					t.Fatalf("RequestToolApproval() error = %v", err)
				}
				if got := len(requester.requests); got != 1 {
					t.Fatalf("permission requests = %d, want 1 despite durable allow", got)
				}
				request := requester.lastRequest(t)
				if got := request.Meta[acp.PermissionToolIDMetaKey]; got != toolspkg.ToolIDTerminalExec.String() {
					t.Fatalf("permission tool id metadata = %#v, want %q", got, toolspkg.ToolIDTerminalExec)
				}
				if call.ApprovalLabel != "approved_once" {
					t.Fatalf("approval label = %q, want approved_once", call.ApprovalLabel)
				}
				if got := request.Options; len(got) != 2 || got[0].OptionId != toolApprovalAllowOnceID ||
					got[1].OptionId != toolApprovalRejectOnceID {
					t.Fatalf("unclassifiable options = %#v, want allow-once and reject-once", got)
				}
			})
		}
	})

	t.Run("Should offer no durable choice for a wrapped irreversible terminal command", func(t *testing.T) {
		t.Parallel()

		terminalView := toolApprovalTestView()
		terminalView.Descriptor.ID = toolspkg.ToolIDTerminalExec
		call := toolApprovalTestCall(toolspkg.ToolIDTerminalExec, "ws-1")
		call.Input = json.RawMessage(
			`{"command":"sudo","args":["-u","root","rm","-rf","/var/lib/atlas/journal-backups"]}`,
		)
		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			&recordingApprovalGrantStore{},
			nil,
		)
		if err := bridge.RequestToolApproval(
			t.Context(), toolspkg.Scope{ProfileID: store.DefaultProfileID}, &call, &terminalView,
		); err != nil {
			t.Fatalf("RequestToolApproval() error = %v", err)
		}
		options := requester.lastRequest(t).Options
		if len(options) != 2 || options[0].OptionId != toolApprovalAllowOnceID ||
			options[1].OptionId != toolApprovalRejectOnceID {
			t.Fatalf("irreversible options = %#v, want allow-once and reject-once", options)
		}
	})

	t.Run("Should ignore wider terminal exec grants returned by the store", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			toolID toolspkg.ToolID
			input  json.RawMessage
		}{{toolID: toolspkg.ToolIDTerminalExec, input: json.RawMessage(`{"command":"bun","args":["test"]}`)}}
		for _, testCase := range cases {
			terminalView := toolApprovalTestView()
			terminalView.Descriptor.ID = testCase.toolID
			call := toolApprovalTestCall(testCase.toolID, "ws-1")
			call.Input = testCase.input
			broad := materializedApprovalGrant("grant-broad", toolspkg.ApprovalGrantKey{
				ProfileID: store.DefaultProfileID, WorkspaceID: "ws-1", AgentName: "codex",
				ToolID: testCase.toolID,
			}, toolspkg.ApprovalGrantAllow)
			requester := selectedPermissionRequester(toolApprovalAllowOnceID)
			bridge := newToolApprovalBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				nil,
				&recordingApprovalGrantStore{lookupGrant: &broad},
				nil,
			)
			if err := bridge.RequestToolApproval(
				t.Context(), toolspkg.Scope{ProfileID: store.DefaultProfileID}, &call, &terminalView,
			); err != nil {
				t.Fatalf("RequestToolApproval(%s) error = %v", testCase.toolID, err)
			}
			if len(requester.requests) != 1 {
				t.Fatalf(
					"permission requests for %s = %d, want prompt after wider grant",
					testCase.toolID,
					len(requester.requests),
				)
			}
		}
	})

	t.Run("Should reject an unavailable durable answer for an irreversible command", func(t *testing.T) {
		t.Parallel()

		terminalView := toolApprovalTestView()
		terminalView.Descriptor.ID = toolspkg.ToolIDTerminalExec
		call := toolApprovalTestCall(toolspkg.ToolIDTerminalExec, "ws-1")
		call.Input = json.RawMessage(`{"command":"rm","args":["-rf","/var/lib/atlas/journal-backups"]}`)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return selectedPermissionRequester(toolApprovalAllowAlwaysID) },
			time.Second,
			nil,
			grants,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(), toolspkg.Scope{ProfileID: store.DefaultProfileID}, &call, &terminalView,
		)
		requireToolApprovalReason(t, err, toolspkg.ReasonApprovalUnreachable)
		if len(grants.grants) != 0 {
			t.Fatalf("durable grants = %#v, want none", grants.grants)
		}
	})

	t.Run("Should digest terminal command shape", func(t *testing.T) {
		t.Parallel()

		scope := toolspkg.Scope{ProfileID: store.DefaultProfileID}
		first := toolApprovalTestCall(toolspkg.ToolIDTerminalExec, "ws-1")
		first.Input = json.RawMessage(
			`{"command":"bun","args":["test"],"cwd":"web","env":{"CI":"1"},"visible":true,"yield_ms":1000}`,
		)
		second := first
		second.Input = json.RawMessage(
			`{"yield_ms":30000,"output":{"max_bytes":10},"env":{"CI":"1"},"cwd":"web","args":["test"],"command":"bun","visible":false}`,
		)
		changed := first
		changed.Input = json.RawMessage(`{"command":"bun","args":["test","unit"],"cwd":"web","env":{"CI":"1"}}`)
		firstKey, err := toolApprovalGrantKey(scope, first, toolspkg.ToolIDTerminalExec)
		if err != nil {
			t.Fatalf("toolApprovalGrantKey(first) error = %v", err)
		}
		secondKey, err := toolApprovalGrantKey(scope, second, toolspkg.ToolIDTerminalExec)
		if err != nil {
			t.Fatalf("toolApprovalGrantKey(second) error = %v", err)
		}
		changedKey, err := toolApprovalGrantKey(scope, changed, toolspkg.ToolIDTerminalExec)
		if err != nil {
			t.Fatalf("toolApprovalGrantKey(changed) error = %v", err)
		}
		if firstKey.InputDigest != secondKey.InputDigest || firstKey.InputDigest == changedKey.InputDigest {
			t.Fatalf(
				"terminal exec digests = %q, %q, %q, want presentation-independent command shape",
				firstKey.InputDigest,
				secondKey.InputDigest,
				changedKey.InputDigest,
			)
		}
	})

	t.Run("Should preserve terminal environment in prompt-origin grant identity", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowAlwaysID)
		grants := &recordingApprovalGrantStore{}
		approval := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		terminalApproval := newTerminalPermissionBridge()
		terminalApproval.bind(approval)
		request := terminalpkg.ExecRequest{
			WS: "ws-1", Command: "bun", Args: []string{"test"}, Cwd: "web", Env: map[string]string{"CI": "1"},
			Actor: terminalpkg.Actor{
				Kind: terminalpkg.ActorKindAgent, ID: "codex", ProfileID: store.DefaultProfileID,
				SessionID: "sess-1",
			},
		}
		classification := terminalpkg.CommandClassification{
			Verdict: terminalpkg.CommandVerdictPrompt,
			Reason:  "approval_required",
		}
		for attempt := range 2 {
			if _, err := terminalApproval.AuthorizeTerminalExec(t.Context(), request, classification); err != nil {
				t.Fatalf("AuthorizeTerminalExec(same env, attempt %d) error = %v", attempt, err)
			}
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests for repeated environment = %d, want 1", got)
		}

		request.Env = map[string]string{"CI": "0"}
		if _, err := terminalApproval.AuthorizeTerminalExec(t.Context(), request, classification); err != nil {
			t.Fatalf("AuthorizeTerminalExec(changed env) error = %v", err)
		}
		if got := len(requester.requests); got != 2 {
			t.Fatalf("permission requests after environment change = %d, want 2", got)
		}
		if got := len(grants.grants); got != 2 {
			t.Fatalf("durable grants = %#v, want two environment-specific grants", grants.grants)
		}
		if grants.grants[0].InputDigest == grants.grants[1].InputDigest {
			t.Fatalf("environment-specific input digests = %q, want distinct values", grants.grants[0].InputDigest)
		}
	})

	t.Run("Should remember allow always and skip the next prompt", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowAlwaysID)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		for attempt := range 2 {
			if err := bridge.RequestToolApproval(
				t.Context(), toolspkg.Scope{ProfileID: store.DefaultProfileID}, &call, &view,
			); err != nil {
				t.Fatalf("RequestToolApproval(%d) error = %v, want nil", attempt, err)
			}
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests = %d, want 1", got)
		}
		if got := len(grants.grants); got != 1 || grants.grants[0].Decision != toolspkg.ApprovalGrantAllow {
			t.Fatalf("durable grants = %#v, want one allow", grants.grants)
		}
		if grants.grants[0].ProfileID != store.DefaultProfileID ||
			grants.grants[0].WorkspaceID != "ws-1" || grants.grants[0].AgentName != "codex" ||
			grants.grants[0].InputDigest == "" {
			t.Fatalf("durable grant key = %#v, want exact prompt context", grants.grants[0].ApprovalGrantKey)
		}
	})

	t.Run("Should remember reject always and auto deny the next call", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalRejectAlwaysID)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		for attempt := range 2 {
			err := bridge.RequestToolApproval(
				t.Context(), toolspkg.Scope{ProfileID: store.DefaultProfileID}, &call, &view,
			)
			requireToolApprovalReason(t, err, toolspkg.ReasonApprovalRequired)
			if len(requester.requests) != 1 {
				t.Fatalf("permission requests after attempt %d = %d, want 1", attempt, len(requester.requests))
			}
		}
		if got := len(grants.grants); got != 1 || grants.grants[0].Decision != toolspkg.ApprovalGrantReject {
			t.Fatalf("durable grants = %#v, want one reject", grants.grants)
		}
	})

	t.Run("Should keep allow once one shot and prompt again", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		grants := &recordingApprovalGrantStore{}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		for attempt := range 2 {
			if err := bridge.RequestToolApproval(
				t.Context(), toolspkg.Scope{ProfileID: store.DefaultProfileID}, &call, &view,
			); err != nil {
				t.Fatalf("RequestToolApproval(%d) error = %v, want nil", attempt, err)
			}
		}
		if got := len(requester.requests); got != 2 {
			t.Fatalf("permission requests = %d, want 2", got)
		}
		if len(grants.grants) != 0 {
			t.Fatalf("durable grants = %#v, want none", grants.grants)
		}
	})

	t.Run("Should fail open to the prompt when durable lookup fails", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		grants := &recordingApprovalGrantStore{lookupErr: errors.New("database unavailable")}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		if err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{ProfileID: store.DefaultProfileID},
			new(toolApprovalTestCall(view.Descriptor.ID, "ws-1")),
			&view,
		); err != nil {
			t.Fatalf("RequestToolApproval() error = %v, want prompt fallback", err)
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests = %d, want 1", got)
		}
	})

	t.Run("Should never reuse a grant across workspaces", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		grants := &recordingApprovalGrantStore{}
		callA := toolApprovalTestCall(view.Descriptor.ID, "ws-a")
		keyA, err := toolApprovalGrantKey(
			toolspkg.Scope{ProfileID: store.DefaultProfileID}, callA, view.Descriptor.ID,
		)
		if err != nil {
			t.Fatalf("toolApprovalGrantKey() error = %v", err)
		}
		grants.grants = append(grants.grants, materializedApprovalGrant("grant-a", keyA, toolspkg.ApprovalGrantAllow))
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		if err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{ProfileID: store.DefaultProfileID},
			new(toolApprovalTestCall(view.Descriptor.ID, "ws-b")),
			&view,
		); err != nil {
			t.Fatalf("RequestToolApproval() error = %v, want workspace B prompt", err)
		}
		if got := len(requester.requests); got != 1 {
			t.Fatalf("permission requests = %d, want 1", got)
		}
	})

	t.Run("Should surface durable write failures as backend failures", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(toolApprovalAllowAlwaysID)
		grants := &recordingApprovalGrantStore{putErr: errors.New("disk full")}
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		err := bridge.RequestToolApproval(
			t.Context(),
			toolspkg.Scope{ProfileID: store.DefaultProfileID},
			new(toolApprovalTestCall(view.Descriptor.ID, "ws-1")),
			&view,
		)
		if !errors.Is(err, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("RequestToolApproval() error = %v, want ErrToolBackendFailed", err)
		}
		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
		if !toolErrMatched || toolErr.Code != toolspkg.ErrorCodeBackendFailed {
			t.Fatalf("RequestToolApproval() error = %#v, want backend failure envelope", err)
		}
	})
}

func requireToolApprovalReason(t *testing.T, err error, want toolspkg.ReasonCode) {
	t.Helper()

	if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
		t.Fatalf("RequestToolApproval() error = %v, want ErrToolApprovalRequired", err)
	}
	toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](err)
	if !toolErrMatched || !slices.Contains(toolErr.ReasonCodes, want) {
		t.Fatalf("approval error = %#v, want reason %q", err, want)
	}
}

func toolApprovalTestView() toolspkg.ToolView {
	return toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{
			ID:               "compozy__approval_probe",
			Backend:          toolspkg.BackendRef{Kind: toolspkg.BackendNativeGo, NativeName: "approval_probe"},
			Description:      "approval probe",
			InputSchema:      []byte(`{"type":"object"}`),
			Source:           toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: "daemon"},
			Visibility:       toolspkg.VisibilityModel,
			Risk:             toolspkg.RiskMutating,
			Destructive:      false,
			ReadOnly:         false,
			OpenWorld:        false,
			ToolPresentation: toolspkg.NewToolPresentation("Approval Probe", "", ""),
		},
		Decision: toolspkg.EffectiveToolDecision{
			VisibleToSession: true,
			Callable:         true,
			ApprovalRequired: true,
		},
	}
}

func toolApprovalTestCall(toolID toolspkg.ToolID, workspaceID string) toolspkg.CallRequest {
	return toolspkg.CallRequest{
		ToolID:      toolID,
		ToolCallID:  "call-1",
		SessionID:   "sess-1",
		WorkspaceID: workspaceID,
		AgentName:   "codex",
		Input:       []byte(`{"message":"hello"}`),
	}
}

func selectedPermissionRequester(optionID acpsdk.PermissionOptionId) *recordingPermissionRequester {
	return &recordingPermissionRequester{
		response: acp.RequestPermissionResponse{
			Outcome: acpsdk.NewRequestPermissionOutcomeSelected(optionID),
		},
	}
}

type recordingPermissionRequester struct {
	response acp.RequestPermissionResponse
	err      error
	fn       func(context.Context, string, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error)
	requests []acp.RequestPermissionRequest
}

type permissionRequesterFunc func(
	context.Context,
	string,
	acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error)

func (f permissionRequesterFunc) RequestPermission(
	ctx context.Context,
	id string,
	req acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	return f(ctx, id, req)
}

var _ sessionPermissionRequester = (*recordingPermissionRequester)(nil)

func (r *recordingPermissionRequester) RequestPermission(
	ctx context.Context,
	id string,
	req acp.RequestPermissionRequest,
) (acp.RequestPermissionResponse, error) {
	r.requests = append(r.requests, req)
	if r.fn != nil {
		return r.fn(ctx, id, req)
	}
	return r.response, r.err
}

func (r *recordingPermissionRequester) lastRequest(t *testing.T) acp.RequestPermissionRequest {
	t.Helper()

	if len(r.requests) == 0 {
		t.Fatal("RequestPermission was not invoked")
	}
	return r.requests[len(r.requests)-1]
}

type recordingApprovalGrantStore struct {
	grants      []toolspkg.ApprovalGrant
	lookupGrant *toolspkg.ApprovalGrant
	lookupErr   error
	putErr      error
}

var _ toolspkg.ApprovalGrantStore = (*recordingApprovalGrantStore)(nil)

func (s *recordingApprovalGrantStore) LookupApprovalGrant(
	_ context.Context,
	key toolspkg.ApprovalGrantKey,
) (toolspkg.ApprovalGrant, bool, error) {
	if s.lookupErr != nil {
		return toolspkg.ApprovalGrant{}, false, s.lookupErr
	}
	if s.lookupGrant != nil {
		return *s.lookupGrant, true, nil
	}
	for _, grant := range s.grants {
		if grant.ApprovalGrantKey == key {
			return grant, true, nil
		}
	}
	return toolspkg.ApprovalGrant{}, false, nil
}

func (s *recordingApprovalGrantStore) PutApprovalGrant(
	_ context.Context,
	grant toolspkg.ApprovalGrant,
) (toolspkg.ApprovalGrant, error) {
	if s.putErr != nil {
		return toolspkg.ApprovalGrant{}, s.putErr
	}
	stored := materializedApprovalGrant("grant-1", grant.ApprovalGrantKey, grant.Decision)
	for index := range s.grants {
		if s.grants[index].ApprovalGrantKey == stored.ApprovalGrantKey {
			s.grants[index] = stored
			return stored, nil
		}
	}
	s.grants = append(s.grants, stored)
	return stored, nil
}

func (s *recordingApprovalGrantStore) ListApprovalGrants(
	_ context.Context,
	readScope store.ReadScope,
	workspaceID string,
) ([]toolspkg.ApprovalGrant, error) {
	grants := make([]toolspkg.ApprovalGrant, 0, len(s.grants))
	for _, grant := range s.grants {
		if grant.WorkspaceID == workspaceID && readScope.Matches(grant.ProfileID) {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

func (s *recordingApprovalGrantStore) RevokeApprovalGrant(
	_ context.Context,
	profileID string,
	workspaceID string,
	id string,
) error {
	for index, grant := range s.grants {
		if grant.ProfileID == profileID && grant.WorkspaceID == workspaceID && grant.ID == id {
			s.grants = append(s.grants[:index], s.grants[index+1:]...)
			return nil
		}
	}
	return toolspkg.ErrApprovalGrantNotFound
}

func materializedApprovalGrant(
	id string,
	key toolspkg.ApprovalGrantKey,
	decision toolspkg.ApprovalGrantDecision,
) toolspkg.ApprovalGrant {
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	if key.ProfileID == "" {
		key.ProfileID = store.DefaultProfileID
	}
	return toolspkg.ApprovalGrant{
		ID:               id,
		ApprovalGrantKey: key,
		Decision:         decision,
		CreatedAt:        now,
		LastUsedAt:       now,
	}
}

func TestToolApprovalKind(t *testing.T) {
	t.Parallel()

	archive := toolspkg.Descriptor{ID: toolspkg.ToolIDSessionArchive, Risk: toolspkg.RiskMutating}
	cases := []struct {
		name  string
		input string
		want  acpsdk.ToolKind
	}{
		{"Should present a plain archive as an execute action", `{"session_id":"sess-1"}`, acpsdk.ToolKindExecute},
		{
			"Should present a stop-first archive as a delete action like session_stop",
			`{"session_id":"sess-1","stop":true}`,
			acpsdk.ToolKindDelete,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := toolApprovalKind(archive, json.RawMessage(tc.input)); got != tc.want {
				t.Fatalf("toolApprovalKind(%s) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestToolApprovalWaitResolver(t *testing.T) {
	t.Parallel()
	t.Run("Should recheck unattended membership for each request and preserve the fallback", func(t *testing.T) {
		t.Parallel()
		bridge := newToolApprovalBridge(nil, 90*time.Second, nil, nil, nil)
		wait := 12 * time.Hour
		bridge.permissionWaitResolver = func(id string) time.Duration {
			if id != "session-a" {
				t.Fatalf("session id = %q", id)
			}
			return wait
		}
		toolID := toolApprovalTestView().Descriptor.ID
		if got := bridge.sessionRequestTimeout("session-a", toolID); got != wait {
			t.Fatalf("wait = %v", got)
		}
		wait = time.Minute
		if got := bridge.sessionRequestTimeout("session-a", toolID); got != wait {
			t.Fatalf("updated wait = %v", got)
		}
		wait = 0
		if got := bridge.sessionRequestTimeout("session-a", toolID); got != 90*time.Second {
			t.Fatalf("fallback = %v", got)
		}
	})
}

// IT-018: native approvals use the instance deadline beyond the normal 600-second limit.
// Owner: native approval bridge; canonical suite: tool_approval_bridge_test.go.
func TestInstanceNativeApprovalBeyondDefaultTimeout(t *testing.T) {
	t.Run("Should accept native approval after eleven minutes", func(t *testing.T) {
		t.Parallel()
		clock := clockwork.NewFakeClock()
		started := make(chan struct{})
		answer := make(chan struct{})
		requester := permissionRequesterFunc(
			func(ctx context.Context, _ string, _ acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
				close(started)
				select {
				case <-ctx.Done():
					return acp.RequestPermissionResponse{}, ctx.Err()
				case <-answer:
					return acp.RequestPermissionResponse{
						Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalAllowOnceID),
					}, nil
				}
			},
		)
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Minute,
			nil,
			nil,
			nil,
		)
		bridge.clock = clock
		bridge.permissionWaitResolver = func(string) time.Duration { return 12 * time.Hour }
		view := toolApprovalTestView()
		view.Descriptor.ID = toolspkg.ToolIDTerminalExec
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		call.Input = json.RawMessage(`{"command":"bun","args":["test"]}`)
		done := make(chan error, 1)
		go func() {
			done <- bridge.RequestToolApproval(t.Context(), toolspkg.Scope{SessionID: "child"}, &call, &view)
		}()
		<-started
		clock.Advance(11 * time.Minute)
		select {
		case err := <-done:
			t.Fatalf("native approval ended before instance deadline: %v", err)
		default:
		}
		close(answer)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	})
}

func TestTerminalApprovalPreservesRunRestrictions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		inactive    bool
		run         string
		wantMessage string
	}{
		{name: "Should execute after allow once for the same active run", run: "run-a"},
		{name: "Should explain an inactive run after allow once", inactive: true, wantMessage: "terminal action requires an active prompt run"},
		{name: "Should explain a replaced run after allow once", run: "run-b", wantMessage: "terminal action belongs to a different prompt run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			approved := false
			requester := &recordingPermissionRequester{
				fn: func(context.Context, string, acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
					approved = true
					return acp.RequestPermissionResponse{
						Outcome: acpsdk.NewRequestPermissionOutcomeSelected(toolApprovalAllowOnceID),
					}, nil
				},
			}
			var approvalLog bytes.Buffer
			writer := &recordingToolEventSummaryStore{}
			bridge := newToolApprovalBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				nil,
				nil,
				slog.New(slog.NewJSONHandler(&approvalLog, nil)),
			)
			manager := &terminalNativeManagerStub{}
			deps := &daemonNativeToolsDeps{
				Terminals:  func() terminalpkg.Manager { return manager },
				Workspaces: nativeTestWorkspaceService(t),
				Sessions: &apitest.StubSessionManager{
					StatusFn: func(context.Context, string) (*sessionpkg.Info, error) {
						return &sessionpkg.Info{
							ID:                "sess-a",
							ProfileID:         "profile-a",
							WorkspaceID:       "workspace-a",
							AgentName:         "agent-a",
							RuntimeGeneration: 1,
						}, nil
					},
					ActivePromptRunFn: func(context.Context, string) (sessionpkg.PromptRunIdentity, error) {
						if approved && tc.inactive {
							return sessionpkg.PromptRunIdentity{}, sessionpkg.ErrPromptNotActive
						}
						run := "run-a"
						if approved {
							run = tc.run
						}
						return sessionpkg.PromptRunIdentity{
							SessionID:   "sess-a",
							ProfileID:   "profile-a",
							WorkspaceID: "workspace-a",
							RunID:       run,
							Generation:  1,
						}, nil
					},
				},
			}
			policy := nativeApproveAllPolicyInputs()
			policy.SystemPermissionMode = toolspkg.PermissionModeApproveReads
			registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(t, deps,
				toolspkg.NewStaticPolicyInputResolver(policy), nil, toolspkg.WithApprovalBridge(bridge),
				toolspkg.WithToolEventSink(&daemonToolEventSink{writer: writer}))
			_, err := registry.Call(t.Context(), toolspkg.Scope{
				ProfileID:   "profile-a",
				WorkspaceID: "workspace-a",
				SessionID:   "sess-a",
				AgentName:   "agent-a",
				RunID:       "run-a",
				Generation:  1,
			}, toolspkg.CallRequest{ToolID: toolspkg.ToolIDTerminalExec, ToolCallID: "call-a", Input: []byte(`{"command":"echo","args":["sensitive-command-argument"]}`)})
			if !approved || len(requester.requests) != 1 {
				t.Fatalf("approval calls = %d, want one accepted allow once", len(requester.requests))
			}
			var applied struct {
				RequestID     string `json:"request_id"`
				CorrelationID string `json:"correlation_id"`
				RunID         string `json:"run_id"`
				Approval      string `json:"approval"`
				Source        string `json:"source"`
			}
			if err := json.Unmarshal(approvalLog.Bytes(), &applied); err != nil {
				t.Fatal(err)
			}
			request := requester.lastRequest(t)
			if applied.RequestID != request.Meta[acp.PermissionRequestIDMetaKey] || applied.CorrelationID != "call-a" ||
				applied.RunID != "run-a" || applied.Approval != "approved_once" || applied.Source != "permission_response" ||
				strings.Contains(approvalLog.String(), "sensitive-command-argument") {
				t.Fatalf("approval application = %s", approvalLog.Bytes())
			}
			if len(writer.summaries) == 0 {
				t.Fatal("no execution outcome was recorded")
			}
			var outcome toolspkg.ToolCallEvent
			if err := json.Unmarshal(writer.summaries[len(writer.summaries)-1].Content, &outcome); err != nil {
				t.Fatal(err)
			}
			if outcome.CorrelationID != applied.CorrelationID || outcome.RunID != applied.RunID ||
				outcome.Generation != 1 {
				t.Fatalf("execution outcome = %#v, want approval correlation", outcome)
			}
			if tc.wantMessage == "" {
				if err != nil || manager.execCalls != 1 || manager.execRequest.Approval != "approved_once" {
					t.Fatalf("exec calls=%d approval=%q error=%v", manager.execCalls, manager.execRequest.Approval, err)
				}
				return
			}
			requireToolReason(t, err, terminalpkg.ErrRunIdentityIncomplete, toolspkg.ReasonSessionDenied)
			response := core.ToolErrorResponseForError(err, core.StatusForToolError(err), true)
			if manager.execCalls != 0 || response.Error.Code != toolspkg.ErrorCodeDenied ||
				response.Error.Message != tc.wantMessage {
				t.Fatalf("exec calls=%d error response=%#v", manager.execCalls, response)
			}
		})
	}
}

func TestToolApprovalResultViews(t *testing.T) {
	t.Parallel()
	inputs := []string{
		`{"result_view":"full","z":1,"a":"value"}`,
		`{"z":1,"result_view":"compact","a":"value"}`,
		`{"z":1,"a":"value","result_view":"full"}`,
	}
	for _, decision := range []toolspkg.ApprovalGrantDecision{toolspkg.ApprovalGrantAllow, toolspkg.ApprovalGrantReject} {
		t.Run("Should preserve pre-view exact "+string(decision)+" grants", func(t *testing.T) {
			t.Parallel()
			view := toolApprovalTestView()
			view.Descriptor.SetResultViews(&toolspkg.ResultViews{})
			scope := toolspkg.Scope{ProfileID: store.DefaultProfileID}
			original := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
			original.Input = json.RawMessage(`{"z":1,"a":"value"}`)
			key, err := toolApprovalGrantKey(scope, original, view.Descriptor.ID)
			if err != nil {
				t.Fatal(err)
			}
			grants := &recordingApprovalGrantStore{
				grants: []toolspkg.ApprovalGrant{{ApprovalGrantKey: key, Decision: decision}},
			}
			requester := selectedPermissionRequester(toolApprovalAllowOnceID)
			bridge := newToolApprovalBridge(
				func() sessionPermissionRequester { return requester },
				time.Second,
				nil,
				grants,
				nil,
			)
			for _, input := range inputs {
				call := original
				call.Input = json.RawMessage(input)
				err := bridge.RequestToolApproval(t.Context(), scope, &call, &view)
				if (err != nil) != (decision == toolspkg.ApprovalGrantReject) {
					t.Fatalf("decision=%s err=%v", decision, err)
				}
			}
			if len(requester.requests) != 0 {
				t.Fatal("matching grant prompted again")
			}
			changed := original
			changed.Input = json.RawMessage(`{"z":2,"a":"value","result_view":"full"}`)
			if err := bridge.RequestToolApproval(t.Context(), scope, &changed, &view); err != nil {
				t.Fatal(err)
			}
			if len(requester.requests) != 1 {
				t.Fatal("changed operation reused the grant")
			}
		})
	}
	t.Run("Should remember a view call under the operation digest", func(t *testing.T) {
		t.Parallel()
		view := toolApprovalTestView()
		view.Descriptor.SetResultViews(&toolspkg.ResultViews{})
		scope := toolspkg.Scope{ProfileID: store.DefaultProfileID}
		grants := &recordingApprovalGrantStore{}
		requester := selectedPermissionRequester(toolApprovalAllowAlwaysID)
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		for _, input := range inputs {
			call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
			call.Input = json.RawMessage(input)
			if err := bridge.RequestToolApproval(t.Context(), scope, &call, &view); err != nil {
				t.Fatal(err)
			}
		}
		if len(requester.requests) != 1 || len(grants.grants) != 1 {
			t.Fatalf("prompts=%d grants=%d", len(requester.requests), len(grants.grants))
		}
		digest, err := toolspkg.ApprovalInputDigest(json.RawMessage(`{"z":1,"a":"value"}`), "")
		if err != nil {
			t.Fatal(err)
		}
		if grants.grants[0].InputDigest != digest {
			t.Fatal("selector affected grant identity")
		}
	})
	t.Run("Should consume pre-view one-shot tokens for either view", func(t *testing.T) {
		t.Parallel()
		view := toolApprovalTestView()
		view.Descriptor.SetResultViews(&toolspkg.ResultViews{})
		scope := toolspkg.Scope{ProfileID: store.DefaultProfileID}
		tokens := toolspkg.NewApprovalTokenStore(time.Minute)
		bridge := newToolApprovalBridge(nil, time.Second, tokens, nil, nil)
		for _, input := range inputs {
			call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
			grant, err := tokens.CreateToolApproval(
				t.Context(),
				scope,
				toolspkg.ApprovalTokenRequest{
					ToolID:      call.ToolID,
					SessionID:   call.SessionID,
					WorkspaceID: call.WorkspaceID,
					AgentName:   call.AgentName,
					Input:       json.RawMessage(`{"z":1,"a":"value"}`),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			call.Input = json.RawMessage(input)
			call.ApprovalToken = grant.ApprovalToken
			if err := bridge.RequestToolApproval(t.Context(), scope, &call, &view); err != nil {
				t.Fatal(err)
			}
		}
	})
	t.Run("Should bind non-native approvals to their entire input", func(t *testing.T) {
		t.Parallel()
		view := toolApprovalTestView()
		view.Descriptor.Backend.Kind = toolspkg.BackendMCP
		scope := toolspkg.Scope{ProfileID: store.DefaultProfileID}
		call := toolApprovalTestCall(view.Descriptor.ID, "ws-1")
		call.Input = json.RawMessage(`{"z":1,"a":"value"}`)
		key, err := toolApprovalGrantKey(scope, call, call.ToolID)
		if err != nil {
			t.Fatal(err)
		}
		grants := &recordingApprovalGrantStore{
			grants: []toolspkg.ApprovalGrant{{ApprovalGrantKey: key, Decision: toolspkg.ApprovalGrantAllow}},
		}
		requester := selectedPermissionRequester(toolApprovalAllowOnceID)
		bridge := newToolApprovalBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			nil,
			grants,
			nil,
		)
		call.Input = json.RawMessage(inputs[0])
		if err := bridge.RequestToolApproval(t.Context(), scope, &call, &view); err != nil {
			t.Fatal(err)
		}
		if len(requester.requests) != 1 {
			t.Fatal("non-native selector was ignored")
		}
	})
}

func TestCheckInResultViewApproval(t *testing.T) {
	t.Parallel()
	t.Run("Should reuse the operation grant for check-in views", func(t *testing.T) {
		t.Parallel()
		d := toolApprovalTestView().Descriptor
		d.SetResultViews(&toolspkg.ResultViews{})
		scope := toolspkg.Scope{ProfileID: store.DefaultProfileID, WorkspaceID: "ws-1", AgentName: "codex"}
		key, err := toolApprovalGrantKey(
			scope,
			toolspkg.CallRequest{Input: json.RawMessage(`{"z":1,"a":"value"}`)},
			d.ID,
		)
		if err != nil {
			t.Fatal(err)
		}
		grants := &recordingApprovalGrantStore{
			grants: []toolspkg.ApprovalGrant{{ApprovalGrantKey: key, Decision: toolspkg.ApprovalGrantAllow}},
		}
		resolver := &nativeToolPolicyResolver{approvalGrants: grants}
		for _, input := range []string{`{"result_view":"full","z":1,"a":"value"}`, `{"z":1,"result_view":"compact","a":"value"}`, `{"z":1,"a":"value","result_view":"full"}`} {
			if !resolver.checkInApprovalAllows(t.Context(), scope, d, json.RawMessage(input)) {
				t.Fatal("selector changed check-in approval")
			}
		}
		if resolver.checkInApprovalAllows(
			t.Context(),
			scope,
			d,
			json.RawMessage(`{"z":2,"a":"value","result_view":"full"}`),
		) {
			t.Fatal("changed operation reused approval")
		}
		grants.grants[0].Decision = toolspkg.ApprovalGrantReject
		if resolver.checkInApprovalAllows(
			t.Context(),
			scope,
			d,
			json.RawMessage(`{"z":1,"a":"value","result_view":"full"}`),
		) {
			t.Fatal("reject grant allowed check-in")
		}
	})
}

func TestBoundResultViewApprovalToken(t *testing.T) {
	t.Parallel()
	t.Run("Should mint tokens from either view and consume them without the selector", func(t *testing.T) {
		t.Parallel()
		registry := newDaemonNativeRegistry(
			t,
			&daemonNativeToolsDeps{
				Sessions:   nativeTestSessionManager("ws-1"),
				Workspaces: nativeTestWorkspaceService(t),
			},
			nativeApproveAllPolicyInputs(),
		)
		tokens := toolspkg.NewApprovalTokenStore(time.Minute)
		issuer := &boundToolApprovalIssuer{registry: registry, tokens: tokens, binder: resultViewApprovalInputBinder{}}
		scope := toolspkg.Scope{ProfileID: store.DefaultProfileID, SessionID: "sess-1"}
		for _, input := range []string{`{"result_view":"full","z":1,"a":"value"}`, `{"z":1,"result_view":"compact","a":"value"}`, `{"z":1,"a":"value","result_view":"full"}`} {
			digest, err := toolspkg.ApprovalInputDigest(json.RawMessage(`{"z":1,"a":"value"}`), "")
			if err != nil {
				t.Fatal(err)
			}
			grant, err := issuer.CreateToolApproval(
				t.Context(),
				scope,
				toolspkg.ApprovalTokenRequest{
					ToolID:      toolspkg.ToolIDSessionArchive,
					Input:       json.RawMessage(input),
					InputDigest: digest,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if grant.InputDigest != digest {
				t.Fatal("token includes selector")
			}
			if err := tokens.ConsumeToolApproval(
				t.Context(),
				scope,
				toolspkg.CallRequest{
					ToolID:        toolspkg.ToolIDSessionArchive,
					Input:         json.RawMessage(`{"z":1,"a":"value"}`),
					ApprovalToken: grant.ApprovalToken,
				},
			); err != nil {
				t.Fatal(err)
			}
		}
	})
}

type resultViewApprovalInputBinder struct{}

var _ toolspkg.CallInputBinder = resultViewApprovalInputBinder{}

func (resultViewApprovalInputBinder) BindCallInput(
	_ context.Context,
	_ toolspkg.Scope,
	_ toolspkg.Descriptor,
	input json.RawMessage,
) (json.RawMessage, error) {
	return input, nil
}
