package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	// ErrApprovalGrantInvalid reports an invalid durable native-tool approval grant.
	ErrApprovalGrantInvalid = errors.New("tools: invalid approval grant")
	// ErrApprovalGrantNotFound reports a missing grant without exposing another workspace's rows.
	ErrApprovalGrantNotFound = errors.New("tools: approval grant not found")
)

// ApprovalGrantDecision is the remembered decision for one native-tool approval key.
type ApprovalGrantDecision string

const (
	ApprovalGrantAllow  ApprovalGrantDecision = "allow"
	ApprovalGrantReject ApprovalGrantDecision = "reject"
)

// ApprovalGrantManagementScope is an explicit wider-grant boundary.
type ApprovalGrantManagementScope string

const (
	// ApprovalGrantScopeAgent remembers one tool decision for every input from one agent.
	ApprovalGrantScopeAgent ApprovalGrantManagementScope = "agent"
	// ApprovalGrantScopeTool remembers one tool decision for every agent and input.
	ApprovalGrantScopeTool ApprovalGrantManagementScope = "tool"
)

// ApprovalGrantSetRequest creates or replaces one explicit wider approval decision.
// Exact input grants remain exclusive to prompt-origin allow_always/reject_always answers.
type ApprovalGrantSetRequest struct {
	ToolID    ToolID                       `json:"tool_id"`
	Decision  ApprovalGrantDecision        `json:"decision"`
	Scope     ApprovalGrantManagementScope `json:"scope"`
	AgentName string                       `json:"agent_name,omitempty"`
}

// Normalize returns a canonical wider-grant request.
func (r ApprovalGrantSetRequest) Normalize() ApprovalGrantSetRequest {
	r.ToolID = ToolID(strings.TrimSpace(r.ToolID.String()))
	r.Decision = ApprovalGrantDecision(strings.TrimSpace(string(r.Decision)))
	r.Scope = ApprovalGrantManagementScope(strings.TrimSpace(string(r.Scope)))
	r.AgentName = strings.TrimSpace(r.AgentName)
	return r
}

// BuildGrant validates the explicit management boundary and returns its durable wider key.
func (r ApprovalGrantSetRequest) BuildGrant(workspaceID string) (ApprovalGrant, error) {
	r = r.Normalize()
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return ApprovalGrant{}, fmt.Errorf("%w: workspace_id is required", ErrApprovalGrantInvalid)
	}
	if err := r.ToolID.Validate(); err != nil {
		return ApprovalGrant{}, fmt.Errorf("%w: tool_id: %v", ErrApprovalGrantInvalid, err)
	}
	switch r.Decision {
	case ApprovalGrantAllow, ApprovalGrantReject:
	default:
		return ApprovalGrant{}, fmt.Errorf("%w: decision must be allow or reject", ErrApprovalGrantInvalid)
	}
	switch r.Scope {
	case ApprovalGrantScopeAgent:
		if r.AgentName == "" {
			return ApprovalGrant{}, fmt.Errorf("%w: agent_name is required for agent scope", ErrApprovalGrantInvalid)
		}
	case ApprovalGrantScopeTool:
		if r.AgentName != "" {
			return ApprovalGrant{}, fmt.Errorf("%w: agent_name is forbidden for tool scope", ErrApprovalGrantInvalid)
		}
	default:
		return ApprovalGrant{}, fmt.Errorf("%w: scope must be agent or tool", ErrApprovalGrantInvalid)
	}
	grant := ApprovalGrant{
		ApprovalGrantKey: ApprovalGrantKey{
			WorkspaceID: workspaceID,
			AgentName:   r.AgentName,
			ToolID:      r.ToolID,
		},
		Decision: r.Decision,
	}
	if err := grant.ValidateForPut(); err != nil {
		return ApprovalGrant{}, err
	}
	return grant, nil
}

// ApprovalGrantKey identifies one durable native-tool approval decision.
type ApprovalGrantKey struct {
	WorkspaceID string `json:"workspace_id"`
	AgentName   string `json:"agent_name,omitempty"`
	ToolID      ToolID `json:"tool_id"`
	InputDigest string `json:"input_digest,omitempty"`
}

// Normalize returns a canonical approval grant key.
func (k ApprovalGrantKey) Normalize() ApprovalGrantKey {
	k.WorkspaceID = strings.TrimSpace(k.WorkspaceID)
	k.AgentName = strings.TrimSpace(k.AgentName)
	k.ToolID = ToolID(strings.TrimSpace(k.ToolID.String()))
	k.InputDigest = strings.TrimSpace(k.InputDigest)
	return k
}

// Validate checks the fields required by every durable approval lookup.
func (k ApprovalGrantKey) Validate() error {
	k = k.Normalize()
	if k.WorkspaceID == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrApprovalGrantInvalid)
	}
	if err := k.ToolID.Validate(); err != nil {
		return fmt.Errorf("%w: tool_id: %v", ErrApprovalGrantInvalid, err)
	}
	if k.InputDigest != "" && !strings.HasPrefix(k.InputDigest, "sha256:") {
		return fmt.Errorf("%w: input_digest must use sha256", ErrApprovalGrantInvalid)
	}
	return nil
}

// ApprovalGrant is one durable native-tool approval decision.
type ApprovalGrant struct {
	ID string `json:"id"`
	ApprovalGrantKey
	Decision   ApprovalGrantDecision `json:"decision"`
	CreatedAt  time.Time             `json:"created_at"`
	LastUsedAt time.Time             `json:"last_used_at"`
}

// Normalize returns a canonical durable grant.
func (g ApprovalGrant) Normalize() ApprovalGrant {
	g.ID = strings.TrimSpace(g.ID)
	g.ApprovalGrantKey = g.ApprovalGrantKey.Normalize()
	if !g.CreatedAt.IsZero() {
		g.CreatedAt = g.CreatedAt.UTC()
	}
	if !g.LastUsedAt.IsZero() {
		g.LastUsedAt = g.LastUsedAt.UTC()
	}
	return g
}

// ValidateForPut checks a grant before the repository assigns identity and timestamps.
func (g ApprovalGrant) ValidateForPut() error {
	g = g.Normalize()
	if err := g.ApprovalGrantKey.Validate(); err != nil {
		return err
	}
	switch g.Decision {
	case ApprovalGrantAllow, ApprovalGrantReject:
		return nil
	default:
		return fmt.Errorf("%w: decision must be allow or reject", ErrApprovalGrantInvalid)
	}
}

// Validate checks a fully materialized grant.
func (g ApprovalGrant) Validate() error {
	g = g.Normalize()
	if err := g.ValidateForPut(); err != nil {
		return err
	}
	if g.ID == "" {
		return fmt.Errorf("%w: id is required", ErrApprovalGrantInvalid)
	}
	if g.CreatedAt.IsZero() || g.LastUsedAt.IsZero() {
		return fmt.Errorf("%w: created_at and last_used_at are required", ErrApprovalGrantInvalid)
	}
	return nil
}

// ApprovalGrantStore persists remembered native-tool approval decisions.
type ApprovalGrantStore interface {
	LookupApprovalGrant(ctx context.Context, key ApprovalGrantKey) (ApprovalGrant, bool, error)
	PutApprovalGrant(ctx context.Context, grant ApprovalGrant) (ApprovalGrant, error)
	ListApprovalGrants(ctx context.Context, workspaceID string) ([]ApprovalGrant, error)
	RevokeApprovalGrant(ctx context.Context, workspaceID, id string) error
}
