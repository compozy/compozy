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
	return hookSessionContextFromInfo(info)
}

func hookSessionContextFromInfo(info *Info) hookspkg.SessionContext {
	if info == nil {
		return hookspkg.SessionContext{}
	}
	ref := workref.NewRoot(info.WorkspaceID, info.Workspace)
	return hookspkg.SessionContext{
		ProfileID:             strings.TrimSpace(info.ProfileID),
		SessionID:             strings.TrimSpace(info.ID),
		SessionName:           strings.TrimSpace(info.Name),
		SessionType:           string(info.Type),
		AgentName:             strings.TrimSpace(info.AgentName),
		WorkspaceID:           ref.WorkspaceID,
		Workspace:             ref.Workspace,
		SessionRuntimeContext: hookspkg.NewSessionRuntimeContext(info.WorktreeID, info.ACPSessionID),
		State:                 string(info.State),
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
	expectedWorktreeID := strings.TrimSpace(expected.WorktreeIDValue())
	actualWorktreeID := strings.TrimSpace(actual.WorktreeIDValue())
	if actualID == expectedID && actualPath == expectedPath &&
		actualWorktreeID == expectedWorktreeID {
		return nil
	}
	return fmt.Errorf(
		"%w: session workspace identity is immutable after creation: expected {%q %q %q}, got {%q %q %q}",
		ErrValidation,
		expectedID,
		expectedPath,
		expectedWorktreeID,
		actualID,
		actualPath,
		actualWorktreeID,
	)
}

func validateImmutableSessionType(expected, actual hookspkg.SessionContext) error {
	expectedType := normalizeSessionType(Type(strings.TrimSpace(expected.SessionType)))
	actualType := normalizeSessionType(Type(strings.TrimSpace(actual.SessionType)))
	if actualType == expectedType {
		return nil
	}
	return fmt.Errorf(
		"%w: session type is immutable after creation: expected %q, got %q",
		ErrValidation,
		expectedType,
		actualType,
	)
}

func validateProtectedPreCreateSessionType(expected, actual hookspkg.SessionContext) error {
	expectedType := normalizeSessionType(Type(strings.TrimSpace(expected.SessionType)))
	actualType := normalizeSessionType(Type(strings.TrimSpace(actual.SessionType)))
	if expectedType == actualType ||
		(!daemonOwnedSessionType(expectedType) && !daemonOwnedSessionType(actualType)) {
		return nil
	}
	return fmt.Errorf(
		"%w: session.pre_create cannot change daemon-owned session type: expected %q, got %q",
		ErrValidation,
		expectedType,
		actualType,
	)
}

func daemonOwnedSessionType(sessionType Type) bool {
	return sessionType == SessionTypeCoordinator || sessionType == SessionTypeSpawned
}

func hookTimestamp(now time.Time, eventTime time.Time) time.Time {
	if !eventTime.IsZero() {
		return eventTime
	}
	return now
}
