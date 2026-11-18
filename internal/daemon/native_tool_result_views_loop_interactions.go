package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

type compactLoopRequest struct {
	WorkspaceID      string     `json:"workspace_id,omitempty"`
	ProfileID        string     `json:"profile_id"`
	LoopRunID        string     `json:"loop_run_id"`
	LoopName         string     `json:"loop_name,omitempty"`
	Generation       int        `json:"generation"`
	NodeID           string     `json:"node_id"`
	ItemIndex        int        `json:"item_index"`
	Kind             string     `json:"kind"`
	State            string     `json:"state"`
	Prompt           string     `json:"prompt"`
	Decisions        []string   `json:"decisions"`
	Agents           string     `json:"agents"`
	AnsweredDecision string     `json:"answered_decision,omitempty"`
	ActorKind        string     `json:"actor_kind,omitempty"`
	ActorID          string     `json:"actor_id,omitempty"`
	OpenedAt         time.Time  `json:"opened_at"`
	AnsweredAt       *time.Time `json:"answered_at,omitempty"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
}

func projectCompactLoopRequest(p *contract.LoopRequestPayload) *compactLoopRequest {
	if p == nil {
		return nil
	}
	result := &compactLoopRequest{
		WorkspaceID:      p.WorkspaceID,
		ProfileID:        p.ProfileID,
		LoopRunID:        p.LoopRunID,
		LoopName:         p.LoopName,
		Generation:       p.Generation,
		NodeID:           p.NodeID,
		ItemIndex:        p.ItemIndex,
		Kind:             p.Kind,
		State:            p.State,
		Prompt:           p.Prompt,
		Decisions:        p.Decisions,
		Agents:           p.Agents,
		AnsweredDecision: p.AnsweredDecision,
		ActorKind:        p.ActorKind,
		ActorID:          p.ActorID,
		OpenedAt:         p.OpenedAt,
		AnsweredAt:       p.AnsweredAt,
		ResolvedAt:       p.ResolvedAt,
		ExpiresAt:        p.ExpiresAt,
	}
	return result
}

type compactLoopRequests struct {
	Items      []compactLoopRequest           `json:"items"`
	Aggregates contract.LoopRequestAggregates `json:"aggregates"`
	NextCursor string                         `json:"next_cursor"`
}

func viewLoopRequests(p contract.LoopRequestsResponse) any {
	var items []compactLoopRequest
	if p.Items != nil {
		items = make([]compactLoopRequest, len(p.Items))
		for i := range p.Items {
			items[i] = *projectCompactLoopRequest(&p.Items[i])
		}
	}
	return compactLoopRequests{Items: items, Aggregates: p.Aggregates, NextCursor: p.NextCursor}
}

type compactLoopAmendment struct {
	LoopRunID       string                              `json:"loop_run_id"`
	Generation      int                                 `json:"generation"`
	NodeID          string                              `json:"node_id"`
	ItemIndex       int                                 `json:"item_index"`
	Sequence        int                                 `json:"amendment_seq"`
	OriginalSummary *contract.LoopAmendmentValueSummary `json:"original_summary,omitzero"`
	AmendedSummary  *contract.LoopAmendmentValueSummary `json:"amended_summary,omitzero"`
	ActorKind       string                              `json:"actor_kind"`
	ActorID         string                              `json:"actor_id"`
	Reason          string                              `json:"reason,omitempty"`
	CreatedAt       time.Time                           `json:"created_at"`
}

func projectCompactLoopAmendment(p *contract.LoopNodeAmendmentPayload) *compactLoopAmendment {
	if p == nil {
		return nil
	}
	result := &compactLoopAmendment{
		LoopRunID:       p.LoopRunID,
		Generation:      p.Generation,
		NodeID:          p.NodeID,
		ItemIndex:       p.ItemIndex,
		Sequence:        p.Sequence,
		OriginalSummary: p.OriginalSummary,
		AmendedSummary:  p.AmendedSummary,
		ActorKind:       p.ActorKind,
		ActorID:         p.ActorID,
		Reason:          p.Reason,
		CreatedAt:       p.CreatedAt,
	}
	return result
}

type compactLoopAmend struct {
	OK        bool                 `json:"ok"`
	Amendment compactLoopAmendment `json:"amendment"`
}

func projectCompactLoopAmend(p *contract.LoopNodeAmendResponse) *compactLoopAmend {
	if p == nil {
		return nil
	}
	result := &compactLoopAmend{
		OK: p.OK,
	}
	result.Amendment = *projectCompactLoopAmendment(&p.Amendment)
	return result
}

func viewLoopAmend(p contract.LoopNodeAmendResponse) any { return projectCompactLoopAmend(&p) }

type compactLoopInventoryItem struct {
	WorkspaceID string                           `json:"workspace_id,omitempty"`
	State       contract.LoopNodeInventoryState  `json:"state"`
	LoopRunID   string                           `json:"loop_run_id"`
	LoopName    string                           `json:"loop_name"`
	Generation  int                              `json:"generation"`
	NodeID      string                           `json:"node_id"`
	ItemIndex   int                              `json:"item_index"`
	StateAt     time.Time                        `json:"state_at"`
	Wait        *contract.LoopNodeWaitPayload    `json:"wait,omitzero"`
	Control     *contract.LoopNodeControlPayload `json:"control,omitzero"`
	Output      *compactLoopOutput               `json:"output,omitzero"`
}

func projectCompactLoopInventoryItem(p *contract.LoopNodeInventoryItem) *compactLoopInventoryItem {
	if p == nil {
		return nil
	}
	result := &compactLoopInventoryItem{
		WorkspaceID: p.WorkspaceID,
		State:       p.State,
		LoopRunID:   p.LoopRunID,
		LoopName:    p.LoopName,
		Generation:  p.Generation,
		NodeID:      p.NodeID,
		ItemIndex:   p.ItemIndex,
		StateAt:     p.StateAt,
		Wait:        p.Wait,
		Control:     p.Control,
	}
	result.Output = projectCompactLoopOutput(p.Output)
	return result
}

type compactLoopNodes struct {
	Items      []compactLoopInventoryItem `json:"items"`
	NextCursor string                     `json:"next_cursor,omitempty"`
}

func projectCompactLoopNodes(p *contract.LoopNodeInventoryResponse) *compactLoopNodes {
	if p == nil {
		return nil
	}
	result := &compactLoopNodes{
		NextCursor: p.NextCursor,
	}
	if p.Items != nil {
		result.Items = make([]compactLoopInventoryItem, len(p.Items))
		for i := range p.Items {
			result.Items[i] = *projectCompactLoopInventoryItem(&p.Items[i])
		}
	}
	return result
}

func viewLoopNodes(p contract.LoopNodeInventoryResponse) any { return projectCompactLoopNodes(&p) }
