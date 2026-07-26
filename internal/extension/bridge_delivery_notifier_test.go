package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
)

func TestBridgeDeliveryProjectionUsesOneCanonicalPath(t *testing.T) {
	t.Run("Should deduplicate equivalent replay and live tool events", func(t *testing.T) {
		t.Parallel()

		event := (acp.AgentEvent{
			Type:       acp.EventTypeToolCall,
			TurnID:     "turn-tool-projection",
			Timestamp:  time.Date(2026, time.April, 11, 12, 8, 0, 0, time.UTC),
			ToolCallID: "call-tool-projection",
		}).WithTool(
			"agh__terminal",
			json.RawMessage(`{"command":"echo sk-live-replay-secret"}`),
			false,
		)

		live, err := projectionEventFromAgentEvent(&event)
		if err != nil {
			t.Fatalf("projectionEventFromAgentEvent() error = %v", err)
		}
		stored := mustStoredPromptEvent(t, "ev-tool-projection", 1, event)
		replayed, err := promptProjectionEventFromStoredEvent(stored)
		if err != nil {
			t.Fatalf("promptProjectionEventFromStoredEvent() error = %v", err)
		}

		if live.Fingerprint == "" || replayed.Fingerprint == "" {
			t.Fatalf(
				"projection fingerprints = (%q, %q), want canonical values",
				live.Fingerprint,
				replayed.Fingerprint,
			)
		}
		if got, want := replayed.Fingerprint, live.Fingerprint; got != want {
			t.Fatalf("replay fingerprint differs from live\nreplay: %s\nlive:   %s", got, want)
		}
		storedFingerprint, err := canonicalProjectionFingerprint(stored.Content)
		if err != nil {
			t.Fatalf("canonicalProjectionFingerprint(stored event) error = %v", err)
		}
		if got, want := live.Fingerprint, storedFingerprint; got != want {
			t.Fatalf("live fingerprint differs from persisted semantic payload\nlive:   %s\nstored: %s", got, want)
		}
		if live.Tool == nil || replayed.Tool == nil {
			t.Fatalf("tool projections = (%#v, %#v), want typed progress", live.Tool, replayed.Tool)
		}
		if got, want := replayed.Tool.ToolCallID, live.Tool.ToolCallID; got != want {
			t.Fatalf("replay tool call id = %q, want live %q", got, want)
		}
		if got, want := replayed.Tool.ToolID, live.Tool.ToolID; got != want {
			t.Fatalf("replay tool id = %q, want live %q", got, want)
		}
		if got, want := replayed.Tool.Phase, bridgepkg.ToolProgressPhaseStarted; got != want {
			t.Fatalf("replay tool phase = %q, want %q", got, want)
		}
		if got, want := string(replayed.ToolInput), string(live.ToolInput); got != want {
			t.Fatalf("replay tool input = %s, want live %s", got, want)
		}
		if strings.Contains(string(live.ToolInput), "sk-live-replay-secret") ||
			!strings.Contains(string(live.ToolInput), "[REDACTED]") {
			t.Fatalf("canonical tool input = %s, want redacted", live.ToolInput)
		}

		transport := &recordingDeliveryTransport{}
		broker := bridgepkg.NewBroker(transport)
		t.Cleanup(broker.Close)
		registration, err := broker.RegisterPromptDelivery(testutil.Context(t), bridgepkg.PromptDeliveryRegistration{
			SessionID:     "sess-tool-projection",
			TurnID:        event.TurnID,
			ExtensionName: "ext-slack",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-tool-projection",
				BridgeInstanceID: "brg-tool-projection",
				PeerID:           "peer-tool-projection",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-tool-projection",
				PeerID:           "peer-tool-projection",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Progress: bridgepkg.ProgressConfig{
				ToolProgress: bridgepkg.ProgressModeAll,
				Grouping:     bridgepkg.ProgressGroupingAccumulate,
			},
			SeedEvents: []bridgepkg.DeliveryProjectionEvent{replayed},
		})
		if err != nil {
			t.Fatalf("RegisterPromptDelivery(seed tool event) error = %v", err)
		}
		waitForExtensionDeliveryCalls(t, transport, 1)

		notifier := NewBridgeDeliveryNotifier(broker, nil)
		notifier.OnAgentEvent(testutil.Context(t), registration.SessionID, event)
		assertExtensionDeliveryCallCountStable(t, transport, 1, 50*time.Millisecond)
		calls := transport.snapshotCalls()
		if got, want := calls[0].Event.EventType, bridgepkg.DeliveryEventTypeProgress; got != want {
			t.Fatalf("seeded delivery event type = %q, want %q", got, want)
		}
		if calls[0].Event.Progress == nil {
			t.Fatal("seeded delivery progress = nil, want typed progress")
		}
		preview := calls[0].Event.Progress.Preview
		if got, want := preview, `"echo"`; got != want {
			t.Fatalf("seeded delivery preview = %q, want command-name-only %q", got, want)
		}
		if strings.Contains(preview, "sk-live-replay-secret") {
			t.Fatalf("seeded delivery preview = %q, want no terminal arguments", preview)
		}
	})

	t.Run("Should deduplicate semantically identical stored JSON with reordered fields", func(t *testing.T) {
		t.Parallel()

		event := acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: "acp-canonical-order",
			TurnID:    "turn-canonical-order",
			Timestamp: time.Date(2026, time.April, 11, 12, 9, 0, 0, time.UTC),
			Text:      "working",
		}
		live, err := projectionEventFromAgentEvent(&event)
		if err != nil {
			t.Fatalf("projectionEventFromAgentEvent() error = %v", err)
		}

		stored := mustStoredPromptEvent(t, "ev-canonical-order", 1, event)
		var decoded map[string]any
		if err := json.Unmarshal([]byte(stored.Content), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(stored event) error = %v", err)
		}
		reordered, err := json.Marshal(decoded)
		if err != nil {
			t.Fatalf("json.Marshal(reordered stored event) error = %v", err)
		}
		stored.Content = string(reordered)

		replayed, err := promptProjectionEventFromStoredEvent(stored)
		if err != nil {
			t.Fatalf("promptProjectionEventFromStoredEvent() error = %v", err)
		}
		if got, want := replayed.Fingerprint, live.Fingerprint; got != want {
			t.Fatalf("replay fingerprint differs from live after key reordering\nreplay: %s\nlive:   %s", got, want)
		}
	})
}

