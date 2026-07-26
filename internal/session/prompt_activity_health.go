package session

import (
	"context"

	"fmt"
	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"

	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/transcript"
)

func (s *promptActivitySupervisor) handleUnhealthyProcess(now time.Time, emitWarning bool) bool {
	if s == nil || s.manager == nil || s.session == nil {
		return false
	}
	proc := s.session.processHandle()
	if proc == nil {
		return false
	}
	health, ok := proc.HealthState()
	if !ok || !subprocess.HealthFailureDetected(health) {
		s.mu.Lock()
		s.unhealthy = false
		s.unhealthyWarned = false
		s.mu.Unlock()
		return false
	}
	healthCtx, healthCancel := s.manager.detachedSessionHealthContext(s.ctx)
	s.manager.notifySubprocessHealth(healthCtx, s.session, health)
	healthCancel()

	shouldPersist := false
	shouldWarn := false
	s.mu.Lock()
	if !s.unhealthy {
		s.unhealthy = true
		shouldPersist = true
	}
	if emitWarning && !s.unhealthyWarned {
		s.unhealthyWarned = true
		shouldWarn = true
	}
	s.mu.Unlock()

	if shouldPersist {
		healthError := unhealthyProcessDiagnostic(health)
		s.session.markRuntimeStalled(store.SessionStallReasonProcessUnhealthy, now)
		if err := s.manager.persistSessionMetadataOnly(s.session); err != nil {
			s.manager.sessionLogger(s.session).
				Warn("session: persist unhealthy runtime stall failed", "turn_id", s.turnID, "error", err)
		}
		if _, err := s.manager.persistSessionHealthForSession(s.ctx, s.session, now, sessionHealthInput{
			activePrompt: true,
			attachable:   sessionAttachableAt(s.session, now),
			lastError:    healthError,
		}); err != nil {
			s.manager.sessionLogger(s.session).
				Warn("session: persist unhealthy runtime health failed", "turn_id", s.turnID, "error", err)
		}
	}
	if shouldWarn {
		s.manager.emitTranscriptMarker(
			s.ctx,
			s.session,
			s.turnID,
			transcript.MarkerSessionUnhealthy,
			"Runtime health check failed.",
			map[string]any{runtimeActivityEvidenceStallReason: store.SessionStallReasonProcessUnhealthy},
		)
		s.emitRuntimeEvent(acp.EventTypeRuntimeWarning, unhealthyProcessText(health), now, nil)
	}
	return true
}

func (s *promptActivitySupervisor) runtimeActivity(now time.Time) acp.RuntimeActivity {
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.IsZero() {
		now = s.now()
	}
	s.activity.IdleSeconds = store.SessionActivityIdleSeconds(&s.activity, now)
	activity := acp.RuntimeActivity{
		TurnID:             strings.TrimSpace(s.activity.TurnID),
		TurnSource:         strings.TrimSpace(s.activity.TurnSource),
		TurnStartedAt:      cloneTimePointer(s.activity.TurnStartedAt),
		DeadlineAt:         cloneTimePointer(s.deadlineAt),
		LastActivityAt:     cloneTimePointer(s.activity.LastActivityAt),
		LastActivityKind:   strings.TrimSpace(s.activity.LastActivityKind),
		LastActivityDetail: strings.TrimSpace(s.activity.LastActivityDetail),
		CurrentTool:        strings.TrimSpace(s.activity.CurrentTool),
		ToolCallID:         strings.TrimSpace(s.activity.ToolCallID),
		LastProgressAt:     cloneTimePointer(s.activity.LastProgressAt),
		IterationCurrent:   s.activity.IterationCurrent,
		IterationMax:       s.activity.IterationMax,
		IdleSeconds:        s.activity.IdleSeconds,
	}
	if !s.startedAt.IsZero() {
		elapsed := now.UTC().Sub(s.startedAt.UTC())
		if elapsed > 0 {
			activity.ElapsedSeconds = int64(elapsed.Seconds())
			activity.ElapsedMS = elapsed.Milliseconds()
		}
	}
	return activity
}

func (s *promptActivitySupervisor) progressText(now time.Time) string {
	activity := s.runtimeActivity(now)
	parts := []string{"Still working..."}
	elapsedMinutes := activity.ElapsedSeconds / 60
	if elapsedMinutes > 0 {
		detail := fmt.Sprintf("%d min elapsed", elapsedMinutes)
		if activity.IterationCurrent > 0 && activity.IterationMax > 0 {
			detail += fmt.Sprintf(" - iteration %d/%d", activity.IterationCurrent, activity.IterationMax)
		}
		if activity.CurrentTool != "" {
			detail += ", running: " + activity.CurrentTool
		} else if activity.LastActivityKind != "" {
			detail += ", last activity: " + activity.LastActivityKind
		}
		parts = append(parts, "("+detail+")")
	}
	return strings.Join(parts, " ")
}

func (s *promptActivitySupervisor) warningText(now time.Time) string {
	activity := s.runtimeActivity(now)
	return fmt.Sprintf("Runtime activity is stale (%d seconds idle).", activity.IdleSeconds)
}

func (s *promptActivitySupervisor) timeoutText(now time.Time) string {
	activity := s.runtimeActivity(now)
	return fmt.Sprintf("Runtime activity timed out (%d seconds idle).", activity.IdleSeconds)
}

func (s *promptActivitySupervisor) promptDeadlineText(now time.Time) string {
	activity := s.runtimeActivity(now)
	if activity.ElapsedMS <= 0 {
		return "Prompt deadline exceeded."
	}
	return fmt.Sprintf("Prompt deadline exceeded after %d ms.", activity.ElapsedMS)
}

func unhealthyProcessText(health subprocess.HealthState) string {
	parts := []string{"Runtime health check failed; prompt may be stalled."}
	if detail := strings.TrimSpace(health.Message); detail != "" {
		parts = append(parts, detail)
	}
	if lastErr := strings.TrimSpace(health.LastError); lastErr != "" {
		parts = append(parts, lastErr)
	}
	return strings.Join(parts, " ")
}

func unhealthyProcessDiagnostic(health subprocess.HealthState) string {
	return diagnostics.RedactAndBound(unhealthyProcessText(health), maxSessionFailureSummaryBytes)
}

func (s *promptActivitySupervisor) idleSecondsLocked(now time.Time) int64 {
	return store.SessionActivityIdleSeconds(&s.activity, now)
}

func (s *promptActivitySupervisor) now() time.Time {
	if s != nil && s.manager != nil && s.manager.now != nil {
		return s.manager.now().UTC()
	}
	return time.Now().UTC()
}

func deadlineFromContext(ctx context.Context) (time.Time, bool) {
	if ctx == nil {
		return time.Time{}, false
	}
	return ctx.Deadline()
}
