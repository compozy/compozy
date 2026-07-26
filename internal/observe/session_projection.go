package observe

import (
	"strings"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func sessionInfoFromSession(info *session.Info) store.SessionInfo {
	if info == nil {
		return store.SessionInfo{}
	}

	result := store.SessionInfo{
		ID:               info.ID,
		Name:             info.Name,
		AgentName:        info.AgentName,
		Provider:         info.Provider,
		WorkspaceID:      info.WorkspaceID,
		SessionType:      string(info.Type),
		Lineage:          store.CloneSessionLineage(info.Lineage),
		State:            string(info.State),
		ACPSessionID:     stringPointer(info.ACPSessionID),
		StopReason:       info.StopReason,
		StopDetail:       info.StopDetail,
		Failure:          store.CloneSessionFailure(info.Failure),
		Liveness:         store.CloneSessionLivenessMeta(info.Liveness),
		Sandbox:          cloneSessionSandboxMeta(info.Sandbox),
		SoulSnapshotID:   strings.TrimSpace(info.SoulSnapshotID),
		SoulDigest:       strings.TrimSpace(info.SoulDigest),
		ParentSoulDigest: strings.TrimSpace(info.ParentSoulDigest),
		TranscriptEpoch:  info.TranscriptEpoch,
		CreatedAt:        info.CreatedAt,
		UpdatedAt:        info.UpdatedAt,
	}
	result.SetNetworkSpec(info.NetworkParticipation)
	return result
}
