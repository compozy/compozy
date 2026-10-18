package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/compozy/compozy/internal/store"
)

type idempotentSessionEventAppender interface {
	AppendEventIfAbsent(context.Context, store.SessionEvent) (store.SessionEvent, bool, error)
}

func (m *Manager) appendDurableSessionEvent(
	ctx context.Context,
	sessionID string,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	for attempt := range 2 {
		persisted, inserted, err := m.appendDurableSessionEventAttempt(ctx, sessionID, event)
		if err == nil || !errors.Is(err, store.ErrClosed) || attempt == 1 {
			return persisted, inserted, err
		}
		if _, waitErr := m.waitForSessionFinalization(ctx, sessionID); waitErr != nil {
			return store.SessionEvent{}, false, fmt.Errorf(
				"session: wait for finalization for %q after closed event recorder: %w",
				sessionID,
				waitErr,
			)
		}
	}
	return store.SessionEvent{}, false, store.ErrClosed
}

func (m *Manager) appendDurableSessionEventAttempt(
	ctx context.Context,
	sessionID string,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	if active, ok := m.Get(sessionID); ok {
		return appendDurableSessionEventWithRecorder(ctx, active.recorderHandle(), event)
	}
	if _, err := m.waitForSessionFinalization(ctx, sessionID); err != nil {
		return store.SessionEvent{}, false, err
	}
	return m.appendStoredSessionEvent(ctx, sessionID, event)
}

// appendStoredSessionEvent is also used by the finalization owner after closing
// its recorder; that owner must not wait for its own finalization to finish.
func (m *Manager) appendStoredSessionEvent(
	ctx context.Context, sessionID string, event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	path := store.SessionDBFile(filepath.Join(m.homePaths.SessionsDir, sessionID))
	meta, err := m.readMetaWithContext(ctx, sessionID)
	if err != nil {
		return store.SessionEvent{}, false, err
	}
	owner, err := m.resolveStoredSessionOwner(ctx, &meta)
	if err != nil {
		return store.SessionEvent{}, false, fmt.Errorf(
			"session: resolve catalog owner for durable event %q: %w",
			sessionID,
			err,
		)
	}
	recorder, err := m.openStore(ctx, owner, path)
	if err != nil {
		return store.SessionEvent{}, false, err
	}
	persisted, inserted, appendErr := appendDurableSessionEventWithRecorder(ctx, recorder, event)
	closeErr := recorder.Close(ctx)
	if appendErr != nil || closeErr != nil {
		return store.SessionEvent{}, false, errors.Join(appendErr, closeErr)
	}
	return persisted, inserted, nil
}

func appendDurableSessionEventWithRecorder(
	ctx context.Context,
	recorder EventRecorder,
	event store.SessionEvent,
) (store.SessionEvent, bool, error) {
	if recorder == nil {
		return store.SessionEvent{}, false, store.ErrClosed
	}
	appender, ok := recorder.(idempotentSessionEventAppender)
	if !ok {
		return store.SessionEvent{}, false, fmt.Errorf(
			"session: event recorder %T does not support idempotent append",
			recorder,
		)
	}
	return appender.AppendEventIfAbsent(ctx, event)
}
