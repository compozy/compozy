package calls

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
)

type callHookRecorder struct {
	mu      sync.Mutex
	events  []HookEvent
	payload []HookPayload
}

func (r *callHookRecorder) ObserveCall(_ context.Context, event HookEvent, payload HookPayload) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	r.payload = append(r.payload, payload)
}

func (r *callHookRecorder) snapshot() ([]HookEvent, []HookPayload) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]HookEvent(nil), r.events...), append([]HookPayload(nil), r.payload...)
}

type hookLifecycleStore struct {
	*publishTestStore
	messages   map[string]MessageRecord
	deliveries []DeliveryRecord
}

func (s *hookLifecycleStore) AcceptMessage(_ context.Context, admission MessageAdmission) (MessageRecord, error) {
	record := admission.Record
	record.ToSessionID = admission.Target
	record.Delivery = "pending"
	s.messages[record.MessageID] = record
	s.deliveries = append(s.deliveries, DeliveryRecord{
		DeliveryID: "delivery-" + record.MessageID, Kind: "message", SubjectID: record.MessageID,
		RecipientSessionID: record.ToSessionID, State: "pending", CreatedAt: record.CreatedAt,
	})
	return record, nil
}

func (s *hookLifecycleStore) GetMessage(_ context.Context, _ CallScope, messageID string) (MessageRecord, error) {
	record, ok := s.messages[messageID]
	if !ok {
		return MessageRecord{}, &Error{Code: CodeMessageNotFound, Message: "message not found"}
	}
	return record, nil
}

func (s *hookLifecycleStore) ListPendingDeliveries(
	_ context.Context,
	recipientSessionID string,
	limit int,
) ([]DeliveryRecord, error) {
	result := make([]DeliveryRecord, 0, len(s.deliveries))
	for _, item := range s.deliveries {
		if item.RecipientSessionID == recipientSessionID && item.State == "pending" {
			result = append(result, item)
			if limit > 0 && len(result) == limit {
				break
			}
		}
	}
	return result, nil
}

func (s *hookLifecycleStore) RecordDelivery(_ context.Context, update DeliveryUpdate) (DeliveryRecord, error) {
	for index := range s.deliveries {
		if s.deliveries[index].DeliveryID != update.DeliveryID {
			continue
		}
		s.deliveries[index].State = update.State
		s.deliveries[index].Reason = update.Reason
		message := s.messages[s.deliveries[index].SubjectID]
		message.Delivery = update.State
		message.DeliveryReason = update.Reason
		message.DeliveryAttempts++
		message.DeliveredAt = update.At
		s.messages[message.MessageID] = message
		return s.deliveries[index], nil
	}
	return DeliveryRecord{}, fmt.Errorf("delivery %q not found", update.DeliveryID)
}

func (*hookLifecycleStore) ParkCallChild(context.Context, string, time.Time, time.Time) (bool, error) {
	return false, nil
}

func (*hookLifecycleStore) ClearCallChildIdleClock(context.Context, string, time.Time) error {
	return nil
}

func (*hookLifecycleStore) FailPendingDeliveriesForRecipient(context.Context, string, string, time.Time) error {
	return nil
}

func (*hookLifecycleStore) FenceSessionReap(context.Context, string, time.Time) (bool, error) {
	return false, nil
}

func (*hookLifecycleStore) FinalizeReapedSession(context.Context, string, string, time.Time) error {
	return nil
}

type hookSessionInvoker struct {
	*fakeSessionInvoker
}

func (i *hookSessionInvoker) DeliverAtBoundary(_ context.Context, delivery Delivery) (DeliveryOutcome, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.deliveries = append(i.deliveries, delivery)
	return DeliveryOutcome{State: "injected", Reason: "active turn"}, nil
}

func TestCallHookTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should observe every committed lifecycle transition with sanitized typed payloads", func(t *testing.T) {
		t.Parallel()

		store := &hookLifecycleStore{
			publishTestStore: &publishTestStore{
				memoryCallStore: newMemoryCallStore(), publications: make(map[string]Publication),
			},
			messages: make(map[string]MessageRecord),
		}
		hooks := &callHookRecorder{}
		claimer := &fakeActivationClaimer{}
		canceler := &fakeActivationCanceler{}
		invoker := &hookSessionInvoker{fakeSessionInvoker: &fakeSessionInvoker{}}
		bridge := &publishTestBridge{}
		var sequence atomic.Int64
		service, err := NewService(
			WithStore(store),
			WithDirectory(staticCallDirectory{target: validAgentTarget()}),
			WithActivationClaimer(claimer),
			WithActivationRunCanceler(canceler),
			WithSessionInvoker(invoker),
			WithPublishBridge(bridge),
			WithHookDispatcher(hooks),
			WithConfig(config.DefaultCallsConfig()),
			WithClock(func() time.Time { return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC) }),
			WithIDGenerator(func(prefix string) (string, error) {
				return fmt.Sprintf("%s-%d", prefix, sequence.Add(1)), nil
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}

		created, err := service.Create(context.Background(), validCreateInput("secret prompt", nil, nil))
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		settled, err := service.Return(context.Background(), ReturnInput{
			CallID: created.CallID, Result: json.RawMessage(`{"answer":"secret result"}`),
			Actor: SettlementActor{Kind: "agent_session", ID: created.ChildSessionID},
		})
		if err != nil {
			t.Fatalf("Return() error = %v", err)
		}
		if _, err := service.Publish(context.Background(), PublishInput{
			ProfileID: settled.Call.ProfileID, Scope: settled.Call.Scope, WorkspaceID: settled.Call.WorkspaceID,
			CallID: settled.Call.CallID, Actor: Actor{Kind: "agent_session", ID: created.ChildSessionID},
			Channel: "reviews", ThreadID: "thread-1",
		}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		cancelInput := validCreateInput("cancel me", nil, nil)
		cancelInput.IdempotencyKey = "cancel-key"
		cancelRecord, err := service.Create(context.Background(), cancelInput)
		if err != nil {
			t.Fatalf("Create(cancel) error = %v", err)
		}
		if _, err := service.Cancel(context.Background(), cancelRecord.CallID, "operator canceled", cancelRecord.Actor); err != nil {
			t.Fatalf("Cancel() error = %v", err)
		}

		drainInput := validCreateInput("drain me", nil, nil)
		drainInput.IdempotencyKey = "drain-key"
		drainRecord, err := service.Create(context.Background(), drainInput)
		if err != nil {
			t.Fatalf("Create(drain) error = %v", err)
		}
		store.subtree = []CallRecord{drainRecord}
		store.preservedResults = 1
		if _, err := service.DrainSubtree(
			context.Background(), drainRecord.GovernedRootID, Actor{Kind: "daemon", ID: "recovery"}, "root stopped",
		); err != nil {
			t.Fatalf("DrainSubtree() error = %v", err)
		}

		message, err := service.SendMessage(context.Background(), SendMessageInput{
			ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
			From: MessageSender{Kind: "operator", ID: "operator:test"}, To: "child-mailbox",
			CallID: created.CallID, Body: "secret message",
		})
		if err != nil {
			t.Fatalf("SendMessage() error = %v", err)
		}
		if err := service.DrainDeliveries(context.Background(), message.ToSessionID, 10); err != nil {
			t.Fatalf("DrainDeliveries() error = %v", err)
		}

		events, payloads := hooks.snapshot()
		for _, want := range []HookEvent{
			HookCallCreated, HookCallSettled, HookCallCanceled, HookCallPublished,
			HookCallMessageSent, HookCallMessageDelivered, HookCallSubtreeDrained,
		} {
			if !containsHookEvent(events, want) {
				t.Fatalf("hook events = %#v, missing %q", events, want)
			}
		}
		for _, payload := range payloads {
			encoded, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(hook payload) error = %v", marshalErr)
			}
			for _, secret := range []string{"secret prompt", "secret result", "secret message"} {
				if stringContains(string(encoded), secret) {
					t.Fatalf("hook payload leaked %q: %s", secret, encoded)
				}
			}
		}
	})
}

func containsHookEvent(events []HookEvent, want HookEvent) bool {
	for _, event := range events {
		if event == want {
			return true
		}
	}
	return false
}

func stringContains(value, fragment string) bool {
	for start := 0; start+len(fragment) <= len(value); start++ {
		if value[start:start+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
