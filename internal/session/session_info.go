package session

import (
	"strings"

	"github.com/compozy/agh/internal/store"
)

// Info returns a consistent snapshot of the current session state.
func (s *Session) Info() *Info {
	if s == nil {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	acpCaps := cloneCaps(s.ACPCaps)
	if s.process != nil {
		acpCaps = cloneCaps(s.process.CapsSnapshot())
	}

	return &Info{
		ID:                   s.ID,
		Name:                 s.Name,
		AgentName:            s.AgentName,
		Provider:             s.Provider,
		Model:                s.Model,
		ReasoningEffort:      s.ReasoningEffort,
		WorkspaceID:          s.WorkspaceID,
		Workspace:            s.Workspace,
		NetworkParticipation: s.NetworkParticipation,
		NetworkOwnerKey:      s.NetworkOwnerKey,
		Type:                 normalizeSessionType(s.Type),
		Lineage:              store.NormalizeSessionLineage(s.ID, s.Lineage),
		State:                s.State,
		StopReason:           s.stopReason,
		StopDetail:           s.stopDetail,
		Failure:              store.CloneSessionFailure(s.failure),
		ACPSessionID:         s.ACPSessionID,
		ACPCaps:              acpCaps,
		AdvertisedCommands:   store.CloneSessionAdvertisedCommands(s.AdvertisedCommands),
		Liveness:             store.CloneSessionLivenessMeta(s.Liveness),
		Sandbox:              cloneSessionSandboxMeta(s.Sandbox),
		SoulSnapshotID:       s.SoulSnapshotID,
		SoulDigest:           s.SoulDigest,
		ParentSoulDigest:     s.ParentSoulDigest,
		AttachedTo:           strings.TrimSpace(s.AttachedTo),
		AttachExpiresAt:      cloneSessionTimePtr(s.AttachExpiresAt),
		TranscriptEpoch:      s.TranscriptEpoch,
		CreatedAt:            s.CreatedAt,
		UpdatedAt:            s.UpdatedAt,
	}
}
