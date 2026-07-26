package bridges

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolmeta"
)

func TestBrokerSetTransportFlushesQueuedResume(t *testing.T) {
	t.Parallel()

	parentCtx, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()

	now := time.Date(2026, time.April, 11, 12, 0, 0, 0, time.UTC)
	broker := NewBroker(
		nil,
		WithDeliveryBrokerNow(func() time.Time { return now }),
		WithDeliveryBrokerRetryDelay(5*time.Millisecond),
		WithDeliveryBrokerRequestTimeout(75*time.Millisecond),
		WithDeliveryBrokerLifecycleContext(parentCtx),
	)
	t.Cleanup(broker.Close)

	if got, want := broker.requestTimeout, 75*time.Millisecond; got != want {
		t.Fatalf("broker.requestTimeout = %s, want %s", got, want)
	}
	if got := broker.now(); !got.Equal(now) {
		t.Fatalf("broker.now() = %s, want %s", got, now)
	}

	reg := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
		SessionID:     "sess-resume-route",
		TurnID:        "turn-resume-route",
		ExtensionName: "ext-telegram",
		RoutingKey:    testRoutingKey("brg-resume-route", "peer-resume-route"),
		DeliveryTarget: DeliveryTarget{
			BridgeInstanceID: "brg-resume-route",
			PeerID:           "peer-resume-route",
			Mode:             DeliveryModeReply,
		},
	})

	ctx := testutil.Context(t)
	if err := broker.Deliver(ctx, testDeliveryEvent(
		reg.DeliveryID,
		reg.BridgeInstanceID,
		reg.RoutingKey,
		reg.DeliveryTarget,
		1,
		DeliveryEventTypeStart,
		"queued",
		false,
	)); err != nil {
		t.Fatalf("Deliver(start) error = %v", err)
	}

	waitForSnapshot(t, broker, reg.DeliveryID, func(snapshot *DeliverySnapshot) bool {
		if snapshot.LastSentSeq != 1 || snapshot.LastAckedSeq != 0 {
			return false
		}
		metrics := broker.DeliveryMetrics()
		entry, ok := metrics[reg.BridgeInstanceID]
		return ok && strings.TrimSpace(entry.LastError) != ""
	})

	transport := &fakeDeliveryTransport{}
	broker.SetTransport(transport)
	waitForCalls(t, transport, 1)

	calls := transport.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("len(delivery calls) = %d, want 1", len(calls))
	}
	if got, want := calls[0].request.Event.EventType, DeliveryEventTypeResume; got != want {
		t.Fatalf("resume event type = %q, want %q", got, want)
	}
	if calls[0].request.Snapshot == nil {
		t.Fatal("resume request snapshot = nil, want non-nil")
	}
	if got, want := calls[0].request.Snapshot.DeliveryID, reg.DeliveryID; got != want {
		t.Fatalf("resume snapshot delivery id = %q, want %q", got, want)
	}

	cancelParent()
	select {
	case <-broker.lifecycleCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("broker lifecycle context was not canceled")
	}
}

func TestBrokerProjectEventDeduplicatesAndFailsSession(t *testing.T) {
	t.Parallel()

	transport := &fakeDeliveryTransport{}
	broker := NewBroker(transport)
	t.Cleanup(broker.Close)

	seedTime := time.Date(2026, time.April, 11, 12, 1, 0, 0, time.UTC)
	reg := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
		SessionID:     "sess-project",
		TurnID:        "turn-project",
		ExtensionName: "ext-telegram",
		RoutingKey:    testRoutingKey("brg-project", "peer-project"),
		DeliveryTarget: DeliveryTarget{
			BridgeInstanceID: "brg-project",
			PeerID:           "peer-project",
			Mode:             DeliveryModeReply,
		},
		SeedEvents: []DeliveryProjectionEvent{
			{
				Type:        "agent_message",
				TurnID:      "turn-project",
				Timestamp:   seedTime,
				Text:        "hello",
				Fingerprint: "seed-1",
			},
		},
	})

	waitForCalls(t, transport, 1)
	calls := transport.snapshotCalls()
	assertDeliveryOrder(t, calls, reg.DeliveryID, []DeliveryEventType{DeliveryEventTypeStart}, []int64{1})
	if got, want := calls[0].request.Event.Content.Text, "hello"; got != want {
		t.Fatalf("seed content = %q, want %q", got, want)
	}

	ctx := testutil.Context(t)
	if err := broker.ProjectEvent(ctx, reg.SessionID, DeliveryProjectionEvent{
		Type:        "agent_message",
		TurnID:      reg.TurnID,
		Timestamp:   seedTime,
		Text:        "hello",
		Fingerprint: "seed-1",
	}); err != nil {
		t.Fatalf("ProjectEvent(duplicate) error = %v", err)
	}
	assertCallCountStable(t, transport, 1, 50*time.Millisecond)

	nextEvent := DeliveryProjectionEvent{
		Type:      "agent_message",
		TurnID:    reg.TurnID,
		Timestamp: seedTime.Add(time.Second),
		Text:      " world",
	}
	if err := broker.ProjectEvent(ctx, reg.SessionID, nextEvent); err != nil {
		t.Fatalf("ProjectEvent(delta) error = %v", err)
	}

	waitForCalls(t, transport, 2)
	calls = transport.snapshotCalls()
	assertDeliveryOrder(
		t,
		calls,
		reg.DeliveryID,
		[]DeliveryEventType{DeliveryEventTypeStart, DeliveryEventTypeDelta},
		[]int64{1, 2},
	)
	if got, want := calls[1].request.Event.Content.Text, "hello world"; got != want {
		t.Fatalf("delta content = %q, want %q", got, want)
	}

	if got := agentEventFingerprint(nextEvent); !strings.Contains(got, "agent_message|turn-project|") {
		t.Fatalf("agentEventFingerprint() = %q, want composed fingerprint", got)
	}

	if err := broker.FailSession(ctx, reg.SessionID, "adapter stopped"); err != nil {
		t.Fatalf("FailSession() error = %v", err)
	}

	waitForCalls(t, transport, 3)
	calls = transport.snapshotCalls()
	assertDeliveryOrder(
		t,
		calls,
		reg.DeliveryID,
		[]DeliveryEventType{DeliveryEventTypeStart, DeliveryEventTypeDelta, DeliveryEventTypeError},
		[]int64{1, 2, 3},
	)
	last := calls[len(calls)-1].request.Event
	if !last.Final {
		t.Fatal("failed-session event Final = false, want true")
	}
	if last.Error == nil {
		t.Fatal("failed-session event Error = nil, want typed error payload")
	}
	if got, want := last.Error.Message, "adapter stopped"; got != want {
		t.Fatalf("last.Error.Message = %q, want %q", got, want)
	}
}

func TestBrokerProgressProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should project tool calls by mode with monotonic per-turn indexes", func(t *testing.T) {
		t.Parallel()

		empty, ok, err := ProjectAgentEvent(DeliveryProjectionEvent{
			Type:   "tool_call",
			TurnID: "turn-progress-contract",
			Tool: &ToolProgress{
				ToolCallID: "call-contract",
				ToolID:     "agh__terminal",
				Phase:      ToolProgressPhaseStarted,
			},
		}, ProgressModeAll)
		if err != nil || !ok {
			t.Fatalf("ProjectAgentEvent(tool_call) = (%v, %v), want nil and ok=true", err, ok)
		}
		if empty.Progress == nil {
			t.Fatal("ProjectAgentEvent(tool_call).Progress = nil, want typed progress")
		}
		if empty.Progress.Label != "" || empty.Progress.Preview != "" || empty.Progress.Emoji != "" {
			t.Fatalf(
				"empty progress chrome = %#v, want label/preview/emoji empty before broker rendering",
				empty.Progress,
			)
		}

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-all",
			TurnID:        "turn-progress-all",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-all", "peer-progress-all"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-all",
				PeerID:           "peer-progress-all",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		for index, tool := range []struct {
			callID string
			toolID string
			input  json.RawMessage
		}{
			{callID: "call-1", toolID: "agh__terminal", input: json.RawMessage(`{"command":"printf '%s' sk-progress-secret"}`)},
			{callID: "call-2", toolID: "agh__task_list", input: json.RawMessage(`{"status":"open"}`)},
		} {
			err := broker.ProjectEvent(ctx, registration.SessionID, DeliveryProjectionEvent{
				Type:   "tool_call",
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: tool.callID,
					ToolID:     tool.toolID,
					Phase:      ToolProgressPhaseStarted,
				},
				ToolInput: tool.input,
			})
			if err != nil {
				t.Fatalf("ProjectEvent(tool_call %d) error = %v", index+1, err)
			}
		}

		waitForCalls(t, transport, 2)
		calls := transport.snapshotCalls()
		for index, call := range calls {
			progress := call.request.Event.Progress
			if got, want := call.request.Event.EventType, DeliveryEventTypeProgress; got != want {
				t.Fatalf("calls[%d].EventType = %q, want %q", index, got, want)
			}
			if progress == nil {
				t.Fatalf("calls[%d].Progress = nil, want typed progress", index)
			}
			if got, want := progress.Index, index+1; got != want {
				t.Fatalf("calls[%d].Progress.Index = %d, want %d", index, got, want)
			}
			if got, want := progress.Phase, ToolProgressPhaseStarted; got != want {
				t.Fatalf("calls[%d].Progress.Phase = %q, want %q", index, got, want)
			}
		}
		preview := calls[0].request.Event.Progress.Preview
		if got, want := preview, `"printf"`; got != want {
			t.Fatalf("terminal preview = %q, want command-name-only %q", got, want)
		}
		for _, forbidden := range []string{"%s", "sk-progress-secret"} {
			if strings.Contains(preview, forbidden) {
				t.Fatalf("terminal preview = %q, want no argument %q", preview, forbidden)
			}
		}
		if calls[0].request.Event.Progress.Label == "" || calls[0].request.Event.Progress.Emoji == "" {
			t.Fatalf("rendered progress = %#v, want label and emoji", calls[0].request.Event.Progress)
		}

		offTransport := &fakeDeliveryTransport{}
		offBroker := NewBroker(offTransport)
		t.Cleanup(offBroker.Close)
		offRegistration := mustRegisterTestDelivery(t, offBroker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-off",
			TurnID:        "turn-progress-off",
			ExtensionName: "ext-teams",
			RoutingKey:    testRoutingKey("brg-progress-off", "peer-progress-off"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-off",
				PeerID:           "peer-progress-off",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeOff,
				Grouping:     ProgressGroupingAccumulate,
			},
		})
		if err := offBroker.ProjectEvent(ctx, offRegistration.SessionID, DeliveryProjectionEvent{
			Type:   "tool_call",
			TurnID: offRegistration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-off",
				ToolID:     "agh__terminal",
				Phase:      ToolProgressPhaseStarted,
			},
		}); err != nil {
			t.Fatalf("ProjectEvent(mode off) error = %v", err)
		}
		assertCallCountStable(t, offTransport, 0, 50*time.Millisecond)
	})

	t.Run("Should preserve the curated connector in final progress fields", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-connector",
			TurnID:        "turn-progress-connector",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-connector", "peer-progress-connector"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-connector",
				PeerID:           "peer-progress-connector",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		err := broker.ProjectEvent(testutil.Context(t), registration.SessionID, DeliveryProjectionEvent{
			Type:   projectionEventToolCall,
			TurnID: registration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-progress-connector",
				ToolID:     "agh__skill_search",
				Phase:      ToolProgressPhaseStarted,
			},
			ToolInput: json.RawMessage(`{"query":"deployment"}`),
		})
		if err != nil {
			t.Fatalf("ProjectEvent(search tool) error = %v", err)
		}

		waitForCalls(t, transport, 1)
		progress := transport.snapshotCalls()[0].request.Event.Progress
		if progress == nil {
			t.Fatal("search progress = nil, want typed progress")
		}
		if got, want := progress.Label, "Searching for"; got != want {
			t.Fatalf("search progress label = %q, want %q", got, want)
		}
		if got, want := progress.Preview, "deployment"; got != want {
			t.Fatalf("search progress preview = %q, want %q", got, want)
		}
		if got, want := progress.Emoji+" "+progress.Label+" "+progress.Preview,
			"🧰 Searching for deployment"; got != want {
			t.Fatalf("search progress rendered fields = %q, want %q", got, want)
		}
	})

	t.Run("Should render the exact unknown tool fallback fields", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-fallback",
			TurnID:        "turn-progress-fallback",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-fallback", "peer-progress-fallback"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-fallback",
				PeerID:           "peer-progress-fallback",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		err := broker.ProjectEvent(testutil.Context(t), registration.SessionID, DeliveryProjectionEvent{
			Type:   projectionEventToolCall,
			TurnID: registration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-fallback",
				ToolID:     "vendor__lookup",
				Phase:      ToolProgressPhaseStarted,
			},
			ToolInput: json.RawMessage(`{"query":"BUG-123"}`),
		})
		if err != nil {
			t.Fatalf("ProjectEvent(unknown tool) error = %v", err)
		}

		waitForCalls(t, transport, 1)
		progress := transport.snapshotCalls()[0].request.Event.Progress
		if progress == nil {
			t.Fatal("unknown tool progress = nil, want typed progress")
		}
		if got, want := progress.Emoji, "⚙️"; got != want {
			t.Fatalf("unknown tool emoji = %q, want %q", got, want)
		}
		if got, want := progress.Label, "vendor__lookup:"; got != want {
			t.Fatalf("unknown tool label = %q, want %q", got, want)
		}
		if got, want := progress.Preview, `"BUG-123"`; got != want {
			t.Fatalf("unknown tool preview = %q, want %q", got, want)
		}
		if got, want := progress.Emoji+" "+progress.Label+" "+progress.Preview,
			`⚙️ vendor__lookup: "BUG-123"`; got != want {
			t.Fatalf("unknown tool rendered fields = %q, want %q", got, want)
		}

		err = broker.ProjectEvent(testutil.Context(t), registration.SessionID, DeliveryProjectionEvent{
			Type:   projectionEventToolCall,
			TurnID: registration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-fallback-quoted",
				ToolID:     "vendor__lookup",
				Phase:      ToolProgressPhaseStarted,
			},
			ToolInput: json.RawMessage(`{"query":"say \"hi\" \\tmp"}`),
		})
		if err != nil {
			t.Fatalf("ProjectEvent(quoted unknown tool) error = %v", err)
		}

		waitForCalls(t, transport, 2)
		quoted := transport.snapshotCalls()[1].request.Event.Progress
		if quoted == nil {
			t.Fatal("quoted unknown tool progress = nil, want typed progress")
		}
		if got, want := quoted.Label, "vendor__lookup:"; got != want {
			t.Fatalf("quoted unknown tool label = %q, want %q", got, want)
		}
		if got, want := quoted.Preview, `"say \"hi\" \\tmp"`; got != want {
			t.Fatalf("quoted unknown tool preview = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve explicit event metadata ahead of the registry fallback", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(
			transport,
			WithDeliveryBrokerDescriptorLookup(descriptorLookupFunc(func(
				string,
				string,
			) (toolmeta.DescriptorMetadata, bool) {
				return toolmeta.DescriptorMetadata{
					ID:           "vendor__lookup",
					FriendlyVerb: "Registry lookup",
					Preview:      "none",
				}, true
			})),
		)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-explicit",
			TurnID:        "turn-progress-explicit",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-explicit", "peer-progress-explicit"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-explicit",
				PeerID:           "peer-progress-explicit",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		err := broker.ProjectEvent(testutil.Context(t), registration.SessionID, DeliveryProjectionEvent{
			Type:   projectionEventToolCall,
			TurnID: registration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-explicit",
				ToolID:     "vendor__lookup",
				Phase:      ToolProgressPhaseStarted,
			},
			ToolInput: json.RawMessage(`{"query":"BUG-456"}`),
			ToolMetadata: toolmeta.DescriptorMetadata{
				ID:           "vendor__lookup",
				FriendlyVerb: "Explicit lookup",
				Preview:      "arg:query",
			},
		})
		if err != nil {
			t.Fatalf("ProjectEvent(explicit metadata) error = %v", err)
		}

		waitForCalls(t, transport, 1)
		progress := transport.snapshotCalls()[0].request.Event.Progress
		if progress == nil {
			t.Fatal("explicit metadata progress = nil, want typed progress")
		}
		if got, want := progress.Label, "Explicit lookup"; got != want {
			t.Fatalf("explicit metadata label = %q, want %q", got, want)
		}
		if got, want := progress.Preview, "BUG-456"; got != want {
			t.Fatalf("explicit metadata preview = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve registry metadata in the delivery workspace", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(
			transport,
			WithDeliveryBrokerDescriptorLookup(descriptorLookupFunc(func(
				workspaceID string,
				toolID string,
			) (toolmeta.DescriptorMetadata, bool) {
				if toolID != "vendor__lookup" {
					return toolmeta.DescriptorMetadata{}, false
				}
				switch workspaceID {
				case "ws-a":
					return toolmeta.DescriptorMetadata{
						ID:           toolID,
						FriendlyVerb: "Workspace A lookup",
						Preview:      "arg:query",
					}, true
				case "ws-b":
					return toolmeta.DescriptorMetadata{
						ID:           toolID,
						FriendlyVerb: "Workspace B lookup",
						Preview:      "arg:query",
					}, true
				default:
					return toolmeta.DescriptorMetadata{}, false
				}
			})),
		)
		t.Cleanup(broker.Close)

		for _, delivery := range []struct {
			workspaceID string
			bridgeID    string
			sessionID   string
			turnID      string
			wantLabel   string
		}{
			{
				workspaceID: "ws-a",
				bridgeID:    "brg-progress-workspace-a",
				sessionID:   "sess-progress-workspace-a",
				turnID:      "turn-progress-workspace-a",
				wantLabel:   "Workspace A lookup",
			},
			{
				workspaceID: "ws-b",
				bridgeID:    "brg-progress-workspace-b",
				sessionID:   "sess-progress-workspace-b",
				turnID:      "turn-progress-workspace-b",
				wantLabel:   "Workspace B lookup",
			},
		} {
			registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
				SessionID:     delivery.sessionID,
				TurnID:        delivery.turnID,
				ExtensionName: "ext-slack",
				RoutingKey: RoutingKey{
					Scope:            ScopeWorkspace,
					WorkspaceID:      delivery.workspaceID,
					BridgeInstanceID: delivery.bridgeID,
					PeerID:           "peer-" + delivery.workspaceID,
				},
				DeliveryTarget: DeliveryTarget{
					BridgeInstanceID: delivery.bridgeID,
					PeerID:           "peer-" + delivery.workspaceID,
					Mode:             DeliveryModeReply,
				},
				Progress: ProgressConfig{
					ToolProgress: ProgressModeAll,
					Grouping:     ProgressGroupingAccumulate,
				},
			})
			wantCallCount := len(transport.snapshotCalls()) + 1
			err := broker.ProjectEvent(testutil.Context(t), registration.SessionID, DeliveryProjectionEvent{
				Type:   projectionEventToolCall,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-" + delivery.workspaceID,
					ToolID:     "vendor__lookup",
					Phase:      ToolProgressPhaseStarted,
				},
				ToolInput: json.RawMessage(`{"query":"BUG-789"}`),
			})
			if err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", delivery.workspaceID, err)
			}

			waitForCalls(t, transport, wantCallCount)
			calls := transport.snapshotCalls()
			progress := calls[len(calls)-1].request.Event.Progress
			if progress == nil {
				t.Fatalf("%s progress = nil, want typed progress", delivery.workspaceID)
			}
			if got := progress.Label; got != delivery.wantLabel {
				t.Fatalf("%s progress label = %q, want %q", delivery.workspaceID, got, delivery.wantLabel)
			}
		}
	})

	t.Run("Should never render a sensitive descriptor argument", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(
			transport,
			WithDeliveryBrokerDescriptorLookup(descriptorLookupFunc(func(
				string,
				string,
			) (toolmeta.DescriptorMetadata, bool) {
				return toolmeta.DescriptorMetadata{
					ID:           "mcp__signing_key",
					FriendlyVerb: "Signing",
					Preview:      "arg:privateKey",
				}, true
			})),
		)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-sensitive-arg",
			TurnID:        "turn-progress-sensitive-arg",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-sensitive-arg", "peer-progress-sensitive-arg"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-sensitive-arg",
				PeerID:           "peer-progress-sensitive-arg",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		err := broker.ProjectEvent(testutil.Context(t), registration.SessionID, DeliveryProjectionEvent{
			Type:   projectionEventToolCall,
			TurnID: registration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-sensitive-arg",
				ToolID:     "mcp__signing_key",
				Phase:      ToolProgressPhaseStarted,
			},
			ToolInput: json.RawMessage(`{"privateKey":"raw-material-without-secret-shape"}`),
		})
		if err != nil {
			t.Fatalf("ProjectEvent(sensitive descriptor argument) error = %v", err)
		}

		waitForCalls(t, transport, 1)
		progress := transport.snapshotCalls()[0].request.Event.Progress
		if progress == nil {
			t.Fatal("sensitive descriptor progress = nil, want typed progress")
		}
		if got, want := progress.Label, "Signing"; got != want {
			t.Fatalf("sensitive descriptor label = %q, want %q", got, want)
		}
		if progress.Preview != "" {
			t.Fatalf("sensitive descriptor preview = %q, want empty", progress.Preview)
		}
	})

	t.Run("Should deduplicate consecutive tool ids in new mode", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-new",
			TurnID:        "turn-progress-new",
			ExtensionName: "ext-discord",
			RoutingKey:    testRoutingKey("brg-progress-new", "peer-progress-new"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-new",
				PeerID:           "peer-progress-new",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeNew,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		for _, tool := range []struct {
			callID string
			toolID string
		}{
			{callID: "call-new-1", toolID: "agh__terminal"},
			{callID: "call-new-2", toolID: "agh__terminal"},
			{callID: "call-new-3", toolID: "agh__task_list"},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, DeliveryProjectionEvent{
				Type:   "tool_call",
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: tool.callID,
					ToolID:     tool.toolID,
					Phase:      ToolProgressPhaseStarted,
				},
			}); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", tool.callID, err)
			}
		}

		waitForCalls(t, transport, 2)
		assertCallCountStable(t, transport, 2, 50*time.Millisecond)
		calls := transport.snapshotCalls()
		if got, want := calls[0].request.Event.Progress.ToolID, "agh__terminal"; got != want {
			t.Fatalf("first progress tool id = %q, want %q", got, want)
		}
		if got, want := calls[1].request.Event.Progress.ToolID, "agh__task_list"; got != want {
			t.Fatalf("second progress tool id = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve terminal phases while new mode deduplicates starts", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-new-terminal",
			TurnID:        "turn-progress-new-terminal",
			ExtensionName: "ext-discord",
			RoutingKey:    testRoutingKey("brg-progress-new-terminal", "peer-progress-new-terminal"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-new-terminal",
				PeerID:           "peer-progress-new-terminal",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeNew,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		startedAt := time.Date(2026, time.July, 12, 12, 0, 0, 0, time.UTC)
		for _, event := range []DeliveryProjectionEvent{
			{
				Type:      projectionEventToolCall,
				TurnID:    registration.TurnID,
				Timestamp: startedAt,
				Tool: &ToolProgress{
					ToolCallID: "call-new-terminal-1",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{
				Type:      projectionEventToolResult,
				TurnID:    registration.TurnID,
				Timestamp: startedAt.Add(31 * time.Second),
				Tool: &ToolProgress{
					ToolCallID: "call-new-terminal-1",
					Phase:      ToolProgressPhaseCompleted,
				},
			},
			{
				Type:      projectionEventToolCall,
				TurnID:    registration.TurnID,
				Timestamp: startedAt.Add(32 * time.Second),
				Tool: &ToolProgress{
					ToolCallID: "call-new-terminal-2",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{
				Type:      projectionEventToolResult,
				TurnID:    registration.TurnID,
				Timestamp: startedAt.Add(34 * time.Second),
				Tool: &ToolProgress{
					ToolCallID: "call-new-terminal-2",
					Phase:      ToolProgressPhaseFailed,
					Error:      "password=hunter2 " + strings.Repeat("x", 200),
				},
			},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", event.Type, err)
			}
		}

		waitForCalls(t, transport, 3)
		assertCallCountStable(t, transport, 3, 50*time.Millisecond)
		calls := transport.snapshotCalls()
		wantPhases := []ToolProgressPhase{
			ToolProgressPhaseStarted,
			ToolProgressPhaseCompleted,
			ToolProgressPhaseFailed,
		}
		for index, want := range wantPhases {
			progress := calls[index].request.Event.Progress
			if progress == nil || progress.Phase != want {
				t.Fatalf("calls[%d].Progress = %#v, want phase %q", index, progress, want)
			}
		}
		if got, want := calls[1].request.Event.Progress.DurationMS, int64(31_000); got != want {
			t.Fatalf("completed duration = %d, want %d", got, want)
		}
		failed := calls[2].request.Event.Progress
		if got, want := failed.DurationMS, int64(2_000); got != want {
			t.Fatalf("failed duration = %d, want %d", got, want)
		}
		if strings.Contains(failed.Error, "hunter2") || !strings.Contains(failed.Error, "[REDACTED]") {
			t.Fatalf("failed error = %q, want canonical redaction", failed.Error)
		}
		if got := utf8.RuneCountInString(failed.Error); got > maxToolProgressErrorRunes {
			t.Fatalf("failed error rune count = %d, want <= %d", got, maxToolProgressErrorRunes)
		}
	})

	t.Run("Should emit one started phase for repeated ACP lifecycle updates", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-lifecycle",
			TurnID:        "turn-progress-lifecycle",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-lifecycle", "peer-progress-lifecycle"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-lifecycle",
				PeerID:           "peer-progress-lifecycle",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		for _, event := range []DeliveryProjectionEvent{
			{
				Type:   projectionEventToolCall,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-lifecycle",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{
				Type:   projectionEventToolCall,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-lifecycle",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{
				Type:   projectionEventToolResult,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-lifecycle",
					Phase:      ToolProgressPhaseCompleted,
				},
			},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", event.Type, err)
			}
		}

		waitForCalls(t, transport, 2)
		assertCallCountStable(t, transport, 2, 50*time.Millisecond)
		calls := transport.snapshotCalls()
		if got, want := calls[0].request.Event.Progress.Phase, ToolProgressPhaseStarted; got != want {
			t.Fatalf("first progress phase = %q, want %q", got, want)
		}
		if got, want := calls[1].request.Event.Progress.Phase, ToolProgressPhaseCompleted; got != want {
			t.Fatalf("second progress phase = %q, want %q", got, want)
		}
	})

	t.Run("Should project the same tool again after content opens a fresh bubble", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-new-content",
			TurnID:        "turn-progress-new-content",
			ExtensionName: "ext-discord",
			RoutingKey:    testRoutingKey("brg-progress-new-content", "peer-progress-new-content"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-new-content",
				PeerID:           "peer-progress-new-content",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeNew,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		firstTool := DeliveryProjectionEvent{
			Type:   projectionEventToolCall,
			TurnID: registration.TurnID,
			Tool: &ToolProgress{
				ToolCallID: "call-new-content-1",
				ToolID:     "agh__terminal",
				Phase:      ToolProgressPhaseStarted,
			},
		}
		if err := broker.ProjectEvent(ctx, registration.SessionID, firstTool); err != nil {
			t.Fatalf("ProjectEvent(first tool) error = %v", err)
		}
		waitForCalls(t, transport, 1)

		if err := broker.ProjectEvent(ctx, registration.SessionID, DeliveryProjectionEvent{
			Type:   projectionEventAgentMessage,
			TurnID: registration.TurnID,
			Text:   "answer",
		}); err != nil {
			t.Fatalf("ProjectEvent(content) error = %v", err)
		}
		waitForCalls(t, transport, 2)

		secondTool := firstTool
		secondTool.Tool = cloneToolProgress(firstTool.Tool)
		secondTool.Tool.ToolCallID = "call-new-content-2"
		if err := broker.ProjectEvent(ctx, registration.SessionID, secondTool); err != nil {
			t.Fatalf("ProjectEvent(second tool) error = %v", err)
		}
		waitForCalls(t, transport, 3)

		calls := transport.snapshotCalls()
		assertDeliveryOrder(t, calls, registration.DeliveryID, []DeliveryEventType{
			DeliveryEventTypeProgress,
			DeliveryEventTypeStart,
			DeliveryEventTypeProgress,
		}, []int64{1, 2, 3})
		if got, want := calls[2].request.Event.Progress.ToolID, "agh__terminal"; got != want {
			t.Fatalf("second progress tool id = %q, want %q", got, want)
		}
	})

	t.Run("Should drain progress-only turns through one lifecycle terminal", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-only",
			TurnID:        "turn-progress-only",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-progress-only", "peer-progress-only"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-only",
				PeerID:           "peer-progress-only",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		for _, event := range []DeliveryProjectionEvent{
			{
				Type:   projectionEventToolCall,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-progress-only",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{
				Type:   projectionEventToolResult,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-progress-only",
					Phase:      ToolProgressPhaseCompleted,
				},
			},
			{Type: projectionEventDone, TurnID: registration.TurnID},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", event.Type, err)
			}
		}

		waitForCalls(t, transport, 3)
		waitForAcks(t, transport, 3)
		calls := transport.snapshotCalls()
		assertDeliveryOrder(t, calls, registration.DeliveryID, []DeliveryEventType{
			DeliveryEventTypeProgress,
			DeliveryEventTypeProgress,
			DeliveryEventTypeFinal,
		}, []int64{1, 2, 3})
		terminal := calls[2].request.Event
		if !terminal.Final || terminal.Content.Text != "" {
			t.Fatalf("progress-only terminal = %#v, want one empty lifecycle final", terminal)
		}
		waitForDeliveryRemoval(t, broker, registration.DeliveryID)
	})

	t.Run("Should keep a projected terminal irrevocable while its acknowledgement is in flight", func(t *testing.T) {
		t.Parallel()

		finalEntered := make(chan struct{}, 1)
		releaseFinal := make(chan struct{})
		t.Cleanup(func() {
			select {
			case <-releaseFinal:
			default:
				close(releaseFinal)
			}
		})
		transport := &fakeDeliveryTransport{handler: func(
			ctx context.Context,
			_ string,
			req DeliveryRequest,
		) (DeliveryAck, error) {
			if req.Event.EventType == DeliveryEventTypeFinal {
				select {
				case finalEntered <- struct{}{}:
				default:
				}
				select {
				case <-releaseFinal:
				case <-ctx.Done():
					return DeliveryAck{}, ctx.Err()
				}
			}
			return DeliveryAck{DeliveryID: req.Event.DeliveryID, Seq: req.Event.Seq}, nil
		}}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-terminal-irrevocable",
			TurnID:        "turn-terminal-irrevocable",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-terminal-irrevocable", "peer-terminal-irrevocable"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-terminal-irrevocable",
				PeerID:           "peer-terminal-irrevocable",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		for _, event := range []DeliveryProjectionEvent{
			{Type: projectionEventAgentMessage, TurnID: registration.TurnID, Text: "answer"},
			{Type: projectionEventDone, TurnID: registration.TurnID},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", event.Type, err)
			}
		}
		select {
		case <-finalEntered:
		case <-time.After(time.Second):
			t.Fatal("final delivery did not enter the transport")
		}

		for _, late := range []DeliveryProjectionEvent{
			{Type: projectionEventAgentMessage, TurnID: registration.TurnID, Text: "late"},
			{
				Type:   projectionEventToolCall,
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-after-terminal",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{Type: projectionEventDone, TurnID: registration.TurnID},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, late); err != nil {
				t.Fatalf("ProjectEvent(late %s) error = %v", late.Type, err)
			}
		}
		close(releaseFinal)

		waitForAcks(t, transport, 2)
		assertCallCountStable(t, transport, 2, 50*time.Millisecond)
		calls := transport.snapshotCalls()
		assertDeliveryOrder(t, calls, registration.DeliveryID, []DeliveryEventType{
			DeliveryEventTypeStart,
			DeliveryEventTypeFinal,
		}, []int64{1, 2})
	})

	t.Run("Should map failed results and keep the first answer chunk as start", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-failed",
			TurnID:        "turn-progress-failed",
			ExtensionName: "ext-slack",
			RoutingKey:    testRoutingKey("brg-progress-failed", "peer-progress-failed"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-failed",
				PeerID:           "peer-progress-failed",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		for _, event := range []DeliveryProjectionEvent{
			{
				Type:   "tool_call",
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-failed",
					ToolID:     "agh__terminal",
					Phase:      ToolProgressPhaseStarted,
				},
				ToolInput: json.RawMessage(`{"command":"false"}`),
			},
			{
				Type:   "tool_result",
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-failed",
					Phase:      ToolProgressPhaseFailed,
				},
			},
			{Type: "agent_message", TurnID: registration.TurnID, Text: "answer"},
			{Type: "done", TurnID: registration.TurnID},
		} {
			if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", event.Type, err)
			}
		}

		waitForCalls(t, transport, 4)
		calls := transport.snapshotCalls()
		assertDeliveryOrder(t, calls, registration.DeliveryID, []DeliveryEventType{
			DeliveryEventTypeProgress,
			DeliveryEventTypeProgress,
			DeliveryEventTypeStart,
			DeliveryEventTypeFinal,
		}, []int64{1, 2, 3, 4})
		failed := calls[1].request.Event.Progress
		if failed == nil || failed.Phase != ToolProgressPhaseFailed {
			t.Fatalf("failed progress = %#v, want failed phase", failed)
		}
		if got, want := failed.ToolID, "agh__terminal"; got != want {
			t.Fatalf("failed progress tool id = %q, want recovered %q", got, want)
		}
		if got, want := calls[2].request.Event.Content.Text, "answer"; got != want {
			t.Fatalf("answer start content = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve one ordered terminal after text and progress", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-progress-order",
			TurnID:        "turn-progress-order",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-progress-order", "peer-progress-order"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-progress-order",
				PeerID:           "peer-progress-order",
				Mode:             DeliveryModeReply,
			},
			Progress: ProgressConfig{
				ToolProgress: ProgressModeAll,
				Grouping:     ProgressGroupingAccumulate,
			},
		})

		ctx := testutil.Context(t)
		events := []DeliveryProjectionEvent{
			{Type: "agent_message", TurnID: registration.TurnID, Text: "working"},
			{
				Type:   "tool_call",
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-order",
					ToolID:     "agh__task_list",
					Phase:      ToolProgressPhaseStarted,
				},
			},
			{
				Type:   "tool_result",
				TurnID: registration.TurnID,
				Tool: &ToolProgress{
					ToolCallID: "call-order",
					Phase:      ToolProgressPhaseCompleted,
				},
			},
			{Type: "done", TurnID: registration.TurnID},
		}
		for _, event := range events {
			if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
				t.Fatalf("ProjectEvent(%s) error = %v", event.Type, err)
			}
		}

		waitForCalls(t, transport, 4)
		calls := transport.snapshotCalls()
		assertDeliveryOrder(t, calls, registration.DeliveryID, []DeliveryEventType{
			DeliveryEventTypeStart,
			DeliveryEventTypeProgress,
			DeliveryEventTypeProgress,
			DeliveryEventTypeFinal,
		}, []int64{1, 2, 3, 4})
		terminalCount := 0
		for _, call := range calls {
			if isTerminalDeliveryEventType(call.request.Event.EventType) {
				terminalCount++
			}
		}
		if got, want := terminalCount, 1; got != want {
			t.Fatalf("terminal event count = %d, want %d", got, want)
		}
	})
}

