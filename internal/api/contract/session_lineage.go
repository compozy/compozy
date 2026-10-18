package contract

import (
	"time"

	"github.com/compozy/compozy/internal/store"
)

// SessionDerivationPayload summarizes how a continued or forked session was seeded.
// It is present only on sessions whose lineage kind is continue or fork.
type SessionDerivationPayload struct {
	Kind            store.LineageKind `json:"kind"`
	SourceSessionID string            `json:"source_session_id"`
	Seed            string            `json:"seed"`
	NativeState     string            `json:"native_state,omitempty"`
	NativeForkError string            `json:"native_fork_error,omitempty"`
	FirstPrompt     string            `json:"first_prompt"`
}

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
		Kind:             normalized.Kind,
		OriginMessageID:  normalized.OriginMessageID,
		OriginAgentName:  normalized.OriginAgentName,
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
			Tools:          append([]string(nil), normalized.PermissionPolicy.Tools...),
			Skills:         append([]string(nil), normalized.PermissionPolicy.Skills...),
			MCPServers:     append([]string(nil), normalized.PermissionPolicy.MCPServers...),
			WorkspacePaths: append([]string(nil), normalized.PermissionPolicy.WorkspacePaths...),
		},
	}
	return NormalizeSessionLineagePayload(payload)
}

func cloneContractTimePtr(source *time.Time) *time.Time {
	if source == nil {
		return nil
	}
	return new(source.UTC())
}
