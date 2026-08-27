package calls

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
)

func TestServicePublish(t *testing.T) {
	t.Parallel()

	t.Run("Should publish bounded evidence with publisher attribution and replay per conversation", func(t *testing.T) {
		t.Parallel()
		service, store, bridge := newPublishHarness(t, true)
		payload, err := json.Marshal(map[string]string{
			"summary": strings.Repeat("bounded evidence ", 600),
			"secret":  "COMPOZY_CLAIM_private-token",
		})
		if err != nil {
			t.Fatalf("json.Marshal(payload) error = %v", err)
		}
		store.seedCompletedCall(payload)
		input := validPublishInput()
		first, err := service.Publish(context.Background(), input)
		if err != nil {
			t.Fatalf("Publish(first) error = %v", err)
		}
		second, err := service.Publish(context.Background(), input)
		if err != nil {
			t.Fatalf("Publish(replay) error = %v", err)
		}
		input.ThreadID = "thread-other"
		third, err := service.Publish(context.Background(), input)
		if err != nil {
			t.Fatalf("Publish(other conversation) error = %v", err)
		}
		if !first.Published || second.Published || !third.Published ||
			first.NetworkMessageID != second.NetworkMessageID || len(bridge.evidence) != 2 {
			t.Fatalf("publication receipts = %#v %#v %#v; bridge=%#v", first, second, third, bridge.evidence)
		}
		for _, evidence := range bridge.evidence {
			if evidence.SourceSessionID != "ses_publisher" || len(evidence.ResultPreview) >= 1<<20 ||
				strings.Contains(string(evidence.ResultPreview), "private-token") {
				t.Fatalf("published evidence = %#v", evidence)
			}
		}
	})

	t.Run("Should reject every resultless or non-completed state", func(t *testing.T) {
		t.Parallel()
		for _, test := range []struct {
			name  string
			state State
		}{
			{name: "Should reject queued calls", state: StateQueued},
			{name: "Should reject running calls", state: StateRunning},
			{name: "Should reject invalid results", state: StateInvalidResult},
			{name: "Should reject completed calls without results", state: StateCompletedWithoutResult},
			{name: "Should reject failed calls", state: StateFailed},
			{name: "Should reject canceled calls", state: StateCanceled},
			{name: "Should reject timed out calls", state: StateTimeout},
			{name: "Should reject expired calls", state: StateExpired},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				service, store, _ := newPublishHarness(t, true)
				store.calls["call-publish"] = publishCallRecord(test.state)
				_, err := service.Publish(context.Background(), validPublishInput())
				if !IsCode(err, CodePublishNotSettled) {
					t.Fatalf("Publish(%s) error = %v, want %s", test.state, err, CodePublishNotSettled)
				}
			})
		}
	})

	t.Run("Should require active Network participation without mutating the call", func(t *testing.T) {
		t.Parallel()
		service, store, _ := newPublishHarness(t, false)
		store.seedCompletedCall(json.RawMessage(`{"verdict":"approved"}`))
		before := store.calls["call-publish"]
		_, err := service.Publish(context.Background(), validPublishInput())
		if !IsCode(err, CodePublishNoParticipation) || store.calls["call-publish"] != before {
			t.Fatalf(
				"Publish(no participation) error = %v; call changed=%t",
				err,
				store.calls["call-publish"] != before,
			)
		}
	})

	t.Run("Should distinguish participation absence from publication failure", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name string
			err  error
			code ErrorCode
		}{
			{name: "Should preserve participation absence", err: participation.ErrNotParticipating, code: CodePublishNoParticipation},
			{name: "Should preserve bridge failure", err: errors.New("network send rejected"), code: CodePublishFailed},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				service, store, bridge := newPublishHarness(t, true)
				store.seedCompletedCall(json.RawMessage(`{"verdict":"approved"}`))
				bridge.err = test.err

				_, err := service.Publish(t.Context(), validPublishInput())

				if !IsCode(err, test.code) || !errors.Is(err, test.err) {
					t.Fatalf("Publish() error = %v, want %s preserving cause", err, test.code)
				}
			})
		}
	})

	t.Run("Should return the committed Network receipt and recover receipt persistence", func(t *testing.T) {
		t.Parallel()
		service, store, bridge := newPublishHarness(t, true)
		store.seedCompletedCall(json.RawMessage(`{"verdict":"approved"}`))
		persistErr := errors.New("publication receipt database unavailable")
		store.recordErrors = []error{persistErr}
		var logs bytes.Buffer
		service.logger = slog.New(slog.NewTextHandler(&logs, nil))

		first, err := service.Publish(t.Context(), validPublishInput())
		if err != nil || !first.Published || first.NetworkMessageID == "" {
			t.Fatalf("Publish(committed) = %#v, %v, want truthful Network receipt", first, err)
		}
		if len(store.publications) != 0 || len(bridge.evidence) != 1 ||
			!strings.Contains(logs.String(), persistErr.Error()) {
			t.Fatalf(
				"post-commit recovery state = publications %d bridge %d logs %q",
				len(store.publications),
				len(bridge.evidence),
				logs.String(),
			)
		}

		recovered, err := service.Publish(t.Context(), validPublishInput())
		if err != nil || !recovered.Published || recovered.NetworkMessageID != first.NetworkMessageID {
			t.Fatalf("Publish(recovery) = %#v, %v, want same deterministic Network identity", recovered, err)
		}
		replay, err := service.Publish(t.Context(), validPublishInput())
		if err != nil || replay.Published || replay.NetworkMessageID != first.NetworkMessageID ||
			len(store.publications) != 1 || len(bridge.evidence) != 2 ||
			bridge.evidence[0].MessageID != bridge.evidence[1].MessageID {
			t.Fatalf(
				"Publish(replay) = %#v, %v publications=%d bridge=%#v",
				replay,
				err,
				len(store.publications),
				bridge.evidence,
			)
		}
	})

	t.Run("Should serialize one conversation without blocking an unrelated publication", func(t *testing.T) {
		t.Parallel()
		service, store, bridge := newPublishHarness(t, true)
		store.seedCompletedCall(json.RawMessage(`{"verdict":"approved"}`))
		other := publishCallRecord(StateCompleted)
		other.CallID = "call-other"
		other.ResultRef = "result-other"
		other.ResultBytes = 20
		store.calls[other.CallID] = other
		store.payloads[callPayloadKey(other.WorkspaceID, other.ResultRef)] =
			json.RawMessage(`{"verdict":"changes"}`)
		firstEntered := make(chan struct{})
		secondEntered := make(chan struct{})
		releaseFirst := make(chan struct{})
		bridge.publish = func(evidence ResultEvidence) (string, error) {
			switch evidence.CallID {
			case "call-publish":
				close(firstEntered)
				<-releaseFirst
			case "call-other":
				close(secondEntered)
			}
			return evidence.MessageID, nil
		}
		type outcome struct {
			receipt PublishReceipt
			err     error
		}
		firstResult := make(chan outcome, 1)
		secondResult := make(chan outcome, 1)
		ctx := t.Context()
		go func() {
			receipt, err := service.Publish(ctx, validPublishInput())
			firstResult <- outcome{receipt: receipt, err: err}
		}()
		<-firstEntered
		otherInput := validPublishInput()
		otherInput.CallID = other.CallID
		go func() {
			receipt, err := service.Publish(ctx, otherInput)
			secondResult <- outcome{receipt: receipt, err: err}
		}()

		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		timedOut := false
		select {
		case <-secondEntered:
			close(releaseFirst)
		case <-timer.C:
			close(releaseFirst)
			timedOut = true
		}
		first, second := <-firstResult, <-secondResult
		if timedOut {
			t.Fatal("unrelated publication waited behind blocked Network I/O")
		}
		if first.err != nil || second.err != nil || !first.receipt.Published || !second.receipt.Published {
			t.Fatalf("publication outcomes = %#v / %#v", first, second)
		}
	})
}