func TestBrokerRejectedProjectedEventDoesNotAdvanceStateOrConsumeFingerprint(t *testing.T) {
	t.Parallel()

	var blockedDeliveryID string
	releaseStart := make(chan struct{})
	transport := &fakeDeliveryTransport{
		handler: func(ctx context.Context, _ string, req DeliveryRequest) (DeliveryAck, error) {
			if req.Event.DeliveryID == blockedDeliveryID && req.Event.EventType == DeliveryEventTypeStart {
				select {
				case <-releaseStart:
				case <-ctx.Done():
					return DeliveryAck{}, ctx.Err()
				}
			}
			return DeliveryAck{DeliveryID: req.Event.DeliveryID, Seq: req.Event.Seq}, nil
		},
	}
	broker := NewBroker(transport, WithDeliveryBrokerQueueCapacity(2))
	t.Cleanup(broker.Close)

	routingKey := testRoutingKey("brg-project-saturated", "peer-project-saturated")
	target := DeliveryTarget{
		BridgeInstanceID: "brg-project-saturated",
		PeerID:           "peer-project-saturated",
		Mode:             DeliveryModeReply,
	}
	regA := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
		SessionID:      "sess-project-saturated-a",
		TurnID:         "turn-project-saturated-a",
		ExtensionName:  "ext-telegram",
		RoutingKey:     routingKey,
		DeliveryTarget: target,
	})
	blockedDeliveryID = regA.DeliveryID
	regB := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
		SessionID:      "sess-project-saturated-b",
		TurnID:         "turn-project-saturated-b",
		ExtensionName:  "ext-telegram",
		RoutingKey:     routingKey,
		DeliveryTarget: target,
	})

	ctx := testutil.Context(t)
	if err := broker.Deliver(
		ctx,
		testDeliveryEvent(
			regA.DeliveryID,
			regA.BridgeInstanceID,
			regA.RoutingKey,
			regA.DeliveryTarget,
			1,
			DeliveryEventTypeStart,
			"hello",
			false,
		),
	); err != nil {
		t.Fatalf("Deliver(regA start) error = %v", err)
	}
	waitForCalls(t, transport, 1)
	if err := broker.Deliver(
		ctx,
		testDeliveryEvent(
			regB.DeliveryID,
			regB.BridgeInstanceID,
			regB.RoutingKey,
			regB.DeliveryTarget,
			1,
			DeliveryEventTypeStart,
			"other",
			false,
		),
	); err != nil {
		t.Fatalf("Deliver(regB start) error = %v", err)
	}
	if err := broker.Deliver(
		ctx,
		testDeliveryEvent(
			regB.DeliveryID,
			regB.BridgeInstanceID,
			regB.RoutingKey,
			regB.DeliveryTarget,
			2,
			DeliveryEventTypeFinal,
			"other done",
			true,
		),
	); err != nil {
		t.Fatalf("Deliver(regB final) error = %v", err)
	}

	projected := DeliveryProjectionEvent{
		Type:        "agent_message",
		TurnID:      regA.TurnID,
		Timestamp:   time.Date(2026, time.April, 11, 12, 5, 0, 0, time.UTC),
		Text:        " world",
		Fingerprint: "fp-project-saturated",
	}
	if err := broker.ProjectEvent(ctx, regA.SessionID, projected); !errors.Is(err, ErrDeliveryQueueSaturated) {
		t.Fatalf("ProjectEvent() error = %v, want %v", err, ErrDeliveryQueueSaturated)
	}

	snapshot, err := broker.Snapshot(ctx, regA.DeliveryID)
	if err != nil {
		t.Fatalf("Snapshot(after rejected project) error = %v", err)
	}
	if got, want := snapshot.LatestSeq, int64(1); got != want {
		t.Fatalf("LatestSeq after rejected project = %d, want %d", got, want)
	}
	if got, want := snapshot.CurrentContent.Text, "hello"; got != want {
		t.Fatalf("CurrentContent after rejected project = %q, want %q", got, want)
	}
	if snapshot.Final {
		t.Fatal("Final after rejected project = true, want false")
	}

	close(releaseStart)
	waitForCalls(t, transport, 3)

	if err := broker.ProjectEvent(ctx, regA.SessionID, projected); err != nil {
		t.Fatalf("ProjectEvent(retry) error = %v", err)
	}
	waitForCalls(t, transport, 4)

	snapshot, err = broker.Snapshot(ctx, regA.DeliveryID)
	if err != nil {
		t.Fatalf("Snapshot(after retry) error = %v", err)
	}
	if got, want := snapshot.LatestSeq, int64(2); got != want {
		t.Fatalf("LatestSeq after retry = %d, want %d", got, want)
	}
	if got, want := snapshot.CurrentContent.Text, "hello world"; got != want {
		t.Fatalf("CurrentContent after retry = %q, want %q", got, want)
	}
}

