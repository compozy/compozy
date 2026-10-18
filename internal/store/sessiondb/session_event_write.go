package sessiondb

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
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
) (store.SessionEvent, error) {
	if s == nil {
		return store.SessionEvent{}, errors.New("store: session database is required")
	}
	if ctx == nil {
		return store.SessionEvent{}, errors.New("store: append event context is required")
	}
	normalized, err := s.normalizeSessionEvent(event)
	if err != nil {
		return store.SessionEvent{}, err
	}
	normalized.ID = strings.TrimSpace(normalized.ID)
	if normalized.ID == "" {
		return store.SessionEvent{}, errors.New("store: append-if-absent event id is required")
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
	return s.enqueueWritePersisted(ctx, sessionWriteRequest{
		ctx:    ctx,
		kind:   sessionWriteEvent,
		event:  normalized,
		result: make(chan sessionWriteResult, 1),
	})
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
) (store.SessionEvent, error) {
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()

	if s.state.Load() != sessionStateOpen {
		return store.SessionEvent{}, store.ErrClosed
	}

	select {
	case s.writeCh <- req:
	case <-ctx.Done():
		return store.SessionEvent{}, fmt.Errorf("store: enqueue session write: %w", ctx.Err())
	}

	select {
	case result := <-req.result:
		if result.err != nil {
			return store.SessionEvent{}, result.err
		}
		return result.event, nil
	case <-ctx.Done():
		return store.SessionEvent{}, fmt.Errorf("store: wait for session write completion: %w", ctx.Err())
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
) (store.SessionEvent, error) {
	if event.Timestamp.IsZero() {
		event.Timestamp = s.now()
	}
	var persisted store.SessionEvent
	err := s.writeWithTranscriptFolds(ctx, func(ctx context.Context, tx *store.WriteTx) error {
		row, err := sqlcgen.New(tx).GetEventByID(ctx, event.ID)
		if err == nil {
			existing, mapErr := sessionEventFromSQLC(
				row.ID,
				row.Sequence,
				row.TurnID,
				row.Type,
				row.AgentName,
				row.Content,
				row.Archived,
				row.Timestamp,
				s.owner.SessionID,
			)
			if mapErr != nil {
				return mapErr
			}
			if !sessionEventsHaveIdenticalIdentity(existing, event) {
				return fmt.Errorf("%w: %s", ErrEventIdentityCollision, event.ID)
			}
			persisted = existing
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		sequence, err := nextEventSequence(ctx, tx)
		if err != nil {
			return err
		}
		event.Sequence = sequence
		if event.Archived {
			if err := insertSessionEvent(ctx, tx, event, ""); err != nil {
				return err
			}
			persisted = event
			return nil
		}
		state, err := loadProjectionState(ctx, tx)
		if err != nil {
			return err
		}
		projector, err := transcript.NewProjector(state, projectionSQLResolver{db: tx})
		if err != nil {
			return err
		}
		assignment, err := projector.Assign(ctx, event)
		if err != nil {
			return err
		}
		if err := insertSessionEvent(ctx, tx, event, assignment.Entry.Key); err != nil {
			return err
		}
		affected := map[string]struct{}{assignment.Entry.Key: {}}
		for _, key := range assignment.CompletedKeys {
			affected[key] = struct{}{}
		}
		if err := s.persistIncrementalTranscriptProjection(ctx, tx, projector, affected); err != nil {
			return err
		}
		persisted = event
		return nil
	})
	if err != nil {
		return store.SessionEvent{}, err
	}
	return persisted, nil
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

	if err := s.writeWithTranscriptFolds(ctx, func(ctx context.Context, tx *store.WriteTx) error {
		state, err := loadProjectionState(ctx, tx)
		if err != nil {
			return err
		}
		projector, err := transcript.NewProjector(state, projectionSQLResolver{db: tx})
		if err != nil {
			return err
		}
		nextSequence, err := nextEventSequence(ctx, tx)
		if err != nil {
			return err
		}
		affected := make(map[string]struct{})
		for idx := range persisted {
			persisted[idx].Sequence = nextSequence + int64(idx)
			if persisted[idx].Archived {
				if err := insertSessionEvent(ctx, tx, persisted[idx], ""); err != nil {
					return err
				}
				continue
			}
			assignment, assignErr := projector.Assign(ctx, persisted[idx])
			if assignErr != nil {
				return assignErr
			}
			if err := insertSessionEvent(ctx, tx, persisted[idx], assignment.Entry.Key); err != nil {
				return err
			}
			affected[assignment.Entry.Key] = struct{}{}
			for _, key := range assignment.CompletedKeys {
				affected[key] = struct{}{}
			}
		}
		return s.persistIncrementalTranscriptProjection(ctx, tx, projector, affected)
	}); err != nil {
		return nil, err
	}

	return persisted, nil
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

func (s *SessionDB) persistIncrementalTranscriptProjection(
	ctx context.Context,
	tx *store.WriteTx,
	projector *transcript.Projector,
	affected map[string]struct{},
) error {
	identities := make([]transcript.EntryIdentity, 0, len(affected))
	for key := range affected {
		identity, ok := projector.Identity(key)
		if !ok {
			return fmt.Errorf("%w: missing affected identity %q", transcript.ErrProjectionCorrupt, key)
		}
		identities = append(identities, identity)
	}
	slices.SortFunc(identities, func(left, right transcript.EntryIdentity) int {
		return cmp.Compare(left.StartSequence, right.StartSequence)
	})

	// Each append rewrites only the independently rebuildable entries it affects.
	// Assistant entries extend a cached fold, so a long turn is not replayed
	// from its first event on every append.
	generation := projector.State().Generation
	for _, identity := range identities {
		candidate := identity
		if candidate.MessageID == "" {
			candidate.MessageID = candidate.BaseMessageID
		}
		entry, err := s.projectAssignedEntry(ctx, tx, generation, candidate)
		if err != nil {
			return err
		}
		if entry != nil && identity.MessageID == "" {
			identity.MessageID, err = allocateProjectionMessageID(
				ctx,
				tx,
				identity.Key,
				identity.BaseMessageID,
			)
			if err != nil {
				return err
			}
			entry, err = s.projectAssignedEntry(ctx, tx, generation, identity)
			if err != nil {
				return err
			}
		}
		if err := persistProjectionEntry(ctx, tx, identity, entry); err != nil {
			return err
		}
	}
	for toolKey, entryKey := range projector.ToolRoutes() {
		if err := sqlcgen.New(tx).UpsertTranscriptToolRoute(ctx, sqlcgen.UpsertTranscriptToolRouteParams{
			ToolKey:  toolKey,
			EntryKey: entryKey,
		}); err != nil {
			return fmt.Errorf("store: upsert transcript tool route %q: %w", toolKey, err)
		}
	}
	return persistProjectionState(ctx, tx, projector.State())
}

func (s *SessionDB) projectAssignedEntry(
	ctx context.Context,
	tx *store.WriteTx,
	generation int64,
	identity transcript.EntryIdentity,
) (*transcript.Entry, error) {
	if identity.Kind == transcript.EntryKindAssistant {
		return s.projectAssistantEntry(ctx, tx, generation, identity)
	}
	events, err := loadAssignedEvents(ctx, tx, s.owner.SessionID, identity.Key)
	if err != nil {
		return nil, err
	}
	entry, err := transcript.ProjectAssignedEntry(events, identity)
	if err != nil {
		return nil, fmt.Errorf("store: project transcript entry %q: %w", identity.Key, err)
	}
	return entry, nil
}
