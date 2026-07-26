package contract

import (
	"time"

	"github.com/compozy/agh/internal/store"
)

// SessionLineagePayloadFromStore converts durable session lineage metadata into
// the safe public payload used across daemon read surfaces.
func SessionLineagePayloadFromStore(lineage *store.SessionLineage) *SessionLineagePayload {
	if lineage == nil {
		return nil
	}
	normalized := store.NormalizeSessionLineage("", lineage)
	payload := &SessionLineagePayload{
		ParentSessionID:  normalized.ParentSessionID,
		RootSessionID:    normalized.RootSessionID,
		SpawnDepth:       normalized.SpawnDepth,
		SpawnRole:        normalized.SpawnRole,
		TTLExpiresAt:     cloneContractTimePtr(normalized.TTLExpiresAt),
		AutoStopOnParent: normalized.AutoStopOnParent,
		SpawnBudget: SpawnBudgetPayload{
			MaxChildren:           normalized.SpawnBudget.MaxChildren,
			MaxDepth:              normalized.SpawnBudget.MaxDepth,
			TTLSeconds:            normalized.SpawnBudget.TTLSeconds,
			MaxActivePerWorkspace: normalized.SpawnBudget.MaxActivePerWorkspace,
		},
		PermissionPolicy: SpawnPermissionPolicyPayload{
			Tools:           append([]string(nil), normalized.PermissionPolicy.Tools...),
			Skills:          append([]string(nil), normalized.PermissionPolicy.Skills...),
			MCPServers:      append([]string(nil), normalized.PermissionPolicy.MCPServers...),
			WorkspacePaths:  append([]string(nil), normalized.PermissionPolicy.WorkspacePaths...),
			NetworkChannels: append([]string(nil), normalized.PermissionPolicy.NetworkChannels...),
			SandboxProfiles: append([]string(nil), normalized.PermissionPolicy.SandboxProfiles...),
		},
	}
	return NormalizeSessionLineagePayload(payload)
}

func cloneContractTimePtr(source *time.Time) *time.Time {
	if source == nil {
		return nil
	}
	value := source.UTC()
	return &value
}
