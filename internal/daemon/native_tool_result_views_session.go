package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

type compactSessionRecord struct {
	PermissionModeDegraded bool                                 `json:"permission_mode_degraded"`
	Branch                 string                               `json:"branch,omitempty"`
	PullRequest            *contract.SessionPullRequestPayload  `json:"pull_request"`
	BoardColumn            session.BoardColumn                  `json:"board_column"`
	WaitingOn              session.WaitingOn                    `json:"waiting_on,omitempty"`
	Children               *contract.SessionChildrenPayload     `json:"children,omitzero"`
	SubagentSummary        *contract.SubagentSummaryPayload     `json:"subagent_summary,omitzero"`
	Queue                  *contract.SessionQueueSummaryPayload `json:"queue,omitzero"`
	ID                     string                               `json:"id"`
	ProfileID              string                               `json:"profile_id"`
	Name                   string                               `json:"name,omitempty"`
	AgentName              string                               `json:"agent_name"`
	WorkspaceID            string                               `json:"workspace_id,omitempty"`
	WorktreeID             string                               `json:"worktree_id,omitempty"`
	Type                   session.Type                         `json:"type,omitempty"`
	State                  session.State                        `json:"state"`
	Badge                  session.Badge                        `json:"badge"`
	TranscriptEpoch        int64                                `json:"transcript_epoch,omitzero"`
	PendingInteractions    []contract.PendingInteractionPayload `json:"pending_interactions"`
	ArchivedAt             *time.Time                           `json:"archived_at"`
	Later                  *contract.SessionLaterPayload        `json:"later,omitempty"`
	StopReason             store.StopReason                     `json:"stop_reason,omitempty"`
	StopCause              string                               `json:"stop_cause,omitempty"`
	Attention              string                               `json:"attention,omitempty"`
	AttentionDismissible   bool                                 `json:"attention_dismissible,omitzero"`
	StopDetail             string                               `json:"stop_detail,omitempty"`
	Failure                *contract.SessionFailurePayload      `json:"failure,omitzero"`
	CreatedAt              time.Time                            `json:"created_at"`
	UpdatedAt              time.Time                            `json:"updated_at"`
	Runtime                compactSessionRuntime                `json:"runtime"`
	Lineage                *compactSessionLineage               `json:"lineage,omitempty"`
}

type compactSessionRuntime struct {
	Status            session.RuntimeStatus             `json:"status"`
	Failure           string                            `json:"failure,omitempty"`
	Effective         *contract.RuntimeSelectionPayload `json:"effective,omitempty"`
	Generation        int64                             `json:"generation,omitempty"`
	SelectionRevision int64                             `json:"selection_revision"`
}

type compactSessionLineage struct {
	Kind            store.LineageKind           `json:"kind,omitempty"`
	ParentSessionID string                      `json:"parent_session_id,omitempty"`
	RootSessionID   string                      `json:"root_session_id,omitempty"`
	TTLExpiresAt    *time.Time                  `json:"ttl_expires_at,omitempty"`
	SpawnBudget     contract.SpawnBudgetPayload `json:"spawn_budget"`
}

func compactSession(p *contract.SessionPayload) compactSessionRecord {
	result := compactSessionRecord{
		PermissionModeDegraded: p.PermissionModeDegraded,
		Branch:                 p.Branch,
		PullRequest:            p.PullRequest,
		BoardColumn:            p.BoardColumn,
		WaitingOn:              p.WaitingOn,
		Children:               p.Children,
		SubagentSummary:        p.SubagentSummary,
		Queue:                  p.Queue,
		ID:                     p.ID,
		ProfileID:              p.ProfileID,
		Name:                   p.Name,
		AgentName:              p.AgentName,
		WorkspaceID:            p.WorkspaceID,
		WorktreeID:             p.WorktreeID,
		Type:                   p.Type,
		State:                  p.State,
		Badge:                  p.Badge,
		TranscriptEpoch:        p.TranscriptEpoch,
		PendingInteractions:    p.PendingInteractions,
		ArchivedAt:             p.ArchivedAt,
		Later:                  p.Later,
		StopReason:             p.StopReason,
		StopCause:              p.StopCause,
		Attention:              p.Attention,
		AttentionDismissible:   p.AttentionDismissible,
		StopDetail:             p.StopDetail,
		Failure:                p.Failure,
		CreatedAt:              p.CreatedAt,
		UpdatedAt:              p.UpdatedAt,
		Runtime: compactSessionRuntime{
			Status:            p.Runtime.Status,
			Failure:           p.Runtime.Failure,
			Effective:         p.Runtime.Effective,
			Generation:        p.Runtime.Generation,
			SelectionRevision: p.Runtime.SelectionRevision,
		},
	}
	if p.Lineage != nil {
		result.Lineage = &compactSessionLineage{
			Kind:            p.Lineage.Kind,
			ParentSessionID: p.Lineage.ParentSessionID,
			RootSessionID:   p.Lineage.RootSessionID,
			TTLExpiresAt:    p.Lineage.TTLExpiresAt,
			SpawnBudget:     p.Lineage.SpawnBudget,
		}
	}
	return result
}

var sessionRecordViews = resultViews[contract.SessionPayload]{
	Compact: func(p contract.SessionPayload) any { return map[string]any{nativeToolsSessionKey: compactSession(&p)} },
	Full:    func(p contract.SessionPayload) any { return map[string]any{nativeToolsSessionKey: p} },
	Preview: func(p contract.SessionPayload) string { return p.ID },
}

var sessionSpawnViews = resultViews[map[string]any]{
	Compact: func(p map[string]any) any {
		result := make(map[string]any, len(p))
		for key, value := range p {
			if key != "permissions" {
				result[key] = value
			}
		}
		return result
	},
	Preview: func(p map[string]any) string {
		id, ok := p[watchEventsPayloadSessionIDKey].(string)
		if !ok {
			return ""
		}
		return id
	},
}
