package bridges

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestBrokerRegisterPromptDeliveryContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject duplicate caller supplied delivery IDs without overwriting original", func(t *testing.T) {
		t.Parallel()

		broker := NewBroker(nil)
		t.Cleanup(broker.Close)

		ctx := testutil.Context(t)
		first, err := broker.RegisterPromptDelivery(ctx, PromptDeliveryRegistration{
			SessionID:     "sess-original",
			TurnID:        "turn-original",
			DeliveryID:    "del-shared",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-original", "peer-original"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-original",
				PeerID:           "peer-original",
				Mode:             DeliveryModeReply,
			},
		})
		if err != nil {
			t.Fatalf("RegisterPromptDelivery(first) error = %v", err)
		}
		if first == nil {
			t.Fatal("RegisterPromptDelivery(first) snapshot = nil, want non-nil")
		}

		duplicate, err := broker.RegisterPromptDelivery(ctx, PromptDeliveryRegistration{
			SessionID:     "sess-colliding",
			TurnID:        "turn-colliding",
			DeliveryID:    "del-shared",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-colliding", "peer-colliding"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-colliding",
				PeerID:           "peer-colliding",
				Mode:             DeliveryModeReply,
			},
		})
		if !errors.Is(err, ErrDeliveryIDConflict) {
			t.Fatalf("RegisterPromptDelivery(duplicate) error = %v, want %v", err, ErrDeliveryIDConflict)
		}
		if duplicate != nil {
			t.Fatalf("RegisterPromptDelivery(duplicate) snapshot = %#v, want nil", duplicate)
		}

		snapshot, err := broker.Snapshot(ctx, "del-shared")
		if err != nil {
			t.Fatalf("Snapshot(original delivery) error = %v", err)
		}
		if snapshot.SessionID != "sess-original" || snapshot.TurnID != "turn-original" {
			t.Fatalf("Snapshot(original delivery) = %#v, want original session/turn", snapshot)
		}
	})

	t.Run("Should roll back registration when seed replay fails", func(t *testing.T) {
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

		routingKey := testRoutingKey("brg-seed-saturated", "peer-seed-saturated")
		target := DeliveryTarget{
			BridgeInstanceID: "brg-seed-saturated",
			PeerID:           "peer-seed-saturated",
			Mode:             DeliveryModeReply,
		}
		regA := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:      "sess-seed-saturated-a",
			TurnID:         "turn-seed-saturated-a",
			ExtensionName:  "ext-telegram",
			RoutingKey:     routingKey,
			DeliveryTarget: target,
		})
		blockedDeliveryID = regA.DeliveryID
		regB := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:      "sess-seed-saturated-b",
			TurnID:         "turn-seed-saturated-b",
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
				"alpha",
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
				"bravo",
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
				"bravo done",
				true,
			),
		); err != nil {
			t.Fatalf("Deliver(regB final) error = %v", err)
		}

		seedEvent := DeliveryProjectionEvent{
			Type:        "agent_message",
			TurnID:      "turn-seed-fail",
			Timestamp:   time.Date(2026, time.May, 17, 15, 45, 0, 0, time.UTC),
			Text:        "seed replayed",
			Fingerprint: "fp-seed-replay-failure",
		}
		seedRegistration := PromptDeliveryRegistration{
			SessionID:      "sess-seed-fail",
			TurnID:         "turn-seed-fail",
			DeliveryID:     "del-seed-fail",
			ExtensionName:  "ext-telegram",
			RoutingKey:     routingKey,
			DeliveryTarget: target,
			SeedEvents:     []DeliveryProjectionEvent{seedEvent},
		}
		failedSnapshot, err := broker.RegisterPromptDelivery(ctx, seedRegistration)
		if !errors.Is(err, ErrDeliveryQueueSaturated) {
			t.Fatalf("RegisterPromptDelivery(seed failure) error = %v, want %v", err, ErrDeliveryQueueSaturated)
		}
		if failedSnapshot != nil {
			t.Fatalf("RegisterPromptDelivery(seed failure) snapshot = %#v, want nil", failedSnapshot)
		}

		leakedSnapshot, err := broker.Snapshot(ctx, "del-seed-fail")
		if !errors.Is(err, ErrDeliveryNotFound) {
			t.Fatalf("Snapshot(failed seed delivery) error = %v, want %v", err, ErrDeliveryNotFound)
		}
		if leakedSnapshot != nil {
			t.Fatalf("Snapshot(failed seed delivery) = %#v, want nil", leakedSnapshot)
		}

		close(releaseStart)
		waitForCalls(t, transport, 3)

		retriedSnapshot, err := broker.RegisterPromptDelivery(ctx, seedRegistration)
		if err != nil {
			t.Fatalf("RegisterPromptDelivery(seed retry) error = %v", err)
		}
		if retriedSnapshot == nil {
			t.Fatal("RegisterPromptDelivery(seed retry) snapshot = nil, want non-nil")
		}
		if got, want := retriedSnapshot.CurrentContent.Text, "seed replayed"; got != want {
			t.Fatalf("retry snapshot CurrentContent.Text = %q, want %q", got, want)
		}
		if got, want := retriedSnapshot.LatestSeq, int64(1); got != want {
			t.Fatalf("retry snapshot LatestSeq = %d, want %d", got, want)
		}
		waitForCalls(t, transport, 4)
	})
}

func TestBrokerProjectEventContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid projected error events without transport or state changes", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)

		reg := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-project-invalid-error",
			TurnID:        "turn-project-invalid-error",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-project-invalid-error", "peer-project-invalid-error"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-project-invalid-error",
				PeerID:           "peer-project-invalid-error",
				Mode:             DeliveryModeReply,
			},
		})

		err := broker.ProjectEvent(testutil.Context(t), reg.SessionID, DeliveryProjectionEvent{
			Type:   "error",
			TurnID: reg.TurnID,
			Error:  " ",
		})
		if err == nil || !strings.Contains(err.Error(), "delivery error message") {
			t.Fatalf("ProjectEvent(invalid error) error = %v, want delivery error message validation", err)
		}
		assertCallCountStable(t, transport, 0, 50*time.Millisecond)

		snapshot, err := broker.Snapshot(testutil.Context(t), reg.DeliveryID)
		if err != nil {
			t.Fatalf("Snapshot(after invalid project) error = %v", err)
		}
		if snapshot.Final {
			t.Fatal("Snapshot(after invalid project).Final = true, want false")
		}
		if got := snapshot.LatestSeq; got != 0 {
			t.Fatalf("Snapshot(after invalid project).LatestSeq = %d, want 0", got)
		}
		if got := snapshot.LatestEventType; got != "" {
			t.Fatalf("Snapshot(after invalid project).LatestEventType = %q, want empty", got)
		}
	})
}

func TestBrokerRouteLifecycleContract(t *testing.T) {
	t.Parallel()

	t.Run("Should retire completed routes while the broker remains alive", func(t *testing.T) {
		t.Parallel()

		transport := &fakeDeliveryTransport{}
		broker := NewBroker(transport)
		t.Cleanup(broker.Close)

		ctx := testutil.Context(t)
		registrations := make([]DeliverySnapshot, 0, 3)
		for idx := range 3 {
			registrations = append(registrations, mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
				SessionID:     "sess-route-retire-" + string(rune('a'+idx)),
				TurnID:        "turn-route-retire-" + string(rune('a'+idx)),
				ExtensionName: "ext-telegram",
				RoutingKey:    testRoutingKey("brg-route-retire", "peer-route-retire-"+string(rune('a'+idx))),
				DeliveryTarget: DeliveryTarget{
					BridgeInstanceID: "brg-route-retire",
					PeerID:           "peer-route-retire-" + string(rune('a'+idx)),
					Mode:             DeliveryModeReply,
				},
			}))
		}
		for _, reg := range registrations {
			if err := broker.Deliver(ctx, testDeliveryEvent(
				reg.DeliveryID,
				reg.BridgeInstanceID,
				reg.RoutingKey,
				reg.DeliveryTarget,
				1,
				DeliveryEventTypeStart,
				"route start",
				false,
			)); err != nil {
				t.Fatalf("Deliver(start %s) error = %v", reg.DeliveryID, err)
			}
			if err := broker.Deliver(ctx, testDeliveryEvent(
				reg.DeliveryID,
				reg.BridgeInstanceID,
				reg.RoutingKey,
				reg.DeliveryTarget,
				2,
				DeliveryEventTypeFinal,
				"route final",
				true,
			)); err != nil {
				t.Fatalf("Deliver(final %s) error = %v", reg.DeliveryID, err)
			}
		}

		waitForAcks(t, transport, len(registrations)*2)
		waitForBrokerRouteCount(t, broker, 0)
	})
}

func waitForBrokerRouteCount(t *testing.T, broker *Broker, want int) {
	t.Helper()

	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		broker.mu.Lock()
		got := len(broker.routes)
		broker.mu.Unlock()
		if got == want {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("len(broker.routes) = %d, want %d", got, want)
		}
	}
}
