package session

import (
	"encoding/json"

	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"
)

func (s *promptActivitySupervisor) emitRuntimeEvent(
	eventType string,
	text string,
	now time.Time,
	raw json.RawMessage,
) {
	if s == nil {
		return
	}
	s.emitPreparedRuntimeEvent(s.buildRuntimeEvent(eventType, text, now, raw))
}

func (s *promptActivitySupervisor) buildRuntimeEvent(
	eventType string,
	text string,
	now time.Time,
	raw json.RawMessage,
) acp.AgentEvent {
	activity := s.runtimeActivity(now)
	if s.session != nil && s.manager != nil {
		if meta := storeActivityFromRuntime(activity); meta != nil {
			s.session.observeRuntimeEventActivity(*meta, now)
			if err := s.manager.persistSessionMetadataOnly(s.session); err != nil {
				s.manager.sessionLogger(s.session).
					Warn("session: persist runtime progress failed", "turn_id", s.turnID, "error", err)
			}
		}
	}
	return acp.AgentEvent{
		Type:      eventType,
		TurnID:    s.turnID,
		Timestamp: now.UTC(),
		Text:      strings.TrimSpace(text),
		Runtime:   &activity,
		Raw:       acp.CloneRawMessage(raw),
	}
}

func (s *promptActivitySupervisor) emitPreparedRuntimeEvent(event acp.AgentEvent) {
	if s == nil {
		return
	}
	select {
	case <-s.ctx.Done():
	case s.events <- event:
	}
}

func (s *promptActivitySupervisor) preparePromptDeadlineWarning(event acp.AgentEvent) chan struct{} {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadlineWarnAck = make(chan struct{})
	s.deadlineWarnEvent = event
	s.deadlineWarnPending = true
	s.deadlineWarnDelivered = false
	return s.deadlineWarnAck
}

func (s *promptActivitySupervisor) waitForPromptDeadlineWarningAck(ack chan struct{}) {
	if s == nil || ack == nil {
		return
	}
	select {
	case <-ack:
	case <-s.ctx.Done():
	}
}

func (s *promptActivitySupervisor) ackPromptDeadlineWarning(event acp.AgentEvent) {
	if s == nil || !isPromptDeadlineWarningEvent(event) {
		return
	}

	s.mu.Lock()
	ack := s.deadlineWarnAck
	s.deadlineWarnPending = false
	s.deadlineWarnDelivered = true
	s.deadlineWarnEvent = acp.AgentEvent{}
	s.deadlineWarnAck = nil
	s.mu.Unlock()

	if ack == nil {
		return
	}
	close(ack)
}

func (s *promptActivitySupervisor) pendingPromptDeadlineWarning() (acp.AgentEvent, bool) {
	if s == nil {
		return acp.AgentEvent{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.deadlineWarnPending || s.deadlineWarnDelivered {
		return acp.AgentEvent{}, false
	}
	return s.deadlineWarnEvent, true
}

func (s *promptActivitySupervisor) shouldSkipDeliveredPromptDeadlineWarning(event acp.AgentEvent) bool {
	if s == nil || !isPromptDeadlineWarningEvent(event) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deadlineWarnDelivered
}

func (s *promptActivitySupervisor) promptDeadlineWarningDelivered() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deadlineWarnDelivered
}

func (s *promptActivitySupervisor) promptDeadlineWarningEvent(now time.Time) (acp.AgentEvent, bool) {
	if s == nil {
		return acp.AgentEvent{}, false
	}

	s.mu.Lock()
	deadline := cloneTimePointer(s.deadlineAt)
	pending := s.deadlineWarnPending
	delivered := s.deadlineWarnDelivered
	existing := s.deadlineWarnEvent
	s.mu.Unlock()

	if deadline == nil || delivered {
		return acp.AgentEvent{}, false
	}
	if pending {
		return existing, true
	}

	raw, err := jsonMarshalDeadlineWarning(s.runtimeActivity(now))
	if err != nil && s.manager != nil && s.session != nil {
		s.manager.sessionLogger(s.session).
			Warn("session: marshal prompt deadline warning failed", "turn_id", s.turnID, "error", err)
	}
	return s.buildRuntimeEvent(acp.EventTypeRuntimeWarning, s.promptDeadlineText(now), now, raw), true
}

func isPromptDeadlineWarningEvent(event acp.AgentEvent) bool {
	return event.Type == acp.EventTypeRuntimeWarning && event.Runtime != nil && event.Runtime.DeadlineAt != nil
}