func TestBrokerFailSessionTerminalSurvivesSaturatedQueue(t *testing.T) {
	t.Run("Should preserve the terminal failure when the route queue is saturated", func(t *testing.T) {
		t.Parallel()

		var blockedDeliveryID string
		releaseStart := make(chan struct{})
		transport := &fakeDeliveryTransport{
			handler: func(ctx context.Context, _ string, req DeliveryRequest) (DeliveryAck, error) {
				if req.Event.DeliveryID == blockedDeliveryID && req.Event.EventType == DeliveryEventTypeStart {
					select {
					case <-releaseStart:
					case <-ctx.Done():
						return DeliveryAck{}, ctx.Err()
					}
				}
				return DeliveryAck{DeliveryID: req.Event.DeliveryID, Seq: req.Event.Seq}, nil
			},
		}
		broker := NewBroker(transport, WithDeliveryBrokerQueueCapacity(2))
		t.Cleanup(broker.Close)

		routingKey := testRoutingKey("brg-fail-saturated", "peer-fail-saturated")
		target := DeliveryTarget{
			BridgeInstanceID: "brg-fail-saturated",
			PeerID:           "peer-fail-saturated",
			Mode:             DeliveryModeReply,
		}
		regA := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:      "sess-fail-saturated-a",
			TurnID:         "turn-fail-saturated-a",
			ExtensionName:  "ext-telegram",
			RoutingKey:     routingKey,
			DeliveryTarget: target,
		})
		blockedDeliveryID = regA.DeliveryID
		regB := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:      "sess-fail-saturated-b",
			TurnID:         "turn-fail-saturated-b",
			ExtensionName:  "ext-telegram",
			RoutingKey:     routingKey,
			DeliveryTarget: target,
		})

		ctx := testutil.Context(t)
		if err := broker.Deliver(
			ctx,
			testDeliveryEvent(
				regA.DeliveryID,
				regA.BridgeInstanceID,
				regA.RoutingKey,
				regA.DeliveryTarget,
				1,
				DeliveryEventTypeStart,
				"hello",
				false,
			),
		); err != nil {
			t.Fatalf("Deliver(regA start) error = %v", err)
		}
		waitForCalls(t, transport, 1)
		if err := broker.Deliver(
			ctx,
			testDeliveryEvent(
				regB.DeliveryID,
				regB.BridgeInstanceID,
				regB.RoutingKey,
				regB.DeliveryTarget,
				1,
				DeliveryEventTypeStart,
				"other",
				false,
			),
		); err != nil {
			t.Fatalf("Deliver(regB start) error = %v", err)
		}
		if err := broker.Deliver(
			ctx,
			testDeliveryEvent(
				regB.DeliveryID,
				regB.BridgeInstanceID,
				regB.RoutingKey,
				regB.DeliveryTarget,
				2,
				DeliveryEventTypeFinal,
				"other done",
				true,
			),
		); err != nil {
			t.Fatalf("Deliver(regB final) error = %v", err)
		}

		if err := broker.FailSession(ctx, regA.SessionID, "adapter stopped"); err != nil {
			t.Fatalf("FailSession() error = %v, want preserved terminal", err)
		}

		snapshot, err := broker.Snapshot(ctx, regA.DeliveryID)
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if got, want := snapshot.LatestSeq, int64(2); got != want {
			t.Fatalf("LatestSeq after fail-session terminal = %d, want %d", got, want)
		}
		if got, want := snapshot.LatestEventType, DeliveryEventTypeError; got != want {
			t.Fatalf("LatestEventType after fail-session terminal = %q, want %q", got, want)
		}
		if !snapshot.Final {
			t.Fatal("Final after fail-session terminal = false, want true")
		}
		if got, want := snapshot.Error, "adapter stopped"; got != want {
			t.Fatalf("Error after fail-session terminal = %q, want %q", got, want)
		}

		close(releaseStart)
		waitForCalls(t, transport, 4)
		errorCount := 0
		for _, call := range transport.snapshotCalls() {
			if call.request.Event.DeliveryID == regA.DeliveryID &&
				call.request.Event.EventType == DeliveryEventTypeError {
				errorCount++
			}
		}
		if got, want := errorCount, 1; got != want {
			t.Fatalf("regA error terminal call count = %d, want %d", got, want)
		}
	})
}

