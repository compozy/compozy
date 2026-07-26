package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestPromptChunkCoalescing(t *testing.T) {
	t.Parallel()

	t.Run("Should persist contiguous same-turn text chunks as one event row", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 4)
			go func() {
				defer close(events)
				now := time.Now().UTC()
				for _, text := range []string{"hel", "lo ", "world"} {
					events <- acp.AgentEvent{
						Type:      acp.EventTypeAgentMessage,
						SessionID: proc.handle.SessionID,
						TurnID:    req.TurnID,
						Timestamp: now,
						Text:      text,
					}
				}
				events <- acp.AgentEvent{
					Type:      acp.EventTypeDone,
					SessionID: proc.handle.SessionID,
					TurnID:    req.TurnID,
					Timestamp: now.Add(time.Millisecond),
				}
			}()
			return events, nil
		}
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		var prepared PromptDelivery
		eventsCh, err := h.manager.PromptWithOpts(testutil.Context(t), session.ID, PromptOpts{
			Message: "hello",
			PrepareDelivery: func(_ context.Context, delivery PromptDelivery) error {
				prepared = delivery
				got := countAgentEvents(
					h.notifier.eventsForSession(session.ID),
					acp.EventTypeAgentMessage,
				)
				if got != 0 {
					t.Fatalf("agent_message notifier events before delivery preparation = %d, want 0", got)
				}
				return nil
			},
		})
		if err != nil {
			t.Fatalf("PromptWithOpts() error = %v", err)
		}
		if prepared.SessionID != session.ID || strings.TrimSpace(prepared.TurnID) == "" {
			t.Fatalf("prepared delivery = %#v, want session %q and a turn id", prepared, session.ID)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		if got, want := countAgentEvents(runtimeEvents, acp.EventTypeAgentMessage), 3; got != want {
			t.Fatalf("agent_message runtime events = %d, want %d", got, want)
		}
		gotTexts := agentEventTexts(runtimeEvents, acp.EventTypeAgentMessage)
		wantTexts := []string{"hel", "lo ", "world"}
		if !slices.Equal(gotTexts, wantTexts) {
			t.Fatalf("agent_message runtime texts = %q, want %q", gotTexts, wantTexts)
		}
		notifiedTexts := agentEventTexts(h.notifier.eventsForSession(session.ID), acp.EventTypeAgentMessage)
		if !slices.Equal(notifiedTexts, wantTexts) {
			t.Fatalf("agent_message notifier texts = %q, want streaming texts %q", notifiedTexts, wantTexts)
		}

		stored := readStoredEvents(t, session)
		if got, want := countEventType(stored, acp.EventTypeAgentMessage), 1; got != want {
			t.Fatalf("agent_message stored rows = %d, want %d", got, want)
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), session.ID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		if len(page.Entries) == 0 ||
			transcript.UIMessageText(page.Entries[len(page.Entries)-1].Message) != "hello world" {
			t.Fatalf("TranscriptPage() = %#v, want coalesced assistant content", page.Entries)
		}
	})

	t.Run("Should reprocess first chunk after a chunk run boundary", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 5)
			go func() {
				defer close(events)
				now := time.Now().UTC()
				events <- acp.AgentEvent{
					Type:      acp.EventTypeAgentMessage,
					SessionID: proc.handle.SessionID,
					TurnID:    req.TurnID,
					Timestamp: now,
					Text:      "a",
				}
				for _, text := range []string{"b", "c"} {
					events <- acp.AgentEvent{
						Type:      acp.EventTypeThought,
						SessionID: proc.handle.SessionID,
						TurnID:    req.TurnID,
						Timestamp: now.Add(time.Millisecond),
						Text:      text,
					}
				}
				events <- acp.AgentEvent{
					Type:      acp.EventTypeDone,
					SessionID: proc.handle.SessionID,
					TurnID:    req.TurnID,
					Timestamp: now.Add(2 * time.Millisecond),
				}
			}()
			return events, nil
		}
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		gotAgentMessages := agentEventTexts(runtimeEvents, acp.EventTypeAgentMessage)
		wantAgentMessages := []string{"a"}
		if !slices.Equal(gotAgentMessages, wantAgentMessages) {
			t.Fatalf("agent_message runtime texts = %q, want %q", gotAgentMessages, wantAgentMessages)
		}
		gotThoughts := agentEventTexts(runtimeEvents, acp.EventTypeThought)
		wantThoughts := []string{"b", "c"}
		if !slices.Equal(gotThoughts, wantThoughts) {
			t.Fatalf("thought runtime texts = %q, want %q", gotThoughts, wantThoughts)
		}

		stored := readStoredEvents(t, session)
		if got, want := countEventType(stored, acp.EventTypeAgentMessage), 1; got != want {
			t.Fatalf("agent_message stored rows = %d, want %d", got, want)
		}
		if got, want := countEventType(stored, acp.EventTypeThought), 1; got != want {
			t.Fatalf("thought stored rows = %d, want %d", got, want)
		}
		if got, want := storedTextByType(t, stored, acp.EventTypeThought), "bc"; got != want {
			t.Fatalf("thought stored text = %q, want %q", got, want)
		}
	})

	t.Run("Should stop without publishing chunks when batch persistence fails", func(t *testing.T) {
		t.Parallel()

		recordErr := errors.New("batch persist failed")
		recorder := &failingBatchRecorder{batchErr: recordErr}
		h := newHarness(t, WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return recorder, nil
		}))
		turnEnds := 0
		h.manager.SetTurnEndNotifier(func(string) { turnEnds++ })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			go func() {
				defer close(events)
				now := time.Now().UTC()
				events <- acp.AgentEvent{
					Type:      acp.EventTypeAgentMessage,
					SessionID: proc.handle.SessionID,
					TurnID:    req.TurnID,
					Timestamp: now,
					Text:      "hidden ",
				}
				events <- acp.AgentEvent{
					Type:      acp.EventTypeAgentMessage,
					SessionID: proc.handle.SessionID,
					TurnID:    req.TurnID,
					Timestamp: now.Add(time.Millisecond),
					Text:      "chunk",
				}
			}()
			return events, nil
		}
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		if len(runtimeEvents) != 0 {
			t.Fatalf("runtime events = %#v, want none after batch persistence failure", runtimeEvents)
		}
		notified := h.notifier.eventsForSession(session.ID)
		if got := countAgentEvents(notified, acp.EventTypeAgentMessage); got != 0 {
			t.Fatalf("agent_message notifier events = %d, want zero after batch persistence failure", got)
		}
		if turnEnds != 0 {
			t.Fatalf("turn end notifications = %d, want zero after persistence failure", turnEnds)
		}
		waitForCondition(t, "session stopped after batch persistence failure", func() bool {
			_, active := h.manager.Get(session.ID)
			return !active
		})
		meta := readMeta(t, session.MetaPath())
		if meta.Failure == nil || meta.Failure.Kind != store.FailureTransport {
			t.Fatalf("stopped session failure = %#v, want transport failure", meta.Failure)
		}
		stored, queryErr := recorder.Query(testutil.Context(t), store.EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query(reload) error = %v", queryErr)
		}
		if got := countEventType(stored, acp.EventTypeAgentMessage); got != 0 {
			t.Fatalf("reloaded agent_message rows = %d, want zero", got)
		}
	})

	t.Run("Should deliver a committed event once when only token usage projection fails", func(t *testing.T) {
		t.Parallel()

		recorder := &failingBatchRecorder{tokenErr: errors.New("token usage unavailable")}
		h := newHarness(t, WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return recorder, nil
		}))
		totalTokens := int64(2)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			go func() {
				defer close(events)
				events <- acp.AgentEvent{
					Type:      acp.EventTypeAgentMessage,
					SessionID: proc.handle.SessionID,
					TurnID:    req.TurnID,
					Text:      "durable",
					Usage:     &acp.TokenUsage{TurnID: req.TurnID, TotalTokens: &totalTokens},
				}
				events <- acp.AgentEvent{Type: acp.EventTypeDone, SessionID: proc.handle.SessionID, TurnID: req.TurnID}
			}()
			return events, nil
		}
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		runtimeEvents := collectEvents(t, eventsCh)
		if got := countAgentEvents(runtimeEvents, acp.EventTypeAgentMessage); got != 1 {
			t.Fatalf("agent_message runtime events = %d, want one committed delivery", got)
		}
		_, usageCalls := recorder.callCounts()
		if usageCalls != 1 {
			t.Fatalf("token usage calls = %d, want one attempt and no retry", usageCalls)
		}
		stored, queryErr := recorder.Query(testutil.Context(t), store.EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query(committed event) error = %v", queryErr)
		}
		if got := countEventType(stored, acp.EventTypeAgentMessage); got != 1 {
			t.Fatalf("committed agent_message rows = %d, want one without retry", got)
		}
		if _, active := h.manager.Get(session.ID); !active {
			t.Fatal("session stopped after auxiliary token usage failure, want active")
		}
	})
}

