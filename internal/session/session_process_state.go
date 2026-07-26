package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

const sessionModelConfigKey = "model"

func (s *Session) updateFromProcessLocked(proc *AgentProcess, now time.Time, adoptCurrentModel bool) {
	s.process = proc
	if proc != nil {
		caps := proc.CapsSnapshot()
		s.ACPSessionID = strings.TrimSpace(proc.SessionID)
		s.ACPCaps = cloneCaps(caps)
		if currentModel := currentACPModel(caps.ConfigOptions); adoptCurrentModel && currentModel != "" {
			s.Model = currentModel
		}
		if s.Liveness == nil {
			s.Liveness = &store.SessionLivenessMeta{}
		}
		s.Liveness.SubprocessPID = proc.PID
		if !proc.StartedAt.IsZero() {
			startedAt := proc.StartedAt.UTC()
			s.Liveness.SubprocessStartedAt = &startedAt
		}
		if !now.IsZero() {
			lastUpdateAt := now.UTC()
			s.Liveness.LastUpdateAt = &lastUpdateAt
		}
		s.Liveness.StallState = ""
		s.Liveness.StallReason = ""
	}
	if !now.IsZero() {
		s.UpdatedAt = now
	}
}

func (s *Session) activateWithProcess(
	proc *AgentProcess,
	now time.Time,
	adoptCurrentModel bool,
	preserveStopReason bool,
) error {
	if s == nil {
		return ErrSessionNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !canTransition(s.State, StateActive) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, s.State, StateActive)
	}
	s.State = StateActive
	if !preserveStopReason {
		s.stopCause = CauseNone
		if s.stopReason != store.StopAgentCrashed {
			s.stopReason = ""
			s.stopDetail = ""
			s.failure = nil
		}
	}
	s.updateFromProcessLocked(proc, now, adoptCurrentModel)
	return nil
}

func currentACPModel(options []acp.SessionConfigOption) string {
	for _, option := range options {
		if strings.TrimSpace(option.ID) == sessionModelConfigKey ||
			strings.TrimSpace(option.Category) == sessionModelConfigKey {
			return strings.TrimSpace(option.Current)
		}
	}
	return ""
}

func (s *Session) setEffectivePermissions(value string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.EffectivePermissions = strings.TrimSpace(value)
	s.mu.Unlock()
}

func (s *Session) effectivePermissions() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.EffectivePermissions)
}