func TestBrokerReconcileDeliveryFailsOpenThroughTerminalPost(t *testing.T) {
	t.Parallel()

	t.Run("Should post a universal terminal error and persist the acknowledgement", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 12, 17, 0, 0, 0, time.UTC)
		store := newRecordingDeliveryLedgerStore()
		record := DeliveryLedgerRecord{
			DeliveryID:       "delivery-restart",
			SessionID:        "sess-restart",
			TurnID:           "turn-restart",
			RoutingKey:       testRoutingKey("brg-restart", "peer-restart"),
			BridgeInstanceID: "brg-restart",
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-1",
			State:            DeliveryLedgerStateActive,
			LastSentSeq:      6,
			LastAckedSeq:     6,
			RemoteMessageID:  "remote-restart",
			CreatedAt:        now.Add(-time.Minute),
			UpdatedAt:        now.Add(-time.Minute),
		}
		if err := store.CreateBridgeDelivery(testutil.Context(t), record); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}

		transport := &fakeDeliveryTransport{
			handler: func(_ context.Context, extensionName string, req DeliveryRequest) (DeliveryAck, error) {
				if got, want := extensionName, "ext-slack"; got != want {
					t.Fatalf("reconcile extension = %q, want %q", got, want)
				}
				if got, want := req.Event.EventType, DeliveryEventTypeError; got != want {
					t.Fatalf("reconcile event type = %q, want %q", got, want)
				}
				if req.Snapshot != nil {
					t.Fatalf("reconcile snapshot = %#v, want fail-open terminal post", req.Snapshot)
				}
				if got := req.Event.Content.Text; !strings.Contains(got, sessionStoppedDeliveryMessage) {
					t.Fatalf("reconcile content = %q, want visible stopped-session error", got)
				}
				if req.Event.Operation != DeliveryOperationPost || req.Event.Reference != nil {
					t.Fatalf(
						"reconcile operation/reference = %q/%#v, want universal post",
						req.Event.Operation,
						req.Event.Reference,
					)
				}
				return DeliveryAck{
					DeliveryID:      req.Event.DeliveryID,
					Seq:             req.Event.Seq,
					RemoteMessageID: record.RemoteMessageID,
				}, nil
			},
		}
		restarted := NewBroker(
			transport,
			WithDeliveryLedgerStore(store),
			WithDeliveryBrokerRegistrationGate(),
			WithDeliveryBrokerNow(func() time.Time { return now }),
		)
		t.Cleanup(restarted.Close)

		if err := restarted.ReconcileDelivery(testutil.Context(t), record, "ext-slack"); err != nil {
			t.Fatalf("ReconcileDelivery() error = %v", err)
		}
		calls := transport.snapshotCalls()
		if got, want := len(calls), 1; got != want {
			t.Fatalf("reconcile transport calls = %d, want %d", got, want)
		}
		persisted, ok := store.record(record.DeliveryID)
		if !ok {
			t.Fatal("reconciled delivery record missing")
		}
		if got, want := persisted.State, DeliveryLedgerStateTerminalError; got != want {
			t.Fatalf("reconciled state = %q, want %q", got, want)
		}
		if got, want := persisted.LastAckedSeq, int64(7); got != want {
			t.Fatalf("reconciled last acked seq = %d, want %d", got, want)
		}
		if got := persisted.TerminalError; !strings.Contains(got, sessionStoppedDeliveryMessage) {
			t.Fatalf("reconciled terminal error = %q, want stopped-session error", got)
		}
		metrics := restarted.DeliveryMetrics()[record.BridgeInstanceID]
		if got, want := metrics.DeliveryFailuresTotal, 1; got != want {
			t.Fatalf("reconciled failure metrics = %d, want %d", got, want)
		}
		if !metrics.LastSuccessAt.IsZero() {
			t.Fatalf("reconciled LastSuccessAt = %s, want unchanged zero value", metrics.LastSuccessAt)
		}
	})

	t.Run("Should preserve an indeterminate semantic acknowledgement during reconciliation", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 20, 0, 0, 0, time.UTC)
		lastSuccessAt := now.Add(-3 * time.Minute)
		store := newRecordingDeliveryLedgerStore()
		record := DeliveryLedgerRecord{
			DeliveryID:       "delivery-reconcile-committed-result",
			SessionID:        "sess-reconcile-committed-result",
			TurnID:           "turn-reconcile-committed-result",
			RoutingKey:       testRoutingKey("brg-reconcile-committed-result", "peer-reconcile-committed-result"),
			BridgeInstanceID: "brg-reconcile-committed-result",
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-1",
			State:            DeliveryLedgerStateActive,
			LastSentSeq:      4,
			LastAckedSeq:     4,
			RemoteMessageID:  "remote-before-reconcile",
			CreatedAt:        now.Add(-time.Minute),
			UpdatedAt:        now.Add(-time.Minute),
		}
		ctx := testutil.Context(t)
		if err := store.CreateBridgeDelivery(ctx, record); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}
		if err := store.PutBridgeDeliveryMetrics(ctx, DeliveryMetricRecord{
			BridgeDeliveryMetrics: BridgeDeliveryMetrics{
				BridgeInstanceID: record.BridgeInstanceID,
				LastSuccessAt:    lastSuccessAt,
			},
			Scope:       record.Scope,
			WorkspaceID: record.WorkspaceID,
			UpdatedAt:   lastSuccessAt,
		}); err != nil {
			t.Fatalf("PutBridgeDeliveryMetrics() error = %v", err)
		}

		const secret = "sk-reconcile-secret"
		transport := &fakeDeliveryTransport{
			handler: func(_ context.Context, _ string, req DeliveryRequest) (DeliveryAck, error) {
				return DeliveryAck{
					DeliveryID: req.Event.DeliveryID,
					Seq:        req.Event.Seq,
					Outcome:    DeliveryAckOutcomeCommittedResultUnavailable,
					Error: &DeliveryErrorDetail{
						Message: "provider accepted api_key=" + secret + " but omitted its result",
					},
				}, nil
			},
		}
		restarted := NewBroker(
			transport,
			WithDeliveryLedgerStore(store),
			WithDeliveryBrokerRegistrationGate(),
			WithDeliveryBrokerNow(func() time.Time { return now }),
		)
		t.Cleanup(restarted.Close)
		if err := restarted.LoadDeliveryMetrics(ctx, DeliveryLedgerQuery{
			Scope:       record.Scope,
			WorkspaceID: record.WorkspaceID,
		}); err != nil {
			t.Fatalf("LoadDeliveryMetrics() error = %v", err)
		}

		if err := restarted.ReconcileDelivery(ctx, record, "ext-slack"); err != nil {
			t.Fatalf("ReconcileDelivery() error = %v", err)
		}
		if got, want := len(transport.snapshotCalls()), 1; got != want {
			t.Fatalf("reconcile transport calls = %d, want %d", got, want)
		}
		persisted, ok := store.record(record.DeliveryID)
		if !ok {
			t.Fatal("reconciled delivery record missing")
		}
		if got, want := persisted.State, DeliveryLedgerStateTerminalError; got != want {
			t.Fatalf("reconciled state = %q, want %q", got, want)
		}
		if got, want := persisted.LastSentSeq, int64(5); got != want {
			t.Fatalf("reconciled last sent seq = %d, want %d", got, want)
		}
		if got, want := persisted.LastAckedSeq, record.LastAckedSeq; got != want {
			t.Fatalf("reconciled last acked seq = %d, want %d", got, want)
		}
		if got, want := persisted.RemoteMessageID, record.RemoteMessageID; got != want {
			t.Fatalf("reconciled remote id = %q, want %q", got, want)
		}
		if strings.Contains(persisted.TerminalError, secret) ||
			!strings.Contains(persisted.TerminalError, "[REDACTED]") {
			t.Fatalf("reconciled terminal error = %q, want redacted provider detail", persisted.TerminalError)
		}
		metrics := restarted.DeliveryMetrics()[record.BridgeInstanceID]
		if got, want := metrics.DeliveryFailuresTotal, 1; got != want {
			t.Fatalf("reconciled failure metrics = %d, want %d", got, want)
		}
		if got, want := metrics.LastSuccessAt, lastSuccessAt; !got.Equal(want) {
			t.Fatalf("reconciled LastSuccessAt = %s, want %s", got, want)
		}
		if strings.Contains(metrics.LastError, secret) || !strings.Contains(metrics.LastError, "[REDACTED]") {
			t.Fatalf("reconciled LastError = %q, want redacted provider detail", metrics.LastError)
		}
	})

	t.Run("Should write ahead then terminalize an interrupted reconcile locally on next restart", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 13, 19, 0, 0, 0, time.UTC)
		lastSuccessAt := now.Add(-2 * time.Minute)
		store := newRecordingDeliveryLedgerStore()
		record := DeliveryLedgerRecord{
			DeliveryID:       "delivery-indeterminate-restart",
			SessionID:        "sess-indeterminate-restart",
			TurnID:           "turn-indeterminate-restart",
			RoutingKey:       testRoutingKey("brg-indeterminate-restart", "peer-indeterminate-restart"),
			BridgeInstanceID: "brg-indeterminate-restart",
			Scope:            ScopeWorkspace,
			WorkspaceID:      "ws-1",
			State:            DeliveryLedgerStateActive,
			LastSentSeq:      6,
			LastAckedSeq:     6,
			CreatedAt:        now.Add(-time.Minute),
			UpdatedAt:        now.Add(-time.Minute),
		}
		ctx := testutil.Context(t)
		if err := store.CreateBridgeDelivery(ctx, record); err != nil {
			t.Fatalf("CreateBridgeDelivery() error = %v", err)
		}
		if err := store.PutBridgeDeliveryMetrics(ctx, DeliveryMetricRecord{
			BridgeDeliveryMetrics: BridgeDeliveryMetrics{
				BridgeInstanceID: record.BridgeInstanceID,
				LastSuccessAt:    lastSuccessAt,
			},
			Scope:       record.Scope,
			WorkspaceID: record.WorkspaceID,
			UpdatedAt:   lastSuccessAt,
		}); err != nil {
			t.Fatalf("PutBridgeDeliveryMetrics() error = %v", err)
		}

		reconcileCtx, cancelReconcile := context.WithCancel(ctx)
		firstTransport := &fakeDeliveryTransport{
			handler: func(_ context.Context, _ string, req DeliveryRequest) (DeliveryAck, error) {
				intent, ok := store.record(record.DeliveryID)
				if !ok || intent.State != DeliveryLedgerStateActive ||
					intent.LastSentSeq != req.Event.Seq || intent.LastAckedSeq != record.LastAckedSeq {
					t.Fatalf(
						"reconcile durable intent = %#v, want active sent=%d acked=%d",
						intent,
						req.Event.Seq,
						record.LastAckedSeq,
					)
				}
				cancelReconcile()
				return DeliveryAck{
					DeliveryID:      req.Event.DeliveryID,
					Seq:             req.Event.Seq,
					RemoteMessageID: "remote-reconcile-committed",
				}, nil
			},
		}
		firstBroker := NewBroker(
			firstTransport,
			WithDeliveryLedgerStore(store),
			WithDeliveryBrokerRegistrationGate(),
			WithDeliveryBrokerNow(func() time.Time { return now }),
		)
		t.Cleanup(firstBroker.Close)
		if err := firstBroker.LoadDeliveryMetrics(ctx, DeliveryLedgerQuery{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-1",
		}); err != nil {
			t.Fatalf("LoadDeliveryMetrics() error = %v", err)
		}
		if err := firstBroker.ReconcileDelivery(reconcileCtx, record, "ext-slack"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ReconcileDelivery(interrupted checkpoint) error = %v, want context cancellation", err)
		}
		if got, want := len(firstTransport.snapshotCalls()), 1; got != want {
			t.Fatalf("first reconcile transport calls = %d, want %d", got, want)
		}
		inflight, ok := store.record(record.DeliveryID)
		if !ok || inflight.State != DeliveryLedgerStateActive ||
			inflight.LastSentSeq != 7 || inflight.LastAckedSeq != 6 {
			t.Fatalf("interrupted durable delivery = %#v, want active sent=7 acked=6", inflight)
		}

		secondTransport := &fakeDeliveryTransport{
			handler: func(_ context.Context, _ string, _ DeliveryRequest) (DeliveryAck, error) {
				t.Fatal("second ReconcileDelivery() called provider for an inflight durable mutation")
				return DeliveryAck{}, nil
			},
		}
		restarted := NewBroker(
			secondTransport,
			WithDeliveryLedgerStore(store),
			WithDeliveryBrokerRegistrationGate(),
			WithDeliveryBrokerNow(func() time.Time { return now }),
		)
		t.Cleanup(restarted.Close)
		if err := restarted.LoadDeliveryMetrics(ctx, DeliveryLedgerQuery{
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-1",
		}); err != nil {
			t.Fatalf("LoadDeliveryMetrics(restarted) error = %v", err)
		}

		if err := restarted.ReconcileDelivery(ctx, inflight, "ext-slack"); err != nil {
			t.Fatalf("ReconcileDelivery() error = %v", err)
		}
		if got := len(secondTransport.snapshotCalls()); got != 0 {
			t.Fatalf("reconcile transport calls = %d, want zero", got)
		}
		persisted, ok := store.record(record.DeliveryID)
		if !ok {
			t.Fatal("terminalized delivery record missing")
		}
		if got, want := persisted.State, DeliveryLedgerStateTerminalError; got != want {
			t.Fatalf("reconciled state = %q, want %q", got, want)
		}
		if got, want := persisted.LastSentSeq, int64(7); got != want {
			t.Fatalf("reconciled last sent seq = %d, want %d", got, want)
		}
		if got, want := persisted.LastAckedSeq, record.LastAckedSeq; got != want {
			t.Fatalf("reconciled last acked seq = %d, want %d", got, want)
		}
		if !strings.Contains(persisted.TerminalError, "indeterminate") {
			t.Fatalf("reconciled terminal error = %q, want indeterminate outcome", persisted.TerminalError)
		}
		metrics := restarted.DeliveryMetrics()[record.BridgeInstanceID]
		if got, want := metrics.DeliveryFailuresTotal, 1; got != want {
			t.Fatalf("reconciled failure metrics = %d, want %d", got, want)
		}
		if got, want := metrics.LastSuccessAt, lastSuccessAt; !got.Equal(want) {
			t.Fatalf("reconciled LastSuccessAt = %s, want %s", got, want)
		}
	})
}

