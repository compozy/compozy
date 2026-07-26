package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/acp"
	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
)

const sessionEventSubscriberBuffer = 64

type sessionEventSubscriptionMode uint8

const (
	sessionEventSubscriptionAfterSequence sessionEventSubscriptionMode = iota
	sessionEventSubscriptionWakeOnly
)

type sessionEventBroadcaster struct {
	mu          sync.Mutex
	subscribers map[string]map[*sessionEventSubscriber]struct{}
}

type sessionEventSubscriber struct {
	sessionID string
	after     int64
	mode      sessionEventSubscriptionMode
	ch        chan store.SessionEvent
	cancel    context.CancelFunc
	closeOnce sync.Once
}

func newSessionEventBroadcaster() *sessionEventBroadcaster {
	return &sessionEventBroadcaster{
		subscribers: make(map[string]map[*sessionEventSubscriber]struct{}),
	}
}

func (b *sessionEventBroadcaster) subscribe(
	ctx context.Context,
	sessionID string,
	afterSequence int64,
	mode sessionEventSubscriptionMode,
) (<-chan store.SessionEvent, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("session: stream subscription context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil, nil, errors.New("session: stream subscription session id is required")
	}
	if afterSequence < 0 {
		return nil, nil, fmt.Errorf("session: stream subscription cursor must be non-negative: %d", afterSequence)
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &sessionEventSubscriber{
		sessionID: target,
		after:     afterSequence,
		mode:      mode,
		ch:        make(chan store.SessionEvent, sessionEventSubscriberBuffer),
		cancel:    cancel,
	}

	b.mu.Lock()
	if b.subscribers == nil {
		b.subscribers = make(map[string]map[*sessionEventSubscriber]struct{})
	}
	subs := b.subscribers[target]
	if subs == nil {
		subs = make(map[*sessionEventSubscriber]struct{})
		b.subscribers[target] = subs
	}
	subs[sub] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			cancel()
			b.remove(sub, true)
		})
	}

	go func() {
		<-subCtx.Done()
		unsubscribe()
	}()

	return sub.ch, unsubscribe, nil
}

func (b *sessionEventBroadcaster) publish(event store.SessionEvent) bool {
	target := strings.TrimSpace(event.SessionID)
	if target == "" {
		return false
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[target]
	if len(subs) == 0 {
		return false
	}

	overflow := false
	for sub := range subs {
		if sub.mode == sessionEventSubscriptionAfterSequence && event.Sequence <= sub.after {
			continue
		}
		select {
		case sub.ch <- event:
			if sub.mode == sessionEventSubscriptionAfterSequence {
				sub.after = event.Sequence
			}
		default:
			overflow = true
			delete(subs, sub)
			sub.cancel()
			sub.close()
		}
	}
	if len(subs) == 0 {
		delete(b.subscribers, target)
	}
	return overflow
}

func (b *sessionEventBroadcaster) remove(sub *sessionEventSubscriber, closeChannel bool) {
	if sub == nil {
		return
	}

	b.mu.Lock()
	subs := b.subscribers[sub.sessionID]
	if subs != nil {
		delete(subs, sub)
		if len(subs) == 0 {
			delete(b.subscribers, sub.sessionID)
		}
	}
	b.mu.Unlock()

	if closeChannel {
		sub.close()
	}
}

func (sub *sessionEventSubscriber) close() {
	if sub == nil {
		return
	}
	sub.closeOnce.Do(func() {
		close(sub.ch)
	})
}

// SubscribeSessionEvents registers an in-process stream subscriber for
// already-persisted session events. Callers must read catch-up events after
// registration and deduplicate by sequence.
func (m *Manager) SubscribeSessionEvents(
	ctx context.Context,
	sessionID string,
	afterSequence int64,
) (<-chan store.SessionEvent, func(), error) {
	if m == nil {
		return nil, nil, errors.New("session: manager is required")
	}
	m.streamEventsMu.Lock()
	if m.streamEvents == nil {
		m.streamEvents = newSessionEventBroadcaster()
	}
	broadcaster := m.streamEvents
	m.streamEventsMu.Unlock()

	ch, cancel, err := broadcaster.subscribe(
		ctx,
		sessionID,
		afterSequence,
		sessionEventSubscriptionAfterSequence,
	)
	if err != nil {
		return nil, nil, err
	}
	m.emitStreamDiagnostic(ctx, strings.TrimSpace(sessionID), eventspkg.SessionStreamSubscribed, afterSequence)
	return ch, cancel, nil
}

// SubscribeSessionEventWakes registers an unfiltered projection wake subscriber.
func (m *Manager) SubscribeSessionEventWakes(
	ctx context.Context,
	sessionID string,
) (<-chan store.SessionEvent, func(), error) {
	if m == nil {
		return nil, nil, errors.New("session: manager is required")
	}
	m.streamEventsMu.Lock()
	if m.streamEvents == nil {
		m.streamEvents = newSessionEventBroadcaster()
	}
	broadcaster := m.streamEvents
	m.streamEventsMu.Unlock()

	ch, cancel, err := broadcaster.subscribe(ctx, sessionID, 0, sessionEventSubscriptionWakeOnly)
	if err != nil {
		return nil, nil, err
	}
	m.emitStreamDiagnostic(ctx, strings.TrimSpace(sessionID), eventspkg.SessionStreamSubscribed, 0)
	return ch, cancel, nil
}

func (m *Manager) publishSessionEvent(ctx context.Context, session *Session, event store.SessionEvent) {
	if m == nil || session == nil {
		return
	}
	m.publishSessionEventByID(ctx, session.ID, event)
}

func (m *Manager) publishSessionEventByID(ctx context.Context, sessionID string, event store.SessionEvent) {
	if m == nil {
		return
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}
	event.SessionID = target
	m.streamEventsMu.Lock()
	broadcaster := m.streamEvents
	m.streamEventsMu.Unlock()
	if broadcaster == nil || !broadcaster.publish(event) {
		return
	}
	m.emitStreamDiagnostic(ctx, target, eventspkg.SessionStreamOverflowFallback, event.Sequence)
}

func (m *Manager) emitStreamDiagnostic(ctx context.Context, sessionID string, eventType string, sequence int64) {
	target := strings.TrimSpace(sessionID)
	if target == "" || strings.TrimSpace(eventType) == "" {
		return
	}
	payload := map[string]any{
		sessionIDFieldKey: target,
		"sequence":        sequence,
	}
	if info, err := m.Status(ctx, target); err == nil && info != nil {
		payload[workspaceIDFieldKey] = info.WorkspaceID
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		if m.logger != nil {
			m.logger.WarnContext(ctx, "session: marshal stream diagnostic failed", "event", eventType, "error", err)
		}
		return
	}
	if m.notifier == nil {
		return
	}
	m.notifier.OnAgentEvent(ctx, target, acp.AgentEvent{
		Type: eventType,
		Raw:  raw,
	})
}
