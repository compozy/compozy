package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	agentTaskActionNext      = "agent.task.next"
	agentTaskActionHeartbeat = "agent.task.heartbeat"
	agentTaskActionComplete  = "agent.task.complete"
	agentTaskActionFail      = "agent.task.fail"
	agentTaskActionRelease   = "agent.task.release"
)

type agentSoulClaimLocker interface {
	WithSoulClaimLock(ctx context.Context, sessionID string, fn func() error) error
}

// AgentTaskClaimNext claims the next eligible task run for the validated caller.
func (h *BaseHandlers) AgentTaskClaimNext(c *gin.Context) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return
	}

	caller, ok := h.requireAgentCaller(c, agentTaskActionNext)
	if !ok {
		return
	}

	var req contract.AgentTaskClaimNextRequest
	if err := decodeOptionalJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode agent task claim request: %w", h.transportName(), err)),
		)
		return
	}

	for {
		result, err := h.claimNextRunForAgent(c.Request.Context(), manager, req, caller)
		switch {
		case err == nil:
			if result == nil {
				h.respondError(
					c,
					http.StatusInternalServerError,
					errors.New("task claim returned an empty result"),
				)
				return
			}
			c.JSON(http.StatusOK, contract.AgentTaskClaimResponse{
				Claim: AgentTaskClaimPayloadFromResult(result),
			})
			return
		case (errors.Is(err, taskpkg.ErrNoClaimableRun) ||
			errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached)) && req.Wait:
			if waitErr := h.waitForAgentTaskPoll(c.Request.Context()); waitErr != nil {
				h.respondError(c, http.StatusRequestTimeout, waitErr)
				return
			}
		case errors.Is(err, taskpkg.ErrNoClaimableRun):
			c.Status(http.StatusNoContent)
			return
		default:
			h.respondError(c, statusForAgentTaskError(err), err)
			return
		}
	}
}

func (h *BaseHandlers) claimNextRunForAgent(
	ctx context.Context,
	manager TaskService,
	req contract.AgentTaskClaimNextRequest,
	caller agentidentity.Caller,
) (*taskpkg.ClaimResult, error) {
	if locker, ok := h.Sessions.(agentSoulClaimLocker); ok {
		var result *taskpkg.ClaimResult
		err := locker.WithSoulClaimLock(ctx, caller.Session.ID, func() error {
			currentCaller, err := h.currentAgentCaller(ctx, caller)
			if err != nil {
				return err
			}
			criteria, err := h.agentTaskClaimCriteria(ctx, req, currentCaller)
			if err != nil {
				return err
			}
			result, err = manager.ClaimNextRun(ctx, criteria, currentCaller.Actor)
			return err
		})
		return result, err
	}

	criteria, err := h.agentTaskClaimCriteria(ctx, req, caller)
	if err != nil {
		return nil, err
	}
	return manager.ClaimNextRun(ctx, criteria, caller.Actor)
}

func (h *BaseHandlers) currentAgentCaller(
	ctx context.Context,
	caller agentidentity.Caller,
) (agentidentity.Caller, error) {
	if h == nil || h.Sessions == nil {
		return caller, nil
	}
	info, err := h.Sessions.Status(ctx, caller.Session.ID)
	if err != nil {
		return agentidentity.Caller{}, err
	}
	if info == nil {
		return agentidentity.Caller{}, fmt.Errorf("%w: session %q", taskpkg.ErrPermissionDenied, caller.Session.ID)
	}
	current := caller
	current.Session = agentidentity.SessionSnapshotFromInfo(info)
	if current.Session.ID != caller.Session.ID || current.Session.AgentName != caller.Session.AgentName {
		return agentidentity.Caller{}, fmt.Errorf(
			"%w: agent session identity changed during task claim",
			taskpkg.ErrPermissionDenied,
		)
	}
	if current.Session.State != session.StateActive {
		return agentidentity.Caller{}, fmt.Errorf(
			"%w: agent session %q is not active",
			taskpkg.ErrPermissionDenied,
			current.Session.ID,
		)
	}
	return current, nil
}

// AgentTaskHeartbeat extends the current lease for one claimed task run.
func (h *BaseHandlers) AgentTaskHeartbeat(c *gin.Context) {
	manager, caller, runID, ok := h.agentTaskLeaseMutationSetup(c, agentTaskActionHeartbeat)
	if !ok {
		return
	}

	var req contract.AgentTaskHeartbeatRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode agent task heartbeat request: %w", h.transportName(), err)),
		)
		return
	}

	leaseDuration, err := agentTaskLeaseDuration(req.LeaseSeconds)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	handle, err := h.lookupAgentTaskLease(c.Request.Context(), manager, caller, runID)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	run, err := manager.HeartbeatRunLease(c.Request.Context(), taskpkg.LeaseHeartbeat{
		RunID:         runID,
		ClaimToken:    handle.ClaimToken,
		LeaseDuration: leaseDuration,
	}, caller.Actor)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.AgentTaskLeaseResponse{Lease: AgentTaskLeasePayloadFromRun(run, nil)})
}

