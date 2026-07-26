package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"

	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

func (h *HostAPIHandler) handleAutomationJobsGet(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	return automation.GetJob(ctx, params.ID)
}

func (h *HostAPIHandler) handleAutomationJobsCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationJobCreateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	job, err := h.jobFromCreateParams(ctx, params)
	if err != nil {
		return nil, err
	}
	if err := job.Validate("job"); err != nil {
		return nil, invalidParamsRPCError(err)
	}

	return automation.CreateJob(ctx, job)
}

func (h *HostAPIHandler) handleAutomationJobsUpdate(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationJobUpdateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}
	if !params.HasChanges() {
		return nil, invalidParamsRPCError(errors.New("automation job update must include at least one field"))
	}

	current, err := automation.GetJob(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	if current.Source == automationpkg.JobSourceConfig {
		if err := validateHostAPIConfigJobUpdate(params.UpdateJobRequest); err != nil {
			return nil, invalidParamsRPCError(err)
		}
		return automation.SetJobEnabled(ctx, current.ID, *params.Enabled)
	}

	next, err := h.applyJobUpdateParams(ctx, current, params.UpdateJobRequest)
	if err != nil {
		return nil, err
	}
	if err := next.Validate("job"); err != nil {
		return nil, invalidParamsRPCError(err)
	}

	return automation.UpdateJob(ctx, next)
}

func (h *HostAPIHandler) handleAutomationJobsDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	current, err := automation.GetJob(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	if current.Source == automationpkg.JobSourceConfig {
		return nil, invalidParamsRPCError(errors.New("config-backed automation jobs cannot be deleted"))
	}
	if err := automation.DeleteJob(ctx, current.ID); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

func (h *HostAPIHandler) handleAutomationJobsTrigger(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationJobTriggerParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	return automation.TriggerJobWithPayload(ctx, params.ID, params.Payload)
}

func (h *HostAPIHandler) handleAutomationJobsRuns(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationJobRunsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	job, err := automation.GetJob(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	return automation.ListRuns(ctx, automationpkg.RunQuery{
		JobID:  job.ID,
		Status: params.Status,
		Limit:  params.Limit,
	})
}

func (h *HostAPIHandler) handleAutomationTriggersGet(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	trigger, err := automation.GetTrigger(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	return apicontract.TriggerPayloadFromTrigger(trigger), nil
}

func (h *HostAPIHandler) handleAutomationTriggersCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTriggerCreateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	trigger, webhookSecret, err := h.triggerFromCreateParams(ctx, params)
	if err != nil {
		return nil, err
	}

	created, err := automation.CreateTrigger(ctx, trigger, webhookSecret)
	if err != nil {
		return nil, err
	}
	return apicontract.TriggerPayloadFromTrigger(created), nil
}

func (h *HostAPIHandler) handleAutomationTriggersUpdate(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTriggerUpdateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}
	if !params.HasChanges() {
		return nil, invalidParamsRPCError(errors.New("automation trigger update must include at least one field"))
	}

	current, err := automation.GetTrigger(ctx, params.ID)
	if err != nil {
		return nil, err
	}

	if current.Source == automationpkg.JobSourceConfig {
		if err := validateHostAPIConfigTriggerUpdate(params.UpdateTriggerRequest); err != nil {
			return nil, invalidParamsRPCError(err)
		}
		updated, err := automation.SetTriggerEnabled(ctx, current.ID, *params.Enabled)
		if err != nil {
			return nil, err
		}
		return apicontract.TriggerPayloadFromTrigger(updated), nil
	}

	next, webhookSecret, err := h.applyTriggerUpdateParams(ctx, current, params.UpdateTriggerRequest)
	if err != nil {
		return nil, err
	}

	updated, err := automation.UpdateTrigger(ctx, next, webhookSecret)
	if err != nil {
		return nil, err
	}
	return apicontract.TriggerPayloadFromTrigger(updated), nil
}

func (h *HostAPIHandler) handleAutomationTriggersDelete(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	current, err := automation.GetTrigger(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	if current.Source == automationpkg.JobSourceConfig {
		return nil, invalidParamsRPCError(errors.New("config-backed automation triggers cannot be deleted"))
	}
	if err := automation.DeleteTrigger(ctx, current.ID); err != nil {
		return nil, err
	}
	return struct{}{}, nil
}

func (h *HostAPIHandler) handleAutomationTriggersRuns(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTriggerRunsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.ID) == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	trigger, err := automation.GetTrigger(ctx, params.ID)
	if err != nil {
		return nil, err
	}
	return automation.ListRuns(ctx, automationpkg.RunQuery{
		TriggerID: trigger.ID,
		Status:    params.Status,
		Limit:     params.Limit,
	})
}

func (h *HostAPIHandler) handleAutomationTriggersFire(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationTriggerFireParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	workspaceID, err := h.resolveAutomationWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}

	request := automationpkg.ExtensionTriggerRequest{
		Event:       strings.TrimSpace(params.Event),
		Scope:       params.Scope,
		WorkspaceID: workspaceID,
		Payload:     cloneJSONMap(params.Payload),
	}
	if err := request.Validate("trigger_fire"); err != nil {
		return nil, invalidParamsRPCError(err)
	}

	return automation.FireExtensionTrigger(ctx, request)
}

func (h *HostAPIHandler) handleAutomationRuns(ctx context.Context, raw json.RawMessage) (any, error) {
	automation, err := h.automationManager()
	if err != nil {
		return nil, err
	}

	var params hostAPIAutomationRunsParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	return automation.ListRuns(ctx, automationpkg.RunQuery{
		JobID:     strings.TrimSpace(params.JobID),
		TriggerID: strings.TrimSpace(params.TriggerID),
		Status:    params.Status,
		Limit:     params.Limit,
	})
}