func countAgentEvents(events []acp.AgentEvent, want string) int {
	count := 0
	for _, event := range events {
		if event.Type == want {
			count++
		}
	}
	return count
}

func agentEventTexts(events []acp.AgentEvent, want string) []string {
	texts := make([]string, 0)
	for _, event := range events {
		if event.Type == want {
			texts = append(texts, event.Text)
		}
	}
	return texts
}

func storedTextByType(t *testing.T, events []store.SessionEvent, want string) string {
	t.Helper()

	for _, event := range events {
		if event.Type != want {
			continue
		}
		agentEvent, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent(%s) error = %v", event.Type, err)
		}
		return agentEvent.Text
	}
	t.Fatalf("stored event type %q not found", want)
	return ""
}

type failingBatchRecorder struct {
	mu         sync.Mutex
	batchErr   error
	tokenErr   error
	failed     bool
	events     []store.SessionEvent
	batchCalls int
	usageCalls int
}

func (r *failingBatchRecorder) Record(_ context.Context, event store.SessionEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failed {
		return r.batchErr
	}
	event.Sequence = int64(len(r.events) + 1)
	r.events = append(r.events, event)
	return nil
}

func (r *failingBatchRecorder) RecordPersistedBatch(
	_ context.Context,
	events []store.SessionEvent,
) ([]store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.batchCalls++
	if r.batchErr != nil {
		r.failed = true
		return nil, r.batchErr
	}
	persisted := make([]store.SessionEvent, len(events))
	for index := range events {
		persisted[index] = events[index]
		persisted[index].Sequence = int64(len(r.events) + 1)
		r.events = append(r.events, persisted[index])
	}
	return persisted, nil
}

func (r *failingBatchRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usageCalls++
	return r.tokenErr
}

func (r *failingBatchRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]store.SessionEvent(nil), r.events...), nil
}

func (r *failingBatchRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *failingBatchRecorder) Close(context.Context) error {
	return nil
}

func (r *failingBatchRecorder) callCounts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.batchCalls, r.usageCalls
}