type recordingDeliveryTransport struct {
	mu     sync.Mutex
	calls  []bridgepkg.DeliveryRequest
	notify chan struct{}
}

func (t *recordingDeliveryTransport) DeliverBridge(
	_ context.Context,
	_ string,
	req bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	t.mu.Lock()
	if t.notify == nil {
		t.notify = make(chan struct{}, 1)
	}
	t.calls = append(t.calls, cloneExtensionDeliveryRequest(req))
	notify := t.notify
	t.mu.Unlock()
	select {
	case notify <- struct{}{}:
	default:
	}

	return bridgepkg.DeliveryAck{
		DeliveryID: req.Event.DeliveryID,
		Seq:        req.Event.Seq,
	}, nil
}

func (t *recordingDeliveryTransport) snapshotCalls() []bridgepkg.DeliveryRequest {
	t.mu.Lock()
	defer t.mu.Unlock()

	out := make([]bridgepkg.DeliveryRequest, 0, len(t.calls))
	for _, call := range t.calls {
		out = append(out, cloneExtensionDeliveryRequest(call))
	}
	return out
}

func (t *recordingDeliveryTransport) notifyChan() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.notify == nil {
		t.notify = make(chan struct{}, 1)
	}
	return t.notify
}

type recordingNotifier struct {
	mu      sync.Mutex
	created []*session.Session
	stopped []*session.Session
	events  []any
}

func (n *recordingNotifier) OnSessionCreated(_ context.Context, sess *session.Session) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.created = append(n.created, sess)
}

func (n *recordingNotifier) OnSessionStopped(_ context.Context, sess *session.Session) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.stopped = append(n.stopped, sess)
}

func (n *recordingNotifier) OnAgentEvent(_ context.Context, _ string, event any) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.events = append(n.events, event)
}