type publishTestStore struct {
	*memoryCallStore
	muPublications sync.Mutex
	publications   map[string]Publication
	recordErrors   []error
}

func (s *publishTestStore) seedCompletedCall(payload []byte) {
	record := publishCallRecord(StateCompleted)
	record.ResultRef = "result-publish"
	record.ResultBytes = len(payload)
	s.calls[record.CallID] = record
	s.payloads[callPayloadKey(record.WorkspaceID, record.ResultRef)] = append([]byte(nil), payload...)
}

func (s *publishTestStore) GetPublication(_ context.Context, callID, channel, threadID string) (Publication, error) {
	s.muPublications.Lock()
	defer s.muPublications.Unlock()
	publication, ok := s.publications[callID+"\x00"+channel+"\x00"+threadID]
	if !ok {
		return Publication{}, ErrPublicationNotFound
	}
	return publication, nil
}

func (s *publishTestStore) RecordPublication(_ context.Context, publication Publication) (Publication, bool, error) {
	s.muPublications.Lock()
	defer s.muPublications.Unlock()
	if len(s.recordErrors) > 0 {
		err := s.recordErrors[0]
		s.recordErrors = s.recordErrors[1:]
		return Publication{}, false, err
	}
	key := publication.CallID + "\x00" + publication.Channel + "\x00" + publication.ThreadID
	if existing, ok := s.publications[key]; ok {
		return existing, false, nil
	}
	s.publications[key] = publication
	return publication, true, nil
}

