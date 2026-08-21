package session

import (
	"context"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) notifyAgentEvent(ctx context.Context, session *Session, event any) {
	if m == nil || m.notifier == nil || session == nil {
		return
	}

	enriched := m.enrichNotifiedAgentEvent(session, event)
	if aware, ok := m.notifier.(AgentEventNotifier); ok {
		aware.OnAgentEventForSession(ctx, session, enriched)
		return
	}

	m.notifier.OnAgentEvent(ctx, session.ID, enriched)
}

func (m *Manager) notifyAgentEventFromInfo(ctx context.Context, info *Info, event any) {
	if info == nil {
		return
	}
	m.notifyAgentEvent(ctx, NotificationSessionFromInfo(info), event)
}

// NotificationSessionFromInfo builds a detached lifecycle snapshot for notifier
// consumers that need the same immutable read model as a managed Session.
func NotificationSessionFromInfo(info *Info) *Session {
	if info == nil {
		return nil
	}
	return &Session{
		ID:                       info.ID,
		ProfileID:                info.ProfileID,
		Name:                     info.Name,
		AgentName:                info.AgentName,
		Provider:                 info.Provider,
		Model:                    info.Model,
		RuntimeStatus:            info.RuntimeStatus,
		RuntimeTransition:        info.RuntimeTransition,
		RuntimeFailure:           info.RuntimeFailure,
		SelectedRuntime:          cloneRuntimeSelection(info.SelectedRuntime),
		RuntimeSelectionRevision: info.RuntimeSelectionRevision,
		EffectivePermissions:     info.EffectivePermissions,
		WorkspaceID:              info.WorkspaceID,
		Workspace:                info.Workspace,
		WorktreeID:               info.WorktreeID,
		Type:                     info.Type,
		Lineage:                  store.CloneSessionLineage(info.Lineage),
		State:                    info.State,
		stopReason:               info.StopReason,
		stopDetail:               info.StopDetail,
		failure:                  store.CloneSessionFailure(info.Failure),
		ACPSessionID:             info.ACPSessionID,
		SoulSnapshotID:           info.SoulSnapshotID,
		SoulDigest:               info.SoulDigest,
		ParentSoulDigest:         info.ParentSoulDigest,
		CreatedAt:                info.CreatedAt,
		UpdatedAt:                info.UpdatedAt,
	}
}

func (m *Manager) enrichNotifiedAgentEvent(session *Session, event any) any {
	switch typed := event.(type) {
	case acp.AgentEvent:
		return m.enrichRecordedAgentEvent(session, typed)
	case *acp.AgentEvent:
		if typed == nil {
			return event
		}
		enriched := m.enrichRecordedAgentEvent(session, *typed)
		return &enriched
	default:
		return event
	}
}
