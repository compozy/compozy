package contract

import (
	"time"

	"github.com/compozy/compozy/internal/store"
)

// SessionLineagePayloadFromStore converts durable lineage for an authorized
// agent surface that needs the concrete permission atoms.
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
		NotifyCreator:    normalized.NotifyCreator,
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

// SessionLineagePayloadForOperatorFromStore omits internal capability atoms
// from operator-facing HTTP, UDS, support, and browser artifact surfaces.
func SessionLineagePayloadForOperatorFromStore(lineage *store.SessionLineage) *SessionLineagePayload {
	payload := SessionLineagePayloadFromStore(lineage)
	if payload == nil {
		return nil
	}
	payload.PermissionPolicy = SpawnPermissionPolicyPayload{
		Tools: []string{}, Skills: []string{}, MCPServers: []string{},
		WorkspacePaths: []string{}, NetworkChannels: []string{}, SandboxProfiles: []string{},
	}
	return payload
}

func cloneContractTimePtr(source *time.Time) *time.Time {
	if source == nil {
		return nil
	}
	value := source.UTC()
	return &value
}
