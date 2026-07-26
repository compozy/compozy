package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

func (m *Manager) dispatchEventPreRecord(ctx context.Context, session *Session, event acp.AgentEvent, content string) {
	if m == nil {
		return
	}
	ctx = hookDispatchContext(ctx, m, session)

	_, err := m.hooks.events().DispatchEventPreRecord(ctx, hookspkg.EventPreRecordPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookEventPreRecord,
			Timestamp: hookTimestamp(m.now(), event.Timestamp),
		},
		SessionContext: hookSessionContext(session),
		TurnContext:    hookspkg.TurnContext{TurnID: strings.TrimSpace(event.TurnID)},
		RecordType:     strings.TrimSpace(event.Type),
		Content:        json.RawMessage(content),
	})
	if err != nil {
		m.warnHookDispatch(ctx, session, hookspkg.HookEventPreRecord, err)
	}
}

func (m *Manager) runContextCompaction(
	ctx context.Context,
	session *Session,
	turnID string,
	reason string,
	strategy string,
	summary string,
	contextBlocks []hookspkg.ContextBlock,
	compact func(context.Context, hookspkg.ContextPreCompactPayload) (hookspkg.ContextPostCompactPayload, error),
) (hookspkg.ContextPostCompactPayload, error) {
	if compact == nil {
		return hookspkg.ContextPostCompactPayload{}, errors.New("session: context compactor is required")
	}

	now := time.Now().UTC
	if m != nil {
		now = m.now
	}
	ctx = hookDispatchContext(ctx, m, session)

	prePayload := hookspkg.ContextPreCompactPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookContextPreCompact,
			Timestamp: now(),
		},
		SessionContext: hookSessionContext(session),
		TurnContext:    hookspkg.TurnContext{TurnID: strings.TrimSpace(turnID)},
		Reason:         strings.TrimSpace(reason),
		Strategy:       strings.TrimSpace(strategy),
		Summary:        strings.TrimSpace(summary),
		ContextBlocks:  cloneSessionContextBlocks(contextBlocks),
	}

	var err error
	if m != nil {
		prePayload, err = m.hooks.compaction().DispatchContextPreCompact(ctx, prePayload)
		if err != nil {
			return hookspkg.ContextPostCompactPayload{}, fmt.Errorf("session: dispatch context.pre_compact: %w", err)
		}
	}

	postPayload, err := compact(ctx, prePayload)
	if err != nil {
		return hookspkg.ContextPostCompactPayload{}, err
	}

	postPayload.Event = hookspkg.HookContextPostCompact
	if postPayload.Timestamp.IsZero() {
		postPayload.Timestamp = now()
	}
	if strings.TrimSpace(postPayload.SessionID) == "" {
		postPayload.SessionContext = prePayload.SessionContext
	}
	if strings.TrimSpace(postPayload.TurnID) == "" {
		postPayload.TurnContext = prePayload.TurnContext
	}
	if strings.TrimSpace(postPayload.Reason) == "" {
		postPayload.Reason = prePayload.Reason
	}
	if strings.TrimSpace(postPayload.Strategy) == "" {
		postPayload.Strategy = prePayload.Strategy
	}
	if postPayload.ContextBlocks == nil {
		postPayload.ContextBlocks = cloneSessionContextBlocks(prePayload.ContextBlocks)
	}

	if m != nil {
		if _, err := m.hooks.compaction().DispatchContextPostCompact(ctx, postPayload); err != nil {
			m.warnHookDispatch(ctx, session, hookspkg.HookContextPostCompact, err)
		}
	}

	return postPayload, nil
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
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookEventPostRecord,
			Timestamp: hookTimestamp(m.now(), event.Timestamp),
		},
		SessionContext: hookSessionContext(session),
		TurnContext:    hookspkg.TurnContext{TurnID: strings.TrimSpace(event.TurnID)},
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
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionMessagePersisted,
			Timestamp: hookTimestamp(m.now(), event.Timestamp),
		},
		SessionContext:  hookSessionContext(session),
		TurnContext:     hookspkg.TurnContext{TurnID: strings.TrimSpace(event.TurnID)},
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
	actorKind := "agent_root"
	if parentSessionID != "" {
		actorKind = "agent_subagent"
	}
	return rootSessionID, parentSessionID, actorKind, strings.TrimSpace(info.ID)
}
