package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
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
}

type terminalApprovalCapture struct {
	calls []terminalApprovalCaptureCall
	err   error
}

func (c *terminalApprovalCapture) RequestToolApproval(
	_ context.Context,
	scope toolspkg.Scope,
	request *toolspkg.CallRequest,
	_ *toolspkg.ToolView,
) error {
	request.ApprovalLabel = toolApprovalApprovedOnceLabel
	c.calls = append(c.calls, terminalApprovalCaptureCall{scope: scope, request: *request})
	return c.err
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
		)
		for attempt := range 2 {
			allowed, err := bridge.RequestWorkspaceAccess(
				t.Context(),
				scope,
				call,
				descriptor,
				"ws-b",
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
			)
			allowed, err := bridge.RequestWorkspaceAccess(
				t.Context(),
				scope,
				call,
				descriptor,
				"ws-b",
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

	t.Run("Should survive originating request cancellation within its own deadline", func(t *testing.T) {
		t.Parallel()

		requester := selectedPermissionRequester(workspaceAccessAllowOnceID)
		bridge := newWorkspaceAccessPromptBridge(
			func() sessionPermissionRequester { return requester },
			time.Second,
			newWorkspaceAccessConsentCache(),
		)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		allowed, err := bridge.RequestWorkspaceAccess(ctx, scope, call, descriptor, "ws-b")
		if err != nil || !allowed {
			t.Fatalf("RequestWorkspaceAccess(canceled origin) = %v, %v, want allowed", allowed, err)
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
				)
				allowed, err := bridge.RequestWorkspaceAccess(
					t.Context(),
					scope,
					call,
					descriptor,
					"ws-b",
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
