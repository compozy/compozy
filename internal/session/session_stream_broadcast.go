package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/acp"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
)

const sessionEventSubscriberBuffer = 64

type sessionEventSubscriptionMode uint8

const (
	sessionEventSubscriptionAfterSequence sessionEventSubscriptionMode = iota
	sessionEventSubscriptionWakeOnly
)

type sessionEventBroadcaster struct {
	mu      sync.Mutex
	streams map[string]*acp.LiveBroadcast
}

func newSessionEventBroadcaster() *sessionEventBroadcaster {
	return &sessionEventBroadcaster{streams: make(map[string]*acp.LiveBroadcast)}
}

func (b *sessionEventBroadcaster) subscribe(
	ctx context.Context, sessionID string, afterSequence int64, mode sessionEventSubscriptionMode,
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
	b.mu.Lock()
	stream := b.streams[target]
	if stream == nil {
		stream = acp.NewLiveBroadcast(sessionEventSubscriberBuffer)
		b.streams[target] = stream
	}
	ch, remove := stream.Subscribe(uint64(afterSequence), mode == sessionEventSubscriptionWakeOnly)
	b.mu.Unlock()
	unsubscribe := sync.OnceFunc(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		remove()
		if stream.Stats().Subscribers == 0 && b.streams[target] == stream {
			delete(b.streams, target)
		}
	})
	stop := context.AfterFunc(ctx, unsubscribe)
	return ch, func() { stop(); unsubscribe() }, nil
}

func (b *sessionEventBroadcaster) publish(event store.SessionEvent) bool {
	if event.Sequence < 0 {
		return false
	}
	b.mu.Lock()
	stream := b.streams[strings.TrimSpace(event.SessionID)]
	b.mu.Unlock()
	return stream != nil && stream.Publish(uint64(event.Sequence), event)
}

func (b *sessionEventBroadcaster) stats(sessionID string) acp.BroadcastStats {
	b.mu.Lock()
	stream := b.streams[sessionID]
	b.mu.Unlock()
	if stream == nil {
		return acp.BroadcastStats{}
	}
	return stream.Stats()
}

// TransportDeliveryStats separates durable ingestion from watcher delivery.
type TransportDeliveryStats struct {
	Ingest    acp.IngestStats
	Broadcast acp.BroadcastStats
}

// TransportStats reports the ephemeral queues used by the active session.
func (m *Manager) TransportStats(sessionID string) (TransportDeliveryStats, error) {
	session, ok := m.Get(sessionID)
	if !ok {
		return TransportDeliveryStats{}, ErrSessionNotFound
	}
	var stats TransportDeliveryStats
	if proc := session.processHandle(); proc != nil {
		if source, ok := proc.native.(interface{ IngestStats() acp.IngestStats }); ok {
			stats.Ingest = source.IngestStats()
		}
	}
	m.streamEventsMu.Lock()
	broadcaster := m.streamEvents
	m.streamEventsMu.Unlock()
	if broadcaster != nil {
		stats.Broadcast = broadcaster.stats(sessionID)
	}
	return stats, nil
}

// SubscribeSessionEvents registers an in-process stream subscriber for
// already-persisted session events. Callers must read catch-up events after
// registration and deduplicate by sequence.
func (m *Manager) SubscribeSessionEvents(
	ctx context.Context,
	sessionID string,
	afterSequence int64,
) (<-chan store.SessionEvent, func(), error) {
	ch, cancel, err := m.subscribePersistedSessionEvents(
		ctx,
		sessionID,
		afterSequence,
	)
	if err != nil {
		return nil, nil, err
	}
	m.emitStreamDiagnostic(ctx, strings.TrimSpace(sessionID), eventspkg.SessionStreamSubscribed, afterSequence, "")
	return ch, cancel, nil
}

func (m *Manager) subscribePersistedSessionEvents(
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

	return broadcaster.subscribe(
		ctx,
		sessionID,
		afterSequence,
		sessionEventSubscriptionAfterSequence,
	)
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
	m.emitStreamDiagnostic(ctx, strings.TrimSpace(sessionID), eventspkg.SessionStreamSubscribed, 0, "")
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
	m.emitStreamDiagnostic(ctx, target, eventspkg.SessionStreamOverflowFallback, event.Sequence, event.TurnID)
	m.emitStreamDiagnostic(ctx, target, eventspkg.StreamConsumerDegraded, event.Sequence, event.TurnID)
}

func (m *Manager) emitStreamDiagnostic(
	ctx context.Context, sessionID string, eventType string, sequence int64, turnID string,
) {
	target := strings.TrimSpace(sessionID)
	if target == "" || strings.TrimSpace(eventType) == "" {
		return
	}
	payload := map[string]any{
		sessionIDFieldKey: target,
		"sequence":        sequence,
		"turn_id":         turnID,
		"actor_kind":      sessionSystemActorKind,
		"actor_id":        sessionDaemonActorID,
	}
	info, err := m.Status(ctx, target)
	if err != nil || info == nil {
		if m.logger != nil {
			m.logger.ErrorContext(
				ctx,
				"session: stream diagnostic requires session identity",
				"session_id",
				target,
				"error",
				err,
			)
		}
		return
	}
	payload[workspaceIDFieldKey] = info.WorkspaceID
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
	m.notifyAgentEventFromInfo(ctx, info, acp.AgentEvent{
		Type: eventType, TurnID: turnID,
		EventCorrelation: store.EventCorrelation{ActorKind: sessionSystemActorKind, ActorID: sessionDaemonActorID},
		Raw:              raw,
	})
}
