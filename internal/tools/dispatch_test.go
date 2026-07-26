package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type recordingHookRunner struct {
	pre       func(context.Context, CallRequest) (CallRequest, EffectiveToolDecision, error)
	post      func(context.Context, CallRequest, ToolResult) (ToolResult, error)
	postError func(context.Context, CallRequest, error) error
}

var _ HookRunner = (*recordingHookRunner)(nil)

func (h *recordingHookRunner) PreCall(
	ctx context.Context,
	call CallRequest,
) (CallRequest, EffectiveToolDecision, error) {
	if h.pre != nil {
		return h.pre(ctx, call)
	}
	return call, hookAllowedDecision(), nil
}

func (h *recordingHookRunner) PostCall(
	ctx context.Context,
	call CallRequest,
	result ToolResult,
) (ToolResult, error) {
	if h.post != nil {
		return h.post(ctx, call, result)
	}
	return result, nil
}

func (h *recordingHookRunner) PostError(ctx context.Context, call CallRequest, err error) error {
	if h.postError != nil {
		return h.postError(ctx, call, err)
	}
	return nil
}

type recordingToolEventSink struct {
	mu     sync.Mutex
	events []ToolCallEvent
}

var _ ToolEventSink = (*recordingToolEventSink)(nil)

func (s *recordingToolEventSink) EmitToolEvent(_ context.Context, event ToolCallEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *recordingToolEventSink) kinds() []ToolCallEventKind {
	s.mu.Lock()
	defer s.mu.Unlock()
	kinds := make([]ToolCallEventKind, 0, len(s.events))
	for _, event := range s.events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func (s *recordingToolEventSink) snapshot() []ToolCallEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ToolCallEvent(nil), s.events...)
}

type recordingApprovalBridge struct {
	err   error
	calls []approvalBridgeCall
}

type approvalBridgeCall struct {
	scope Scope
	req   CallRequest
	view  ToolView
}

var _ ApprovalBridge = (*recordingApprovalBridge)(nil)

func (b *recordingApprovalBridge) RequestToolApproval(
	_ context.Context,
	scope Scope,
	req CallRequest,
	view *ToolView,
) error {
	recorded := ToolView{}
	if view != nil {
		recorded = cloneToolView(view)
	}
	b.calls = append(b.calls, approvalBridgeCall{scope: scope, req: req, view: recorded})
	return b.err
}

func TestRuntimeRegistryDispatchValidationAndPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid input before provider invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		registry := mustDispatchRegistry(t, provider, WithToolEventSink(events))

		_, err := registry.Call(t.Context(), Scope{}, CallRequest{
			ToolID: descriptor.ID,
			Input:  json.RawMessage(`{"query":42}`),
		})
		if !errors.Is(err, ErrToolInvalidInput) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolInvalidInput", err)
		}
		if called {
			t.Fatal("provider handle was called for invalid input")
		}
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallStarted, ToolCallFailed}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
	})

	t.Run("Should recheck policy before provider invocation and emit denial", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		denyPattern, err := ParseToolPattern(descriptor.ID.String())
		if err != nil {
			t.Fatalf("ParseToolPattern() error = %v", err)
		}
		called := false
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		registry := mustDispatchRegistry(
			t,
			provider,
			WithToolEventSink(events),
			WithPolicyInputs(PolicyInputs{
				SystemPermissionMode: PermissionModeApproveAll,
				DenyTools:            []ToolPattern{denyPattern},
			}, ToolsetCatalog{}),
		)

		_, err = registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if !errors.Is(err, ErrToolDenied) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolDenied", err)
		}
		if called {
			t.Fatal("provider handle was called for denied tool")
		}
		eventSnapshot := events.snapshot()
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallDenied}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
		if !slices.Contains(eventSnapshot[0].ReasonCodes, ReasonPolicyDenied) {
			t.Fatalf("denial reasons = %#v, want policy_denied", eventSnapshot[0].ReasonCodes)
		}
	})

	t.Run("Should recheck availability before provider invocation and emit denial", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor: descriptor,
			availability: Availability{
				Registered:  true,
				Enabled:     true,
				Available:   false,
				Authorized:  true,
				Executable:  false,
				ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
			},
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		registry := mustDispatchRegistry(t, provider, WithToolEventSink(events))

		_, err := registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if !errors.Is(err, ErrToolUnavailable) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolUnavailable", err)
		}
		if called {
			t.Fatal("provider handle was called for unavailable tool")
		}
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallDenied}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
	})
}

