package contract

import (
	"time"

	"github.com/compozy/compozy/internal/session"
)

// AgentSessionPayload is the compact session context used by agent endpoints.
type AgentSessionPayload struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	Type      session.Type           `json:"type,omitempty"`
	State     session.State          `json:"state"`
	Lineage   *SessionLineagePayload `json:"lineage,omitzero"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// NormalizeAgentSessionPayload clones nested snapshots and normalizes lineage lists.
func NormalizeAgentSessionPayload(payload AgentSessionPayload) AgentSessionPayload {
	payload.Lineage = NormalizeSessionLineagePayload(payload.Lineage)
	return payload
}
