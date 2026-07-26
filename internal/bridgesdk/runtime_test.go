// Suite: bridge SDK runtime contracts
// Invariant: negotiated runtime methods route requests to the correct handler and validate every response.
// Boundary IN: bridgesdk runtime request dispatch and session helpers.
// Boundary OUT: provider APIs and daemon transport, owned by integration suites.
package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestSessionAckDeliveryBuildsValidatedAck(t *testing.T) {
	t.Parallel()

	request := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "dlv-1",
			Seq:              2,
			EventType:        bridgepkg.DeliveryEventTypeDelta,
			BridgeInstanceID: "brg-1",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-1",
				PeerID:           "peer-1",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-1",
				PeerID:           "peer-1",
				Mode:             bridgepkg.DeliveryModeDirectSend,
			},
			Content: bridgepkg.MessageContent{
				Text: "hello",
			},
		},
	}

	session := &Session{}
	ack, err := session.AckDelivery(request, "remote-1", "")
	if err != nil {
		t.Fatalf("AckDelivery() error = %v", err)
	}
	if got, want := ack.DeliveryID, "dlv-1"; got != want {
		t.Fatalf("ack.DeliveryID = %q, want %q", got, want)
	}
	if got, want := ack.Seq, int64(2); got != want {
		t.Fatalf("ack.Seq = %d, want %d", got, want)
	}
	if got, want := ack.RemoteMessageID, "remote-1"; got != want {
		t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
	}
}

func TestRuntimeAcknowledgesCommittedMutationWithoutReturningRetryableRPCError(t *testing.T) {
	var typedNil *CommittedMutationError
	testCases := []struct {
		name        string
		handlerErr  error
		wantMessage string
	}{
		{
			name: "Should acknowledge a committed mutation with provider detail",
			handlerErr: &CommittedMutationError{
				Err: errors.New("provider accepted the message but omitted its id"),
			},
			wantMessage: "omitted its id",
		},
		{
			name:        "Should acknowledge an empty committed mutation marker",
			handlerErr:  &CommittedMutationError{},
			wantMessage: committedMutationFallbackMessage,
		},
		{
			name:        "Should acknowledge a typed nil committed mutation marker",
			handlerErr:  typedNil,
			wantMessage: committedMutationFallbackMessage,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			request := testRuntimeFinalDeliveryRequest("answer")
			runtime, err := NewRuntime(RuntimeConfig{
				ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "committed-result", Version: "1.0.0"},
				Check:         testCheckHandler,
				Deliver: func(
					context.Context,
					*Session,
					bridgepkg.DeliveryRequest,
				) (bridgepkg.DeliveryAck, error) {
					return bridgepkg.DeliveryAck{}, testCase.handlerErr
				},
			})
			if err != nil {
				t.Fatalf("NewRuntime() error = %v", err)
			}
			runtime.session = &Session{}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("json.Marshal(request) error = %v", err)
			}

			result, err := runtime.handleDeliver(context.Background(), raw)
			if err != nil {
				t.Fatalf("handleDeliver() error = %v, want semantic acknowledgement", err)
			}
			ack, ok := result.(bridgepkg.DeliveryAck)
			if !ok {
				t.Fatalf("handleDeliver() result type = %T, want DeliveryAck", result)
			}
			if got, want := ack.Outcome, bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable; got != want {
				t.Fatalf("ack.Outcome = %q, want %q", got, want)
			}
			if ack.Error == nil || !strings.Contains(ack.Error.Message, testCase.wantMessage) {
				t.Fatalf("ack.Error = %#v, want detail containing %q", ack.Error, testCase.wantMessage)
			}
			if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
				t.Fatalf(
					"committed-result ack remote ids = (%q, %q), want empty",
					ack.RemoteMessageID,
					ack.ReplaceRemoteMessageID,
				)
			}
		})
	}
}