func TestDeliveryValidationAndMetadataHelpers(t *testing.T) {
	t.Parallel()

	routingKey := testRoutingKey("brg-validate", "peer-validate")
	target := DeliveryTarget{
		BridgeInstanceID: "brg-validate",
		PeerID:           "peer-validate",
		Mode:             DeliveryModeReply,
	}

	t.Run("Should normalize projection event fields", func(t *testing.T) {
		normalized := (DeliveryProjectionEvent{
			Type:        " agent_message ",
			TurnID:      " turn-validate ",
			Error:       " boom ",
			Fingerprint: " fp-1 ",
		}).normalize()
		if got, want := normalized.Type, "agent_message"; got != want {
			t.Fatalf("normalized.Type = %q, want %q", got, want)
		}
		if got, want := normalized.TurnID, "turn-validate"; got != want {
			t.Fatalf("normalized.TurnID = %q, want %q", got, want)
		}
		if got, want := normalized.Error, "boom"; got != want {
			t.Fatalf("normalized.Error = %q, want %q", got, want)
		}
		if got, want := normalized.Fingerprint, "fp-1"; got != want {
			t.Fatalf("normalized.Fingerprint = %q, want %q", got, want)
		}
	})

	t.Run("Should clone typed delivery payloads", func(t *testing.T) {
		event := DeliveryEvent{
			DeliveryID:       "del-clone",
			BridgeInstanceID: "brg-clone",
			RoutingKey:       routingKey,
			DeliveryTarget:   target,
			Seq:              2,
			EventType:        DeliveryEventTypeError,
			Content:          MessageContent{Text: "hello"},
			Final:            true,
			Operation:        DeliveryOperationEdit,
			Reference:        &DeliveryMessageReference{RemoteMessageID: "remote-1"},
			Error:            &DeliveryErrorDetail{Message: "broken"},
			ProviderMetadata: json.RawMessage(`{"provider":"slack"}`),
		}

		cloned := cloneDeliveryEvent(event)
		if cloned.Reference == nil || cloned.Error == nil {
			t.Fatalf("cloneDeliveryEvent() = %#v, want cloned typed payloads", cloned)
		}
		if got, want := string(cloned.ProviderMetadata), `{"provider":"slack"}`; got != want {
			t.Fatalf("cloned.ProviderMetadata = %s, want %s", got, want)
		}
		event.Reference.RemoteMessageID = "changed"
		event.Error.Message = "changed"
		event.ProviderMetadata[0] = '['
		if got, want := cloned.Reference.RemoteMessageID, "remote-1"; got != want {
			t.Fatalf("cloned.Reference.RemoteMessageID = %q, want %q", got, want)
		}
		if got, want := cloned.Error.Message, "broken"; got != want {
			t.Fatalf("cloned.Error.Message = %q, want %q", got, want)
		}
		if got, want := string(cloned.ProviderMetadata), `{"provider":"slack"}`; got != want {
			t.Fatalf("cloned.ProviderMetadata mutated to %s, want %s", got, want)
		}
	})
}

