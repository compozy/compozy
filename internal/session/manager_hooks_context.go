package session

import (
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/workref"
)

func hookSessionLifecyclePayload(
	session *Session,
	event hookspkg.HookEvent,
	timestamp time.Time,
) hookspkg.SessionLifecyclePayload {
	return hookspkg.SessionLifecyclePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     event,
			Timestamp: timestamp,
		},
		SessionContext: hookSessionContext(session),
	}
}

func hookSessionContext(session *Session) hookspkg.SessionContext {
	if session == nil {
		return hookspkg.SessionContext{}
	}

	info := session.Info()
	if info == nil {
		return hookspkg.SessionContext{}
	}

	ref := workref.NewRoot(info.WorkspaceID, info.Workspace)
	return hookspkg.SessionContext{
		SessionID:    strings.TrimSpace(info.ID),
		SessionName:  strings.TrimSpace(info.Name),
		SessionType:  string(info.Type),
		AgentName:    strings.TrimSpace(info.AgentName),
		WorkspaceID:  ref.WorkspaceID,
		Workspace:    ref.Workspace,
		ACPSessionID: strings.TrimSpace(info.ACPSessionID),
		State:        string(info.State),
		SessionSoulContext: hookSessionSoulContext(
			info.SoulSnapshotID,
			info.SoulDigest,
		),
		CreatedAt: info.CreatedAt,
		UpdatedAt: info.UpdatedAt,
	}
}

func hookSessionSoulContext(snapshotID string, digest string) *hookspkg.SessionSoulContext {
	trimmedSnapshotID := strings.TrimSpace(snapshotID)
	trimmedDigest := strings.TrimSpace(digest)
	if trimmedSnapshotID == "" && trimmedDigest == "" {
		return nil
	}
	return &hookspkg.SessionSoulContext{
		SoulSnapshotID: trimmedSnapshotID,
		SoulDigest:     trimmedDigest,
	}
}

func (s *Session) applyHookSessionContext(payload hookspkg.SessionContext, now time.Time) {
	if s == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Name = strings.TrimSpace(payload.SessionName)
	s.AgentName = strings.TrimSpace(payload.AgentName)
	s.WorkspaceID = strings.TrimSpace(payload.WorkspaceID)
	s.Workspace = strings.TrimSpace(payload.Workspace)
	s.Type = normalizeSessionType(Type(strings.TrimSpace(payload.SessionType)))
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func hookTimestamp(now time.Time, eventTime time.Time) time.Time {
	if !eventTime.IsZero() {
		return eventTime
	}
	return now
}
