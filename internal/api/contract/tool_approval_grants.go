package contract

import (
	"time"

	toolspkg "github.com/compozy/agh/internal/tools"
)

// ToolApprovalGrantSetRequest creates or replaces one explicit wider remembered decision.
type ToolApprovalGrantSetRequest struct {
	ToolID    toolspkg.ToolID                       `json:"tool_id"`
	Decision  toolspkg.ApprovalGrantDecision        `json:"decision"`
	Scope     toolspkg.ApprovalGrantManagementScope `json:"scope"`
	AgentName string                                `json:"agent_name,omitempty"`
}

// Domain converts the transport request into the domain-owned validation shape.
func (r ToolApprovalGrantSetRequest) Domain() toolspkg.ApprovalGrantSetRequest {
	return toolspkg.ApprovalGrantSetRequest{
		ToolID:    r.ToolID,
		Decision:  r.Decision,
		Scope:     r.Scope,
		AgentName: r.AgentName,
	}
}

// ToolApprovalGrantPayload is one workspace-scoped remembered native-tool approval decision.
type ToolApprovalGrantPayload struct {
	ID          string                         `json:"id"`
	WorkspaceID string                         `json:"workspace_id"`
	AgentName   string                         `json:"agent_name,omitempty"`
	ToolID      toolspkg.ToolID                `json:"tool_id"`
	InputDigest string                         `json:"input_digest,omitempty"`
	Decision    toolspkg.ApprovalGrantDecision `json:"decision"`
	CreatedAt   time.Time                      `json:"created_at"`
	LastUsedAt  time.Time                      `json:"last_used_at"`
}

// ToolApprovalGrantListResponse wraps one workspace's remembered decisions.
type ToolApprovalGrantListResponse struct {
	Grants []ToolApprovalGrantPayload `json:"grants"`
	Total  int                        `json:"total"`
}

// ToolApprovalGrantResponse wraps one stored wider approval decision.
type ToolApprovalGrantResponse struct {
	Grant ToolApprovalGrantPayload `json:"grant"`
}

// ToolApprovalGrantPayloadFromDomain converts one durable decision into the shared transport shape.
func ToolApprovalGrantPayloadFromDomain(grant toolspkg.ApprovalGrant) ToolApprovalGrantPayload {
	return ToolApprovalGrantPayload{
		ID:          grant.ID,
		WorkspaceID: grant.WorkspaceID,
		AgentName:   grant.AgentName,
		ToolID:      grant.ToolID,
		InputDigest: grant.InputDigest,
		Decision:    grant.Decision,
		CreatedAt:   grant.CreatedAt.UTC(),
		LastUsedAt:  grant.LastUsedAt.UTC(),
	}
}
