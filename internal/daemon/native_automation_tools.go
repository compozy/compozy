package daemon

import (
	"context"
	"errors"

	core "github.com/compozy/compozy/internal/api/core"
	automationpkg "github.com/compozy/compozy/internal/automation"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	nativeAutomationToolsDeletedKey = "deleted"
	nativeAutomationToolsJobKey     = "job"
	nativeAutomationToolsRunKey     = "run"
	nativeAutomationToolsRunsKey    = "runs"
	nativeAutomationToolsTriggerKey = "trigger"
)

func (n *daemonNativeTools) automationJobsGet(
	ctx context.Context,
	scope toolspkg.Scope,
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
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	job, err := n.nativeAutomationJobForWorkspace(ctx, input.WorkspaceID, jobID, readScope)
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
	scope toolspkg.Scope,
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
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	job.ProfileID = readScope.ProfileID
	created, err := n.automationManager().CreateJob(ctx, job)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := n.automationJobPayloadBestEffort(ctx, created)
	return structuredResult(map[string]any{nativeAutomationToolsJobKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationJobsUpdate(
	ctx context.Context,
	scope toolspkg.Scope,
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
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	current, err := n.nativeAutomationJobForWorkspace(ctx, input.WorkspaceID, jobID, readScope)
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
	scope toolspkg.Scope,
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
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	current, err := n.nativeAutomationJobForWorkspace(ctx, input.WorkspaceID, jobID, readScope)
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
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.automationSetJobEnabled(ctx, scope, req, true)
}

func (n *daemonNativeTools) automationJobsDisable(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.automationSetJobEnabled(ctx, scope, req, false)
}

func (n *daemonNativeTools) automationJobsTrigger(
	ctx context.Context,
	scope toolspkg.Scope,
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
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	job, err := n.nativeAutomationJobForWorkspace(ctx, input.WorkspaceID, jobID, readScope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	run, err := n.automationManager().TriggerJob(ctx, job.ID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	payload := core.RunPayloadFromRun(run)
	return structuredResult(map[string]any{nativeAutomationToolsRunKey: payload}, payload.ID)
}

func (n *daemonNativeTools) automationJobsHistory(
	ctx context.Context,
	scope toolspkg.Scope,
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
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	job, err := n.nativeAutomationJobForWorkspace(ctx, input.WorkspaceID, jobID, readScope)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutomationToolError(req.ToolID, err)
	}
	query, err := input.query(req.ToolID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query.JobID = job.ID
	query.TriggerID = ""
	query.ReadScope = readScope
	return n.automationRunsForQuery(ctx, req.ToolID, input.WorkspaceID, query)
}
