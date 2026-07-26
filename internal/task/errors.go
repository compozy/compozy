package task

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrTaskNotFound reports that no persisted task matched the lookup.
	ErrTaskNotFound = errors.New("task: task not found")
	// ErrTaskRunNotFound reports that no persisted task run matched the lookup.
	ErrTaskRunNotFound = errors.New("task: task run not found")
	// ErrTaskRunIdempotencyNotFound reports that no persisted task-run idempotency record matched the lookup.
	ErrTaskRunIdempotencyNotFound = errors.New("task: task run idempotency not found")
	// ErrTaskDependencyNotFound reports that no persisted dependency edge matched the lookup.
	ErrTaskDependencyNotFound = errors.New("task: task dependency not found")
	// ErrTaskBlockNotFound reports that no persisted task block matched the lookup.
	ErrTaskBlockNotFound = errors.New("task: task block not found")
	// ErrTaskEventNotFound reports that no persisted task event matched the lookup.
	ErrTaskEventNotFound = errors.New("task: task event not found")
	// ErrTaskTriageStateNotFound reports that no persisted triage state matched the lookup.
	ErrTaskTriageStateNotFound = errors.New("task: task triage state not found")
	// ErrExecutionProfileNotFound reports that no persisted task execution profile matched the lookup.
	ErrExecutionProfileNotFound = errors.New("task: task execution profile not found")
	// ErrRunReviewNotFound reports that no persisted task-run review matched the lookup.
	ErrRunReviewNotFound = errors.New("task: task run review not found")
	// ErrValidation reports that a task-domain payload or state failed validation.
	ErrValidation = errors.New("task: validation failed")
	// ErrImmutableField reports that a caller attempted to change an immutable task field.
	ErrImmutableField = errors.New("task: immutable field")
	// ErrInvalidScopeBinding reports that a scope and workspace binding combination is invalid.
	ErrInvalidScopeBinding = errors.New("task: invalid scope binding")
	// ErrPayloadTooLarge reports that a JSON payload exceeded the task-domain size guardrails.
	ErrPayloadTooLarge = errors.New("task: payload too large")
	// ErrGraphLimitExceeded reports that a task hierarchy or dependency operation exceeded a bounded limit.
	ErrGraphLimitExceeded = errors.New("task: graph limit exceeded")
	// ErrCycleDetected reports that a dependency insert would introduce a cycle.
	ErrCycleDetected = errors.New("task: dependency cycle detected")
	// ErrInvalidStatusTransition reports that a task or run lifecycle transition is not allowed.
	ErrInvalidStatusTransition = errors.New("task: invalid status transition")
	// ErrConflict reports that an idempotent write conflicts with previously persisted state.
	ErrConflict = errors.New("task: conflict")
	// ErrSessionAlreadyBound reports that a run already owns a session binding.
	ErrSessionAlreadyBound = errors.New("task: session already bound")
	// ErrSessionAttachNotAllowed reports that a run cannot attach an existing session in its current state.
	ErrSessionAttachNotAllowed = errors.New("task: session attach not allowed")
	// ErrPermissionDenied reports that the resolved actor context lacks authority for the requested task action.
	ErrPermissionDenied = errors.New("task: permission denied")
	// ErrNoClaimableRun reports that no task run matched claim criteria.
	ErrNoClaimableRun = errors.New("task: no claimable run")
	// ErrInvalidClaimToken reports that a lease mutation did not prove ownership with the current token.
	ErrInvalidClaimToken = errors.New("task: invalid claim token")
	// ErrHallucinatedTaskRefs reports completion claims for task IDs not created by the completing session.
	ErrHallucinatedTaskRefs = errors.New("task: completion claims task ids not created by this session")
	// ErrLeaseExpired reports that a lease mutation targeted an expired ownership lease.
	ErrLeaseExpired = errors.New("task: lease expired")
	// ErrSessionNotLive reports that a creator session cannot accept a wake delivery.
	ErrSessionNotLive = errors.New("task: creator session is not live")
	// ErrActiveRunLease reports that a session already owns an active task-run lease.
	ErrActiveRunLease = errors.New("task: active run lease exists")
	// ErrWorkspaceActiveRunCapReached reports that workspace execution capacity is currently full.
	ErrWorkspaceActiveRunCapReached = errors.New("task: workspace active run cap reached")
	// ErrNetworkWakeSettlementConflict reports a terminal wake outcome that conflicts with durable truth.
	ErrNetworkWakeSettlementConflict = errors.New("task: network wake settlement conflict")
	// ErrForbiddenOperatorAction reports that config or policy forbids a force operation for the actor.
	ErrForbiddenOperatorAction = errors.New("task: forbidden operator action")
	// ErrForceOpRequiresReason reports that a force operation requires a non-empty reason.
	ErrForceOpRequiresReason = errors.New("task: force operation requires reason")
	// ErrForceOpRateLimited reports that an actor exceeded the force-operation rate limit.
	ErrForceOpRateLimited = errors.New("task: force operation rate limited")
	// ErrRetryChainTooDeep reports that retry would exceed the configured retry lineage depth.
	ErrRetryChainTooDeep = errors.New("task: retry chain too deep")
	// ErrBulkTooLarge reports that a bulk operation exceeded its bounded item limit.
	ErrBulkTooLarge = errors.New("task: bulk operation too large")
)

// HallucinatedTaskRefsError carries the rejected completion claim without exposing the raw claim token.
type HallucinatedTaskRefsError struct {
	RunID          string
	TaskID         string
	RunStatus      RunStatus
	SessionID      string
	ClaimTokenHash string
	ClaimedTaskIDs []string
	InvalidTaskIDs []string
}

// NewHallucinatedTaskRefsError builds the typed completion-claim rejection.
func NewHallucinatedTaskRefsError(
	run Run,
	claimedTaskIDs []string,
	invalidTaskIDs []string,
) *HallucinatedTaskRefsError {
	return &HallucinatedTaskRefsError{
		RunID:          strings.TrimSpace(run.ID),
		TaskID:         strings.TrimSpace(run.TaskID),
		RunStatus:      run.Status.Normalize(),
		SessionID:      strings.TrimSpace(run.SessionID),
		ClaimTokenHash: strings.TrimSpace(run.ClaimTokenHash),
		ClaimedTaskIDs: cloneErrorStringSlice(claimedTaskIDs),
		InvalidTaskIDs: cloneErrorStringSlice(invalidTaskIDs),
	}
}

func (e *HallucinatedTaskRefsError) Error() string {
	if e == nil {
		return "<nil>"
	}
	invalid := strings.Join(e.InvalidTaskIDs, ",")
	if invalid == "" {
		return ErrHallucinatedTaskRefs.Error()
	}
	return fmt.Sprintf("%s: run %q claimed invalid task ids %q", ErrHallucinatedTaskRefs, e.RunID, invalid)
}

// Unwrap exposes the stable sentinel used by API and agent-tool callers.
func (e *HallucinatedTaskRefsError) Unwrap() error {
	if e == nil {
		return nil
	}
	return ErrHallucinatedTaskRefs
}

func cloneErrorStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
