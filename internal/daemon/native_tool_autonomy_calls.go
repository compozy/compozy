package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
	if n.deps.TaskClaimHandoff != nil && n.deps.TaskClaimHandoff.HasPending(sessionID) {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeConflict,
			req.ToolID,
			"task execution already transferred; end this turn",
			toolspkg.ErrToolConflict,
			toolspkg.ReasonAutonomyLeaseAlreadyHeld,
		)
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
	if nativeClaimStartsWorker(result.Run) {
		if err := n.startNativeClaimedRun(ctx, sessionID, result, actor); err != nil {
			return toolspkg.ToolResult{}, nativeAutonomyToolError(req.ToolID, err)
		}
	}
	payload := core.AgentTaskClaimPayloadFromResult(result)
	summary := fmt.Sprintf("claimed and started %s", payload.Lease.RunID)
	if strings.TrimSpace(result.Run.SessionID) != sessionID {
		summary = fmt.Sprintf(
			"started %s in session %s; execution transferred, end this turn without doing task work",
			payload.Lease.RunID,
			strings.TrimSpace(result.Run.SessionID),
		)
	}
	return structuredResult(
		map[string]any{nativeToolsClaimedKey: true, "claim": payload},
		summary,
	)
}

func (n *daemonNativeTools) startNativeClaimedRun(
	ctx context.Context,
	bootstrapSessionID string,
	result *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
) error {
	started, err := n.deps.Tasks.StartRun(ctx, result.Run.ID, taskpkg.StartRun{
		IdempotencyKey: "native-start:" + strings.TrimSpace(result.Run.ID),
		ClaimToken:     result.ClaimToken,
	}, actor)
	if err != nil {
		return n.cleanupNativeClaimStart(ctx, bootstrapSessionID, result, actor, err)
	}
	if started == nil {
		return n.cleanupNativeClaimStart(
			ctx,
			bootstrapSessionID,
			result,
			actor,
			errors.New("daemon: task-run start returned an empty result"),
		)
	}
	result.Run = *started
	if strings.TrimSpace(started.SessionID) == bootstrapSessionID {
		return nil
	}
	if n.deps.TaskClaimHandoff == nil {
		handoffErr := errors.New("daemon: task claim handoff is unavailable")
		abortErr := redactTaskClaimCleanupError(n.abortTaskClaimHandoff(ctx, result, actor))
		return errors.Join(handoffErr, abortErr)
	}
	if err := n.deps.TaskClaimHandoff.Activate(ctx, bootstrapSessionID, *started); err != nil {
		abortErr := redactTaskClaimCleanupError(n.abortTaskClaimHandoff(ctx, result, actor))
		bootstrapErr := n.scheduleTaskClaimBootstrapStop(bootstrapSessionID, result.Run.ID, "")
		return errors.Join(err, abortErr, bootstrapErr)
	}
	return nil
}

func (n *daemonNativeTools) cleanupNativeClaimStart(
	ctx context.Context,
	bootstrapSessionID string,
	result *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	cause error,
) error {
	failErr := redactTaskClaimCleanupError(n.failNativeTaskClaim(
		ctx,
		result,
		actor,
		"task execution start failed",
	))
	bootstrapErr := n.scheduleTaskClaimBootstrapStop(bootstrapSessionID, result.Run.ID, "")
	return errors.Join(cause, failErr, bootstrapErr)
}

func (n *daemonNativeTools) scheduleTaskClaimBootstrapStop(
	bootstrapSessionID string,
	runID string,
	targetSessionID string,
) error {
	if n == nil || n.deps == nil || n.deps.TaskClaimHandoff == nil {
		return nil
	}
	return n.deps.TaskClaimHandoff.ScheduleBootstrapStop(bootstrapSessionID, runID, targetSessionID)
}

func redactTaskClaimCleanupError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(taskpkg.RedactClaimTokens(err.Error()))
}

func nativeClaimStartsWorker(run taskpkg.Run) bool {
	kind := run.RunKind.Normalize()
	return kind == taskpkg.RunKindUnknown || kind == taskpkg.RunKindWorker
}

func (n *daemonNativeTools) abortTaskClaimHandoff(
	ctx context.Context,
	result *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
) error {
	if n == nil || n.deps == nil || result == nil {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	failErr := n.failNativeTaskClaim(cleanupCtx, result, actor, "task execution handoff failed")
	var stopErr error
	if n.deps.Sessions != nil {
		stopErr = n.deps.Sessions.StopWithCause(
			cleanupCtx,
			result.Run.SessionID,
			session.CauseFailed,
			"task execution handoff failed",
		)
	}
	return errors.Join(failErr, stopErr)
}

func (n *daemonNativeTools) failNativeTaskClaim(
	ctx context.Context,
	result *taskpkg.ClaimResult,
	actor taskpkg.ActorContext,
	reason string,
) error {
	if n == nil || n.deps == nil || result == nil {
		return nil
	}
	_, err := n.deps.Tasks.FailRunLease(ctx, taskpkg.LeaseFailure{
		RunID:      result.Run.ID,
		ClaimToken: result.ClaimToken,
		Failure: taskpkg.RunFailure{
			Error: strings.TrimSpace(reason),
		},
	}, actor)
	return err
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