func (n *recordingNotifier) snapshot() (created int, stopped int, events []any) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return len(n.created), len(n.stopped), append([]any(nil), n.events...)
}

type recordingAgentEventNotifier struct {
	mu            sync.Mutex
	directCalls   int
	sessionCalls  int
	lastSessionID string
	lastEvent     any
}

func (n *recordingAgentEventNotifier) OnSessionCreated(context.Context, *session.Session) {}

func (n *recordingAgentEventNotifier) OnSessionStopped(context.Context, *session.Session) {}

func (n *recordingAgentEventNotifier) OnAgentEvent(_ context.Context, _ string, event any) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.directCalls++
	n.lastEvent = event
}

func (n *recordingAgentEventNotifier) OnAgentEventForSession(
	_ context.Context,
	sess *session.Session,
	event any,
) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.sessionCalls++
	if sess != nil {
		n.lastSessionID = sess.ID
	}
	n.lastEvent = event
}

func (n *recordingAgentEventNotifier) snapshot() (int, int, string, any) {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.directCalls, n.sessionCalls, n.lastSessionID, n.lastEvent
}

func TestBridgeDeliveryNotifierProjectsEventsAndForwardsLifecycle(t *testing.T) {
	t.Parallel()

	transport := &recordingDeliveryTransport{}
	broker := bridgepkg.NewBroker(transport)
	t.Cleanup(broker.Close)

	registration, err := broker.RegisterPromptDelivery(testutil.Context(t), bridgepkg.PromptDeliveryRegistration{
		SessionID:     "sess-notify",
		TurnID:        "turn-notify",
		ExtensionName: "ext-telegram",
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      "ws-1",
			BridgeInstanceID: "brg-notify",
			PeerID:           "peer-notify",
		},
		DeliveryTarget: bridgepkg.DeliveryTarget{
			BridgeInstanceID: "brg-notify",
			PeerID:           "peer-notify",
			Mode:             bridgepkg.DeliveryModeReply,
		},
	})
	if err != nil {
		t.Fatalf("RegisterPromptDelivery() error = %v", err)
	}

	downstream := &recordingNotifier{}
	notifier := NewBridgeDeliveryNotifier(broker, downstream)
	if notifier == nil {
		t.Fatal("NewBridgeDeliveryNotifier() = nil, want notifier")
	}

	sess := &session.Session{ID: registration.SessionID}
	notifier.OnSessionCreated(testutil.Context(t), sess)

	messageEvent := acp.AgentEvent{
		Type:      "agent_message",
		TurnID:    registration.TurnID,
		Timestamp: time.Date(2026, time.April, 11, 12, 3, 0, 0, time.UTC),
		Text:      "hello",
	}
	projected, err := projectionEventFromAgentEvent(&messageEvent)
	if err != nil {
		t.Fatalf("projectionEventFromAgentEvent() error = %v", err)
	}
	if projected.Fingerprint == "" {
		t.Fatal("projectionEventFromAgentEvent() fingerprint = empty, want stable fingerprint")
	}

	notifier.OnAgentEvent(testutil.Context(t), registration.SessionID, messageEvent)
	waitForExtensionDeliveryCalls(t, transport, 1)

	calls := transport.snapshotCalls()
	if got, want := calls[0].Event.EventType, bridgepkg.DeliveryEventTypeStart; got != want {
		t.Fatalf("projected event type = %q, want %q", got, want)
	}
	if got, want := calls[0].Event.Content.Text, "hello"; got != want {
		t.Fatalf("projected content = %q, want %q", got, want)
	}

	notifier.OnAgentEvent(testutil.Context(t), registration.SessionID, acp.AgentEvent{
		Type:      acp.EventTypeRuntimeProgress,
		TurnID:    registration.TurnID,
		Timestamp: time.Date(2026, time.April, 11, 12, 13, 0, 0, time.UTC),
		Text:      "Still working...",
	})
	if got, want := len(transport.snapshotCalls()), 1; got != want {
		t.Fatalf("delivery calls after runtime progress = %d, want %d", got, want)
	}

	notifier.OnAgentEvent(testutil.Context(t), registration.SessionID, "ignored")
	createdCount, stoppedCount, forwarded := downstream.snapshot()
	if createdCount != 1 || stoppedCount != 0 || len(forwarded) != 3 {
		t.Fatalf(
			"downstream lifecycle/events = created:%d stopped:%d events:%d, want 1/0/3",
			createdCount,
			stoppedCount,
			len(forwarded),
		)
	}

	notifier.OnSessionStopped(testutil.Context(t), sess)
	waitForExtensionDeliveryCalls(t, transport, 2)

	calls = transport.snapshotCalls()
	last := calls[len(calls)-1].Event
	if got, want := last.EventType, bridgepkg.DeliveryEventTypeError; got != want {
		t.Fatalf("session stop event type = %q, want %q", got, want)
	}
	if !last.Final {
		t.Fatal("session stop event Final = false, want true")
	}

	createdCount, stoppedCount, forwarded = downstream.snapshot()
	if createdCount != 1 || stoppedCount != 1 || len(forwarded) != 3 {
		t.Fatalf(
			"downstream lifecycle/events after stop = created:%d stopped:%d events:%d, want 1/1/3",
			createdCount,
			stoppedCount,
			len(forwarded),
		)
	}
}

