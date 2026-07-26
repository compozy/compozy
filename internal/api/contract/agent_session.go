package contract

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
)

// AgentSessionPayload is the compact session context used by agent endpoints.
type AgentSessionPayload struct {
	ID                           string                 `json:"id"`
	Name                         string                 `json:"name,omitempty"`
	Type                         session.Type           `json:"type,omitempty"`
	State                        session.State          `json:"state"`
	ResolvedNetworkParticipation *participation.Spec    `json:"resolved_network_participation,omitempty"`
	Lineage                      *SessionLineagePayload `json:"lineage,omitempty"`
	CreatedAt                    time.Time              `json:"created_at"`
	UpdatedAt                    time.Time              `json:"updated_at"`
}

// NormalizeAgentSessionPayload clones nested snapshots and normalizes lineage lists.
func NormalizeAgentSessionPayload(payload AgentSessionPayload) AgentSessionPayload {
	if payload.ResolvedNetworkParticipation != nil {
		payload.ResolvedNetworkParticipation = participation.CloneSpec(*payload.ResolvedNetworkParticipation)
	}
	payload.Lineage = NormalizeSessionLineagePayload(payload.Lineage)
	return payload
}
