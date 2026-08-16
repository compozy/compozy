package session

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/acp"
)

const sessionCatalogSubscriberBuffer = 64

// CatalogEventName identifies one named SSE delivery on the catalog stream.
type CatalogEventName string

const (
	CatalogEventNameChanged   CatalogEventName = "session_catalog_changed"
	CatalogEventNameAttention CatalogEventName = "session_attention_changed"
)

// CatalogEventKind identifies a durable session-catalog mutation.
type CatalogEventKind string

const (
	// CatalogEventUpserted reports a created or changed catalog row.
	CatalogEventUpserted CatalogEventKind = "upserted"
	// CatalogEventDeleted reports a removed catalog row.
	CatalogEventDeleted CatalogEventKind = "deleted"
)

// CatalogEvent identifies the workspace-scoped catalog snapshot to reconcile.
type CatalogEvent struct {
	Name        CatalogEventName
	Kind        CatalogEventKind
	WorkspaceID string
	SessionID   string
	Attention   *AttentionEvent
}

type sessionCatalogBroadcaster struct {
	mu          sync.Mutex
	subscribers map[*sessionCatalogSubscriber]struct{}
}

type sessionCatalogSubscriber struct {
	ch        chan CatalogEvent
	closeOnce sync.Once
}

func newSessionCatalogBroadcaster() *sessionCatalogBroadcaster {
	return &sessionCatalogBroadcaster{subscribers: make(map[*sessionCatalogSubscriber]struct{})}
}

func (b *sessionCatalogBroadcaster) subscribe(
	ctx context.Context,
) (<-chan CatalogEvent, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("session: catalog stream context is required")
	}
	subscriber := &sessionCatalogSubscriber{
		ch: make(chan CatalogEvent, sessionCatalogSubscriberBuffer),
	}
	b.mu.Lock()
	b.subscribers[subscriber] = struct{}{}
	b.mu.Unlock()

	cancel := sync.OnceFunc(func() {
		b.mu.Lock()
		delete(b.subscribers, subscriber)
		b.mu.Unlock()
		subscriber.close()
	})
	return subscriber.ch, cancel, nil
}

func (b *sessionCatalogBroadcaster) publish(event CatalogEvent) {
	if strings.TrimSpace(event.WorkspaceID) == "" || strings.TrimSpace(event.SessionID) == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for subscriber := range b.subscribers {
		select {
		case subscriber.ch <- event:
		default:
			delete(b.subscribers, subscriber)
			subscriber.close()
		}
	}
}

func (s *sessionCatalogSubscriber) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() { close(s.ch) })
}

// SubscribeSessionCatalogEvents registers a request-owned catalog wake stream.
// The caller must invoke the returned cancel function on every exit.
func (m *Manager) SubscribeSessionCatalogEvents(
	ctx context.Context,
) (<-chan CatalogEvent, func(), error) {
	if m == nil {
		return nil, nil, errors.New("session: manager is required")
	}
	m.catalogEventsMu.Lock()
	if m.catalogEvents == nil {
		m.catalogEvents = newSessionCatalogBroadcaster()
	}
	broadcaster := m.catalogEvents
	m.catalogEventsMu.Unlock()
	return broadcaster.subscribe(ctx)
}

func (m *Manager) publishSessionCatalogEvent(event CatalogEvent) {
	if m == nil {
		return
	}
	m.catalogEventsMu.Lock()
	broadcaster := m.catalogEvents
	m.catalogEventsMu.Unlock()
	if broadcaster != nil {
		broadcaster.publish(event)
	}
}

func (m *Manager) publishSessionCatalogWakeForEvent(session *Session, event acp.AgentEvent) {
	if session == nil || !catalogWakeEventType(event.Type) {
		return
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, session.Info()))
}

func sessionCatalogEventFromInfo(kind CatalogEventKind, info *Info) CatalogEvent {
	if !isPublicSessionCatalogInfo(info) {
		return CatalogEvent{}
	}
	return CatalogEvent{
		Name:        CatalogEventNameChanged,
		Kind:        kind,
		WorkspaceID: strings.TrimSpace(info.WorkspaceID),
		SessionID:   strings.TrimSpace(info.ID),
	}
}

func catalogWakeEventType(eventType string) bool {
	switch eventType {
	case acp.EventTypePermission, acp.EventTypeClarify, acp.EventTypeDone, acp.EventTypeError:
		return true
	default:
		return false
	}
}

func sessionAttentionCatalogEvent(event AttentionEvent) CatalogEvent {
	cloned := event
	return CatalogEvent{
		Name:        CatalogEventNameAttention,
		WorkspaceID: strings.TrimSpace(event.WorkspaceID),
		SessionID:   strings.TrimSpace(event.SessionID),
		Attention:   &cloned,
	}
}