// AgentTaskRelease releases one claimed task run back to the queue.
func (h *BaseHandlers) AgentTaskRelease(c *gin.Context) {
	manager, caller, runID, ok := h.agentTaskLeaseMutationSetup(c, agentTaskActionRelease)
	if !ok {
		return
	}

	var req contract.AgentTaskReleaseRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode agent task release request: %w", h.transportName(), err)),
		)
		return
	}

	handle, err := h.lookupAgentTaskLease(c.Request.Context(), manager, caller, runID)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	run, err := manager.ReleaseRunLease(c.Request.Context(), taskpkg.LeaseRelease{
		RunID:      runID,
		ClaimToken: handle.ClaimToken,
		Reason:     req.Reason,
	}, caller.Actor)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.AgentTaskLeaseResponse{Lease: AgentTaskLeasePayloadFromRun(run, nil)})
}

// AgentTaskComplete completes one claimed task run after token verification.
func (h *BaseHandlers) AgentTaskComplete(c *gin.Context) {
	manager, caller, runID, ok := h.agentTaskLeaseMutationSetup(c, agentTaskActionComplete)
	if !ok {
		return
	}

	var req contract.AgentTaskCompleteRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode agent task complete request: %w", h.transportName(), err)),
		)
		return
	}

	result, err := completeTaskRunFromRequest(contract.CompleteTaskRunRequest(req))
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	handle, err := h.lookupAgentTaskLease(c.Request.Context(), manager, caller, runID)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	run, err := manager.CompleteRunLease(c.Request.Context(), taskpkg.LeaseCompletion{
		RunID:          runID,
		ClaimToken:     handle.ClaimToken,
		Result:         result,
		CreatedTaskIDs: append([]string(nil), req.CreatedTaskIDs...),
	}, caller.Actor)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.AgentTaskLeaseResponse{Lease: AgentTaskLeasePayloadFromRun(run, nil)})
}

// AgentTaskFail fails one claimed task run after token verification.
func (h *BaseHandlers) AgentTaskFail(c *gin.Context) {
	manager, caller, runID, ok := h.agentTaskLeaseMutationSetup(c, agentTaskActionFail)
	if !ok {
		return
	}

	var req contract.AgentTaskFailRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			NewTaskValidationError(fmt.Errorf("%s: decode agent task fail request: %w", h.transportName(), err)),
		)
		return
	}

	failure, err := failTaskRunFromRequest(contract.FailTaskRunRequest(req))
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	handle, err := h.lookupAgentTaskLease(c.Request.Context(), manager, caller, runID)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}
	run, err := manager.FailRunLease(c.Request.Context(), taskpkg.LeaseFailure{
		RunID:      runID,
		ClaimToken: handle.ClaimToken,
		Failure:    failure,
	}, caller.Actor)
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.AgentTaskLeaseResponse{Lease: AgentTaskLeasePayloadFromRun(run, nil)})
}

func (h *BaseHandlers) agentTaskLeaseMutationSetup(
	c *gin.Context,
	action string,
) (TaskService, agentidentity.Caller, string, bool) {
	manager, ok := h.requireTaskManager(c)
	if !ok {
		return nil, agentidentity.Caller{}, "", false
	}
	caller, ok := h.requireAgentCaller(c, action)
	if !ok {
		return nil, agentidentity.Caller{}, "", false
	}
	runID, err := requiredPathID(c.Param("run_id"), "run id")
	if err != nil {
		h.respondError(c, statusForAgentTaskError(err), err)
		return nil, agentidentity.Caller{}, "", false
	}
	return manager, caller, runID, true
}

func (h *BaseHandlers) lookupAgentTaskLease(
	ctx context.Context,
	manager TaskService,
	caller agentidentity.Caller,
	runID string,
) (taskpkg.AutonomyLeaseHandle, error) {
	authority, ok := manager.(taskpkg.AutonomyLeaseAuthority)
	if !ok {
		return taskpkg.AutonomyLeaseHandle{}, errors.New("task autonomy lease authority is unavailable")
	}
	return authority.LookupActiveRunForSession(ctx, caller.Session.ID, runID)
}

