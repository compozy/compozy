package session

import (
	"context"
	"encoding/json"

	"strings"

	"github.com/compozy/compozy/internal/acp"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
)

const (
	sessionActorKindRoot     = "agent_root"
	sessionActorKindSubagent = "agent_subagent"
)

func (m *Manager) dispatchEventPreRecord(ctx context.Context, session *Session, event acp.AgentEvent, content string) {
	if m == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, session)

	_, err := m.hooks.events().DispatchEventPreRecord(ctx, hookspkg.EventPreRecordPayload{
		Event:          hookspkg.HookEventPreRecord,
		Timestamp:      hookTimestamp(m.now(), event.Timestamp),
		SessionContext: hookSessionContext(session),
		TurnID:         strings.TrimSpace(event.TurnID),
		RecordType:     strings.TrimSpace(event.Type),
		Content:        json.RawMessage(content),
	})
	if err != nil {
		m.warnHookDispatch(ctx, session, hookspkg.HookEventPreRecord, err)
	}
}

func (m *Manager) dispatchEventPostRecord(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	content string,
	sequence int64,
) {
	if m == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, session)

	_, err := m.hooks.events().DispatchEventPostRecord(ctx, hookspkg.EventPostRecordPayload{
		Event:          hookspkg.HookEventPostRecord,
		Timestamp:      hookTimestamp(m.now(), event.Timestamp),
		SessionContext: hookSessionContext(session),
		TurnID:         strings.TrimSpace(event.TurnID),
		RecordType:     strings.TrimSpace(event.Type),
		Sequence:       sequence,
		Content:        json.RawMessage(content),
	})
	if err != nil {
		m.warnHookDispatch(ctx, session, hookspkg.HookEventPostRecord, err)
	}
}

func (m *Manager) dispatchSessionMessagePersisted(
	ctx context.Context,
	session *Session,
	event acp.AgentEvent,
	persisted store.SessionEvent,
	content string,
) {
	if m == nil || strings.TrimSpace(event.Type) != acp.EventTypeAgentMessage {
		return
	}
	ctx = hookDispatchContext(ctx, m, session)
	rootSessionID, parentSessionID, actorKind, actorID := messagePersistedLineage(session)
	_, err := m.hooks.conversation().DispatchSessionMessagePersisted(ctx, hookspkg.SessionMessagePersistedPayload{
		Event:           hookspkg.HookSessionMessagePersisted,
		Timestamp:       hookTimestamp(m.now(), event.Timestamp),
		SessionContext:  hookSessionContext(session),
		TurnID:          strings.TrimSpace(event.TurnID),
		MessageID:       strings.TrimSpace(persisted.ID),
		MessageSeq:      persisted.Sequence,
		Role:            hookMessageRoleAssistant,
		Text:            event.Text,
		Raw:             cloneSessionRawMessage(event.Raw),
		Persisted:       json.RawMessage(content),
		RootSessionID:   rootSessionID,
		ParentSessionID: parentSessionID,
		ActorKind:       actorKind,
		ActorID:         actorID,
	})
	if err != nil {
		m.warnHookDispatch(ctx, session, hookspkg.HookSessionMessagePersisted, err)
	}
}

func messagePersistedLineage(session *Session) (string, string, string, string) {
	info := session.Info()
	if info == nil {
		return "", "", "", ""
	}
	rootSessionID := strings.TrimSpace(info.ID)
	parentSessionID := ""
	if info.Lineage != nil {
		if root := strings.TrimSpace(info.Lineage.RootSessionID); root != "" {
			rootSessionID = root
		}
		parentSessionID = strings.TrimSpace(info.Lineage.ParentSessionID)
	}
	actorKind := sessionActorKindRoot
	if info.Type == SessionTypeSpawned {
		actorKind = sessionActorKindSubagent
	}
	return rootSessionID, parentSessionID, actorKind, strings.TrimSpace(info.ID)
}