func TestBridgeDeliveryNotifierNilPathsAreNoOps(t *testing.T) {
	t.Parallel()

	var notifier *BridgeDeliveryNotifier
	notifier.OnSessionCreated(testutil.Context(t), nil)
	notifier.OnSessionStopped(testutil.Context(t), nil)
	notifier.OnAgentEvent(testutil.Context(t), "sess-nil", nil)
	notifier.OnAgentEventForSession(testutil.Context(t), nil, nil)

	standalone := NewBridgeDeliveryNotifier(nil, nil)
	standalone.OnSessionCreated(testutil.Context(t), &session.Session{ID: "sess-nil"})
	standalone.OnSessionStopped(testutil.Context(t), &session.Session{ID: "sess-nil"})
	standalone.OnAgentEvent(testutil.Context(t), "sess-nil", "ignored")
	standalone.OnAgentEventForSession(testutil.Context(t), &session.Session{ID: "sess-nil"}, "ignored")
}

func TestBridgeDeliveryNotifierPreservesAgentEventNotifier(t *testing.T) {
	t.Parallel()

	downstream := &recordingAgentEventNotifier{}
	notifier := NewBridgeDeliveryNotifier(nil, downstream)

	aware, ok := any(notifier).(session.AgentEventNotifier)
	if !ok {
		t.Fatal("BridgeDeliveryNotifier does not implement session.AgentEventNotifier")
	}

	event := acp.AgentEvent{Type: "agent_message", TurnID: "turn-aware", Text: "hello"}
	sess := &session.Session{ID: "sess-aware"}
	aware.OnAgentEventForSession(testutil.Context(t), sess, event)

	directCalls, sessionCalls, lastSessionID, lastEvent := downstream.snapshot()
	if directCalls != 0 || sessionCalls != 1 {
		t.Fatalf(
			"downstream direct/session agent-event calls = %d/%d, want 0/1",
			directCalls,
			sessionCalls,
		)
	}
	if lastSessionID != "sess-aware" {
		t.Fatalf("downstream last session id = %q, want %q", lastSessionID, "sess-aware")
	}
	gotEvent, ok := lastEvent.(acp.AgentEvent)
	if !ok {
		t.Fatalf("downstream last event type = %T, want acp.AgentEvent", lastEvent)
	}
	if gotEvent.TurnID != "turn-aware" || gotEvent.Text != "hello" {
		t.Fatalf("downstream last event = %#v, want turn-aware/hello", gotEvent)
	}
}

