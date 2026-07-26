package bridges

import (
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestBrokerProjectEventRefacs(t *testing.T) {
	t.Parallel()

	t.Run("Should not deduplicate unfingerprinted zero-time message chunks", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)

		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-unfingerprinted",
			TurnID:        "turn-unfingerprinted",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-unfingerprinted", "peer-unfingerprinted"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-unfingerprinted",
				PeerID:           "peer-unfingerprinted",
				Mode:             DeliveryModeReply,
			},
		})

		event := DeliveryProjectionEvent{
			Type:   "agent_message",
			TurnID: registration.TurnID,
			Text:   "ha",
		}
		ctx := testutil.Context(t)
		if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
			t.Fatalf("ProjectEvent(first) error = %v", err)
		}
		if err := broker.ProjectEvent(ctx, registration.SessionID, event); err != nil {
			t.Fatalf("ProjectEvent(second) error = %v", err)
		}

		waitForCalls(t, transport, 2)
		calls := transport.snapshotCalls()
		assertDeliveryOrder(
			t,
			calls,
			registration.DeliveryID,
			[]DeliveryEventType{DeliveryEventTypeStart, DeliveryEventTypeDelta},
			[]int64{1, 2},
		)
		if got, want := calls[1].request.Event.Content.Text, "haha"; got != want {
			t.Fatalf("second projected content = %q, want %q", got, want)
		}
	})
}
