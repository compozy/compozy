package session

import (
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// Meta returns the current metadata snapshot for persistence.
func (s *Session) Meta() store.SessionMeta {
	if s == nil {
		return store.SessionMeta{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.metaLocked()
}

func (s *Session) metaLocked() store.SessionMeta {
	profile := cloneCreationProfile(s.creationProfile)
	creationOptions := cloneCreationOptions(s.creationOptions)
	identity := cloneCreationIdentity(s.creationIdentity)
	meta := store.SessionMeta{
		ID:                s.ID,
		Name:              s.Name,
		AgentName:         s.AgentName,
		Provider:          s.Provider,
		Model:             s.Model,
		ReasoningEffort:   s.ReasoningEffort,
		Speed:             s.Speed,
		SpeedResolution:   speedpkg.CloneResolution(s.SpeedResolution),
		RuntimeStatus:     s.RuntimeStatus,
		RuntimeTransition: s.RuntimeTransition,
		RuntimeFailure:    store.SessionRuntimeFailurePointer(s.RuntimeFailure),
		RuntimeSelection: store.NewSessionRuntimeSelectionState(
			storeSessionRuntimeSelection(s.SelectedRuntime),
			s.RuntimeSelectionRevision,
		),
		EffectivePermissions: s.EffectivePermissions,
		WorkspaceID:          s.WorkspaceID,
		CWD:                  s.CWD,
		NetworkParticipation: participation.CloneSpec(s.NetworkParticipation),
		SessionType:          string(normalizeSessionType(s.Type)),
		Lineage:              store.NormalizeSessionLineage(s.ID, s.Lineage),
		State:                string(s.State),
		StopReason:           stopReasonPointer(s.stopReason),
		StopDetail:           s.stopDetail,
		Failure:              store.CloneSessionFailure(s.failure),
		ACPSessionID:         stringPointer(s.ACPSessionID),
		Liveness:             store.CloneSessionLivenessMeta(s.Liveness),
		Sandbox:              cloneSessionSandboxMeta(s.Sandbox),
		CreationProfile:      profile,
		CreationOptions:      creationOptions,
		SoulSnapshotID:       s.SoulSnapshotID,
		SoulDigest:           s.SoulDigest,
		ParentSoulDigest:     s.ParentSoulDigest,
		CreatedAt:            s.CreatedAt,
		UpdatedAt:            s.UpdatedAt,
	}
	meta.SetEffectiveProviderAuthMode(string(s.effectiveProviderAuthMode))
	meta.SetWorktreeID(s.WorktreeID)
	meta.SetAdvertisedCommands(s.AdvertisedCommands)
	if identity != nil {
		meta.CreationProfileRef = identity.CreationProfileRef
		meta.PolicySpecDigest = identity.PolicySpecDigest
		meta.CreationDigest = identity.CreationDigest
	}
	return meta
}

func (s *Session) meta() store.SessionMeta {
	return s.Meta()
}
