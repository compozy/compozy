package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

const (
	sessionCatalogSubscriberBuffer = 64
	sessionCatalogReplayLimit      = 32
)

var ErrCatalogScopeInvalid = errors.New("session: catalog scope is invalid")

// CatalogScope selects one explicit workspace or the labeled aggregate stream.
type CatalogScope struct {
	ReadScope     store.ReadScope
	WorkspaceID   string
	AllWorkspaces bool
	Replay        bool
	ReplayAfter   int64
}

func (s CatalogScope) Validate() error {
	if err := s.ReadScope.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrCatalogScopeInvalid, err)
	}
	hasWorkspace := strings.TrimSpace(s.WorkspaceID) != ""
	if hasWorkspace == s.AllWorkspaces {
		return fmt.Errorf("%w: choose exactly one workspace or all workspaces", ErrCatalogScopeInvalid)
	}
	if s.ReplayAfter < 0 || (!s.Replay && s.ReplayAfter != 0) {
		return fmt.Errorf("%w: replay cursor must be a supplied non-negative sequence", ErrCatalogScopeInvalid)
	}
	return nil
}

func (s CatalogScope) matches(event CatalogEvent) bool {
	if event.Kind == CatalogEventWorkspaceLayout {
		// The workspace layout spans every workspace, so only aggregate
		// subscribers of the owning profile reconcile it.
		return s.AllWorkspaces && s.ReadScope.Matches(event.ProfileID)
	}
	workspaceMatches := (s.WorkspaceID == workspaceaccess.UserSelector && event.WorkspaceID == "") || s.AllWorkspaces ||
		strings.TrimSpace(event.WorkspaceID) == strings.TrimSpace(s.WorkspaceID)
	return workspaceMatches && s.ReadScope.Matches(event.ProfileID)
}

// CatalogEventName identifies one named SSE delivery on the catalog stream.
type CatalogEventName string

const (
	CatalogEventNameChanged              CatalogEventName = "session_catalog_changed"
	CatalogEventNameAttention            CatalogEventName = "session_attention_changed"
	CatalogEventNameOperatorNotification CatalogEventName = "operator_notification"
)

// CatalogEventKind identifies a durable session-catalog mutation.
type CatalogEventKind string

const (
	// CatalogEventUpserted reports a created or changed catalog row.
	CatalogEventUpserted CatalogEventKind = "upserted"
	// CatalogEventDeleted reports a removed catalog row.
	CatalogEventDeleted CatalogEventKind = "deleted"
	// CatalogEventLayout reports a workspace folder or manual-order change.
	// It carries no session id: clients reread folders and the catalog.
	CatalogEventLayout CatalogEventKind = "layout"
	// CatalogEventWorkspaceLayout reports a workspace folder or manual
	// workspace-order change. It names only the owning profile: All-projects
	// clients reread the workspace layout.
	CatalogEventWorkspaceLayout CatalogEventKind = "workspace_layout"
)

// CatalogEvent identifies the workspace-scoped catalog snapshot to reconcile.
type CatalogEvent struct {
	ExecutionLocationChanged bool
	Sequence                 int64
	Name                     CatalogEventName
	Kind                     CatalogEventKind
	ProfileID                string
	ProfileName              string
	WorkspaceID              string
	SessionID                string
	Attention                *AttentionEvent
	OperatorNotification     *OperatorNotification
}

type sessionCatalogBroadcaster struct {
	mu           sync.Mutex
	subscribers  map[*sessionCatalogSubscriber]struct{}
	nextSequence int64
	replay       []CatalogEvent
}

type sessionCatalogSubscriber struct {
	ch        chan CatalogEvent
	scope     CatalogScope
	closeOnce sync.Once
}

func newSessionCatalogBroadcaster() *sessionCatalogBroadcaster {
	return &sessionCatalogBroadcaster{subscribers: make(map[*sessionCatalogSubscriber]struct{})}
}

func (b *sessionCatalogBroadcaster) subscribe(
	ctx context.Context,
	scope CatalogScope,
) (<-chan CatalogEvent, func(), error) {
	if ctx == nil {
		return nil, nil, errors.New("session: catalog stream context is required")
	}
	if err := scope.Validate(); err != nil {
		return nil, nil, err
	}
	subscriber := &sessionCatalogSubscriber{
		ch:    make(chan CatalogEvent, sessionCatalogSubscriberBuffer),
		scope: scope,
	}
	b.mu.Lock()
	if scope.Replay {
		for _, event := range b.replay {
			if event.Sequence > scope.ReplayAfter && scope.matches(event) {
				subscriber.ch <- event
			}
		}
	}
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

func (b *sessionCatalogBroadcaster) publish(event CatalogEvent) int {
	if strings.TrimSpace(event.ProfileID) == "" {
		return 0
	}
	if strings.TrimSpace(event.SessionID) == "" && event.Kind != CatalogEventWorkspaceLayout &&
		(event.Kind != CatalogEventLayout || strings.TrimSpace(event.WorkspaceID) == "") {
		return 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextSequence++
	event.Sequence = b.nextSequence
	b.replay = append(b.replay, event)
	if len(b.replay) > sessionCatalogReplayLimit {
		b.replay = append([]CatalogEvent(nil), b.replay[len(b.replay)-sessionCatalogReplayLimit:]...)
	}
	delivered := 0
	for subscriber := range b.subscribers {
		if !subscriber.scope.matches(event) {
			continue
		}
		select {
		case subscriber.ch <- event:
			delivered++
		default:
			delete(b.subscribers, subscriber)
			subscriber.close()
		}
	}
	return delivered
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
	scope CatalogScope,
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
	return broadcaster.subscribe(ctx, scope)
}

func (m *Manager) publishSessionCatalogEvent(event CatalogEvent) {
	m.publishSessionCatalogEventCount(event)
}

func (m *Manager) publishSessionCatalogEventCount(event CatalogEvent) int {
	if m == nil {
		return 0
	}
	m.catalogEventsMu.Lock()
	broadcaster := m.catalogEvents
	m.catalogEventsMu.Unlock()
	if broadcaster != nil {
		return broadcaster.publish(event)
	}
	return 0
}

func (m *Manager) publishSessionCatalogWakeForEvent(session *Session, event acp.AgentEvent) {
	if session == nil || !catalogWakeEventType(event.Type) {
		return
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, session.Info()))
}

// PublishSessionCatalogChanged wakes catalog subscribers for one session whose
// daemon-owned projection (such as its active monitors) changed outside the
// session record. An unknown or deleted session publishes nothing.
func (m *Manager) PublishSessionCatalogChanged(ctx context.Context, sessionID string) {
	if m == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	info, err := m.Status(ctx, sessionID)
	if err != nil || info == nil {
		return
	}
	m.publishSessionCatalogEvent(sessionCatalogEventFromInfo(CatalogEventUpserted, info))
}

func sessionCatalogEventFromInfo(kind CatalogEventKind, info *Info) CatalogEvent {
	if !isPublicSessionCatalogInfo(info) {
		return CatalogEvent{}
	}
	return CatalogEvent{
		Name:        CatalogEventNameChanged,
		Kind:        kind,
		ProfileID:   strings.TrimSpace(info.ProfileID),
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
		ProfileID:   strings.TrimSpace(event.ProfileID),
		WorkspaceID: strings.TrimSpace(event.WorkspaceID),
		SessionID:   strings.TrimSpace(event.SessionID),
		Attention:   &cloned,
	}
}
