package daemon

import (
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func sessionInfoFromSandboxReconcileMeta(meta store.SessionMeta) store.SessionInfo {
	stopReason := store.StopReason("")
	if meta.StopReason != nil {
		stopReason = *meta.StopReason
	}
	info := store.SessionInfo{
		ID:                strings.TrimSpace(meta.ID),
		Name:              strings.TrimSpace(meta.Name),
		AgentName:         strings.TrimSpace(meta.AgentName),
		Provider:          strings.TrimSpace(meta.Provider),
		Model:             strings.TrimSpace(meta.Model),
		ReasoningEffort:   strings.TrimSpace(meta.ReasoningEffort),
		Speed:             meta.Speed,
		SpeedResolution:   speedpkg.CloneResolution(meta.SpeedResolution),
		RuntimeStatus:     meta.RuntimeStatus,
		RuntimeTransition: meta.RuntimeTransition,
		RuntimeFailure:    store.SessionRuntimeFailureValue(meta.RuntimeFailure),
		WorkspaceID:       strings.TrimSpace(meta.WorkspaceID),
		SessionType:       strings.TrimSpace(meta.SessionType),
		Lineage:           store.NormalizeSessionLineage(meta.ID, meta.Lineage),
		State:             strings.TrimSpace(meta.State),
		ACPSessionID:      cloneDaemonStringPointer(meta.ACPSessionID),
		StopReason:        stopReason,
		StopDetail:        strings.TrimSpace(meta.StopDetail),
		Sandbox:           cloneDaemonSessionSandboxMeta(meta.Sandbox),
		CreatedAt:         meta.CreatedAt,
		UpdatedAt:         meta.UpdatedAt,
	}
	info.SetNetworkSpec(meta.NetworkSpecSnapshot())
	return info
}
