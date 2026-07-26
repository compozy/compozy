package daemon

import (
	"context"
	"errors"

	core "github.com/compozy/agh/internal/api/core"
	automationpkg "github.com/compozy/agh/internal/automation"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeAutomationToolsDeletedKey = "deleted"
	nativeAutomationToolsJobKey     = "job"
	nativeAutomationToolsRunsKey    = "runs"
	nativeAutomationToolsTriggerKey = "trigger"
)

func (n *daemonNativeTools) automationJobsGet(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	jobID, err := requiredNativeString(req.ToolID, daemonPayloadJobIDKey, input.JobID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	job, err := n.automationManager().GetJob(ctx, jobID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload, err := n.automationJobPayload(ctx, job)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	return structuredResult(map[string]any{nativeAutomationToolsJobKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationJobsCreate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobCreateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	job, err := nativeAutomationJobFromCreateRequest(req.ToolID, input.request())
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	created, err := n.automationManager().CreateJob(ctx, job)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := n.automationJobPayloadBestEffort(ctx, created)
	return structuredResult(map[string]any{nativeAutomationToolsJobKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationJobsUpdate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobUpdateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	jobID, err := requiredNativeString(req.ToolID, daemonPayloadJobIDKey, input.JobID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	patch := input.request()
	if !patch.HasChanges() {
		return toolspkg.ToolResult{}, nativeAutomationValidationError(
			req.ToolID,
			errors.New("automation job update must include at least one field"),
		)
	}
	current, err := n.automationManager().GetJob(ctx, jobID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	var updated automationpkg.Job
	switch current.Source {
	case automationpkg.JobSourceConfig, automationpkg.JobSourcePackage:
		if err := core.ValidateAutomationManagedJobUpdate(patch); err != nil {
			return toolspkg.ToolResult{}, nativeAutomationValidationError(req.ToolID, err)
		}
		updated, err = n.automationManager().SetJobEnabled(ctx, current.ID, *patch.Enabled)
	default:
		next, patchErr := nativeAutomationJobFromPatch(req.ToolID, current, patch)
		if patchErr != nil {
			return toolspkg.ToolResult{}, patchErr
		}
		updated, err = n.automationManager().UpdateJob(ctx, next)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := n.automationJobPayloadBestEffort(ctx, updated)
	return structuredResult(map[string]any{nativeAutomationToolsJobKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationJobsDelete(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	jobID, err := requiredNativeString(req.ToolID, daemonPayloadJobIDKey, input.JobID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	current, err := n.automationManager().GetJob(ctx, jobID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	if current.Source != automationpkg.JobSourceDynamic {
		return toolspkg.ToolResult{}, nativeAutomationScopeError(
			req.ToolID,
			nativeAutomationToolsJobKey,
			current.ID,
			current.Source,
		)
	}
	if err := n.automationManager().DeleteJob(ctx, current.ID); err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	return structuredResult(
		map[string]any{daemonPayloadJobIDKey: current.ID, nativeAutomationToolsDeletedKey: true},
		current.ID,
	)
}

func (n *daemonNativeTools) automationJobsEnable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.automationSetJobEnabled(ctx, req, true)
}

func (n *daemonNativeTools) automationJobsDisable(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.automationSetJobEnabled(ctx, req, false)
}

func (n *daemonNativeTools) automationJobsTrigger(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	jobID, err := requiredNativeString(req.ToolID, daemonPayloadJobIDKey, input.JobID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	run, err := n.automationManager().TriggerJob(ctx, jobID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.RunPayloadFromRun(run)
	return structuredResult(map[string]any{"run": payload}, payload.ID)
}

func (n *daemonNativeTools) automationJobsHistory(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input automationJobHistoryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	jobID, err := requiredNativeString(req.ToolID, daemonPayloadJobIDKey, input.JobID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	job, err := n.automationManager().GetJob(ctx, jobID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	query, err := input.query(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query.JobID = job.ID
	query.TriggerID = ""
	return n.automationRunsForQuery(ctx, req.ToolID, query)
}
