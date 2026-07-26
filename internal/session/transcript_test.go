package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestManagerTranscriptPageDelegatesToProjection(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Logf("h.manager.Stop failed for session %s: %v", session.ID, err)
		}
	})

	recorder := session.recorderHandle()
	events := []store.SessionEvent{
		{
			Sequence:  1,
			TurnID:    "turn-1",
			Type:      acp.EventTypeUserMessage,
			AgentName: session.Info().AgentName,
			Content:   `{"schema":"agh.session.event.v1","type":"user_message","text":"hello"}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		},
		{
			Sequence:  2,
			TurnID:    "turn-1",
			Type:      acp.EventTypeAgentMessage,
			AgentName: session.Info().AgentName,
			Content:   `{"schema":"agh.session.event.v1","type":"agent_message","text":"hi"}`,
			Timestamp: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
		},
	}
	for _, event := range events {
		if err := recorder.Record(testutil.Context(t), event); err != nil {
			t.Fatalf("Record(%s) error = %v", event.Type, err)
		}
	}

	page, err := h.manager.TranscriptPage(testutil.Context(t), session.ID, transcript.PageQuery{})
	if err != nil {
		t.Fatalf("TranscriptPage() error = %v", err)
	}
	messages := transcript.MessagesFromEntries(page.Entries)
	if len(messages) != 2 {
		t.Fatalf("TranscriptPage() len = %d, want 2", len(messages))
	}
	if got := messages[0].Role; got != transcript.UIRoleUser {
		t.Fatalf("messages[0].Role = %q, want %q", got, transcript.UIRoleUser)
	}
	if got := messages[1].Role; got != transcript.UIRoleAssistant {
		t.Fatalf("messages[1].Role = %q, want %q", got, transcript.UIRoleAssistant)
	}
}

func TestManagerTranscriptPageIncludesSyntheticOriginMessages(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Logf("h.manager.Stop failed for session %s: %v", session.ID, err)
		}
	})

	recorder := session.recorderHandle()
	events := []store.SessionEvent{
		{
			Sequence:  1,
			TurnID:    "turn-user",
			Type:      acp.EventTypeUserMessage,
			AgentName: session.Info().AgentName,
			Content:   `{"schema":"agh.session.event.v1","type":"user_message","text":"hello"}`,
			Timestamp: time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC),
		},
		{
			Sequence:  2,
			TurnID:    "turn-synth",
			Type:      acp.EventTypeSyntheticReentry,
			AgentName: session.Info().AgentName,
			Content:   `{"schema":"agh.session.event.v1","type":"synthetic_reentry","text":"daemon wake-up"}`,
			Timestamp: time.Date(2026, 4, 18, 13, 0, 1, 0, time.UTC),
		},
		{
			Sequence:  3,
			TurnID:    "turn-synth",
			Type:      acp.EventTypeAgentMessage,
			AgentName: session.Info().AgentName,
			Content:   `{"schema":"agh.session.event.v1","type":"agent_message","text":"resuming work"}`,
			Timestamp: time.Date(2026, 4, 18, 13, 0, 2, 0, time.UTC),
		},
	}
	for _, event := range events {
		if err := recorder.Record(testutil.Context(t), event); err != nil {
			t.Fatalf("Record(%s) error = %v", event.Type, err)
		}
	}

	page, err := h.manager.TranscriptPage(testutil.Context(t), session.ID, transcript.PageQuery{})
	if err != nil {
		t.Fatalf("TranscriptPage() error = %v", err)
	}
	messages := transcript.MessagesFromEntries(page.Entries)
	if len(messages) != 3 {
		t.Fatalf("TranscriptPage() len = %d, want 3", len(messages))
	}
	if got := messages[0].Role; got != transcript.UIRoleUser {
		t.Fatalf("messages[0].Role = %q, want %q", got, transcript.UIRoleUser)
	}
	if got := messages[1].Role; got != transcript.UIRoleSystem {
		t.Fatalf("messages[1].Role = %q, want %q", got, transcript.UIRoleSystem)
	}
	if got := transcript.UIMessageText(messages[1]); got != "daemon wake-up" {
		t.Fatalf("messages[1] text = %q, want %q", got, "daemon wake-up")
	}
	if got := messages[2].Role; got != transcript.UIRoleAssistant {
		t.Fatalf("messages[2].Role = %q, want %q", got, transcript.UIRoleAssistant)
	}
}

func TestManagerTranscriptPageReturnsStoredQueryErrors(t *testing.T) {
	t.Parallel()

	queryErr := errors.New("query failed")
	recorder := &transcriptRecorderStub{pageErr: queryErr}
	h := newHarness(t, WithStore(func(_ context.Context, _ string, _ string) (EventRecorder, error) {
		return recorder, nil
	}))
	writeStoppedSessionArtifacts(t, h, "stored-query-failure", true)

	_, err := h.manager.TranscriptPage(testutil.Context(t), "stored-query-failure", transcript.PageQuery{})
	if !errors.Is(err, queryErr) {
		t.Fatalf("TranscriptPage() error = %v, want wrapped %v", err, queryErr)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder.closeCalls = %d, want 1", recorder.closeCalls)
	}
}

func TestManagerTranscriptPageLogsCleanupErrorsWithoutFailingSuccessfulRead(t *testing.T) {
	t.Parallel()

	recorder := &transcriptRecorderStub{
		queryRecorderStub: queryRecorderStub{
			events: []store.SessionEvent{{
				Sequence:  1,
				TurnID:    "turn-synth",
				Type:      acp.EventTypeSyntheticReentry,
				AgentName: "coder",
				Content:   `{"schema":"agh.session.event.v1","type":"synthetic_reentry","text":"daemon wake-up"}`,
				Timestamp: time.Date(2026, 4, 18, 13, 30, 0, 0, time.UTC),
			}},
		},
		closeErr: errors.New("close failed"),
	}
	h := newHarness(t, WithStore(func(_ context.Context, _ string, _ string) (EventRecorder, error) {
		return recorder, nil
	}))
	h.manager.logger = nil
	writeStoppedSessionArtifacts(t, h, "stored-cleanup-error", true)

	page, err := h.manager.TranscriptPage(testutil.Context(t), "stored-cleanup-error", transcript.PageQuery{})
	if err != nil {
		t.Fatalf("TranscriptPage() error = %v", err)
	}
	messages := transcript.MessagesFromEntries(page.Entries)
	if len(messages) != 1 {
		t.Fatalf("TranscriptPage() len = %d, want 1", len(messages))
	}
	if got := messages[0].Role; got != transcript.UIRoleSystem {
		t.Fatalf("messages[0].Role = %q, want %q", got, transcript.UIRoleSystem)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder.closeCalls = %d, want 1", recorder.closeCalls)
	}
}

func TestManagerTranscriptProjectionReads(t *testing.T) {
	t.Parallel()

	t.Run("Should read materialized pages without querying raw events", func(t *testing.T) {
		t.Parallel()

		recorder := &filteringTranscriptRecorder{}
		h := newHarness(
			t,
			WithStore(func(_ context.Context, _ string, _ string) (EventRecorder, error) {
				return recorder, nil
			}),
		)
		sessionID := "projected-transcript"
		writeStoppedSessionArtifacts(t, h, sessionID, true)
		recorder.Append(
			transcriptProjectionEvent(t, sessionID, 1, "turn-projection", acp.EventTypeUserMessage, "hello"),
			transcriptProjectionEvent(t, sessionID, 2, "turn-projection", acp.EventTypeAgentMessage, "hel"),
		)

		if _, err := h.manager.TranscriptPage(testutil.Context(t), sessionID, transcript.PageQuery{}); err != nil {
			t.Fatalf("TranscriptPage(initial) error = %v", err)
		}
		recorder.Append(
			transcriptProjectionEvent(t, sessionID, 3, "turn-projection", acp.EventTypeAgentMessage, "lo"),
			transcriptProjectionEvent(t, sessionID, 4, "turn-projection", acp.EventTypeDone, ""),
		)

		page, err := h.manager.TranscriptPage(testutil.Context(t), sessionID, transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(updated) error = %v", err)
		}
		want, err := transcript.ToUIEntries(recorder.Events())
		if err != nil {
			t.Fatalf("ToUIEntries() error = %v", err)
		}
		if !reflect.DeepEqual(page.Entries, want) {
			t.Fatalf("TranscriptPage(updated) = %#v, want %#v", page.Entries, want)
		}

		calls := recorder.PageCalls()
		if got, want := len(calls), 2; got != want {
			t.Fatalf("page calls = %d, want %d; calls=%#v", got, want, calls)
		}
		if calls[0].BeforeSequence != 0 || calls[1].BeforeSequence != 0 {
			t.Fatalf("page cursors = %#v, want tail reads", calls)
		}
		if rawCalls := recorder.QueryCalls(); len(rawCalls) != 0 {
			t.Fatalf("raw event query calls = %#v, want none", rawCalls)
		}
	})

	t.Run("Should keep projection reads scoped by session id", func(t *testing.T) {
		t.Parallel()

		recorders := map[string]*filteringTranscriptRecorder{
			"sess-cache-a": {},
			"sess-cache-b": {},
		}
		h := newHarness(
			t,
			WithStore(func(_ context.Context, sessionID string, _ string) (EventRecorder, error) {
				return recorders[sessionID], nil
			}),
		)
		for sessionID, recorder := range recorders {
			writeStoppedSessionArtifacts(t, h, sessionID, true)
			recorder.Append(transcriptProjectionEvent(
				t,
				sessionID,
				1,
				"turn-"+sessionID,
				acp.EventTypeAgentMessage,
				"text-"+sessionID,
			))
		}

		pageA, err := h.manager.TranscriptPage(testutil.Context(t), "sess-cache-a", transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(sess-cache-a) error = %v", err)
		}
		pageB, err := h.manager.TranscriptPage(testutil.Context(t), "sess-cache-b", transcript.PageQuery{})
		if err != nil {
			t.Fatalf("TranscriptPage(sess-cache-b) error = %v", err)
		}

		textA := transcript.JoinUIMessageText(transcript.MessagesFromEntries(pageA.Entries))
		textB := transcript.JoinUIMessageText(transcript.MessagesFromEntries(pageB.Entries))
		if textA != "text-sess-cache-a" {
			t.Fatalf("session A transcript = %q, want text-sess-cache-a", textA)
		}
		if textB != "text-sess-cache-b" {
			t.Fatalf("session B transcript = %q, want text-sess-cache-b", textB)
		}
	})

	t.Run("Should not block another session while one transcript page is querying", func(t *testing.T) {
		t.Parallel()

		releaseSlowQuery := make(chan struct{})
		var releaseSlowQueryOnce sync.Once
		releaseSlowQueryFn := func() {
			releaseSlowQueryOnce.Do(func() { close(releaseSlowQuery) })
		}
		t.Cleanup(releaseSlowQueryFn)
		slowRecorder := &blockingTranscriptRecorder{
			filteringTranscriptRecorder: filteringTranscriptRecorder{},
			started:                     make(chan struct{}),
			release:                     releaseSlowQuery,
		}
		fastRecorder := &filteringTranscriptRecorder{}
		h := newHarness(t)
		activate := func(sessionID string, recorder EventRecorder) {
			t.Helper()
			h.manager.mu.Lock()
			h.manager.pending[sessionID] = sessionReservation{}
			h.manager.mu.Unlock()
			if err := h.manager.activate(&Session{
				ID:       sessionID,
				State:    StateActive,
				recorder: recorder,
			}); err != nil {
				t.Fatalf("activate(%s) error = %v", sessionID, err)
			}
		}
		activate("sess-cache-slow", slowRecorder)
		activate("sess-cache-fast", fastRecorder)
		slowRecorder.Append(transcriptProjectionEvent(
			t,
			"sess-cache-slow",
			1,
			"turn-slow",
			acp.EventTypeAgentMessage,
			"slow",
		))
		fastRecorder.Append(transcriptProjectionEvent(
			t,
			"sess-cache-fast",
			1,
			"turn-fast",
			acp.EventTypeAgentMessage,
			"fast",
		))

		slowDone := make(chan error, 1)
		go func() {
			_, err := h.manager.TranscriptPage(testutil.Context(t), "sess-cache-slow", transcript.PageQuery{})
			slowDone <- err
		}()
		select {
		case <-slowRecorder.started:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("slow transcript query did not start")
		}

		fastDone := make(chan error, 1)
		go func() {
			page, err := h.manager.TranscriptPage(testutil.Context(t), "sess-cache-fast", transcript.PageQuery{})
			if err != nil {
				fastDone <- err
				return
			}
			if got := transcript.JoinUIMessageText(transcript.MessagesFromEntries(page.Entries)); got != "fast" {
				fastDone <- fmt.Errorf("fast transcript = %q, want fast", got)
				return
			}
			fastDone <- nil
		}()
		select {
		case err := <-fastDone:
			if err != nil {
				releaseSlowQueryFn()
				t.Fatalf("TranscriptPage(fast) error = %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			releaseSlowQueryFn()
			t.Fatal("Transcript(fast) blocked behind unrelated transcript cache query")
		}

		releaseSlowQueryFn()
		if err := <-slowDone; err != nil {
			t.Fatalf("TranscriptPage(slow) error = %v", err)
		}
	})
}

type transcriptRecorderStub struct {
	queryRecorderStub
	closeErr error
	pageErr  error
}

func (s *transcriptRecorderStub) Close(context.Context) error {
	s.closeCalls++
	return s.closeErr
}

func (s *transcriptRecorderStub) TranscriptPage(
	_ context.Context,
	query transcript.PageQuery,
) (transcript.Page, error) {
	if s.pageErr != nil {
		return transcript.Page{}, s.pageErr
	}
	return projectionPageFromEvents(s.events, query)
}

func (s *transcriptRecorderStub) TranscriptChanges(
	_ context.Context,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	if s.pageErr != nil {
		return transcript.ChangePage{}, s.pageErr
	}
	return projectionChangesFromEvents(s.events, query)
}

type filteringTranscriptRecorder struct {
	mu          sync.Mutex
	events      []store.SessionEvent
	queryCalls  []store.EventQuery
	pageCalls   []transcript.PageQuery
	changeCalls []transcript.ChangeQuery
}

type blockingTranscriptRecorder struct {
	filteringTranscriptRecorder
	started chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (r *filteringTranscriptRecorder) Append(events ...store.SessionEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, events...)
}

func (r *filteringTranscriptRecorder) Events() []store.SessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]store.SessionEvent(nil), r.events...)
}

func (r *filteringTranscriptRecorder) QueryCalls() []store.EventQuery {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]store.EventQuery(nil), r.queryCalls...)
}

func (r *filteringTranscriptRecorder) PageCalls() []transcript.PageQuery {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]transcript.PageQuery(nil), r.pageCalls...)
}

func (r *filteringTranscriptRecorder) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (r *filteringTranscriptRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *filteringTranscriptRecorder) Query(_ context.Context, query store.EventQuery) ([]store.SessionEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.queryCalls = append(r.queryCalls, query)

	events := make([]store.SessionEvent, 0, len(r.events))
	for _, event := range r.events {
		if query.AfterSequence > 0 && event.Sequence <= query.AfterSequence {
			continue
		}
		if query.BeforeSequence > 0 && event.Sequence >= query.BeforeSequence {
			continue
		}
		events = append(events, event)
	}
	if query.Limit > 0 && len(events) > query.Limit {
		events = events[len(events)-query.Limit:]
	}
	return append([]store.SessionEvent(nil), events...), nil
}

func (r *blockingTranscriptRecorder) Query(
	ctx context.Context,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	shouldBlock := false
	r.once.Do(func() {
		shouldBlock = true
		close(r.started)
	})
	if shouldBlock {
		select {
		case <-r.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.filteringTranscriptRecorder.Query(ctx, query)
}

func (r *filteringTranscriptRecorder) TranscriptPage(
	_ context.Context,
	query transcript.PageQuery,
) (transcript.Page, error) {
	r.mu.Lock()
	r.pageCalls = append(r.pageCalls, query)
	events := append([]store.SessionEvent(nil), r.events...)
	r.mu.Unlock()
	return projectionPageFromEvents(events, query)
}

func (r *filteringTranscriptRecorder) TranscriptChanges(
	_ context.Context,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	r.mu.Lock()
	r.changeCalls = append(r.changeCalls, query)
	events := append([]store.SessionEvent(nil), r.events...)
	r.mu.Unlock()
	return projectionChangesFromEvents(events, query)
}

func (r *blockingTranscriptRecorder) TranscriptPage(
	ctx context.Context,
	query transcript.PageQuery,
) (transcript.Page, error) {
	shouldBlock := false
	r.once.Do(func() {
		shouldBlock = true
		close(r.started)
	})
	if shouldBlock {
		select {
		case <-r.release:
		case <-ctx.Done():
			return transcript.Page{}, ctx.Err()
		}
	}
	return r.filteringTranscriptRecorder.TranscriptPage(ctx, query)
}

func (r *filteringTranscriptRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *filteringTranscriptRecorder) Close(context.Context) error {
	return nil
}

func projectionPageFromEvents(
	events []store.SessionEvent,
	query transcript.PageQuery,
) (transcript.Page, error) {
	query, err := query.Normalize()
	if err != nil {
		return transcript.Page{}, err
	}
	entries, err := transcript.ToUIEntries(events)
	if err != nil {
		return transcript.Page{}, err
	}
	maxSequence := maxEventSequenceForTest(events)
	filtered := make([]transcript.Entry, 0, len(entries))
	for _, entry := range entries {
		if query.BeforeSequence == 0 || entry.StartSequence < query.BeforeSequence {
			filtered = append(filtered, entry)
		}
	}
	page := transcript.Page{Generation: 0, MaxSequence: maxSequence}
	if len(filtered) > query.Limit {
		page.HasOlder = true
		filtered = filtered[len(filtered)-query.Limit:]
	}
	page.Entries = filtered
	if page.HasOlder {
		page.NextBeforeSequence = filtered[0].StartSequence
	}
	return page, nil
}

func projectionChangesFromEvents(
	events []store.SessionEvent,
	query transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	query, err := query.Normalize()
	if err != nil {
		return transcript.ChangePage{}, err
	}
	entries, err := transcript.ToUIEntries(events)
	if err != nil {
		return transcript.ChangePage{}, err
	}
	filtered := make([]transcript.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Sequence > query.AfterSequence {
			filtered = append(filtered, entry)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Sequence < filtered[j].Sequence
	})
	page := transcript.ChangePage{Generation: 0, MaxSequence: maxEventSequenceForTest(events)}
	if len(filtered) > query.Limit {
		page.HasMore = true
		filtered = filtered[:query.Limit]
	}
	page.Entries = filtered
	if len(filtered) > 0 {
		page.NextAfter = filtered[len(filtered)-1].Sequence
	} else {
		page.NextAfter = query.AfterSequence
	}
	return page, nil
}

func maxEventSequenceForTest(events []store.SessionEvent) int64 {
	var maxSequence int64
	for _, event := range events {
		if event.Sequence > maxSequence {
			maxSequence = event.Sequence
		}
	}
	return maxSequence
}

func transcriptProjectionEvent(
	t *testing.T,
	sessionID string,
	sequence int64,
	turnID string,
	eventType string,
	text string,
) store.SessionEvent {
	t.Helper()
	timestamp := time.Date(2026, 7, 7, 12, 0, 0, int(sequence), time.UTC)
	payload, err := json.Marshal(map[string]string{
		"schema":     "agh.session.event.v1",
		"type":       eventType,
		"session_id": sessionID,
		"turn_id":    turnID,
		"timestamp":  timestamp.Format(time.RFC3339Nano),
		"text":       text,
	})
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", eventType, err)
	}
	return store.SessionEvent{
		ID:        sessionID + "-" + eventType,
		SessionID: sessionID,
		Sequence:  sequence,
		TurnID:    turnID,
		Type:      eventType,
		AgentName: "coder",
		Content:   string(payload),
		Timestamp: timestamp,
	}
}