func TestRuntimeProgressDeliveryDefaultsToContractNoop(t *testing.T) {
	t.Run("Should acknowledge progress without invoking the text handler", func(t *testing.T) {
		t.Parallel()

		deliverCalls := 0
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "progress-noop", Version: "1.0.0"},
			Check:         testCheckHandler,
			Deliver: func(
				context.Context,
				*Session,
				bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				deliverCalls++
				return bridgepkg.DeliveryAck{}, errors.New("text handler must not receive progress")
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		runtime.session = &Session{}
		request := bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "dlv-progress-noop",
			BridgeInstanceID: "brg-progress-noop",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-progress-noop",
				BridgeInstanceID: "brg-progress-noop",
				PeerID:           "peer-progress-noop",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-progress-noop",
				PeerID:           "peer-progress-noop",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       2,
			EventType: bridgepkg.DeliveryEventTypeProgress,
			Progress: &bridgepkg.ToolProgress{
				ToolCallID: "call-progress-noop",
				ToolID:     "agh__terminal",
				Phase:      bridgepkg.ToolProgressPhaseStarted,
				Label:      "Running",
				Emoji:      "⚙️",
				Index:      1,
			},
		}}
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("json.Marshal(request) error = %v", err)
		}

		result, err := runtime.handleDeliver(context.Background(), raw)
		if err != nil {
			t.Fatalf("handleDeliver(progress) error = %v", err)
		}
		ack, ok := result.(bridgepkg.DeliveryAck)
		if !ok {
			t.Fatalf("handleDeliver(progress) result type = %T, want DeliveryAck", result)
		}
		if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
			t.Fatalf("progress ack delivery id = %q, want %q", got, want)
		}
		if got, want := ack.Seq, request.Event.Seq; got != want {
			t.Fatalf("progress ack seq = %d, want %d", got, want)
		}
		if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
			t.Fatalf(
				"progress noop ack remote ids = (%q, %q), want empty",
				ack.RemoteMessageID,
				ack.ReplaceRemoteMessageID,
			)
		}
		if got, want := deliverCalls, 0; got != want {
			t.Fatalf("text deliver handler calls = %d, want %d", got, want)
		}
	})
}

func TestRuntimeProgressDeliveryUsesDedicatedHandler(t *testing.T) {
	t.Parallel()

	request := testRuntimeProgressDeliveryRequest()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(request) error = %v", err)
	}

	t.Run("Should route progress exclusively to the dedicated handler", func(t *testing.T) {
		t.Parallel()

		deliverCalls := 0
		progressCalls := 0
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "progress-handler", Version: "1.0.0"},
			Check:         testCheckHandler,
			Deliver: func(
				context.Context,
				*Session,
				bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				deliverCalls++
				return bridgepkg.DeliveryAck{}, errors.New("text handler must not receive progress")
			},
			Progress: func(
				_ context.Context,
				session *Session,
				received bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				progressCalls++
				return session.AckDelivery(received, "", "")
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		runtime.session = &Session{}

		result, err := runtime.handleDeliver(context.Background(), raw)
		if err != nil {
			t.Fatalf("handleDeliver(progress) error = %v", err)
		}
		ack, ok := result.(bridgepkg.DeliveryAck)
		if !ok {
			t.Fatalf("handleDeliver(progress) result type = %T, want DeliveryAck", result)
		}
		if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
			t.Fatalf("progress ack delivery id = %q, want %q", got, want)
		}
		if got, want := ack.Seq, request.Event.Seq; got != want {
			t.Fatalf("progress ack seq = %d, want %d", got, want)
		}
		if got, want := progressCalls, 1; got != want {
			t.Fatalf("progress handler calls = %d, want %d", got, want)
		}
		if got := deliverCalls; got != 0 {
			t.Fatalf("text handler calls = %d, want zero", got)
		}
	})

	t.Run("Should propagate a dedicated progress handler failure", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("provider progress failed")
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "progress-error", Version: "1.0.0"},
			Check:         testCheckHandler,
			Deliver: func(
				context.Context,
				*Session,
				bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{}, errors.New("text handler must not receive progress")
			},
			Progress: func(
				context.Context,
				*Session,
				bridgepkg.DeliveryRequest,
			) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{}, wantErr
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		runtime.session = &Session{}

		result, err := runtime.handleDeliver(context.Background(), raw)
		if !errors.Is(err, wantErr) {
			t.Fatalf("handleDeliver(progress) error = %v, want %v", err, wantErr)
		}
		if result != nil {
			t.Fatalf("handleDeliver(progress) result = %#v, want nil after handler failure", result)
		}
	})

	t.Run(
		"Should convert an invalid progress acknowledgement into a committed-result acknowledgement",
		func(t *testing.T) {
			t.Parallel()

			progressCalls := 0
			runtime, err := NewRuntime(RuntimeConfig{
				ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "progress-invalid-ack", Version: "1.0.0"},
				Check:         testCheckHandler,
				Deliver: func(
					context.Context,
					*Session,
					bridgepkg.DeliveryRequest,
				) (bridgepkg.DeliveryAck, error) {
					return bridgepkg.DeliveryAck{}, errors.New("text handler must not receive progress")
				},
				Progress: func(
					_ context.Context,
					_ *Session,
					_ bridgepkg.DeliveryRequest,
				) (bridgepkg.DeliveryAck, error) {
					progressCalls++
					return bridgepkg.DeliveryAck{}, nil
				},
			})
			if err != nil {
				t.Fatalf("NewRuntime() error = %v", err)
			}
			runtime.session = &Session{}

			result, err := runtime.handleDeliver(context.Background(), raw)
			if err != nil {
				t.Fatalf("handleDeliver(progress) error = %v, want semantic acknowledgement", err)
			}
			ack, ok := result.(bridgepkg.DeliveryAck)
			if !ok {
				t.Fatalf("handleDeliver(progress) result type = %T, want DeliveryAck", result)
			}
			if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
				t.Fatalf("ack.DeliveryID = %q, want %q", got, want)
			}
			if got, want := ack.Seq, request.Event.Seq; got != want {
				t.Fatalf("ack.Seq = %d, want %d", got, want)
			}
			if got, want := ack.Outcome, bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable; got != want {
				t.Fatalf("ack.Outcome = %q, want %q", got, want)
			}
			if ack.Error == nil || !strings.Contains(ack.Error.Message, "delivery id is required") {
				t.Fatalf("ack.Error = %#v, want missing delivery id detail", ack.Error)
			}
			if got, want := progressCalls, 1; got != want {
				t.Fatalf("progress handler calls = %d, want %d", got, want)
			}
		},
	)
}

func TestRuntimeFinalDeliveryRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		text                string
		operation           bridgepkg.DeliveryOperation
		progressHandler     bool
		wantTextHandlerCall bool
		wantProgressCalls   int
		wantRemoteMessageID string
	}{
		{
			name: "Should acknowledge a lifecycle-only final without invoking the text handler",
		},
		{
			name:              "Should route a lifecycle-only final through the progress handler when configured",
			progressHandler:   true,
			wantProgressCalls: 1,
		},
		{
			name:                "Should route a materialized final through the text handler",
			text:                "answer",
			wantTextHandlerCall: true,
			wantRemoteMessageID: "answer-message",
		},
		{
			name:                "Should route an empty edit final through the text handler",
			operation:           bridgepkg.DeliveryOperationEdit,
			wantTextHandlerCall: true,
			wantRemoteMessageID: "answer-message",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			textHandlerCalls := 0
			progressHandlerCalls := 0
			config := RuntimeConfig{
				ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "final-routing", Version: "1.0.0"},
				Check:         testCheckHandler,
				Deliver: func(
					_ context.Context,
					session *Session,
					request bridgepkg.DeliveryRequest,
				) (bridgepkg.DeliveryAck, error) {
					textHandlerCalls++
					return session.AckDelivery(request, "answer-message", "")
				},
			}
			if test.progressHandler {
				config.Progress = func(
					_ context.Context,
					session *Session,
					request bridgepkg.DeliveryRequest,
				) (bridgepkg.DeliveryAck, error) {
					progressHandlerCalls++
					return session.AckDelivery(request, "", "")
				}
			}
			runtime, err := NewRuntime(config)
			if err != nil {
				t.Fatalf("NewRuntime() error = %v", err)
			}
			runtime.session = &Session{}

			request := testRuntimeFinalDeliveryRequest(test.text)
			if test.operation != "" {
				request.Event.Operation = test.operation
				request.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: "answer-message"}
			}
			raw, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("json.Marshal(request) error = %v", err)
			}
			result, err := runtime.handleDeliver(context.Background(), raw)
			if err != nil {
				t.Fatalf("handleDeliver(final) error = %v", err)
			}
			ack, ok := result.(bridgepkg.DeliveryAck)
			if !ok {
				t.Fatalf("handleDeliver(final) result type = %T, want DeliveryAck", result)
			}
			if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
				t.Fatalf("final ack delivery id = %q, want %q", got, want)
			}
			if got, want := ack.Seq, request.Event.Seq; got != want {
				t.Fatalf("final ack seq = %d, want %d", got, want)
			}
			if got, want := ack.RemoteMessageID, test.wantRemoteMessageID; got != want {
				t.Fatalf("final ack remote message id = %q, want %q", got, want)
			}
			wantCalls := 0
			if test.wantTextHandlerCall {
				wantCalls = 1
			}
			if got := textHandlerCalls; got != wantCalls {
				t.Fatalf("text handler calls = %d, want %d", got, wantCalls)
			}
			if got := progressHandlerCalls; got != test.wantProgressCalls {
				t.Fatalf("progress handler calls = %d, want %d", got, test.wantProgressCalls)
			}
		})
	}
}