func TestManagerDeliverBridge(t *testing.T) {
	t.Parallel()

	req := bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "del-manager",
			BridgeInstanceID: "brg-manager",
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-manager",
				PeerID:           "peer-manager",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-manager",
				PeerID:           "peer-manager",
				Mode:             bridgepkg.DeliveryModeReply,
			},
			Seq:       1,
			EventType: bridgepkg.DeliveryEventTypeStart,
			Content:   bridgepkg.MessageContent{Text: "hello"},
		},
	}

	t.Run("Should success", func(t *testing.T) {
		t.Parallel()

		process := newFakeProcess(9001)
		process.callFn = func(_ context.Context, method string, params, result any) error {
			if got, want := method, string(extensionprotocol.ExtensionServiceMethodBridgesDeliver); got != want {
				t.Fatalf("Call() method = %q, want %q", got, want)
			}
			typedReq, ok := params.(bridgepkg.DeliveryRequest)
			if !ok {
				t.Fatalf("Call() params type = %T, want bridge delivery request", params)
			}
			if got, want := typedReq.Event.DeliveryID, req.Event.DeliveryID; got != want {
				t.Fatalf("Call() delivery id = %q, want %q", got, want)
			}
			ack, ok := result.(*bridgepkg.DeliveryAck)
			if !ok {
				t.Fatalf("Call() result type = %T, want *DeliveryAck", result)
			}
			ack.DeliveryID = typedReq.Event.DeliveryID
			ack.Seq = typedReq.Event.Seq
			ack.RemoteMessageID = "remote-manager"
			return nil
		}

		manager := &Manager{
			extensions: map[string]*managedExtension{
				"ext-telegram": {
					active:  true,
					process: process,
					initialize: &subprocess.InitializeResponse{
						ImplementedMethods: []string{string(extensionprotocol.ExtensionServiceMethodBridgesDeliver)},
					},
				},
			},
		}

		ack, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", req)
		if err != nil {
			t.Fatalf("DeliverBridge() error = %v", err)
		}
		if got, want := ack.RemoteMessageID, "remote-manager"; got != want {
			t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
		}
	})

	t.Run("Should canceled context", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		manager := &Manager{}
		_, err := manager.DeliverBridge(ctx, "ext-telegram", req)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DeliverBridge() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should nil manager", func(t *testing.T) {
		t.Parallel()

		var manager *Manager
		_, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", req)
		if err == nil || !strings.Contains(err.Error(), "manager is required") {
			t.Fatalf("DeliverBridge() error = %v, want nil manager error", err)
		}
	})

	t.Run("Should inactive extension", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			extensions: map[string]*managedExtension{
				"ext-telegram": {},
			},
		}

		_, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", req)
		if !errors.Is(err, bridgepkg.ErrDeliveryTransportUnavailable) {
			t.Fatalf("DeliverBridge() error = %v, want ErrDeliveryTransportUnavailable", err)
		}
	})

	t.Run("Should missing negotiated method", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{
			extensions: map[string]*managedExtension{
				"ext-telegram": {
					active:     true,
					process:    newFakeProcess(9002),
					initialize: &subprocess.InitializeResponse{},
				},
			},
		}

		_, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", req)
		if !errors.Is(err, bridgepkg.ErrDeliveryTransportUnavailable) {
			t.Fatalf("DeliverBridge() error = %v, want wrapped ErrDeliveryTransportUnavailable", err)
		}
	})

	t.Run("Should process call failure", func(t *testing.T) {
		t.Parallel()

		process := newFakeProcess(9003)
		process.callFn = func(context.Context, string, any, any) error {
			return errors.New("rpc failed")
		}

		manager := &Manager{
			extensions: map[string]*managedExtension{
				"ext-telegram": {
					active:  true,
					process: process,
					initialize: &subprocess.InitializeResponse{
						ImplementedMethods: []string{string(extensionprotocol.ExtensionServiceMethodBridgesDeliver)},
					},
				},
			},
		}

		_, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", req)
		if err == nil || !strings.Contains(err.Error(), "rpc failed") {
			t.Fatalf("DeliverBridge() error = %v, want wrapped process failure", err)
		}
	})

	t.Run("Should terminalize a received but invalid acknowledgement", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name    string
			payload string
		}{
			{name: "Should reject an omitted sequence", payload: `{"delivery_id":"del-manager"}`},
			{name: "Should reject a null sequence", payload: `{"delivery_id":"del-manager","seq":null}`},
			{name: "Should reject a string sequence", payload: `{"delivery_id":"del-manager","seq":"1"}`},
			{name: "Should reject a mismatched identity", payload: `{"delivery_id":"other","seq":1}`},
			{name: "Should reject a non-object result", payload: `[]`},
		}
		for index, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				process := newFakeProcess(9100 + index)
				process.callFn = func(_ context.Context, method string, _ any, result any) error {
					if err := json.Unmarshal([]byte(test.payload), result); err != nil {
						return &subprocess.ResponseDecodeError{Method: method, Err: err}
					}
					return nil
				}
				manager := &Manager{
					extensions: map[string]*managedExtension{
						"ext-telegram": {
							active:  true,
							process: process,
							initialize: &subprocess.InitializeResponse{
								ImplementedMethods: []string{
									string(extensionprotocol.ExtensionServiceMethodBridgesDeliver),
								},
							},
						},
					},
				}

				ack, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", req)
				if err != nil {
					t.Fatalf("DeliverBridge() error = %v, want semantic acknowledgement", err)
				}
				if err := ack.ValidateFor(req.Event); err != nil {
					t.Fatalf("semantic acknowledgement validation error = %v", err)
				}
				if ack.Outcome != bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable ||
					ack.Error == nil || ack.Error.Message != bridgepkg.DeliveryResultUnavailableMessage {
					t.Fatalf("DeliverBridge() ack = %#v, want fixed indeterminate outcome", ack)
				}
				if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
					t.Fatalf("DeliverBridge() ack = %#v, want no fabricated remote identities", ack)
				}
			})
		}
	})

	t.Run("Should invalid request", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{}
		_, err := manager.DeliverBridge(testutil.Context(t), "ext-telegram", bridgepkg.DeliveryRequest{})
		if err == nil {
			t.Fatal("DeliverBridge() error = nil, want validation error")
		}
	})

	t.Run("Should missing extension name", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{}
		_, err := manager.DeliverBridge(testutil.Context(t), "  ", req)
		if err == nil || !strings.Contains(err.Error(), "extension name is required") {
			t.Fatalf("DeliverBridge() error = %v, want missing extension name error", err)
		}
	})
}