func TestBrokerProjectEventLockedCoversTerminalAndIgnoredPaths(t *testing.T) {
	t.Parallel()

	broker := NewBroker(nil, WithDeliveryBrokerNow(func() time.Time {
		return time.Date(2026, time.April, 11, 12, 4, 0, 0, time.UTC)
	}))
	t.Cleanup(broker.Close)

	delivery := &activeDelivery{
		deliveryID:       "del-locked",
		bridgeInstanceID: "brg-locked",
		routingKey:       testRoutingKey("brg-locked", "peer-locked"),
		target: DeliveryTarget{
			BridgeInstanceID: "brg-locked",
			PeerID:           "peer-locked",
			Mode:             DeliveryModeReply,
		},
	}

	if _, ok, err := broker.projectEventLocked(
		nil,
		DeliveryProjectionEvent{Type: "agent_message"},
	); !errors.Is(err, ErrDeliveryNotFound) ||
		ok {
		t.Fatalf("projectEventLocked(nil) = (%v, %v), want ErrDeliveryNotFound and ok=false", err, ok)
	}

	if _, ok, err := broker.projectEventLocked(delivery, DeliveryProjectionEvent{Type: "done"}); err != nil || ok {
		t.Fatalf("projectEventLocked(done without content) = (%v, %v), want nil and ok=false", err, ok)
	}

	start, ok, err := broker.projectEventLocked(delivery, DeliveryProjectionEvent{Type: "agent_message", Text: "hello"})
	if err != nil || !ok {
		t.Fatalf("projectEventLocked(agent_message) = (%v, %v), want nil and ok=true", err, ok)
	}
	if got, want := start.EventType, DeliveryEventTypeStart; got != want {
		t.Fatalf("start event type = %q, want %q", got, want)
	}
	broker.applyQueuedEventLocked(delivery, start)

	final, ok, err := broker.projectEventLocked(delivery, DeliveryProjectionEvent{Type: "done"})
	if err != nil || !ok {
		t.Fatalf("projectEventLocked(done) = (%v, %v), want nil and ok=true", err, ok)
	}
	if got, want := final.EventType, DeliveryEventTypeFinal; got != want {
		t.Fatalf("final event type = %q, want %q", got, want)
	}

	errorDelivery := &activeDelivery{
		deliveryID:       "del-error",
		bridgeInstanceID: "brg-locked",
		routingKey:       testRoutingKey("brg-locked", "peer-locked"),
		target: DeliveryTarget{
			BridgeInstanceID: "brg-locked",
			PeerID:           "peer-locked",
			Mode:             DeliveryModeReply,
		},
		currentContent: MessageContent{Text: "partial"},
	}
	errorEvent, ok, err := broker.projectEventLocked(
		errorDelivery,
		DeliveryProjectionEvent{Type: "error", Error: "boom"},
	)
	if err != nil || !ok {
		t.Fatalf("projectEventLocked(error) = (%v, %v), want nil and ok=true", err, ok)
	}
	if got, want := errorEvent.EventType, DeliveryEventTypeError; got != want {
		t.Fatalf("error event type = %q, want %q", got, want)
	}
	if errorEvent.Error == nil {
		t.Fatal("error event Error = nil, want typed error payload")
	}
	if got, want := errorEvent.Error.Message, "boom"; got != want {
		t.Fatalf("errorEvent.Error.Message = %q, want %q", got, want)
	}

	if _, ok, err := broker.projectEventLocked(delivery, DeliveryProjectionEvent{Type: "unknown"}); err != nil || ok {
		t.Fatalf("projectEventLocked(unknown) = (%v, %v), want nil and ok=false", err, ok)
	}
}