func TestSessionReportClassifiedErrorReportsStateThroughHostAPI(t *testing.T) {
	t.Parallel()

	reported := bridgepkg.BridgesInstancesReportStateParams{}
	session := &Session{
		host: NewHostAPIClientFromCall(func(_ context.Context, method string, params any, result any) error {
			if got, want := method, "bridges/instances/report_state"; got != want {
				t.Fatalf("method = %q, want %q", got, want)
			}
			reported = params.(bridgepkg.BridgesInstancesReportStateParams)
			target := result.(*bridgepkg.BridgeInstance)
			*target = testBridgeInstance(reported.BridgeInstanceID)
			target.Status = reported.Status
			target.Degradation = reported.Degradation
			return nil
		}),
	}

	updated, recovery, err := session.ReportClassifiedError(
		context.Background(),
		"brg-1",
		ClassifyError(&RateLimitError{
			Err:        errors.New("slow down"),
			RetryAfter: time.Second,
		}),
	)
	if err != nil {
		t.Fatalf("ReportClassifiedError() error = %v", err)
	}
	if !recovery.Retry {
		t.Fatal("recovery.Retry = false, want true")
	}
	if updated == nil || updated.Status != bridgepkg.BridgeStatusDegraded {
		t.Fatalf("updated.Status = %#v, want degraded", updated)
	}
	if got, want := reported.BridgeInstanceID, "brg-1"; got != want {
		t.Fatalf("reported.BridgeInstanceID = %q, want %q", got, want)
	}
	if reported.Degradation == nil || reported.Degradation.Reason != bridgepkg.BridgeDegradationReasonRateLimited {
		t.Fatalf("reported.Degradation = %#v, want rate_limited", reported.Degradation)
	}
}

func TestNewRuntimeRejectsMissingRequiredConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewRuntime(RuntimeConfig{}); err == nil {
		t.Fatal("NewRuntime(empty) error = nil, want non-nil")
	}
	if _, err := NewRuntime(RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    "telegram-adapter",
			Version: "1.0.0",
		},
	}); err == nil {
		t.Fatal("NewRuntime(missing deliver) error = nil, want non-nil")
	}
}

func TestSessionReportClassifiedErrorNoActionWhenRecoveryHasNoStatus(t *testing.T) {
	t.Parallel()

	session := &Session{
		host: NewHostAPIClientFromCall(func(context.Context, string, any, any) error {
			t.Fatal("host call executed for empty recovery")
			return nil
		}),
	}

	updated, recovery, err := session.ReportClassifiedError(context.Background(), "brg-1", ClassifiedError{})
	if err != nil {
		t.Fatalf("ReportClassifiedError() error = %v", err)
	}
	if updated != nil {
		t.Fatalf("updated = %#v, want nil", updated)
	}
	if recovery.Status != "" {
		t.Fatalf("recovery.Status = %q, want empty", recovery.Status)
	}
}

func TestDecodeParamsHandlesNullAndInvalidJSON(t *testing.T) {
	t.Parallel()

	var target map[string]any
	if err := decodeParams(nil, &target); err != nil {
		t.Fatalf("decodeParams(nil) error = %v", err)
	}
	if err := decodeParams(json.RawMessage("{"), &target); err == nil {
		t.Fatal("decodeParams(invalid json) error = nil, want non-nil")
	}
}

func TestSessionAccessorsExposeConfiguredHelpers(t *testing.T) {
	t.Parallel()

	cache := NewInstanceCache(testManagedRuntime("brg-1"))
	host := NewHostAPIClientFromCall(func(context.Context, string, any, any) error { return nil })
	session := &Session{cache: cache, host: host}

	if session.BridgeRuntime() == nil {
		t.Fatal("session.BridgeRuntime() = nil, want non-nil")
	}
	if session.HostAPI() != host {
		t.Fatal("session.HostAPI() did not return configured host client")
	}
	if session.Cache() != cache {
		t.Fatal("session.Cache() did not return configured cache")
	}
}