func cloneExtensionDeliveryRequest(req bridgepkg.DeliveryRequest) bridgepkg.DeliveryRequest {
	cloned := req
	cloned.Event.ProviderMetadata = append([]byte(nil), req.Event.ProviderMetadata...)
	if req.Event.Reference != nil {
		reference := *req.Event.Reference
		cloned.Event.Reference = &reference
	}
	if req.Event.Error != nil {
		errorDetail := *req.Event.Error
		cloned.Event.Error = &errorDetail
	}
	if req.Event.Resume != nil {
		resume := *req.Event.Resume
		cloned.Event.Resume = &resume
	}
	if req.Snapshot != nil {
		snapshot := *req.Snapshot
		snapshot.ProviderMetadata = append([]byte(nil), req.Snapshot.ProviderMetadata...)
		if req.Snapshot.Reference != nil {
			reference := *req.Snapshot.Reference
			snapshot.Reference = &reference
		}
		cloned.Snapshot = &snapshot
	}
	return cloned
}

func waitForExtensionDeliveryCalls(t *testing.T, transport *recordingDeliveryTransport, want int) {
	t.Helper()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()

	notify := transport.notifyChan()
	for {
		if len(transport.snapshotCalls()) >= want {
			return
		}
		select {
		case <-notify:
		case <-timer.C:
			t.Fatalf(
				"delivery call count did not reach %d before timeout; got %d",
				want,
				len(transport.snapshotCalls()),
			)
		}
	}
}

func assertExtensionDeliveryCallCountStable(
	t *testing.T,
	transport *recordingDeliveryTransport,
	want int,
	duration time.Duration,
) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()
	notify := transport.notifyChan()
	for {
		select {
		case <-notify:
			if got := len(transport.snapshotCalls()); got > want {
				t.Fatalf("delivery call count = %d, want stable %d", got, want)
			}
		case <-timer.C:
			if got := len(transport.snapshotCalls()); got != want {
				t.Fatalf("delivery call count = %d, want stable %d", got, want)
			}
			return
		}
	}
}
