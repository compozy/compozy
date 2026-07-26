package session

import (
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

// Meta returns the current metadata snapshot for persistence.
func (s *Session) Meta() store.SessionMeta {
	if s == nil {
		return store.SessionMeta{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	profile := cloneCreationProfile(s.creationProfile)
	creationOptions := cloneCreationOptions(s.creationOptions)
	identity := cloneCreationIdentity(s.creationIdentity)
	meta := store.SessionMeta{
		ID:                   s.ID,
		Name:                 s.Name,
		AgentName:            s.AgentName,
		Provider:             s.Provider,
		Model:                s.Model,
		ReasoningEffort:      s.ReasoningEffort,
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
		AdvertisedCommands:   store.CloneSessionAdvertisedCommands(s.AdvertisedCommands),
		SoulSnapshotID:       s.SoulSnapshotID,
		SoulDigest:           s.SoulDigest,
		ParentSoulDigest:     s.ParentSoulDigest,
		CreatedAt:            s.CreatedAt,
		UpdatedAt:            s.UpdatedAt,
	}
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
