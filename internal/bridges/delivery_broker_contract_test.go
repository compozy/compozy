package bridges

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should keep whitespace-distinct delivery IDs independent", func(t *testing.T) {
		t.Parallel()

		const (
			plainDeliveryID  = "delivery"
			spacedDeliveryID = " delivery "
		)
		broker := NewBroker(nil)
		t.Cleanup(broker.Close)
		ctx := testutil.Context(t)
		registrations := []PromptDeliveryRegistration{
			{
				SessionID:     "sess-plain",
				TurnID:        "turn-plain",
				DeliveryID:    plainDeliveryID,
				ExtensionName: "ext-telegram",
				RoutingKey:    testRoutingKey("brg-plain", "peer-plain"),
				DeliveryTarget: DeliveryTarget{
					BridgeInstanceID: "brg-plain",
					PeerID:           "peer-plain",
					Mode:             DeliveryModeReply,
				},
			},
			{
				SessionID:     "sess-spaced",
				TurnID:        "turn-spaced",
				DeliveryID:    spacedDeliveryID,
				ExtensionName: "ext-telegram",
				RoutingKey:    testRoutingKey("brg-spaced", "peer-spaced"),
				DeliveryTarget: DeliveryTarget{
					BridgeInstanceID: "brg-spaced",
					PeerID:           "peer-spaced",
					Mode:             DeliveryModeReply,
				},
			},
		}
		for _, registration := range registrations {
			if _, err := broker.RegisterPromptDelivery(ctx, registration); err != nil {
				t.Fatalf("RegisterPromptDelivery(%q) error = %v", registration.DeliveryID, err)
			}
		}

		plain, err := broker.Snapshot(ctx, plainDeliveryID)
		if err != nil {
			t.Fatalf("Snapshot(plain) error = %v", err)
		}
		spaced, err := broker.Snapshot(ctx, spacedDeliveryID)
		if err != nil {
			t.Fatalf("Snapshot(spaced) error = %v", err)
		}
		if plain.DeliveryID != plainDeliveryID || plain.SessionID != "sess-plain" {
			t.Fatalf("Snapshot(plain) = %#v, want exact plain identity", plain)
		}
		if spaced.DeliveryID != spacedDeliveryID || spaced.SessionID != "sess-spaced" {
			t.Fatalf("Snapshot(spaced) = %#v, want exact whitespace-bearing identity", spaced)
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

	t.Run("Should close admission idempotently and reject every mutable API", func(t *testing.T) {
		t.Parallel()

		store := newRecordingDeliveryLedgerStore()
		originalTransport := &fakeDeliveryTransport{}
		broker := NewBroker(originalTransport, WithDeliveryLedgerStore(store))
		registration := mustRegisterTestDelivery(t, broker, PromptDeliveryRegistration{
			SessionID:     "sess-close-admission",
			TurnID:        "turn-close-admission",
			ExtensionName: "ext-telegram",
			RoutingKey:    testRoutingKey("brg-close-admission", "peer-close-admission"),
			DeliveryTarget: DeliveryTarget{
				BridgeInstanceID: "brg-close-admission",
				PeerID:           "peer-close-admission",
				Mode:             DeliveryModeReply,
			},
		})

		closed := make(chan struct{}, 2)
		for range 2 {
			go func() {
				broker.Close()
				closed <- struct{}{}
			}()
		}
		for range 2 {
			select {
			case <-closed:
			case <-t.Context().Done():
				t.Fatalf("concurrent Close() did not return: %v", t.Context().Err())
			}
		}

		ctx := testutil.Context(t)
		_, err := broker.RegisterPromptDelivery(ctx, PromptDeliveryRegistration{})
		if !errors.Is(err, ErrBrokerClosed) {
			t.Fatalf("RegisterPromptDelivery(after close) error = %v, want ErrBrokerClosed", err)
		}
		event := testDeliveryEvent(
			registration.DeliveryID,
			registration.BridgeInstanceID,
			registration.RoutingKey,
			registration.DeliveryTarget,
			1,
			DeliveryEventTypeStart,
			"closed",
			false,
		)
		if err := broker.Deliver(ctx, event); !errors.Is(err, ErrBrokerClosed) {
			t.Fatalf("Deliver(after close) error = %v, want ErrBrokerClosed", err)
		}
		if err := broker.ProjectEvent(
			ctx,
			registration.SessionID,
			DeliveryProjectionEvent{TurnID: registration.TurnID},
		); !errors.Is(
			err,
			ErrBrokerClosed,
		) {
			t.Fatalf("ProjectEvent(after close) error = %v, want ErrBrokerClosed", err)
		}
		if err := broker.FailSession(ctx, registration.SessionID, "closed"); !errors.Is(err, ErrBrokerClosed) {
			t.Fatalf("FailSession(after close) error = %v, want ErrBrokerClosed", err)
		}
		if err := broker.ReconcileDelivery(
			ctx, DeliveryLedgerRecord{}, "ext-telegram",
		); !errors.Is(err, ErrBrokerClosed) {
			t.Fatalf("ReconcileDelivery(after close) error = %v, want ErrBrokerClosed", err)
		}
		if err := broker.LoadDeliveryMetrics(
			ctx, DeliveryLedgerQuery{Scope: ScopeGlobal},
		); !errors.Is(err, ErrBrokerClosed) {
			t.Fatalf("LoadDeliveryMetrics(after close) error = %v, want ErrBrokerClosed", err)
		}
		broker.CompleteDeliveryReconciliation()
		if err := broker.CheckPromptDeliveryAdmission(ctx); !errors.Is(err, ErrBrokerClosed) {
			t.Fatalf("CheckPromptDeliveryAdmission(after close) error = %v, want ErrBrokerClosed", err)
		}
		replacementTransport := &fakeDeliveryTransport{}
		broker.SetTransport(replacementTransport)
		if got := broker.currentTransport(); got != originalTransport {
			t.Fatalf("currentTransport(after close) = %p, want original %p", got, originalTransport)
		}
	})

	t.Run("Should cancel and join an admitted registration before Close returns", func(t *testing.T) {
		t.Parallel()

		store := newBlockingDeliveryCreateStore()
		broker := NewBroker(nil, WithDeliveryLedgerStore(store))
		registrationDone := make(chan error, 1)
		go func() {
			_, err := broker.RegisterPromptDelivery(testutil.Context(t), PromptDeliveryRegistration{
				SessionID:     "sess-close-race",
				TurnID:        "turn-close-race",
				ExtensionName: "ext-telegram",
				RoutingKey:    testRoutingKey("brg-close-race", "peer-close-race"),
				DeliveryTarget: DeliveryTarget{
					BridgeInstanceID: "brg-close-race",
					PeerID:           "peer-close-race",
					Mode:             DeliveryModeReply,
				},
			})
			registrationDone <- err
		}()
		select {
		case <-store.started:
		case <-t.Context().Done():
			t.Fatalf("CreateBridgeDelivery() did not start: %v", t.Context().Err())
		}

		closeDone := make(chan struct{})
		go func() {
			broker.Close()
			close(closeDone)
		}()
		select {
		case err := <-registrationDone:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("RegisterPromptDelivery(during close) error = %v, want context.Canceled", err)
			}
		case <-t.Context().Done():
			t.Fatalf("RegisterPromptDelivery() did not receive lifecycle cancellation: %v", t.Context().Err())
		}
		select {
		case <-closeDone:
		case <-t.Context().Done():
			t.Fatalf("Close() did not join admitted registration: %v", t.Context().Err())
		}
		waitForBrokerRouteCount(t, broker, 0)
	})
}

type blockingDeliveryCreateStore struct {
	*recordingDeliveryLedgerStore
	started     chan struct{}
	startedOnce sync.Once
}

func newBlockingDeliveryCreateStore() *blockingDeliveryCreateStore {
	return &blockingDeliveryCreateStore{
		recordingDeliveryLedgerStore: newRecordingDeliveryLedgerStore(),
		started:                      make(chan struct{}),
	}
}

func (s *blockingDeliveryCreateStore) CreateBridgeDelivery(ctx context.Context, _ DeliveryLedgerRecord) error {
	s.startedOnce.Do(func() { close(s.started) })
	<-ctx.Done()
	return ctx.Err()
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
