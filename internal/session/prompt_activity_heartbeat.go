package session

import (
	"strings"

	"time"

	"github.com/compozy/agh/internal/store"
)

func (s *promptActivitySupervisor) recordWaitingHeartbeat(now time.Time, detail string) {
	if s == nil || s.manager == nil || s.session == nil {
		return
	}
	if now.IsZero() {
		now = s.now()
	}
	processUnhealthy := s.handleUnhealthyProcess(now, true)
	if processUnhealthy {
		return
	}
	s.mu.Lock()
	s.unhealthy = false
	s.unhealthyWarned = false
	s.activity.TurnID = s.turnID
	s.activity.TurnSource = string(s.turnSource)
	s.activity.TurnStartedAt = timePtr(s.startedAt)
	if s.activity.LastActivityAt == nil || s.activity.LastActivityAt.IsZero() {
		startedAt := s.startedAt.UTC()
		s.activity.LastActivityAt = &startedAt
	}
	s.activity.LastActivityKind = runtimeActivityKindAgentWaiting
	s.activity.LastActivityDetail = strings.TrimSpace(detail)
	s.activity.IdleSeconds = store.SessionActivityIdleSeconds(&s.activity, now)
	activity := *store.CloneSessionActivityMeta(&s.activity)
	lastActivityAt := time.Time{}
	if s.activity.LastActivityAt != nil {
		lastActivityAt = s.activity.LastActivityAt.UTC()
	}
	s.mu.Unlock()

	stallState, stallReason := s.session.observeRuntimeActivity(activity, now)
	if err := s.manager.persistSessionMetadataOnly(s.session); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime heartbeat failed", "turn_id", s.turnID, "error", err)
	}
	if _, err := s.manager.persistSessionPromptActivity(s.ctx, s.session, lastActivityAt); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime heartbeat health failed", "turn_id", s.turnID, "error", err)
	}
	if stallState == store.SessionStallStateDetected {
		s.emitRecoveredMarkerIfNeeded(stallReason)
	}
}

func (s *promptActivitySupervisor) touchWithTool(
	now time.Time,
	kind string,
	detail string,
	currentTool string,
	toolCallID string,
	clearTool bool,
) {
	if s == nil || s.manager == nil || s.session == nil {
		return
	}
	if now.IsZero() {
		now = s.now()
	}
	processUnhealthy := s.handleUnhealthyProcess(now, kind == runtimeActivityKindAgentWaiting)
	if processUnhealthy && kind == runtimeActivityKindAgentWaiting {
		return
	}
	s.mu.Lock()
	s.unhealthy = false
	s.unhealthyWarned = false
	s.activity.TurnID = s.turnID
	s.activity.TurnSource = string(s.turnSource)
	s.activity.TurnStartedAt = timePtr(s.startedAt)
	lastActivityAt := now.UTC()
	s.activity.LastActivityAt = &lastActivityAt
	s.activity.LastActivityKind = strings.TrimSpace(kind)
	s.activity.LastActivityDetail = strings.TrimSpace(detail)
	if strings.TrimSpace(currentTool) != "" {
		s.activity.CurrentTool = strings.TrimSpace(currentTool)
	}
	if strings.TrimSpace(toolCallID) != "" {
		s.activity.ToolCallID = strings.TrimSpace(toolCallID)
	}
	if clearTool {
		s.activity.CurrentTool = ""
		s.activity.ToolCallID = ""
	}
	s.activity.IdleSeconds = 0
	activity := *store.CloneSessionActivityMeta(&s.activity)
	s.mu.Unlock()

	stallState, stallReason := s.session.observeRuntimeActivity(activity, now)
	if err := s.manager.persistSessionMetadataOnly(s.session); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime activity failed", "turn_id", s.turnID, "error", err)
	}
	if _, err := s.manager.persistSessionPromptActivity(s.ctx, s.session, lastActivityAt); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime activity health failed", "turn_id", s.turnID, "error", err)
	}
	if stallState == store.SessionStallStateDetected {
		s.emitRecoveredMarkerIfNeeded(stallReason)
	}
}