func (s *publishTestStore) AcceptMessage(context.Context, MessageAdmission) (MessageRecord, error) {
	return MessageRecord{}, errors.New("not configured")
}
func (s *publishTestStore) GetMessage(context.Context, CallScope, string) (MessageRecord, error) {
	return MessageRecord{}, errors.New("not configured")
}
func (s *publishTestStore) ListPendingDeliveries(context.Context, string, int) ([]DeliveryRecord, error) {
	return nil, errors.New("not configured")
}
func (s *publishTestStore) RecordDelivery(context.Context, DeliveryUpdate) (DeliveryRecord, error) {
	return DeliveryRecord{}, errors.New("not configured")
}
func (s *publishTestStore) ParkCallChild(context.Context, string, time.Time, time.Time) (bool, error) {
	return false, errors.New("not configured")
}
func (s *publishTestStore) ClearCallChildIdleClock(context.Context, string, time.Time) error {
	return errors.New("not configured")
}
func (s *publishTestStore) FailPendingDeliveriesForRecipient(context.Context, string, string, time.Time) error {
	return errors.New("not configured")
}
func (s *publishTestStore) FenceSessionReap(context.Context, string, time.Time) (bool, error) {
	return false, errors.New("not configured")
}
func (s *publishTestStore) FinalizeReapedSession(context.Context, string, string, time.Time) error {
	return errors.New("not configured")
}

type publishTestBridge struct {
	mu       sync.Mutex
	evidence []ResultEvidence
	err      error
	publish  func(ResultEvidence) (string, error)
}

func (b *publishTestBridge) PublishResultEvidence(_ context.Context, evidence ResultEvidence) (string, error) {
	b.mu.Lock()
	b.evidence = append(b.evidence, evidence)
	err := b.err
	publish := b.publish
	b.mu.Unlock()
	if err != nil {
		return "", err
	}
	if publish != nil {
		return publish(evidence)
	}
	return evidence.MessageID, nil
}

func newPublishHarness(t *testing.T, withBridge bool) (*Service, *publishTestStore, *publishTestBridge) {
	t.Helper()
	store := &publishTestStore{memoryCallStore: newMemoryCallStore(), publications: make(map[string]Publication)}
	bridge := &publishTestBridge{}
	options := []Option{
		WithStore(store), WithDirectory(staticCallDirectory{target: validAgentTarget()}),
		WithConfig(config.DefaultCallsConfig()), WithClock(func() time.Time {
			return time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
		}),
	}
	if withBridge {
		options = append(options, WithPublishBridge(bridge))
	}
	var sequence atomic.Int64
	options = append(options, WithIDGenerator(func(prefix string) (string, error) {
		return fmt.Sprintf("%s-%d", prefix, sequence.Add(1)), nil
	}))
	service, err := NewService(options...)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, store, bridge
}

func publishCallRecord(state State) CallRecord {
	return CallRecord{
		CallID: "call-publish", ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1",
		Caller: validCreateInput("publish", nil, nil).Caller,
		Actor:  Actor{Kind: "human", ID: "operator:test"},
		State:  state, GovernedRootID: "root-1", AgentName: "reviewer", ChildSessionID: "ses_child",
		CreatedAt: time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 8, 25, 11, 0, 0, 0, time.UTC),
	}
}

func validPublishInput() PublishInput {
	return PublishInput{
		ProfileID: "default", Scope: ScopeWorkspace, WorkspaceID: "ws-1", CallID: "call-publish",
		Actor: Actor{Kind: "agent_session", ID: "ses_publisher"}, Channel: "eng-room", ThreadID: "thread-reviews",
	}
}
