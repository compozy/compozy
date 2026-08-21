package extensionpkg

import (
	"context"
	"encoding/json"
	"strings"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/store"
)

func (h *HostAPIHandler) handleAutomationJobs(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationJobsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	workspaceID, err := h.resolveAutomationWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	profileID, err := hostAPIProfileID(ctx)
	if err != nil {
		return nil, err
	}

	query := automationpkg.JobListQuery{
		ReadScope:   store.ReadScope{ProfileID: profileID},
		Scope:       params.Scope,
		WorkspaceID: workspaceID,
		Enabled:     params.Enabled,
		Search:      strings.TrimSpace(params.Query),
		Cursor:      strings.TrimSpace(params.Cursor),
		Limit:       params.Limit,
	}
	if query.Limit == 0 {
		query.Limit = automationpkg.DefaultListLimit
	}
	if err := automationpkg.ValidateJobListQuery(query); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	page, err := automation.ListJobs(ctx, query)
	if err != nil {
		return nil, err
	}
	return extensioncontract.AutomationJobsResult{
		Jobs: page.Jobs,
		Page: apicontract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}, nil
}

func (h *HostAPIHandler) handleAutomationTriggers(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTriggersParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	workspaceID, err := h.resolveAutomationWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	profileID, err := hostAPIProfileID(ctx)
	if err != nil {
		return nil, err
	}

	query := automationpkg.TriggerListQuery{
		ReadScope:   store.ReadScope{ProfileID: profileID},
		Scope:       params.Scope,
		WorkspaceID: workspaceID,
		Event:       strings.TrimSpace(params.Event),
		Enabled:     params.Enabled,
		Search:      strings.TrimSpace(params.Query),
		Cursor:      strings.TrimSpace(params.Cursor),
		Limit:       params.Limit,
	}
	if query.Limit == 0 {
		query.Limit = automationpkg.DefaultListLimit
	}
	if err := automationpkg.ValidateTriggerListQuery(query); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	page, err := automation.ListTriggers(ctx, query)
	if err != nil {
		return nil, err
	}
	return extensioncontract.AutomationTriggersResult{
		Triggers: apicontract.TriggerPayloadsFromTriggers(page.Triggers),
		Page: apicontract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}, nil
}