func (h *BaseHandlers) waitForAgentTaskPoll(ctx context.Context) error {
	pollInterval := h.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	timer := time.NewTimer(pollInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func agentTaskLeaseDuration(seconds int64) (time.Duration, error) {
	switch {
	case seconds < 0:
		return 0, fmt.Errorf("%w: lease_seconds must be zero or positive: %d", taskpkg.ErrValidation, seconds)
	case seconds == 0:
		return 0, nil
	case seconds > int64(taskpkg.MaxRunLeaseDuration/time.Second):
		return 0, fmt.Errorf(
			"%w: lease_seconds exceeds %d",
			taskpkg.ErrValidation,
			int64(taskpkg.MaxRunLeaseDuration/time.Second),
		)
	default:
		return time.Duration(seconds) * time.Second, nil
	}
}

// AgentTaskClaimPayloadFromResult builds the public, redacted claim payload.
func AgentTaskClaimPayloadFromResult(result *taskpkg.ClaimResult) contract.AgentTaskClaimPayload {
	if result == nil || result.Task == nil {
		return contract.AgentTaskClaimPayload{}
	}
	channel := coordinationChannelPayloadFromMetadata(result.CoordinationChannel)
	run := TaskRunPayloadFromRun(&result.Run)
	if channel != nil {
		run.CoordinationChannel = channel
	}
	lease := AgentTaskLeasePayloadFromRun(&result.Run, channel)
	if lease.LeaseUntil == nil && !result.LeaseUntil.IsZero() {
		lease.LeaseUntil = optionalTime(result.LeaseUntil)
	}
	return contract.AgentTaskClaimPayload{
		Task:                taskReferencePayloadFromTask(*result.Task),
		Run:                 run,
		Lease:               lease,
		CoordinationChannel: channel,
	}
}

// AgentTaskLeasePayloadFromRun builds the public, redacted lease payload.
func AgentTaskLeasePayloadFromRun(
	run *taskpkg.Run,
	channel *contract.CoordinationChannelPayload,
) contract.TaskRunLeaseSummaryPayload {
	if run == nil {
		return contract.TaskRunLeaseSummaryPayload{}
	}
	payload := contract.TaskRunLeaseSummaryPayload{
		TaskID:                       run.TaskID,
		RunID:                        run.ID,
		Status:                       run.Status,
		SessionID:                    run.SessionID,
		ClaimedBy:                    cloneActorIdentity(run.ClaimedBy),
		ClaimTokenHash:               run.ClaimTokenHash,
		LeaseUntil:                   optionalTime(run.LeaseUntil),
		HeartbeatAt:                  optionalTime(run.HeartbeatAt),
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
		CoordinationChannel:          channel,
	}
	return contract.NormalizeTaskRunLeaseSummaryPayload(payload)
}

func coordinationChannelPayloadFromMetadata(
	metadata *taskpkg.CoordinationChannelMetadata,
) *contract.CoordinationChannelPayload {
	if metadata == nil || strings.TrimSpace(metadata.ID) == "" {
		return nil
	}
	kinds := make([]contract.CoordinationMessageKind, 0, len(metadata.AllowedMessageKinds))
	for _, kind := range metadata.AllowedMessageKinds {
		if trimmed := strings.TrimSpace(kind); trimmed != "" {
			kinds = append(kinds, contract.CoordinationMessageKind(trimmed))
		}
	}
	displayName := strings.TrimSpace(metadata.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(metadata.ID)
	}
	payload := contract.CoordinationChannelPayload{
		ID:                  strings.TrimSpace(metadata.ID),
		DisplayName:         displayName,
		Purpose:             strings.TrimSpace(metadata.Purpose),
		WorkspaceID:         strings.TrimSpace(metadata.WorkspaceID),
		TaskID:              strings.TrimSpace(metadata.TaskID),
		RunID:               strings.TrimSpace(metadata.RunID),
		WorkflowID:          strings.TrimSpace(metadata.WorkflowID),
		AllowedMessageKinds: kinds,
		LastActivityAt:      optionalTime(metadata.LastActivityAt),
	}
	normalized := contract.NormalizeCoordinationChannelPayload(payload)
	return &normalized
}

func taskReferencePayloadFromTask(record taskpkg.Task) contract.TaskReferencePayload {
	return contract.TaskReferencePayload{
		ID:          record.ID,
		Identifier:  record.Identifier,
		Title:       record.Title,
		Status:      record.Status,
		Priority:    record.Priority,
		Owner:       cloneOwnership(record.Owner),
		Scope:       record.Scope,
		WorkspaceID: record.WorkspaceID,
		Paused:      record.Paused,
	}
}

func statusForAgentTaskError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, taskpkg.ErrValidation),
		errors.Is(err, taskpkg.ErrInvalidScopeBinding),
		errors.Is(err, taskpkg.ErrImmutableField):
		return http.StatusUnprocessableEntity
	default:
		return StatusForTaskError(err)
	}
}
