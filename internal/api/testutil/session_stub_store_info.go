package testutil

import (
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func storeSessionInfoFromRuntime(info *session.Info) store.SessionInfo {
	storeInfo := store.SessionInfo{
		ID:                info.ID,
		Name:              info.Name,
		AgentName:         info.AgentName,
		Provider:          info.Provider,
		Model:             info.Model,
		ReasoningEffort:   info.ReasoningEffort,
		Speed:             info.Speed,
		SpeedResolution:   speedpkg.CloneResolution(info.SpeedResolution),
		RuntimeStatus:     info.RuntimeStatus,
		RuntimeTransition: info.RuntimeTransition,
		RuntimeFailure:    info.RuntimeFailure,
		WorkspaceID:       info.WorkspaceID,
		SessionNetworkState: &store.SessionNetworkState{
			NetworkSpec: info.NetworkParticipation,
		},
		SessionType:      string(info.Type),
		Lineage:          info.Lineage,
		State:            string(info.State),
		StopReason:       info.StopReason,
		StopDetail:       info.StopDetail,
		Failure:          info.Failure,
		Liveness:         info.Liveness,
		Sandbox:          info.Sandbox,
		SoulSnapshotID:   info.SoulSnapshotID,
		SoulDigest:       info.SoulDigest,
		ParentSoulDigest: info.ParentSoulDigest,
		TranscriptEpoch:  info.TranscriptEpoch,
		CreatedAt:        info.CreatedAt,
		UpdatedAt:        info.UpdatedAt,
	}
	storeInfo.SetAttach(info.AttachedTo, info.AttachExpiresAt)
	if info.ACPSessionID != "" {
		acpSessionID := info.ACPSessionID
		storeInfo.ACPSessionID = &acpSessionID
	}
	return storeInfo
}
