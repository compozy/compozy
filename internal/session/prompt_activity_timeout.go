package session

import (
	"context"
	"encoding/json"

	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/store"

	"github.com/compozy/agh/internal/transcript"
)

func (s *promptActivitySupervisor) handleTimeout(now time.Time) {
	if s == nil || s.session == nil || s.manager == nil {
		return
	}
	s.handleTimeoutWithDetail(now, store.SessionStallReasonActivityTimeout, s.timeoutText(now), nil)
}

func (s *promptActivitySupervisor) handlePromptDeadline(now time.Time) {
	if s == nil || s.session == nil || s.manager == nil {
		return
	}
	if s.promptDeadlineWarningDelivered() {
		return
	}

	warning, ok := s.promptDeadlineWarningEvent(now)
	if !ok {
		return
	}
	ack := s.preparePromptDeadlineWarning(warning)
	s.recordRuntimeTimeout(now, store.SessionStallReasonPromptDeadlineExceeded)
	s.emitPreparedRuntimeEvent(warning)
	s.waitForPromptDeadlineWarningAck(ack)
	s.cancelPromptAfterRuntimeTimeout()
	s.stopSessionAfterRuntimeTimeout(store.SessionStallReasonPromptDeadlineExceeded)
}

func (s *promptActivitySupervisor) handleTimeoutWithDetail(
	now time.Time,
	stopDetail string,
	text string,
	raw json.RawMessage,
) {
	if s == nil || s.session == nil || s.manager == nil {
		return
	}
	s.recordRuntimeTimeout(now, stopDetail)
	s.emitRuntimeEvent(acp.EventTypeRuntimeWarning, text, now, raw)
	s.cancelPromptAfterRuntimeTimeout()
	s.stopSessionAfterRuntimeTimeout(stopDetail)
}

func (s *promptActivitySupervisor) recordRuntimeTimeout(now time.Time, stopDetail string) {
	if s == nil || s.session == nil || s.manager == nil {
		return
	}
	s.session.markRuntimeStalled(stopDetail, now)
	if err := s.manager.persistSessionMetadataOnly(s.session); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime timeout stall failed", "turn_id", s.turnID, "error", err)
	}
	if _, err := s.manager.persistSessionPromptActivity(s.ctx, s.session, now); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: persist runtime timeout health failed", "turn_id", s.turnID, "error", err)
	}
	s.manager.emitTranscriptMarker(
		s.ctx,
		s.session,
		s.turnID,
		transcript.MarkerPromptTimeout,
		"Runtime activity timed out.",
		map[string]any{runtimeActivityEvidenceStallReason: stopDetail},
	)
}

func (s *promptActivitySupervisor) cancelPromptAfterRuntimeTimeout() {
	if s == nil || s.session == nil || s.manager == nil {
		return
	}
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.config.TimeoutCancelGrace)
	cancelErr := s.manager.CancelPrompt(cancelCtx, s.session.ID)
	cancel()
	if cancelErr != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: cancel prompt after runtime timeout failed", "turn_id", s.turnID, "error", cancelErr)
	}
}

func (s *promptActivitySupervisor) stopSessionAfterRuntimeTimeout(stopDetail string) {
	if s == nil || s.session == nil || s.manager == nil {
		return
	}
	timer := time.NewTimer(s.config.TimeoutCancelGrace)
	defer timer.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			goto forceStop
		case <-ticker.C:
			if !s.session.IsPrompting() {
				return
			}
		}
	}

forceStop:
	if !s.session.IsPrompting() {
		return
	}

	stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.timeoutStopDeadline())
	defer stopCancel()
	if err := s.manager.StopWithCause(
		stopCtx,
		s.session.ID,
		CauseTimeout,
		stopDetail,
	); err != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: stop session after runtime timeout failed", "turn_id", s.turnID, "error", err)
	}
}

func (s *promptActivitySupervisor) timeoutStopDeadline() time.Duration {
	if s == nil || s.config.TimeoutCancelGrace <= 0 {
		return aghconfig.DefaultSessionSupervisionConfig().TimeoutCancelGrace
	}
	if s.config.TimeoutCancelGrace < defaultLifecycleTimeout {
		return defaultLifecycleTimeout
	}
	return s.config.TimeoutCancelGrace
}

func (s *promptActivitySupervisor) touch(now time.Time, kind string, detail string) {
	s.touchWithTool(now, kind, detail, "", "", false)
}
