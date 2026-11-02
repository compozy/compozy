package session

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// Info returns a consistent snapshot of the current session state.
func (s *Session) Info() *Info {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.infoLocked()
}

// infoLocked returns a session snapshot; the caller must hold s.mu for reading or writing.
func (s *Session) infoLocked() *Info {
	var acpCaps acp.Caps
	acpCapsKnown := s.ACPCapsKnown
	var leaderKernelToken string
	if s.process != nil {
		leaderKernelToken = s.process.KernelToken
		acpCaps = s.process.CapsSnapshot()
		acpCapsKnown = true
	} else {
		acpCaps = cloneCaps(s.ACPCaps)
	}
	pendingPermission := s.process != nil && s.process.HasPendingPermission()
	resolution := speedpkg.CloneResolution(s.SpeedResolution)
	if acpCaps.SpeedResolution != nil {
		resolution = s.speedResolutionForCapsLocked(acpCaps)
	}

	return &Info{
		LeaderKernelToken:        leaderKernelToken,
		runtimeStrategy:          s.runtimeStrategyLocked(),
		Supervision:              CloneSupervisionState(s.supervisionState),
		BusyInput:                s.busyInputStateLocked(acpCaps),
		ID:                       s.ID,
		ProfileID:                s.ProfileID,
		Name:                     s.Name,
		AgentName:                s.AgentName,
		Provider:                 s.Provider,
		ProviderHomePolicy:       s.providerHomePolicy,
		Model:                    s.Model,
		ReasoningEffort:          s.ReasoningEffort,
		Speed:                    s.Speed,
		ACPOptions:               acp.CloneSessionConfigOptionSelections(s.ACPOptions),
		SpeedResolution:          resolution,
		VerdictOnly:              s.creationProfile != nil && s.creationProfile.RuntimeMode == RuntimeModeVerdictOnly,
		RuntimeStatus:            s.RuntimeStatus,
		RuntimeTransition:        s.RuntimeTransition,
		RuntimeFailure:           strings.TrimSpace(s.RuntimeFailure),
		RuntimeGeneration:        s.RuntimeGeneration,
		RuntimeRecovery:          store.CloneSessionRuntimeRecovery(s.RuntimeRecovery),
		SelectedRuntime:          cloneRuntimeSelection(s.SelectedRuntime),
		RuntimeSelectionRevision: s.RuntimeSelectionRevision,
		EffectiveWorkspaceAccess: s.EffectiveWorkspaceAccess,
		EffectivePermissions:     s.EffectivePermissions,
		WorkspaceID:              s.WorkspaceID,
		Workspace:                s.Workspace,
		WorktreeID:               s.WorktreeID,
		Type:                     normalizeSessionType(s.Type),
		Lineage:                  store.NormalizeSessionLineage(s.ID, s.Lineage),
		Derivation:               store.CloneSessionDerivation(s.derivation),
		State:                    s.State,
		PendingPermission:        pendingPermission,
		StopReason:               s.stopReason,
		StopCause:                s.stopCause,
		StopDetail:               s.stopDetail,
		StopEscalated:            s.stopEscalated,
		StopVerificationFailed:   s.stopVerificationFailed,
		Failure:                  store.CloneSessionFailure(s.failure),
		ACPSessionID:             s.ACPSessionID,
		ACPCaps:                  acpCaps,
		ACPCapsKnown:             acpCapsKnown,
		AdvertisedCommands:       store.CloneSessionAdvertisedCommands(s.AdvertisedCommands),
		Liveness:                 store.CloneSessionLivenessMeta(s.Liveness),
		SoulSnapshotID:           s.SoulSnapshotID,
		SoulDigest:               s.SoulDigest,
		ParentSoulDigest:         s.ParentSoulDigest,
		AttachedTo:               strings.TrimSpace(s.AttachedTo),
		AttachExpiresAt:          cloneSessionTimePtr(s.AttachExpiresAt),
		TranscriptEpoch:          s.TranscriptEpoch,
		PendingPermissionCount:   s.PendingPermissionCount,
		PendingClarifyCount:      s.PendingClarifyCount,
		AttentionRevision:        s.AttentionRevision,
		LastSettledRevision:      s.LastSettledRevision,
		LastSeenRevision:         s.LastSeenRevision,
		LastSeenAt:               cloneSessionTimePtr(s.LastSeenAt),
		AttentionChangedAt:       cloneSessionTimePtr(s.AttentionChangedAt),
		EndedEarlyTurnID:         s.EndedEarlyTurnID,
		PendingInteractions:      clonePendingInteractions(s.PendingInteractions),
		ArchivedAt:               nil,
		CreatedAt:                s.CreatedAt,
		UpdatedAt:                s.UpdatedAt,
	}
}

// RequestedRuntime returns the persisted intent of the current or last binding.
func (info *Info) RequestedRuntime() RuntimeSelection {
	if info == nil {
		return RuntimeSelection{}
	}
	return RuntimeSelection{
		Provider: info.Provider, Model: info.Model, ReasoningEffort: info.ReasoningEffort,
		Speed: info.Speed, ACPOptions: acp.CloneSessionConfigOptionSelections(info.ACPOptions),
	}
}

// EffectiveRuntime projects the latest provider observations while Info retains persisted intent.
func (info *Info) EffectiveRuntime() RuntimeSelection {
	if info == nil {
		return RuntimeSelection{}
	}
	runtime := info.RequestedRuntime()
	strategy := info.runtimeStrategy
	if strategy == "" {
		strategy = RuntimeStrategyForProvider(info.Provider)
	}
	if strategy != acp.RuntimeApplicationSessionConfig {
		return runtime
	}
	if option, ok := acp.ReasoningConfigOption(info.ACPCaps.ConfigOptions); ok {
		runtime.ReasoningEffort = strings.TrimSpace(option.CurrentValueID)
	} else if info.ACPCapsKnown {
		runtime.ReasoningEffort = ""
	}
	if current := acp.CurrentSpeed(info.ACPCaps.ConfigOptions); current != "" {
		runtime.Speed = current
	} else if info.SpeedResolution != nil && info.SpeedResolution.Status != speedpkg.ResolutionApplied {
		runtime.Speed = ""
	}
	return runtime
}

func (s *Session) runtimeStrategyLocked() acp.RuntimeApplicationStrategy {
	provider := strings.TrimSpace(s.providerRoute.RuntimeProvider)
	if provider == "" {
		provider = s.Provider
	}
	return RuntimeStrategyForProvider(provider)
}
