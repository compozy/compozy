package daemon

import (
	"strings"

	"github.com/compozy/agh/internal/store"
)

func sessionInfoFromSandboxReconcileMeta(meta store.SessionMeta) store.SessionInfo {
	stopReason := store.StopReason("")
	if meta.StopReason != nil {
		stopReason = *meta.StopReason
	}
	info := store.SessionInfo{
		ID:           strings.TrimSpace(meta.ID),
		Name:         strings.TrimSpace(meta.Name),
		AgentName:    strings.TrimSpace(meta.AgentName),
		Provider:     strings.TrimSpace(meta.Provider),
		WorkspaceID:  strings.TrimSpace(meta.WorkspaceID),
		SessionType:  strings.TrimSpace(meta.SessionType),
		Lineage:      store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		State:        strings.TrimSpace(meta.State),
		ACPSessionID: cloneDaemonStringPointer(meta.ACPSessionID),
		StopReason:   stopReason,
		StopDetail:   strings.TrimSpace(meta.StopDetail),
		Sandbox:      cloneDaemonSessionSandboxMeta(meta.Sandbox),
		CreatedAt:    meta.CreatedAt,
		UpdatedAt:    meta.UpdatedAt,
	}
	info.SetNetworkSpec(meta.NetworkSpecSnapshot())
	return info
}
