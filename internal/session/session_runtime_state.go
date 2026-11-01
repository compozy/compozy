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
	runtimeStrategy        acp.RuntimeApplicationStrategy
	process                *AgentProcess
	selection              RuntimeSelection
	status                 RuntimeStatus
	transition             RuntimeTransitionStrategy
	failure                string
	generation             int64
	recovery               *store.SessionRuntimeRecovery
	acpSessionID           string
	acceptedRoute          *store.SessionAcceptedRoute
	acceptedCommand        string
	acceptedSessionEnv     compozyconfig.SessionEnv
	pendingResumeReplay    string
	acpCaps                acp.Caps
	acpCapsKnown           bool
	speedResolution        *speedpkg.Resolution
	liveness               *store.SessionLivenessMeta
	agentDef               compozyconfig.AgentDef
	advertisedCommands     []store.SessionAdvertisedCommand
	startupManifest        acp.StartupManifest
	nativeToolsUnavailable bool
	runtimeRelease         *runtimeRelease
	configUnverified       bool
}

func (s *Session) runtimeBindingSnapshot() runtimeBindingSnapshot {
	if s == nil {
		return runtimeBindingSnapshot{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return runtimeBindingSnapshot{
		runtimeStrategy: s.runtimeStrategyLocked(),
		process:         s.process,
		selection: RuntimeSelection{
			Provider:        s.Provider,
			Model:           s.Model,
			ReasoningEffort: s.ReasoningEffort,
			Speed:           s.Speed,
			ACPOptions:      acp.CloneSessionConfigOptionSelections(s.ACPOptions),
		},
		status:                 s.RuntimeStatus,
		transition:             s.RuntimeTransition,
		failure:                s.RuntimeFailure,
		generation:             s.RuntimeGeneration,
		recovery:               store.CloneSessionRuntimeRecovery(s.RuntimeRecovery),
		acpSessionID:           s.ACPSessionID,
		acceptedRoute:          store.CloneSessionAcceptedRoute(s.acceptedRoute),
		acceptedCommand:        s.acceptedCommand,
		acceptedSessionEnv:     compozyconfig.MergeSessionEnv(nil, s.acceptedSessionEnv),
		pendingResumeReplay:    s.pendingResumeReplay,
		acpCaps:                cloneCaps(s.ACPCaps),
		acpCapsKnown:           s.ACPCapsKnown,
		speedResolution:        speedpkg.CloneResolution(s.SpeedResolution),
		liveness:               store.CloneSessionLivenessMeta(s.Liveness),
		agentDef:               compozyconfig.CloneAgentDef(s.agentDef),
		advertisedCommands:     store.CloneSessionAdvertisedCommands(s.AdvertisedCommands),
		startupManifest:        acp.CloneStartupManifest(s.startupManifest),
		nativeToolsUnavailable: s.nativeToolsUnavailable,
		runtimeRelease:         s.runtimeRelease,
		configUnverified:       s.runtimeConfigUnverified,
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
	s.runtimeFailureFromLiveApply = false
	s.RuntimeRecovery = nil
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
	return s.completeRuntimeTransitionLocked(proc, selection, strategy, now)
}

func (s *Session) completeRecoveryTransition(
	expected *AgentProcess,
	proc *AgentProcess,
	selection RuntimeSelection,
	agentDef compozyconfig.AgentDef,
	now time.Time,
) (*AgentProcess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive || s.process != expected {
		return nil, ErrSessionNotActive
	}
	previous := s.completeRuntimeTransitionLocked(proc, selection, RuntimeTransitionAutomaticRecovery, now)
	s.agentDef = compozyconfig.CloneAgentDef(agentDef)
	return previous, nil
}

func (s *Session) completeRuntimeTransitionLocked(
	proc *AgentProcess,
	selection RuntimeSelection,
	strategy RuntimeTransitionStrategy,
	now time.Time,
) *AgentProcess {
	previous := s.process
	if proc != nil {
		s.runtimeRelease = nil
	}
	s.Provider = strings.TrimSpace(selection.Provider)
	s.Model = strings.TrimSpace(selection.Model)
	s.ReasoningEffort = strings.TrimSpace(selection.ReasoningEffort)
	s.Speed = selection.Speed
	s.ACPOptions = acp.CloneSessionConfigOptionSelections(selection.ACPOptions)
	s.RuntimeStatus = RuntimeStatusReady
	s.RuntimeTransition = strategy
	s.RuntimeFailure = ""
	s.runtimeFailureFromLiveApply = false
	s.runtimeConfigUnverified = false
	s.RuntimeRecovery = nil
	if proc != nil && proc != previous {
		s.RuntimeGeneration++
	}
	s.updateFromProcessLocked(proc, now, strings.TrimSpace(selection.Model) == "")
	return previous
}

func (s *Session) beginAutomaticRecovery(
	attempt int,
	maxAttempts int,
	nextAttemptAt time.Time,
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
	startedAt := now.UTC()
	if s.RuntimeRecovery != nil && !s.RuntimeRecovery.StartedAt.IsZero() {
		startedAt = s.RuntimeRecovery.StartedAt.UTC()
	}
	var next *time.Time
	if !nextAttemptAt.IsZero() {
		value := nextAttemptAt.UTC()
		next = &value
	}
	s.RuntimeStatus = RuntimeStatusRecovering
	s.RuntimeTransition = RuntimeTransitionAutomaticRecovery
	s.RuntimeFailure = ""
	s.RuntimeRecovery = &store.SessionRuntimeRecovery{
		Attempt: attempt, MaxAttempts: maxAttempts, Generation: s.RuntimeGeneration + 1,
		StartedAt: startedAt, LastAttemptAt: now.UTC(), NextAttemptAt: next,
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
	return nil
}

func (s *Session) recordAutomaticRecoveryFailure(cause error, now time.Time) {
	if s == nil || cause == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RuntimeFailure = strings.TrimSpace(cause.Error())
	if s.RuntimeRecovery != nil {
		s.RuntimeRecovery.LastError = s.RuntimeFailure
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}

func (s *Session) restoreRuntimeBinding(snapshot *runtimeBindingSnapshot, failure string, now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoreRuntimeBindingLocked(snapshot, failure, now)
}

func (s *Session) restoreRecoveryBinding(
	expected *AgentProcess, snapshot *runtimeBindingSnapshot, failure string, now time.Time,
) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State == StateActive && s.process == expected {
		s.restoreRuntimeBindingLocked(snapshot, failure, now)
	}
}

func (s *Session) restoreRuntimeBindingLocked(snapshot *runtimeBindingSnapshot, failure string, now time.Time) {
	s.process = snapshot.process
	s.runtimeRelease = snapshot.runtimeRelease
	s.Provider = snapshot.selection.Provider
	s.Model = snapshot.selection.Model
	s.ReasoningEffort = snapshot.selection.ReasoningEffort
	s.Speed = snapshot.selection.Speed
	s.ACPOptions = acp.CloneSessionConfigOptionSelections(snapshot.selection.ACPOptions)
	s.RuntimeStatus = snapshot.status
	s.RuntimeTransition = snapshot.transition
	s.RuntimeFailure = strings.TrimSpace(failure)
	s.runtimeFailureFromLiveApply = false
	s.runtimeConfigUnverified = snapshot.configUnverified
	s.RuntimeGeneration = snapshot.generation
	s.RuntimeRecovery = store.CloneSessionRuntimeRecovery(snapshot.recovery)
	s.ACPSessionID = snapshot.acpSessionID
	s.acceptedRoute = store.CloneSessionAcceptedRoute(snapshot.acceptedRoute)
	s.acceptedCommand = snapshot.acceptedCommand
	s.acceptedSessionEnv = compozyconfig.MergeSessionEnv(nil, snapshot.acceptedSessionEnv)
	s.pendingResumeReplay = snapshot.pendingResumeReplay
	s.ACPCaps = cloneCaps(snapshot.acpCaps)
	s.ACPCapsKnown = snapshot.acpCapsKnown
	s.SpeedResolution = speedpkg.CloneResolution(snapshot.speedResolution)
	s.Liveness = store.CloneSessionLivenessMeta(snapshot.liveness)
	s.agentDef = compozyconfig.CloneAgentDef(snapshot.agentDef)
	s.AdvertisedCommands = store.CloneSessionAdvertisedCommands(snapshot.advertisedCommands)
	s.startupManifest = acp.CloneStartupManifest(snapshot.startupManifest)
	s.nativeToolsUnavailable = snapshot.nativeToolsUnavailable
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
}

// detachRuntimeAfterFailedRollback preserves intent without reusing an unconfirmed process.
// The release owns the old process until exit is verified, including failed stop retries.
func (s *Session) detachRuntimeAfterFailedRollback(
	snapshot *runtimeBindingSnapshot, failure string, now time.Time,
) *runtimeRelease {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoreRuntimeBindingLocked(snapshot, failure, now)
	s.captureProcessCapabilitiesLocked()
	release := &runtimeRelease{
		acpSessionID: strings.TrimSpace(s.ACPSessionID), process: s.process, done: make(chan struct{}),
	}
	s.process = nil
	s.runtimeRelease = release
	s.RuntimeStatus = RuntimeStatusUnbound
	s.RuntimeTransition = RuntimeTransitionNone
	s.RuntimeRecovery = nil
	return release
}

func (s *Session) markRuntimeUnbound(now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.captureProcessCapabilitiesLocked()
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
