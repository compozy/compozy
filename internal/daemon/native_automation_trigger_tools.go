package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) automationTriggersGet(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationTriggerIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	triggerID, err := requiredNativeString(req.ToolID, daemonPayloadTriggerIDKey, input.TriggerID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	trigger, err := n.automationManager().GetTrigger(ctx, triggerID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.TriggerPayloadFromTrigger(trigger)
	return structuredResult(map[string]any{nativeAutomationToolsTriggerKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationTriggersCreate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationTriggerCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	trigger := core.AutomationTriggerFromCreateRequest(input.request())
	created, err := n.automationManager().CreateTrigger(ctx, trigger, input.webhookSecretWrite())
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.TriggerPayloadFromTrigger(created)
	return structuredResult(map[string]any{nativeAutomationToolsTriggerKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationTriggersUpdate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationTriggerUpdateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	triggerID, err := requiredNativeString(req.ToolID, daemonPayloadTriggerIDKey, input.TriggerID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	patch := input.request()
	if !patch.HasChanges() {
		return toolspkg.ToolResult{}, nativeAutomationValidationError(
			req.ToolID,
			errors.New("automation trigger update must include at least one field"),
		)
	}
	current, err := n.automationManager().GetTrigger(ctx, triggerID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	var updated automationpkg.Trigger
	switch current.Source {
	case automationpkg.JobSourceConfig, automationpkg.JobSourcePackage:
		if err := core.ValidateAutomationManagedTriggerUpdate(patch); err != nil {
			return toolspkg.ToolResult{}, nativeAutomationValidationError(req.ToolID, err)
		}
		updated, err = n.automationManager().SetTriggerEnabled(ctx, current.ID, *patch.Enabled)
	default:
		next, patchErr := core.ApplyAutomationTriggerPatch(current, patch)
		if patchErr != nil {
			return toolspkg.ToolResult{}, nativeAutomationValidationError(req.ToolID, patchErr)
		}
		updated, err = n.automationManager().UpdateTrigger(ctx, next, input.webhookSecretWrite())
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.TriggerPayloadFromTrigger(updated)
	return structuredResult(map[string]any{nativeAutomationToolsTriggerKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationTriggersDelete(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationTriggerIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	triggerID, err := requiredNativeString(req.ToolID, daemonPayloadTriggerIDKey, input.TriggerID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	current, err := n.automationManager().GetTrigger(ctx, triggerID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	if current.Source != automationpkg.JobSourceDynamic {
		return toolspkg.ToolResult{}, nativeAutomationScopeError(
			req.ToolID,
			nativeAutomationToolsTriggerKey,
			current.ID,
			current.Source,
		)
	}
	if err := n.automationManager().DeleteTrigger(ctx, current.ID); err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{daemonPayloadTriggerIDKey: current.ID, nativeAutomationToolsDeletedKey: true},
		current.ID,
	)
}

func (n *daemonNativeTools) automationTriggersEnable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.automationSetTriggerEnabled(ctx, req, true)
}

func (n *daemonNativeTools) automationTriggersDisable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.automationSetTriggerEnabled(ctx, req, false)
}

func (n *daemonNativeTools) automationTriggersHistory(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationTriggerHistoryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	triggerID, err := requiredNativeString(req.ToolID, daemonPayloadTriggerIDKey, input.TriggerID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	trigger, err := n.automationManager().GetTrigger(ctx, triggerID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	query, err := input.query(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query.JobID = ""
	query.TriggerID = trigger.ID
	return n.automationRunsForQuery(ctx, req.ToolID, query)
}

func (n *daemonNativeTools) automationRunsList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationRunQueryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err := input.query(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return n.automationRunsForQuery(ctx, req.ToolID, query)
}

func (n *daemonNativeTools) automationRunsGet(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationRunIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID, err := requiredNativeString(req.ToolID, "run_id", input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	run, err := n.automationManager().GetRun(ctx, runID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.RunPayloadFromRun(run)
	return structuredResult(map[string]any{"run": payload}, payload.ID)
}

func (n *daemonNativeTools) automationSetJobEnabled(
	ctx context.Context,
	req toolspkg.CallRequest,
	enabled bool,
) (toolspkg.ToolResult, error) {
	var input automationJobIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	jobID, err := requiredNativeString(req.ToolID, daemonPayloadJobIDKey, input.JobID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	updated, err := n.automationManager().SetJobEnabled(ctx, jobID, enabled)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := n.automationJobPayloadBestEffort(ctx, updated)
	return structuredResult(map[string]any{nativeAutomationToolsJobKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationSetTriggerEnabled(
	ctx context.Context,
	req toolspkg.CallRequest,
	enabled bool,
) (toolspkg.ToolResult, error) {
	var input automationTriggerIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	triggerID, err := requiredNativeString(req.ToolID, daemonPayloadTriggerIDKey, input.TriggerID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	updated, err := n.automationManager().SetTriggerEnabled(ctx, triggerID, enabled)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.TriggerPayloadFromTrigger(updated)
	return structuredResult(map[string]any{nativeAutomationToolsTriggerKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationRunsForQuery(
	ctx context.Context,
	toolID toolspkg.ToolID,
	query automationpkg.RunQuery,
) (toolspkg.ToolResult, error) {
	runs, err := n.automationManager().ListRuns(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(toolID, err)
	}
	payload := core.RunPayloadsFromRuns(runs)
	return structuredResult(
		map[string]any{nativeAutomationToolsRunsKey: payload},
		fmt.Sprintf("%d automation runs", len(payload)),
	)
}

func (n *daemonNativeTools) automationJobPayloads(
	ctx context.Context,
	jobs []automationpkg.Job,
) ([]contract.JobPayload, error) {
	stateByID, err := n.automationSchedulerStateByJobID(ctx)
	if err != nil {
		return nil, err
	}
	return core.JobPayloadsFromJobs(jobs, stateByID), nil
}

func (n *daemonNativeTools) automationJobPayload(
	ctx context.Context,
	job automationpkg.Job,
) (contract.JobPayload, error) {
	payloads, err := n.automationJobPayloads(ctx, []automationpkg.Job{job})
	if err != nil {
		return contract.JobPayload{}, err
	}
	if len(payloads) == 0 {
		return contract.JobPayload{}, errors.New("daemon: automation job payload missing")
	}
	return payloads[0], nil
}

func (n *daemonNativeTools) automationJobPayloadBestEffort(
	ctx context.Context,
	job automationpkg.Job,
) contract.JobPayload {
	payload, err := n.automationJobPayload(ctx, job)
	if err == nil {
		return payload
	}
	return core.JobPayloadFromJob(job, nil, nil)
}

func (n *daemonNativeTools) automationSchedulerStateByJobID(
	ctx context.Context,
) (map[string]contract.AutomationSchedulerStatePayload, error) {
	status, err := n.automationManager().Status(ctx)
	if err != nil {
		return nil, err
	}
	stateByID := make(map[string]contract.AutomationSchedulerStatePayload, len(status.ScheduledJobs))
	for _, scheduled := range status.ScheduledJobs {
		stateByID[scheduled.JobID] = core.AutomationSchedulerStatePayloadFromState(scheduled)
	}
	return stateByID, nil
}