func TestBrokerEnqueueEventLockedCoversReplacementAndSaturation(t *testing.T) {
	t.Parallel()

	broker := NewBroker(nil, WithDeliveryBrokerQueueCapacity(2))
	t.Cleanup(broker.Close)

	route := &routeWorker{bridgeInstanceID: "brg-queue"}
	delivery := &activeDelivery{deliveryID: "del-queue"}

	start := testDeliveryEvent(
		delivery.deliveryID,
		"brg-queue",
		testRoutingKey("brg-queue", "peer-queue"),
		DeliveryTarget{
			BridgeInstanceID: "brg-queue",
			PeerID:           "peer-queue",
			Mode:             DeliveryModeReply,
		},
		1,
		DeliveryEventTypeStart,
		"hello",
		false,
	)
	if err := broker.enqueueEventLocked(route, delivery, start); err != nil {
		t.Fatalf("enqueueEventLocked(start) error = %v", err)
	}

	startUpdate := start
	startUpdate.Seq = 2
	startUpdate.Content.Text = "hello again"
	if err := broker.enqueueEventLocked(route, delivery, startUpdate); err != nil {
		t.Fatalf("enqueueEventLocked(start update) error = %v", err)
	}
	if got, want := delivery.pendingStart.Seq, int64(2); got != want {
		t.Fatalf("pendingStart.Seq = %d, want %d", got, want)
	}

	route.queue = append(route.queue, deliveryQueueItem{deliveryID: "other", kind: deliveryQueueKindStart})
	deltaPromoted := start
	deltaPromoted.Seq = 3
	deltaPromoted.EventType = DeliveryEventTypeDelta
	deltaPromoted.Content.Text = "hello promoted"
	if err := broker.enqueueEventLocked(route, delivery, deltaPromoted); err != nil {
		t.Fatalf("enqueueEventLocked(delta promoted into start) error = %v", err)
	}
	if got, want := delivery.pendingStart.EventType, DeliveryEventTypeStart; got != want {
		t.Fatalf("pendingStart.EventType = %q, want %q", got, want)
	}
	if got, want := delivery.pendingStart.Content.Text, "hello promoted"; got != want {
		t.Fatalf("pendingStart.Content.Text = %q, want %q", got, want)
	}

	route.queue = route.queue[:1]
	delivery.startDelivered = true
	delta := deltaPromoted
	if err := broker.enqueueEventLocked(route, delivery, delta); err != nil {
		t.Fatalf("enqueueEventLocked(delta) error = %v", err)
	}
	deltaUpdate := delta
	deltaUpdate.Seq = 4
	deltaUpdate.Content.Text = "hello final delta"
	if err := broker.enqueueEventLocked(route, delivery, deltaUpdate); err != nil {
		t.Fatalf("enqueueEventLocked(delta update) error = %v", err)
	}
	if got, want := delivery.pendingDelta.Seq, int64(4); got != want {
		t.Fatalf("pendingDelta.Seq = %d, want %d", got, want)
	}

	final := deltaUpdate
	final.Seq = 5
	final.EventType = DeliveryEventTypeFinal
	final.Final = true
	final.Content.Text = "done"
	if err := broker.enqueueEventLocked(route, delivery, final); err != nil {
		t.Fatalf("enqueueEventLocked(final) error = %v", err)
	}
	if delivery.pendingDelta != nil {
		t.Fatal("pendingDelta = non-nil after final enqueue, want cleared delta")
	}
	if delivery.pendingTerminal == nil {
		t.Fatal("pendingTerminal = nil, want queued terminal event")
	}

	fullRoute := &routeWorker{
		bridgeInstanceID: "brg-queue",
		queue: []deliveryQueueItem{
			{deliveryID: "del-a", kind: deliveryQueueKindStart},
			{deliveryID: "del-b", kind: deliveryQueueKindStart},
		},
	}
	if err := broker.enqueueEventLocked(
		fullRoute,
		&activeDelivery{deliveryID: "del-c"},
		start,
	); !errors.Is(
		err,
		ErrDeliveryQueueSaturated,
	) {
		t.Fatalf("enqueueEventLocked(full route) error = %v, want ErrDeliveryQueueSaturated", err)
	}

	metrics := broker.DeliveryMetrics()["brg-queue"]
	if got, want := metrics.DeliveryDroppedByReason["queue_saturated"], 1; got != want {
		t.Fatalf("DeliveryMetrics().DeliveryDroppedByReason[queue_saturated] = %d, want %d", got, want)
	}
}

type descriptorLookupFunc func(string, string) (toolmeta.DescriptorMetadata, bool)

func (f descriptorLookupFunc) LookupToolMetadata(
	workspaceID string,
	toolID string,
) (toolmeta.DescriptorMetadata, bool) {
	return f(workspaceID, toolID)
}

func assertCallCountStable(t *testing.T, transport *fakeDeliveryTransport, want int, duration time.Duration) {
	t.Helper()

	timer := time.NewTimer(duration)
	defer timer.Stop()
	updates := transport.updateCh()
	for {
		if got := len(transport.snapshotCalls()); got != want {
			t.Fatalf("delivery call count = %d, want stable count %d", got, want)
		}
		select {
		case <-updates:
		case <-timer.C:
			return
		}
	}
}

func waitForSnapshot(
	t *testing.T,
	broker *Broker,
	deliveryID string,
	match func(*DeliverySnapshot) bool,
) *DeliverySnapshot {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

	for {
		snapshot, err := broker.Snapshot(testutil.Context(t), deliveryID)
		if err == nil && match(snapshot) {
			return snapshot
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatalf("snapshot %q did not reach expected state before timeout", deliveryID)
		}
	}
}

func waitForDeliveryRemoval(t *testing.T, broker *Broker, deliveryID string) {
	t.Helper()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

	for {
		_, err := broker.Snapshot(testutil.Context(t), deliveryID)
		if errors.Is(err, ErrDeliveryNotFound) {
			return
		}
		if err != nil {
			t.Fatalf("Snapshot(%q) error = %v", deliveryID, err)
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatalf("delivery %q was not removed before timeout", deliveryID)
		}
	}
}