func TestSessionInitializeAccessorsReturnClones(t *testing.T) {
	t.Parallel()

	session := &Session{
		request: subprocess.InitializeRequest{
			Capabilities: subprocess.InitializeCapabilities{
				Provides:        []string{"bridge.adapter"},
				GrantedActions:  []extensionprotocol.HostAPIMethod{extensionprotocol.HostAPIMethodBridgesInstancesList},
				GrantedSecurity: []string{"bridge.read"},
			},
			Methods: subprocess.InitializeMethods{
				DaemonRequests:    []string{"ping"},
				ExtensionServices: []string{"bridges/deliver"},
			},
			Runtime: subprocess.InitializeRuntime{
				Bridge: testManagedRuntime("brg-1"),
			},
		},
		response: subprocess.InitializeResponse{
			AcceptedCapabilities: subprocess.AcceptedCapabilities{
				Provides: []string{"bridge.adapter"},
				Actions:  []extensionprotocol.HostAPIMethod{extensionprotocol.HostAPIMethodBridgesInstancesGet},
				Security: []string{"bridge.write"},
			},
			ImplementedMethods:  []string{"bridges/deliver"},
			SupportedHookEvents: []string{"hook"},
		},
	}

	request := session.InitializeRequest()
	response := session.InitializeResponse()

	request.Capabilities.Provides[0] = "mutated"
	request.Capabilities.GrantedActions[0] = extensionprotocol.HostAPIMethodBridgesInstancesGet
	request.Capabilities.GrantedSecurity[0] = "mutated"
	request.Methods.DaemonRequests[0] = "mutated"
	request.Methods.ExtensionServices[0] = "mutated"
	request.Runtime.Bridge.ManagedInstances[0].Instance.ID = "mutated"

	response.AcceptedCapabilities.Provides[0] = "mutated"
	response.AcceptedCapabilities.Actions[0] = extensionprotocol.HostAPIMethodBridgesMessagesIngest
	response.AcceptedCapabilities.Security[0] = "mutated"
	response.ImplementedMethods[0] = "mutated"
	response.SupportedHookEvents[0] = "mutated"

	if got, want := session.request.Capabilities.Provides[0], "bridge.adapter"; got != want {
		t.Fatalf("session.request.Capabilities.Provides[0] = %q, want %q", got, want)
	}
	if got, want := session.request.Methods.DaemonRequests[0], "ping"; got != want {
		t.Fatalf("session.request.Methods.DaemonRequests[0] = %q, want %q", got, want)
	}
	if got, want := session.request.Runtime.Bridge.ManagedInstances[0].Instance.ID, "brg-1"; got != want {
		t.Fatalf("session.request.Runtime.Bridge.ManagedInstances[0].Instance.ID = %q, want %q", got, want)
	}
	if got, want := session.response.AcceptedCapabilities.Provides[0], "bridge.adapter"; got != want {
		t.Fatalf("session.response.AcceptedCapabilities.Provides[0] = %q, want %q", got, want)
	}
	if got, want := session.response.ImplementedMethods[0], "bridges/deliver"; got != want {
		t.Fatalf("session.response.ImplementedMethods[0] = %q, want %q", got, want)
	}
}

func testRuntimeProgressDeliveryRequest() bridgepkg.DeliveryRequest {
	return bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
		DeliveryID:       "dlv-progress-handler",
		BridgeInstanceID: "brg-progress-handler",
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      "ws-progress-handler",
			BridgeInstanceID: "brg-progress-handler",
			PeerID:           "peer-progress-handler",
		},
		DeliveryTarget: bridgepkg.DeliveryTarget{
			BridgeInstanceID: "brg-progress-handler",
			PeerID:           "peer-progress-handler",
			Mode:             bridgepkg.DeliveryModeReply,
		},
		Seq:       2,
		EventType: bridgepkg.DeliveryEventTypeProgress,
		Progress: &bridgepkg.ToolProgress{
			ToolCallID: "call-progress-handler",
			ToolID:     "agh__terminal",
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      "Running",
			Emoji:      "⚙️",
			Index:      1,
		},
	}}
}

func testRuntimeFinalDeliveryRequest(text string) bridgepkg.DeliveryRequest {
	request := testRuntimeProgressDeliveryRequest()
	request.Event.Seq = 3
	request.Event.EventType = bridgepkg.DeliveryEventTypeFinal
	request.Event.Content = bridgepkg.MessageContent{Text: text}
	request.Event.Final = true
	request.Event.Progress = nil
	return request
}
