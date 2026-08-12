package session

import (
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/compozy/internal/hooks"

	"github.com/compozy/compozy/internal/workref"
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
		WorktreeID:   strings.TrimSpace(info.WorktreeID),
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
	s.Type = normalizeSessionType(Type(strings.TrimSpace(payload.SessionType)))
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func validateImmutableSessionWorkspace(expected, actual hookspkg.SessionContext) error {
	expectedID := strings.TrimSpace(expected.WorkspaceID)
	expectedPath := strings.TrimSpace(expected.Workspace)
	actualID := strings.TrimSpace(actual.WorkspaceID)
	actualPath := strings.TrimSpace(actual.Workspace)
	if actualID == expectedID && actualPath == expectedPath &&
		strings.TrimSpace(actual.WorktreeID) == strings.TrimSpace(expected.WorktreeID) {
		return nil
	}
	return fmt.Errorf(
		"%w: session workspace identity is immutable after creation: expected {%q %q %q}, got {%q %q %q}",
		ErrValidation,
		expectedID,
		expectedPath,
		strings.TrimSpace(expected.WorktreeID),
		actualID,
		actualPath,
		strings.TrimSpace(actual.WorktreeID),
	)
}

func hookTimestamp(now time.Time, eventTime time.Time) time.Time {
	if !eventTime.IsZero() {
		return eventTime
	}
	return now
}
