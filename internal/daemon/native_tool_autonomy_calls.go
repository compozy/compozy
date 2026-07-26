package daemon

import (
	"context"

	"errors"
	"fmt"

	"strings"

	core "github.com/compozy/agh/internal/api/core"

	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) autonomyClaimNext(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input autonomyClaimNextInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, sessionID, err := autonomyActorContext(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	criteria, err := input.criteria(scope, sessionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	result, err := n.deps.Tasks.ClaimNextRun(ctx, criteria, actor)
	if err != nil {
		if errors.Is(err, taskpkg.ErrNoClaimableRun) {
			return structuredResult(map[string]any{nativeToolsClaimedKey: false}, "no claimable task runs")
		}
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	if result == nil {
		return toolspkg.ToolResult{}, errors.New("daemon: task-run claim returned an empty result")
	}
	payload := core.AgentTaskClaimPayloadFromResult(result)
	return structuredResult(
		map[string]any{nativeToolsClaimedKey: true, "claim": payload},
		fmt.Sprintf("claimed %s", payload.Lease.RunID),
	)
}

func (n *daemonNativeTools) autonomyHeartbeat(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input autonomyHeartbeatInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, sessionID, err := autonomyActorContext(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID, err := requiredNativeString(req.ToolID, "run_id", input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	leaseDuration, err := autonomyLeaseDuration(input.LeaseSeconds)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	handle, err := n.lookupAutonomyLease(ctx, req.ToolID, sessionID, runID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	run, err := n.deps.Tasks.HeartbeatRunLease(ctx, taskpkg.LeaseHeartbeat{
		RunID:         runID,
		ClaimToken:    handle.ClaimToken,
		LeaseDuration: leaseDuration,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	lease := core.AgentTaskLeasePayloadFromRun(run, nil)
	return structuredResult(map[string]any{nativeToolsLeaseKey: lease}, fmt.Sprintf("heartbeat %s", lease.RunID))
}

func (n *daemonNativeTools) autonomyComplete(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input autonomyCompleteInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, sessionID, err := autonomyActorContext(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID, err := requiredNativeString(req.ToolID, "run_id", input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result := taskpkg.RunResult{Value: cloneJSON(input.Result)}
	if err := result.Validate("run_result"); err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	handle, err := n.lookupAutonomyLease(ctx, req.ToolID, sessionID, runID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	run, err := n.deps.Tasks.CompleteRunLease(ctx, taskpkg.LeaseCompletion{
		RunID:          runID,
		ClaimToken:     handle.ClaimToken,
		Result:         result,
		CreatedTaskIDs: trimNativeStrings(input.CreatedTaskIDs),
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	lease := core.AgentTaskLeasePayloadFromRun(run, nil)
	return structuredResult(map[string]any{nativeToolsLeaseKey: lease}, fmt.Sprintf("completed %s", lease.RunID))
}

func (n *daemonNativeTools) autonomyFail(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input autonomyFailInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, sessionID, err := autonomyActorContext(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID, err := requiredNativeString(req.ToolID, "run_id", input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	failure := taskpkg.RunFailure{
		Error:    strings.TrimSpace(input.Error),
		Metadata: cloneJSON(input.Metadata),
	}
	if err := failure.Validate("run_failure"); err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	handle, err := n.lookupAutonomyLease(ctx, req.ToolID, sessionID, runID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	run, err := n.deps.Tasks.FailRunLease(ctx, taskpkg.LeaseFailure{
		RunID:      runID,
		ClaimToken: handle.ClaimToken,
		Failure:    failure,
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	lease := core.AgentTaskLeasePayloadFromRun(run, nil)
	return structuredResult(map[string]any{nativeToolsLeaseKey: lease}, fmt.Sprintf("failed %s", lease.RunID))
}

func (n *daemonNativeTools) autonomyRelease(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input autonomyReleaseInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, sessionID, err := autonomyActorContext(req.ToolID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	runID, err := requiredNativeString(req.ToolID, "run_id", input.RunID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	handle, err := n.lookupAutonomyLease(ctx, req.ToolID, sessionID, runID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	run, err := n.deps.Tasks.ReleaseRunLease(ctx, taskpkg.LeaseRelease{
		RunID:      runID,
		ClaimToken: handle.ClaimToken,
		Reason:     strings.TrimSpace(input.Reason),
	}, actor)
	if err != nil {
		return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
	}
	lease := core.AgentTaskLeasePayloadFromRun(run, nil)
	return structuredResult(map[string]any{nativeToolsLeaseKey: lease}, fmt.Sprintf("released %s", lease.RunID))
}
