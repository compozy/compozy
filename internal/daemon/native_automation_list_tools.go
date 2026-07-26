package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) automationJobsList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobsListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := input.query(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	page, err := n.automationManager().ListJobs(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload, err := n.automationJobPayloads(ctx, page.Jobs)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	response := contract.JobsResponse{
		Jobs: payload,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}
	return structuredResult(response, fmt.Sprintf("%d of %d automation jobs", len(payload), page.Total))
}

func (n *daemonNativeTools) automationTriggersList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationTriggersListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := input.query(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	page, err := n.automationManager().ListTriggers(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.TriggerPayloadsFromTriggers(page.Triggers)
	response := contract.TriggersResponse{
		Triggers: payload,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}
	return structuredResult(response, fmt.Sprintf("%d of %d automation triggers", len(payload), page.Total))
}

type automationJobsListInput struct {
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Source      string `json:"source,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	LoopName    string `json:"loop,omitempty"`
	Query       string `json:"q,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (i automationJobsListInput) query(id toolspkg.ToolID) (automationpkg.JobListQuery, error) {
	query := automationpkg.JobListQuery{
		WorkspaceID: strings.TrimSpace(i.WorkspaceID),
		LoopName:    strings.TrimSpace(i.LoopName),
		Enabled:     i.Enabled,
		Search:      strings.TrimSpace(i.Query),
		Cursor:      strings.TrimSpace(i.Cursor),
		Limit:       i.Limit,
	}
	if query.Limit == 0 {
		query.Limit = automationpkg.DefaultListLimit
	}
	if scope := strings.TrimSpace(i.Scope); scope != "" {
		query.Scope = automationpkg.Scope(scope)
	}
	if source := strings.TrimSpace(i.Source); source != "" {
		query.Source = automationpkg.JobSource(source)
	}
	if err := automationpkg.ValidateJobListQuery(query); err != nil {
		return automationpkg.JobListQuery{}, nativeAutomationValidationError(id, err)
	}
	return query, nil
}

type automationTriggersListInput struct {
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Event       string `json:"event,omitempty"`
	Source      string `json:"source,omitempty"`
	Enabled     *bool  `json:"enabled,omitempty"`
	LoopName    string `json:"loop,omitempty"`
	Query       string `json:"q,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (i automationTriggersListInput) query(id toolspkg.ToolID) (automationpkg.TriggerListQuery, error) {
	query := automationpkg.TriggerListQuery{
		WorkspaceID: strings.TrimSpace(i.WorkspaceID),
		Event:       strings.TrimSpace(i.Event),
		LoopName:    strings.TrimSpace(i.LoopName),
		Enabled:     i.Enabled,
		Search:      strings.TrimSpace(i.Query),
		Cursor:      strings.TrimSpace(i.Cursor),
		Limit:       i.Limit,
	}
	if query.Limit == 0 {
		query.Limit = automationpkg.DefaultListLimit
	}
	if scope := strings.TrimSpace(i.Scope); scope != "" {
		query.Scope = automationpkg.Scope(scope)
	}
	if source := strings.TrimSpace(i.Source); source != "" {
		query.Source = automationpkg.JobSource(source)
	}
	if err := automationpkg.ValidateTriggerListQuery(query); err != nil {
		return automationpkg.TriggerListQuery{}, nativeAutomationValidationError(id, err)
	}
	return query, nil
}
