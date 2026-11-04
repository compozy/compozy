package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
)

const canonicalEventSchema = "compozy.session.event.v1"

// ErrEventIdentityCollision reports a deterministic event ID reused with different content.
var ErrEventIdentityCollision = errors.New("store: session event identity collision")

// Record appends a session event using the dedicated writer goroutine.
func (s *SessionDB) Record(ctx context.Context, event store.SessionEvent) error {
	_, err := s.recordSessionEvent(ctx, event)
	return err
}

// RecordPersisted appends a session event and returns the stored row with sequence metadata.
func (s *SessionDB) RecordPersisted(ctx context.Context, event store.SessionEvent) (store.SessionEvent, error) {
	return s.recordSessionEvent(ctx, event)
}

// AppendEventIfAbsent inserts one deterministic event or returns its byte-identical existing row.
func (s *SessionDB) AppendEventIfAbsent(
	ctx context.Context,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	if s == nil {
		return store.SessionEvent{}, false, errors.New("store: session database is required")
	}
	if ctx == nil {
		return store.SessionEvent{}, false, errors.New("store: append event context is required")
	}
	normalized, err := s.normalizeSessionEvent(event)
	if err != nil {
		return store.SessionEvent{}, false, err
	}
	normalized.ID = strings.TrimSpace(normalized.ID)
	if normalized.ID == "" {
		return store.SessionEvent{}, false, errors.New("store: append-if-absent event id is required")
	}
	return s.enqueueWritePersisted(ctx, sessionWriteRequest{
		ctx: ctx, kind: sessionWriteEventIfAbsent, event: normalized, result: make(chan sessionWriteResult, 1),
	})
}

// RecordPersistedBatch appends session events in one writer-owned transaction.
func (s *SessionDB) RecordPersistedBatch(
	ctx context.Context,
	events []store.SessionEvent,
) ([]store.SessionEvent, error) {
	if s == nil {
		return nil, errors.New("store: session database is required")
	}
	if ctx == nil {
		return nil, errors.New("store: record event batch context is required")
	}
	if len(events) == 0 {
		return nil, nil
	}
	normalized := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		prepared, err := s.normalizeSessionEvent(event)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, prepared)
	}
	return s.enqueueWritePersistedBatch(ctx, sessionWriteRequest{
		ctx:    ctx,
		kind:   sessionWriteEventBatch,
		events: normalized,
		result: make(chan sessionWriteResult, 1),
	})
}

func (s *SessionDB) recordSessionEvent(
	ctx context.Context,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	if s == nil {
		return store.SessionEvent{}, errors.New("store: session database is required")
	}
	if ctx == nil {
		return store.SessionEvent{}, errors.New("store: record event context is required")
	}
	normalized, err := s.normalizeSessionEvent(event)
	if err != nil {
		return store.SessionEvent{}, err
	}
	persisted, _, err := s.enqueueWritePersisted(ctx, sessionWriteRequest{
		ctx:    ctx,
		kind:   sessionWriteEvent,
		event:  normalized,
		result: make(chan sessionWriteResult, 1),
	})
	return persisted, err
}

func (s *SessionDB) normalizeSessionEvent(event store.SessionEvent) (store.SessionEvent, error) {
	if err := event.Validate(); err != nil {
		return store.SessionEvent{}, err
	}
	if event.SessionID != "" && event.SessionID != s.owner.SessionID {
		return store.SessionEvent{}, fmt.Errorf(
			"store: event session id %q does not match session database %q",
			event.SessionID,
			s.owner.SessionID,
		)
	}
	event.SessionID = s.owner.SessionID
	event.Content = redactSessionEventContent(event.Content)
	return event, nil
}

func (s *SessionDB) enqueueWritePersisted(
	ctx context.Context,
	req sessionWriteRequest,
) (store.SessionEvent, bool, error) {
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()

	if s.state.Load() != sessionStateOpen {
		return store.SessionEvent{}, false, store.ErrClosed
	}

	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return store.SessionEvent{}, false, fmt.Errorf("store: enqueue session write: %w", ctx.Err())
	}

	select {
	case result := <-req.result:
		if result.err != nil {
			return store.SessionEvent{}, false, result.err
		}
		return result.event, result.inserted, nil
	case <-ctx.Done():
		return store.SessionEvent{}, false, fmt.Errorf("store: wait for session write completion: %w", ctx.Err())
	}
}

func (s *SessionDB) enqueueWritePersistedBatch(
	ctx context.Context,
	req sessionWriteRequest,
) ([]store.SessionEvent, error) {
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()

	if s.state.Load() != sessionStateOpen {
		return nil, store.ErrClosed
	}

	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return nil, fmt.Errorf("store: enqueue session write batch: %w", ctx.Err())
	}

	select {
	case result := <-req.result:
		if result.err != nil {
			return nil, result.err
		}
		return result.events, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("store: wait for session write batch completion: %w", ctx.Err())
	}
}

func (s *SessionDB) writeEvent(ctx context.Context, event store.SessionEvent) (store.SessionEvent, error) {
	persisted, err := s.writeEventBatch(ctx, []store.SessionEvent{event})
	if err != nil {
		return store.SessionEvent{}, err
	}
	if len(persisted) != 1 {
		return store.SessionEvent{}, fmt.Errorf("store: persisted %d rows for one session event", len(persisted))
	}
	return persisted[0], nil
}

func (s *SessionDB) writeEventIfAbsent(
	ctx context.Context,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	if event.Timestamp.IsZero() {
		event.Timestamp = s.now()
	}
	persisted, inserted, err := s.writeProjectedEvents(ctx, []store.SessionEvent{event}, true)
	if err != nil {
		return store.SessionEvent{}, false, err
	}
	return persisted[0], inserted, nil
}

func sessionEventsHaveIdenticalIdentity(existing store.SessionEvent, candidate store.SessionEvent) bool {
	return existing.ID == candidate.ID && existing.SessionID == candidate.SessionID &&
		existing.TurnID == candidate.TurnID && existing.Type == candidate.Type &&
		existing.AgentName == candidate.AgentName && existing.Content == candidate.Content &&
		existing.Archived == candidate.Archived && existing.Timestamp.Equal(candidate.Timestamp)
}

func (s *SessionDB) writeEventBatch(
	ctx context.Context,
	events []store.SessionEvent,
) ([]store.SessionEvent, error) {
	if len(events) == 0 {
		return nil, nil
	}
	prepared := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		if strings.TrimSpace(event.ID) == "" {
			generatedID, err := store.NewID("ev")
			if err != nil {
				return nil, fmt.Errorf("store: generate session event id: %w", err)
			}
			event.ID = generatedID
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = s.now()
		}
		prepared = append(prepared, event)
	}
	persisted, err := coalesceSessionEventBatch(prepared)
	if err != nil {
		return nil, err
	}

	persisted, _, err = s.writeProjectedEvents(ctx, persisted, false)
	return persisted, err
}

func insertSessionEvent(
	ctx context.Context,
	tx *store.WriteTx,
	event store.SessionEvent,
	entryKey string,
) error {
	if err := sqlcgen.New(tx).InsertEvent(ctx, sqlcgen.InsertEventParams{
		ID:                 event.ID,
		Sequence:           event.Sequence,
		TurnID:             event.TurnID,
		Type:               event.Type,
		AgentName:          event.AgentName,
		Content:            event.Content,
		Archived:           boolToSQLite(event.Archived),
		Timestamp:          store.FormatTimestamp(event.Timestamp),
		TranscriptEntryKey: entryKey,
	}); err != nil {
		return fmt.Errorf("store: insert session event: %w", err)
	}
	return nil
}
