package daemon

import (
	"time"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type compactApprovalGrant struct {
	ID          string                         `json:"id"`
	Decision    toolspkg.ApprovalGrantDecision `json:"decision"`
	CreatedAt   time.Time                      `json:"created_at"`
	LastUsedAt  time.Time                      `json:"last_used_at"`
	ProfileID   string                         `json:"profile_id"`
	WorkspaceID string                         `json:"workspace_id"`
	AgentName   string                         `json:"agent_name,omitempty"`
	ToolID      toolspkg.ToolID                `json:"tool_id"`
	InputDigest string                         `json:"input_digest,omitempty"`
	AccessScope string                         `json:"access_scope,omitempty"`
}

func projectCompactApprovalGrant(p toolspkg.ApprovalGrant) compactApprovalGrant {
	return compactApprovalGrant{
		ID:          p.ID,
		Decision:    p.Decision,
		CreatedAt:   p.CreatedAt,
		LastUsedAt:  p.LastUsedAt,
		ProfileID:   p.ProfileID,
		WorkspaceID: p.WorkspaceID,
		AgentName:   p.AgentName,
		ToolID:      p.ToolID,
		InputDigest: p.InputDigest,
		AccessScope: p.AccessScope,
	}
}

type compactToolDecision struct {
	Callable         bool                  `json:"callable"`
	ApprovalRequired bool                  `json:"approval_required"`
	ReasonCodes      []toolspkg.ReasonCode `json:"reason_codes,omitempty"`
}

func projectCompactToolDecision(p toolspkg.EffectiveToolDecision) compactToolDecision {
	return compactToolDecision{
		Callable:         p.Callable,
		ApprovalRequired: p.ApprovalRequired,
		ReasonCodes:      p.ReasonCodes,
	}
}

type compactToolListDescriptor struct {
	ID             toolspkg.ToolID    `json:"id"`
	HasResultViews bool               `json:"has_result_views,omitzero"`
	Risk           toolspkg.RiskClass `json:"risk"`
}

func projectCompactToolListDescriptor(p nativeToolListDescriptor) compactToolListDescriptor {
	return compactToolListDescriptor{
		ID:             p.ID,
		HasResultViews: p.HasResultViews,
		Risk:           p.Risk,
	}
}

type compactSearchViewContract struct {
	Digest string `json:"digest,omitempty"`
}
type compactSearchViews struct {
	Compact compactSearchViewContract `json:"compact"`
	Full    compactSearchViewContract `json:"full"`
}

type compactToolSearchDescriptor struct {
	OutputViews *compactSearchViews `json:"output_views,omitempty"`
	ID          toolspkg.ToolID     `json:"id"`
	Description string              `json:"description"`
	Risk        toolspkg.RiskClass  `json:"risk"`
}

func projectCompactToolSearchDescriptor(p toolspkg.Descriptor) compactToolSearchDescriptor {
	result := compactToolSearchDescriptor{
		ID:          p.ID,
		Description: p.Description,
		Risk:        p.Risk,
	}
	if views := p.ResultViews(); views != nil {
		result.OutputViews = &compactSearchViews{
			Compact: compactSearchViewContract{Digest: views.Compact.Digest},
			Full:    compactSearchViewContract{Digest: views.Full.Digest},
		}
	}
	return result
}
