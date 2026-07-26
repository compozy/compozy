//go:build integration

package contract_test

import (
	"encoding/json"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestInboundTypedInteractionRoundTrip(t *testing.T) {
	t.Parallel()

	event := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: "brg-1",
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		ReceivedAt:       time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC),
		Sender:           bridgepkg.MessageSender{ID: "user-1", DisplayName: "Alice"},
		EventFamily:      bridgepkg.InboundEventFamilyAction,
		Action: &bridgepkg.InboundAction{
			ActionID:  "approve",
			MessageID: "msg-1",
			Value:     "run-1",
		},
		ProviderMetadata: json.RawMessage(`{"provider":"slack","raw_action_id":"A123"}`),
		IdempotencyKey:   "idem-1",
	}

	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded bridgepkg.InboundMessageEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if err := decoded.Validate(); err != nil {
		t.Fatalf("decoded.Validate() error = %v", err)
	}
	if got, want := decoded.EventFamily, bridgepkg.InboundEventFamilyAction; got != want {
		t.Fatalf("decoded.EventFamily = %q, want %q", got, want)
	}
	if decoded.Action == nil || decoded.Action.ActionID != "approve" || decoded.Action.MessageID != "msg-1" {
		t.Fatalf("decoded.Action = %#v", decoded.Action)
	}
}

func TestDeliveryEditAndDeleteRoundTrip(t *testing.T) {
	t.Parallel()

	target := bridgepkg.DeliveryTarget{
		BridgeInstanceID: "brg-1",
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
		Mode:             bridgepkg.DeliveryModeReply,
	}
	key := bridgepkg.RoutingKey{
		Scope:            bridgepkg.ScopeWorkspace,
		WorkspaceID:      "ws-1",
		BridgeInstanceID: "brg-1",
		PeerID:           "peer-1",
		ThreadID:         "thread-1",
	}

	tests := []bridgepkg.DeliveryRequest{
		{
			Event: bridgepkg.DeliveryEvent{
				DeliveryID:       "del-edit",
				BridgeInstanceID: "brg-1",
				RoutingKey:       key,
				DeliveryTarget:   target,
				Seq:              2,
				EventType:        bridgepkg.DeliveryEventTypeFinal,
				Content:          bridgepkg.MessageContent{Text: "updated"},
				Final:            true,
				Operation:        bridgepkg.DeliveryOperationEdit,
				Reference:        &bridgepkg.DeliveryMessageReference{RemoteMessageID: "remote-1"},
			},
		},
		{
			Event: bridgepkg.DeliveryEvent{
				DeliveryID:       "del-delete",
				BridgeInstanceID: "brg-1",
				RoutingKey:       key,
				DeliveryTarget:   target,
				Seq:              3,
				EventType:        bridgepkg.DeliveryEventTypeDelete,
				Final:            true,
				Operation:        bridgepkg.DeliveryOperationDelete,
				Reference:        &bridgepkg.DeliveryMessageReference{DeliveryID: "del-edit"},
			},
		},
	}

	for _, req := range tests {
		req := req
		t.Run(req.Event.DeliveryID, func(t *testing.T) {
			t.Parallel()

			if err := req.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}

			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			var decoded bridgepkg.DeliveryRequest
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if err := decoded.Validate(); err != nil {
				t.Fatalf("decoded.Validate() error = %v", err)
			}
			if got, want := decoded.Event.DeliveryTarget, target; got != want {
				t.Fatalf("decoded.Event.DeliveryTarget = %#v, want %#v", got, want)
			}
			if got, want := decoded.Event.Operation, req.Event.Operation; got != want {
				t.Fatalf("decoded.Event.Operation = %q, want %q", got, want)
			}
			if decoded.Event.Reference == nil {
				t.Fatal("decoded.Event.Reference = nil, want non-nil")
			}
		})
	}
}
