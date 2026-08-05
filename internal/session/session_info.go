package session

import (
	"strings"

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
	acpCaps := cloneCaps(s.ACPCaps)
	if s.process != nil {
		acpCaps = cloneCaps(s.process.CapsSnapshot())
	}
	pendingPermission := s.process != nil && s.process.HasPendingPermission()

	return &Info{
		ID:                       s.ID,
		Name:                     s.Name,
		AgentName:                s.AgentName,
		Provider:                 s.Provider,
		Model:                    s.Model,
		ReasoningEffort:          s.ReasoningEffort,
		Speed:                    s.Speed,
		SpeedResolution:          speedpkg.CloneResolution(s.SpeedResolution),
		RuntimeStatus:            s.RuntimeStatus,
		RuntimeTransition:        s.RuntimeTransition,
		RuntimeFailure:           strings.TrimSpace(s.RuntimeFailure),
		SelectedRuntime:          cloneRuntimeSelection(s.SelectedRuntime),
		RuntimeSelectionRevision: s.RuntimeSelectionRevision,
		EffectivePermissions:     s.EffectivePermissions,
		WorkspaceID:              s.WorkspaceID,
		Workspace:                s.Workspace,
		NetworkParticipation:     s.NetworkParticipation,
		NetworkOwnerKey:          s.NetworkOwnerKey,
		Type:                     normalizeSessionType(s.Type),
		Lineage:                  store.NormalizeSessionLineage(s.ID, s.Lineage),
		State:                    s.State,
		PendingPermission:        pendingPermission,
		StopReason:               s.stopReason,
		StopDetail:               s.stopDetail,
		Failure:                  store.CloneSessionFailure(s.failure),
		ACPSessionID:             s.ACPSessionID,
		ACPCaps:                  acpCaps,
		AdvertisedCommands:       store.CloneSessionAdvertisedCommands(s.AdvertisedCommands),
		Liveness:                 store.CloneSessionLivenessMeta(s.Liveness),
		Sandbox:                  cloneSessionSandboxMeta(s.Sandbox),
		SoulSnapshotID:           s.SoulSnapshotID,
		SoulDigest:               s.SoulDigest,
		ParentSoulDigest:         s.ParentSoulDigest,
		AttachedTo:               strings.TrimSpace(s.AttachedTo),
		AttachExpiresAt:          cloneSessionTimePtr(s.AttachExpiresAt),
		TranscriptEpoch:          s.TranscriptEpoch,
		ArchivedAt:               nil,
		CreatedAt:                s.CreatedAt,
		UpdatedAt:                s.UpdatedAt,
	}
}
