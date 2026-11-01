package session

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
)

// liveTurnRuntime is the runtime a running turn keeps for event attribution.
type liveTurnRuntime struct {
	turnID    string
	selection RuntimeSelection
}

// lockRuntimeApply waits for exclusive runtime configuration until ctx ends.
func (s *Session) lockRuntimeApply(ctx context.Context) error {
	// select picks randomly among ready cases, so a canceled caller must not reach it.
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.runtimeApplySlot == nil {
		s.runtimeApplySlot = make(chan struct{}, 1)
	}
	slot := s.runtimeApplySlot
	s.mu.Unlock()
	select {
	case slot <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Session) unlockRuntimeApply() {
	s.mu.RLock()
	slot := s.runtimeApplySlot
	s.mu.RUnlock()
	<-slot
}

// commitLiveRuntimeSelection records a configured selection on the still-bound process.
// It leaves runtime status, generation, and liveness untouched so a live turn's
// supervision and health are not disturbed. A non-nil hold keeps a dispatched turn's
// events attributed to the runtime it started with when the provider honors the change
// only from its next turn.
func (s *Session) commitLiveRuntimeSelection(
	proc *AgentProcess,
	selection RuntimeSelection,
	hold *liveTurnRuntime,
	now time.Time,
) bool {
	if s == nil || proc == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive || s.process != proc {
		return false
	}
	if hold != nil && (s.liveTurnRuntime == nil || s.liveTurnRuntime.turnID != hold.turnID) {
		s.liveTurnRuntime = hold
	}
	s.Provider = strings.TrimSpace(selection.Provider)
	s.Model = strings.TrimSpace(selection.Model)
	s.ReasoningEffort = strings.TrimSpace(selection.ReasoningEffort)
	s.Speed = selection.Speed
	s.ACPOptions = acp.CloneSessionConfigOptionSelections(selection.ACPOptions)
	s.RuntimeTransition = RuntimeTransitionLiveConfiguration
	s.RuntimeFailure = ""
	s.runtimeFailureFromLiveApply = false
	s.runtimeConfigUnverified = false
	s.refreshLiveProcessCapsLocked(proc, s.Model == "", now)
	return true
}

// recordLiveRuntimeFailure surfaces a refused live apply without unbinding the process.
// A failure is recorded only while the selection revision it applied is still current:
// a newer saved selection supersedes it. unverified marks a failed rollback, which
// requires a successful configure or replacement before the next dispatch.
func (s *Session) recordLiveRuntimeFailure(
	proc *AgentProcess,
	revision int64,
	failure string,
	unverified bool,
	now time.Time,
) bool {
	if s == nil || proc == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A concurrent idle release may reattach this process, so this outlives the guard below.
	if unverified {
		s.runtimeConfigUnverified = true
	}
	if s.State != StateActive || s.process != proc {
		return false
	}
	if s.RuntimeSelectionRevision == revision {
		s.RuntimeFailure = strings.TrimSpace(failure)
		s.runtimeFailureFromLiveApply = true
	}
	s.refreshLiveProcessCapsLocked(proc, false, now)
	return true
}

// clearRuntimeFailure reports whether it removed a failure while revision was current.
func (s *Session) clearRuntimeFailure(proc *AgentProcess, revision int64, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive || s.process != proc || s.RuntimeSelectionRevision != revision ||
		s.RuntimeFailure == "" {
		return false
	}
	s.RuntimeFailure = ""
	s.runtimeFailureFromLiveApply = false
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
	return true
}

// markRuntimeConfigUnverified records that the provider config may differ from the
// effective selection. Any later bind or successful configure clears it.
func (s *Session) markRuntimeConfigUnverified() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeConfigUnverified = true
}

// selectedRuntimeWithRevision reads the stored selection and its revision together.
func (s *Session) selectedRuntimeWithRevision() (*RuntimeSelection, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneRuntimeSelection(s.SelectedRuntime), s.RuntimeSelectionRevision
}

// promptRuntimeForEventTurn attributes an event to the runtime its turn actually used.
func (s *Session) promptRuntimeForEventTurn(turnID string) RuntimeSelection {
	s.mu.RLock()
	held := s.liveTurnRuntime
	s.mu.RUnlock()
	if held != nil && strings.TrimSpace(turnID) != "" && held.turnID == strings.TrimSpace(turnID) {
		selection := held.selection
		selection.ACPOptions = acp.CloneSessionConfigOptionSelections(held.selection.ACPOptions)
		return selection
	}
	return s.Info().EffectiveRuntime()
}

func (s *Session) refreshLiveProcessCapsLocked(proc *AgentProcess, adoptCurrentModel bool, now time.Time) {
	caps := proc.CapsSnapshot()
	s.ACPCaps = cloneCaps(caps)
	s.ACPCapsKnown = true
	s.SpeedResolution = s.speedResolutionForCapsLocked(caps)
	if currentModel := currentACPModel(caps.ConfigOptions); adoptCurrentModel && currentModel != "" {
		s.Model = currentModel
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}