func TestPolicyDenyAll(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		descriptor Descriptor
	}{
		{
			name:       "Should block native Go tools before provider invocation",
			descriptor: nativeDenyAllDescriptor(),
		},
		{
			name:       "Should block extension host tools before provider invocation",
			descriptor: extensionDenyAllDescriptor(),
		},
		{
			name:       "Should block MCP tools before provider invocation",
			descriptor: mcpDescriptor("mcp__smoke__echo", "smoke", "echo"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			called := false
			bridge := &recordingApprovalBridge{}
			events := &recordingToolEventSink{}
			provider := dispatchProviderWithSourceAndHandle(tc.descriptor.Source, tc.descriptor, &registryTestHandle{
				descriptor:   tc.descriptor,
				availability: availableDispatchHandle(),
				call: func(context.Context, CallRequest) (ToolResult, error) {
					called = true
					return ToolResult{}, nil
				},
			})
			registry := mustDispatchRegistry(
				t,
				provider,
				WithToolEventSink(events),
				WithApprovalBridge(bridge),
				WithPolicyInputs(PolicyInputs{
					SystemPermissionMode: PermissionModeDenyAll,
					ExternalDefault:      ExternalDefaultEnabled,
				}, ToolsetCatalog{}),
			)

			_, err := registry.Call(
				t.Context(),
				Scope{},
				CallRequest{ToolID: tc.descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
			)
			if !errors.Is(err, ErrToolApprovalRequired) {
				t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolApprovalRequired", err)
			}
			if called {
				t.Fatal("provider handle was called for deny-all policy")
			}
			if len(bridge.calls) != 0 {
				t.Fatalf("approval bridge calls = %d, want 0 so grants cannot override deny-all", len(bridge.calls))
			}
			eventSnapshot := events.snapshot()
			if got, want := events.kinds(), []ToolCallEventKind{ToolCallDenied}; !slices.Equal(got, want) {
				t.Fatalf("event kinds = %#v, want %#v", got, want)
			}
			if !slices.Contains(eventSnapshot[0].ReasonCodes, ReasonApprovalRequired) {
				t.Fatalf("denial reasons = %#v, want approval_required", eventSnapshot[0].ReasonCodes)
			}
			if !slices.Contains(eventSnapshot[0].ReasonCodes, ReasonApprovalUnreachable) {
				t.Fatalf("denial reasons = %#v, want approval_unreachable", eventSnapshot[0].ReasonCodes)
			}
		})
	}
}

func TestRuntimeRegistryDispatchApprovalBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should require bridge approval before provider invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		bridge := &recordingApprovalBridge{}
		called := false
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(_ context.Context, req CallRequest) (ToolResult, error) {
				called = true
				if string(req.Input) != `{"query":"patched"}` {
					t.Fatalf("provider input = %s, want patched approval input", req.Input)
				}
				return ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
			},
		})
		hooks := &recordingHookRunner{
			pre: func(_ context.Context, call CallRequest) (CallRequest, EffectiveToolDecision, error) {
				call.Input = json.RawMessage(`{"query":"patched"}`)
				return call, hookAllowedDecision(), nil
			},
		}
		registry := mustDispatchRegistry(
			t,
			provider,
			WithPolicyEvaluator(staticPolicyEvaluator{decision: approvalRequiredDispatchDecision()}),
			WithHookRunner(hooks),
			WithApprovalBridge(bridge),
		)

		_, err := registry.Call(
			t.Context(),
			Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "codex"},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"original"}`)},
		)
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
		if !called {
			t.Fatal("provider handle was not called after approval")
		}
		if got, want := len(bridge.calls), 1; got != want {
			t.Fatalf("approval calls = %d, want %d", got, want)
		}
		call := bridge.calls[0]
		if call.scope.SessionID != "sess-1" || call.req.WorkspaceID != "ws-1" || call.req.AgentName != "codex" {
			t.Fatalf("approval call context = %#v / %#v, want scoped request", call.scope, call.req)
		}
		if string(call.req.Input) != `{"query":"patched"}` {
			t.Fatalf("approval input = %s, want patched input", call.req.Input)
		}
		if !call.view.Decision.ApprovalRequired || call.view.Descriptor.ID != descriptor.ID {
			t.Fatalf("approval view = %#v, want descriptor decision requiring approval", call.view)
		}
	})

	t.Run("Should fail closed when approval channel is unavailable", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		registry := mustDispatchRegistry(
			t,
			provider,
			WithPolicyEvaluator(staticPolicyEvaluator{decision: approvalRequiredDispatchDecision()}),
			WithToolEventSink(events),
		)

		_, err := registry.Call(t.Context(), Scope{}, CallRequest{
			ToolID: descriptor.ID,
			Input:  json.RawMessage(`{"query":"x"}`),
		})
		if !errors.Is(err, ErrToolApprovalRequired) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolApprovalRequired", err)
		}
		if called {
			t.Fatal("provider handle was called without approval bridge")
		}
		var toolErr *ToolError
		if !errors.As(err, &toolErr) || !slices.Contains(toolErr.ReasonCodes, ReasonApprovalUnreachable) {
			t.Fatalf("approval error = %#v, want approval_unreachable", err)
		}
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallStarted, ToolCallDenied}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
	})

	t.Run("Should map approval cancellation and timeout to approval reason codes", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name string
			err  error
			want ReasonCode
		}{
			{name: "Canceled", err: context.Canceled, want: ReasonApprovalCanceled},
			{name: "TimedOut", err: context.DeadlineExceeded, want: ReasonApprovalTimedOut},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				descriptor := validDispatchDescriptor()
				bridge := &recordingApprovalBridge{err: tc.err}
				called := false
				provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
					descriptor:   descriptor,
					availability: availableDispatchHandle(),
					call: func(context.Context, CallRequest) (ToolResult, error) {
						called = true
						return ToolResult{}, nil
					},
				})
				registry := mustDispatchRegistry(
					t,
					provider,
					WithPolicyEvaluator(staticPolicyEvaluator{decision: approvalRequiredDispatchDecision()}),
					WithApprovalBridge(bridge),
				)

				_, err := registry.Call(t.Context(), Scope{}, CallRequest{
					ToolID: descriptor.ID,
					Input:  json.RawMessage(`{"query":"x"}`),
				})
				if !errors.Is(err, ErrToolApprovalRequired) {
					t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolApprovalRequired", err)
				}
				if called {
					t.Fatal("provider handle was called after approval denial")
				}
				var toolErr *ToolError
				if !errors.As(err, &toolErr) || !slices.Contains(toolErr.ReasonCodes, tc.want) {
					t.Fatalf("approval error = %#v, want reason %q", err, tc.want)
				}
			})
		}
	})
}

func TestRuntimeRegistryDispatchHooksAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve call context when pre-call hook patches input", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(_ context.Context, req CallRequest) (ToolResult, error) {
				if req.ToolCallID != "tool-call-1" || req.TurnID != "turn-1" || req.CorrelationID != "corr-1" {
					t.Fatalf("call context ids = %#v, want preserved ids", req)
				}
				if req.SessionID != "sess-1" || req.WorkspaceID != "ws-1" || req.AgentName != "codex" {
					t.Fatalf("call scope = %#v, want preserved scope", req)
				}
				if req.ApprovalToken != "approval-ref" || !slices.Contains(req.SensitiveInputFields, "token") {
					t.Fatalf("security context = %#v, want approval and sensitive field markers", req)
				}
				if string(req.Input) != `{"query":"patched"}` {
					t.Fatalf("CallRequest.Input = %s, want patched input", req.Input)
				}
				return ToolResult{Content: []ToolContent{{Type: "text", Text: "ok"}}}, nil
			},
		})
		hooks := &recordingHookRunner{
			pre: func(_ context.Context, _ CallRequest) (CallRequest, EffectiveToolDecision, error) {
				return CallRequest{Input: json.RawMessage(`{"query":"patched"}`)}, hookAllowedDecision(), nil
			},
		}
		registry := mustDispatchRegistry(t, provider, WithHookRunner(hooks))

		_, err := registry.Call(
			t.Context(),
			Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "codex"},
			CallRequest{
				ToolID:               descriptor.ID,
				ToolCallID:           "tool-call-1",
				TurnID:               "turn-1",
				CorrelationID:        "corr-1",
				Input:                json.RawMessage(`{"query":"original","token":"secret"}`),
				SensitiveInputFields: []string{"token"},
				ApprovalToken:        "approval-ref",
			},
		)
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
	})

	t.Run("Should reject request scope spoofing before provider invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		registry := mustDispatchRegistry(t, provider)

		_, err := registry.Call(
			t.Context(),
			Scope{SessionID: "sess-a", WorkspaceID: "ws-a", AgentName: "codex"},
			CallRequest{
				ToolID:      descriptor.ID,
				SessionID:   "sess-b",
				WorkspaceID: "ws-b",
				Input:       json.RawMessage(`{"query":"x"}`),
			},
		)
		requireToolReason(t, err, ReasonScopeMismatch)
		if called {
			t.Fatal("provider handle was called for spoofed request scope")
		}
	})

	t.Run("Should reject pre-call hook scope spoofing before provider invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		hooks := &recordingHookRunner{
			pre: func(_ context.Context, call CallRequest) (CallRequest, EffectiveToolDecision, error) {
				call.SessionID = "sess-b"
				call.WorkspaceID = "ws-b"
				return call, hookAllowedDecision(), nil
			},
		}
		registry := mustDispatchRegistry(t, provider, WithHookRunner(hooks))

		_, err := registry.Call(
			t.Context(),
			Scope{SessionID: "sess-a", WorkspaceID: "ws-a", AgentName: "codex"},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		requireToolReason(t, err, ReasonScopeMismatch)
		if called {
			t.Fatal("provider handle was called for hook-spoofed scope")
		}
	})

	t.Run("Should let pre-call hook denial prevent provider invocation", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		called := false
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			call: func(context.Context, CallRequest) (ToolResult, error) {
				called = true
				return ToolResult{}, nil
			},
		})
		hooks := &recordingHookRunner{
			pre: func(_ context.Context, call CallRequest) (CallRequest, EffectiveToolDecision, error) {
				if call.ToolID != descriptor.ID {
					t.Fatalf("PreCall ToolID = %q, want %q", call.ToolID, descriptor.ID)
				}
				return call, EffectiveToolDecision{
					Callable:   false,
					HookResult: policyResultDenied,
				}, nil
			},
		}
		registry := mustDispatchRegistry(t, provider, WithHookRunner(hooks), WithToolEventSink(events))

		_, err := registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if !errors.Is(err, ErrToolDenied) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolDenied", err)
		}
		if called {
			t.Fatal("provider handle was called after pre-call denial")
		}
		eventSnapshot := events.snapshot()
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallStarted, ToolCallDenied}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
		if !slices.Contains(eventSnapshot[1].ReasonCodes, ReasonHookDenied) {
			t.Fatalf("denial reasons = %#v, want hook_denied", eventSnapshot[1].ReasonCodes)
		}
	})

	t.Run("Should run post-error hook with canonical tool ID on provider failure", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			callErr:      errors.New("provider exploded"),
		})
		postErrorCalled := false
		hooks := &recordingHookRunner{
			postError: func(_ context.Context, call CallRequest, err error) error {
				postErrorCalled = true
				if call.ToolID != descriptor.ID {
					t.Fatalf("PostError ToolID = %q, want %q", call.ToolID, descriptor.ID)
				}
				if !errors.Is(err, ErrToolBackendFailed) {
					t.Fatalf("PostError error = %v, want ErrToolBackendFailed", err)
				}
				return nil
			},
		}
		registry := mustDispatchRegistry(t, provider, WithHookRunner(hooks), WithToolEventSink(events))

		_, err := registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if !errors.Is(err, ErrToolBackendFailed) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolBackendFailed", err)
		}
		if !postErrorCalled {
			t.Fatal("post-error hook was not called")
		}
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallStarted, ToolCallFailed}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
	})

	t.Run("Should normalize provider cancellation errors deterministically", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			callErr:      context.Canceled,
		})
		registry := mustDispatchRegistry(t, provider, WithToolEventSink(events))

		_, err := registry.Call(
			t.Context(),
			Scope{},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if !errors.Is(err, ErrToolCanceled) {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolCanceled", err)
		}
		eventSnapshot := events.snapshot()
		if got, want := events.kinds(), []ToolCallEventKind{ToolCallStarted, ToolCallFailed}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
		if got, want := eventSnapshot[1].ErrorCode, ErrorCodeCanceled; got != want {
			t.Fatalf("failed event error code = %q, want %q", got, want)
		}
		if !slices.Contains(eventSnapshot[1].ReasonCodes, ReasonCallCanceled) {
			t.Fatalf("failed event reasons = %#v, want call_canceled", eventSnapshot[1].ReasonCodes)
		}
	})
}

func TestRuntimeRegistryDispatchResultLimitingAndRedaction(t *testing.T) {
	t.Parallel()

	t.Run("Should redact sensitive result fields before post-call hooks and events", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		descriptor.MaxResultBytes = 4096
		events := &recordingToolEventSink{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			result: ToolResult{
				Preview: "preview token=preview-secret agh_claim_preview_123",
				Content: []ToolContent{
					{
						Type: "json",
						Text: "stdout token=stdout-secret agh_claim_content_456",
						Data: json.RawMessage(`{"access_token":"secret","visible":"ok"}`),
						Metadata: map[string]json.RawMessage{
							"refresh_token": json.RawMessage(`"secret"`),
							"safe":          json.RawMessage(`"ok"`),
						},
					},
				},
				Structured: json.RawMessage(
					`{"password":"secret","token_present":true,"max_input_tokens":1050000,` +
						`"max_output_tokens":128000,"visible":"ok"}`,
				),
				Metadata: map[string]json.RawMessage{
					"api_key": json.RawMessage(`"secret"`),
					"safe":    json.RawMessage(`"ok"`),
				},
			},
		})
		postCalled := false
		hooks := &recordingHookRunner{
			post: func(_ context.Context, call CallRequest, result ToolResult) (ToolResult, error) {
				postCalled = true
				if call.ToolID != descriptor.ID {
					t.Fatalf("PostCall ToolID = %q, want %q", call.ToolID, descriptor.ID)
				}
				data, err := json.Marshal(result)
				if err != nil {
					t.Fatalf("json.Marshal(result) error = %v", err)
				}
				if strings.Contains(string(data), `"secret"`) {
					t.Fatalf("post-call result leaked secret: %s", data)
				}
				if _, ok := result.Metadata["api_key"]; ok {
					t.Fatalf("result.Metadata contains api_key after redaction: %#v", result.Metadata)
				}
				if _, ok := result.Content[0].Metadata["refresh_token"]; ok {
					t.Fatalf("content metadata contains refresh_token after redaction: %#v", result.Content[0].Metadata)
				}
				if len(result.Redactions) == 0 {
					t.Fatal("result.Redactions is empty, want redaction metadata")
				}
				return result, nil
			},
		}
		registry := mustDispatchRegistry(
			t,
			provider,
			WithHookRunner(hooks),
			WithToolEventSink(events),
			WithSensitiveResultFields("api_key"),
		)

		result, err := registry.Call(t.Context(), Scope{}, CallRequest{
			ToolID:               descriptor.ID,
			Input:                json.RawMessage(`{"query":"x","token":"secret"}`),
			SensitiveInputFields: []string{"token"},
		})
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
		if !postCalled {
			t.Fatal("post-call hook was not called")
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("json.Marshal(result) error = %v", err)
		}
		if strings.Contains(string(data), `"secret"`) {
			t.Fatalf("result leaked secret: %s", data)
		}
		for _, leaked := range []string{
			"preview-secret",
			"stdout-secret",
			"agh_claim_preview_123",
			"agh_claim_content_456",
		} {
			if strings.Contains(string(data), leaked) {
				t.Fatalf("result leaked display secret %q: %s", leaked, data)
			}
		}
		for _, path := range []string{"$.preview", "$.content[0].text"} {
			if !slices.ContainsFunc(result.Redactions, func(redaction Redaction) bool {
				return redaction.Path == path && redaction.Reason == ReasonSecretMetadata
			}) {
				t.Fatalf("result.Redactions = %#v, want display redaction path %s", result.Redactions, path)
			}
		}
		if !strings.Contains(string(data), `"token_present":true`) {
			t.Fatalf("result = %s, want public token_present diagnostic preserved", data)
		}
		if !strings.Contains(string(data), `"max_input_tokens":1050000`) ||
			!strings.Contains(string(data), `"max_output_tokens":128000`) {
			t.Fatalf("result = %s, want public model token limits preserved", data)
		}
		eventData, err := json.Marshal(events.snapshot())
		if err != nil {
			t.Fatalf("json.Marshal(events) error = %v", err)
		}
		if strings.Contains(string(eventData), `"secret"`) {
			t.Fatalf("events leaked secret: %s", eventData)
		}
		eventSnapshot := events.snapshot()
		if len(eventSnapshot[0].RedactedInputFields) == 0 {
			t.Fatalf(
				"started event redacted input fields = %#v, want token redaction",
				eventSnapshot[0].RedactedInputFields,
			)
		}
	})

	t.Run("Should return a result exactly at the cap without writing an artifact", func(t *testing.T) {
		t.Parallel()

		resultAtCap := ToolResult{Content: []ToolContent{{Type: "text", Text: "exact-cap"}}}
		if err := refreshResultEnvelopeBytes(&resultAtCap); err != nil {
			t.Fatalf("refreshResultEnvelopeBytes() error = %v", err)
		}
		descriptor := validDispatchDescriptor()
		descriptor.MaxResultBytes = resultAtCap.Bytes
		artifactStore := &recordingToolArtifactStore{}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			result:       resultAtCap,
		})
		registry := mustDispatchRegistry(
			t,
			provider,
			WithResultProcessor(NewResultProcessor(0, artifactStore)),
		)

		result, err := registry.Call(
			t.Context(),
			Scope{WorkspaceID: "workspace-a"},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
		if result.Truncated || len(result.Artifacts) != 0 || result.Bytes != descriptor.MaxResultBytes {
			t.Fatalf("result = %#v, want unchanged exact-cap envelope", result)
		}
		if got := artifactStore.putCount(); got != 0 {
			t.Fatalf("artifact Put calls = %d, want 0", got)
		}
	})

	t.Run("Should offload one post-hook redacted result and page back byte-identical content", func(t *testing.T) {
		t.Parallel()

		descriptor := validDispatchDescriptor()
		descriptor.MaxResultBytes = 768
		events := &recordingToolEventSink{}
		filesystemStore := openTestToolArtifactStore(
			t,
			filepath.Join(t.TempDir(), "tool-artifacts"),
			testToolArtifactRetention(),
		)
		artifactStore := &recordingToolArtifactStore{delegate: filesystemStore}
		provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
			descriptor:   descriptor,
			availability: availableDispatchHandle(),
			result: ToolResult{
				Content: []ToolContent{{Type: "text", Text: strings.Repeat("x", 4096) + " token=secret"}},
			},
		})
		hooks := &recordingHookRunner{post: func(
			_ context.Context,
			_ CallRequest,
			result ToolResult,
		) (ToolResult, error) {
			result.Content[0].Text += "::post-hook-tail"
			return result, nil
		}}
		registry := mustDispatchRegistry(
			t,
			provider,
			WithHookRunner(hooks),
			WithToolEventSink(events),
			WithResultProcessor(NewResultProcessor(0, artifactStore)),
		)

		result, err := registry.Call(
			t.Context(),
			Scope{WorkspaceID: "workspace-a"},
			CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
		)
		if err != nil {
			t.Fatalf("RuntimeRegistry.Call() error = %v, want nil", err)
		}
		if !result.Truncated {
			t.Fatalf("result.Truncated = false, want true: %#v", result)
		}
		if _, ok := result.Metadata["truncated_from_bytes"]; !ok {
			t.Fatalf("result.Metadata = %#v, want truncated_from_bytes", result.Metadata)
		}
		if len(result.Artifacts) != 1 {
			t.Fatalf("result.Artifacts = %#v, want one page-back reference", result.Artifacts)
		}
		if got := artifactStore.putCount(); got != 1 {
			t.Fatalf("artifact Put calls = %d, want 1", got)
		}
		stored := artifactStore.lastContent()
		var canonical ToolResult
		if err := json.Unmarshal(stored, &canonical); err != nil {
			t.Fatalf("json.Unmarshal(stored result) error = %v", err)
		}
		if got := canonical.Content[0].Text; !strings.HasSuffix(got, "::post-hook-tail") {
			t.Fatalf("stored content tail = %q, want post-hook result", got)
		}
		if strings.Contains(string(stored), "token=secret") {
			t.Fatalf("stored artifact leaked display secret: %s", stored)
		}
		page, err := filesystemStore.ReadPage(
			t.Context(),
			"workspace-a",
			result.Artifacts[0].URI,
			0,
			MaxToolArtifactPageBytes,
		)
		if err != nil {
			t.Fatalf("ReadPage() error = %v", err)
		}
		decoded, err := decodeToolArtifactPage(page)
		if err != nil {
			t.Fatalf("decodeToolArtifactPage() error = %v", err)
		}
		if !bytes.Equal(decoded, stored) {
			t.Fatalf("page-back bytes differ from stored canonical result")
		}
		if !slices.ContainsFunc(result.Redactions, func(redaction Redaction) bool {
			return redaction.Reason == ReasonResultBudgetExceeded
		}) {
			t.Fatalf("result.Redactions = %#v, want result budget redaction", result.Redactions)
		}
		if got, want := events.kinds(), []ToolCallEventKind{
			ToolCallStarted,
			ToolCallCompleted,
			ToolResultTruncated,
		}; !slices.Equal(got, want) {
			t.Fatalf("event kinds = %#v, want %#v", got, want)
		}
	})

	t.Run(
		"Should return a redacted preview and typed partial result when artifact persistence fails",
		func(t *testing.T) {
			t.Parallel()

			descriptor := validDispatchDescriptor()
			descriptor.MaxResultBytes = 768
			events := &recordingToolEventSink{}
			artifactStore := &recordingToolArtifactStore{putErr: errors.New("injected disk full")}
			provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
				descriptor:   descriptor,
				availability: availableDispatchHandle(),
				result: ToolResult{
					Content: []ToolContent{{Type: "text", Text: strings.Repeat("x", 4096) + " token=secret"}},
				},
			})
			postErrorCalled := false
			hooks := &recordingHookRunner{postError: func(_ context.Context, _ CallRequest, err error) error {
				postErrorCalled = true
				var toolErr *ToolError
				if !errors.As(err, &toolErr) || toolErr.PartialResult == nil {
					t.Fatalf("PostError error = %T %[1]v, want typed partial result", err)
				}
				if strings.Contains(toolErr.PartialResult.Preview, "secret") {
					t.Fatalf("PostError partial preview leaked secret: %q", toolErr.PartialResult.Preview)
				}
				return nil
			}}
			registry := mustDispatchRegistry(
				t,
				provider,
				WithHookRunner(hooks),
				WithToolEventSink(events),
				WithResultProcessor(NewResultProcessor(0, artifactStore)),
			)

			result, err := registry.Call(
				t.Context(),
				Scope{WorkspaceID: "workspace-a"},
				CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
			)
			if !errors.Is(err, ErrToolResultPersistence) {
				t.Fatalf("RuntimeRegistry.Call() error = %v, want ErrToolResultPersistence", err)
			}
			if !postErrorCalled {
				t.Fatal("post-error hook was not called")
			}
			if !result.Truncated || result.Preview == "" || len(result.Artifacts) != 0 {
				t.Fatalf("partial result = %#v, want preview without artifact reference", result)
			}
			var toolErr *ToolError
			if !errors.As(err, &toolErr) || toolErr.Code != ErrorCodeResultPersistenceFailed ||
				toolErr.PartialResult == nil {
				t.Fatalf("RuntimeRegistry.Call() error = %#v, want persistence code and partial result", toolErr)
			}
			if got := artifactStore.putCount(); got != 1 {
				t.Fatalf("artifact Put calls = %d, want 1", got)
			}
			if got, want := events.kinds(), []ToolCallEventKind{
				ToolCallStarted,
				ToolCallFailed,
			}; !slices.Equal(got, want) {
				t.Fatalf("event kinds = %#v, want %#v", got, want)
			}
			snapshot := events.snapshot()
			if snapshot[1].ErrorCode != ErrorCodeResultPersistenceFailed || !snapshot[1].Truncated {
				t.Fatalf("failed event = %#v, want partial persistence evidence", snapshot[1])
			}
		},
	)

	t.Run("Should reject invalid JSON metadata before returning results", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			result ToolResult
		}{
			{
				name: "Should reject invalid result metadata",
				result: ToolResult{
					Preview:  "ok",
					Metadata: map[string]json.RawMessage{"safe": json.RawMessage(`{bad`)},
				},
			},
			{
				name: "Should reject invalid content metadata",
				result: ToolResult{
					Preview: "ok",
					Content: []ToolContent{{
						Type:     "json",
						Data:     json.RawMessage(`{"ok":true}`),
						Metadata: map[string]json.RawMessage{"safe": json.RawMessage(`{bad`)},
					}},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				descriptor := validDispatchDescriptor()
				events := &recordingToolEventSink{}
				provider := dispatchProviderWithHandle(descriptor, &registryTestHandle{
					descriptor:   descriptor,
					availability: availableDispatchHandle(),
					result:       tt.result,
				})
				registry := mustDispatchRegistry(t, provider, WithToolEventSink(events))

				result, err := registry.Call(
					t.Context(),
					Scope{},
					CallRequest{ToolID: descriptor.ID, Input: json.RawMessage(`{"query":"x"}`)},
				)
				if err == nil {
					data, marshalErr := json.Marshal(result)
					t.Fatalf(
						"RuntimeRegistry.Call() error = nil, want invalid result rejection; marshalErr=%v data=%s",
						marshalErr,
						data,
					)
				}
				var toolErr *ToolError
				if !errors.As(err, &toolErr) {
					t.Fatalf("RuntimeRegistry.Call() error = %T %[1]v, want ToolError", err)
				}
				if !slices.Contains(toolErr.ReasonCodes, ReasonSchemaInvalid) {
					t.Fatalf("ToolError reasons = %#v, want schema_invalid", toolErr.ReasonCodes)
				}
				if got, want := events.kinds(), []ToolCallEventKind{
					ToolCallStarted,
					ToolCallFailed,
				}; !slices.Equal(
					got,
					want,
				) {
					t.Fatalf("event kinds = %#v, want %#v", got, want)
				}
				eventSnapshot := events.snapshot()
				if !slices.Contains(eventSnapshot[1].ReasonCodes, ReasonSchemaInvalid) {
					t.Fatalf("failed event reasons = %#v, want schema_invalid", eventSnapshot[1].ReasonCodes)
				}
			})
		}
	})
}

type recordingToolArtifactStore struct {
	delegate ToolArtifactStore
	putErr   error
	mu       sync.Mutex
	contents [][]byte
}

var _ ToolArtifactStore = (*recordingToolArtifactStore)(nil)

func (s *recordingToolArtifactStore) Put(
	ctx context.Context,
	workspaceID string,
	content []byte,
) (ArtifactRef, error) {
	s.mu.Lock()
	s.contents = append(s.contents, append([]byte(nil), content...))
	putErr := s.putErr
	delegate := s.delegate
	s.mu.Unlock()
	if putErr != nil {
		return ArtifactRef{}, putErr
	}
	if delegate == nil {
		return toolArtifactRef(artifactID(content), int64(len(content))), nil
	}
	return delegate.Put(ctx, workspaceID, content)
}

func (s *recordingToolArtifactStore) ReadPage(
	ctx context.Context,
	workspaceID string,
	uri string,
	offset int64,
	limit int64,
) (ToolArtifactPage, error) {
	if s.delegate == nil {
		return ToolArtifactPage{}, ErrToolArtifactNotFound
	}
	return s.delegate.ReadPage(ctx, workspaceID, uri, offset, limit)
}

func (s *recordingToolArtifactStore) Sweep(ctx context.Context) error {
	if s.delegate == nil {
		return nil
	}
	return s.delegate.Sweep(ctx)
}

func (s *recordingToolArtifactStore) putCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.contents)
}

func (s *recordingToolArtifactStore) lastContent() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.contents) == 0 {
		return nil
	}
	return append([]byte(nil), s.contents[len(s.contents)-1]...)
}

func validDispatchDescriptor() Descriptor {
	descriptor := validDescriptor()
	descriptor.ID = "agh__dispatch_probe"
	descriptor.Backend.NativeName = "dispatch_probe"
	descriptor.InputSchema = json.RawMessage(`{
		"type":"object",
		"required":["query"],
		"properties":{"query":{"type":"string"},"token":{"type":"string"}},
		"additionalProperties":false
	}`)
	return descriptor
}

func nativeDenyAllDescriptor() Descriptor {
	descriptor := validDispatchDescriptor()
	descriptor.ID = "agh__deny_native"
	descriptor.Backend.NativeName = "deny_native"
	return descriptor
}

func extensionDenyAllDescriptor() Descriptor {
	descriptor := validDispatchDescriptor()
	descriptor.ID = "ext__smoke__lookup"
	descriptor.DisplayTitle = "Extension lookup"
	descriptor.Backend = BackendRef{Kind: BackendExtensionHost, ExtensionID: "smoke", Handler: "lookup"}
	descriptor.Source = SourceRef{Kind: SourceExtension, Owner: "smoke"}
	return descriptor
}

func hookAllowedDecision() EffectiveToolDecision {
	return EffectiveToolDecision{
		Callable:   true,
		HookResult: policyResultAllowed,
	}
}

func approvalRequiredDispatchDecision() EffectiveToolDecision {
	return EffectiveToolDecision{
		VisibleToOperator:    true,
		VisibleToSession:     true,
		Callable:             true,
		ApprovalRequired:     true,
		SystemPermissionMode: string(PermissionModeDenyAll),
		RegistryPolicyResult: policyResultAllowed,
		HookResult:           policyResultAllowed,
		ReasonCodes:          []ReasonCode{ReasonApprovalRequired},
	}
}

func availableDispatchHandle() Availability {
	return Availability{
		Registered: true,
		Enabled:    true,
		Available:  true,
		Authorized: true,
		Executable: true,
	}
}

func dispatchProviderWithHandle(descriptor Descriptor, handle *registryTestHandle) registryTestProvider {
	return dispatchProviderWithSourceAndHandle(SourceRef{Kind: SourceBuiltin, Owner: "daemon"}, descriptor, handle)
}

func dispatchProviderWithSourceAndHandle(
	source SourceRef,
	descriptor Descriptor,
	handle *registryTestHandle,
) registryTestProvider {
	return registryTestProvider{
		source:      source,
		descriptors: []Descriptor{descriptor},
		handles:     map[ToolID]Handle{descriptor.ID: handle},
		resolveErr:  map[ToolID]error{},
	}
}

func mustDispatchRegistry(t *testing.T, provider registryTestProvider, opts ...RegistryOption) *RuntimeRegistry {
	t.Helper()

	options := append([]RegistryOption{WithProviders(provider)}, opts...)
	registry, err := NewRegistry(options...)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}
