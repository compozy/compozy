package session

import (
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

type runtimeBindingSnapshot struct {
	process         *AgentProcess
	selection       RuntimeSelection
	status          RuntimeStatus
	transition      RuntimeTransitionStrategy
	failure         string
	acpSessionID    string
	acpCaps         acp.Caps
	acpCapsKnown    bool
	speedResolution *speedpkg.Resolution
	liveness        *store.SessionLivenessMeta
	agentDef        compozyconfig.AgentDef
}

func (s *Session) runtimeBindingSnapshot() runtimeBindingSnapshot {
	if s == nil {
		return runtimeBindingSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return runtimeBindingSnapshot{
		process: s.process,
		selection: RuntimeSelection{
			Provider:        s.Provider,
			Model:           s.Model,
			ReasoningEffort: s.ReasoningEffort,
			Speed:           s.Speed,
		},
		status:          s.RuntimeStatus,
		transition:      s.RuntimeTransition,
		failure:         s.RuntimeFailure,
		acpSessionID:    s.ACPSessionID,
		acpCaps:         cloneCaps(s.ACPCaps),
		acpCapsKnown:    s.ACPCapsKnown,
		speedResolution: speedpkg.CloneResolution(s.SpeedResolution),
		liveness:        store.CloneSessionLivenessMeta(s.Liveness),
		agentDef:        compozyconfig.CloneAgentDef(s.agentDef),
	}
}

func (s *Session) beginRuntimeTransition(
	status RuntimeStatus,
	strategy RuntimeTransitionStrategy,
	now time.Time,
) error {
	if s == nil {
		return errors.New("session: session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive {
		return ErrSessionNotActive
	}
	s.RuntimeStatus = status
	s.RuntimeTransition = strategy
	s.RuntimeFailure = ""
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
	return nil
}

func (s *Session) completeRuntimeTransition(
	proc *AgentProcess,
	selection RuntimeSelection,
	strategy RuntimeTransitionStrategy,
	now time.Time,
) *AgentProcess {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous := s.process
	s.Provider = strings.TrimSpace(selection.Provider)
	s.Model = strings.TrimSpace(selection.Model)
	s.ReasoningEffort = strings.TrimSpace(selection.ReasoningEffort)
	s.Speed = selection.Speed
	s.RuntimeStatus = RuntimeStatusReady
	s.RuntimeTransition = strategy
	s.RuntimeFailure = ""
	s.updateFromProcessLocked(proc, now, strings.TrimSpace(selection.Model) == "")
	return previous
}

func (s *Session) restoreRuntimeBinding(snapshot *runtimeBindingSnapshot, failure string, now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.process = snapshot.process
	s.Provider = snapshot.selection.Provider
	s.Model = snapshot.selection.Model
	s.ReasoningEffort = snapshot.selection.ReasoningEffort
	s.Speed = snapshot.selection.Speed
	s.RuntimeStatus = snapshot.status
	s.RuntimeTransition = snapshot.transition
	s.RuntimeFailure = strings.TrimSpace(failure)
	s.ACPSessionID = snapshot.acpSessionID
	s.ACPCaps = cloneCaps(snapshot.acpCaps)
	s.ACPCapsKnown = snapshot.acpCapsKnown
	s.SpeedResolution = speedpkg.CloneResolution(snapshot.speedResolution)
	s.Liveness = store.CloneSessionLivenessMeta(snapshot.liveness)
	s.agentDef = compozyconfig.CloneAgentDef(snapshot.agentDef)
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}

func (s *Session) markRuntimeUnbound(now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.process = nil
	s.RuntimeStatus = RuntimeStatusUnbound
	s.RuntimeFailure = ""
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}

func (s *Session) isCurrentProcess(proc *AgentProcess) bool {
	if s == nil || proc == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.process == proc
}
