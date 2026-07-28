package task

import (
	"fmt"
	"strings"
)

const (
	runFieldTaskID             = "task_id"
	runFieldWorkspaceID        = "workspace_id"
	runFieldAttempt            = "attempt"
	runFieldRecoveryCount      = "recovery_count"
	runFieldRunKind            = "run_kind"
	runFieldLoopRunID          = "loop_run_id"
	runFieldPreviousRunID      = "previous_run_id"
	runFieldOrigin             = "origin"
	runFieldIdempotencyKey     = "idempotency_key"
	runFieldNetworkSpec        = "network_spec"
	runFieldDesignationGroupID = "designation_group_id"
	runFieldNetworkWake        = "network_wake"
)

// ValidateImmutableRunFields rejects lifecycle updates that rewrite durable execution identity.
func ValidateImmutableRunFields(current Run, next Run) error {
	checks := []struct {
		field string
		same  bool
	}{
		{field: runFieldTaskID, same: strings.TrimSpace(current.TaskID) == strings.TrimSpace(next.TaskID)},
		{
			field: runFieldWorkspaceID,
			same:  strings.TrimSpace(current.WorkspaceID) == strings.TrimSpace(next.WorkspaceID),
		},
		{field: runFieldAttempt, same: current.Attempt == next.Attempt},
		{field: runFieldRecoveryCount, same: current.RecoveryCount == next.RecoveryCount},
		{field: runFieldRunKind, same: current.RunKind.Normalize() == next.RunKind.Normalize()},
		{field: runFieldLoopRunID, same: strings.TrimSpace(current.LoopRunID) == strings.TrimSpace(next.LoopRunID)},
		{
			field: runFieldPreviousRunID,
			same:  strings.TrimSpace(current.PreviousRunID) == strings.TrimSpace(next.PreviousRunID),
		},
		{field: runFieldOrigin, same: sameOrigin(current.Origin, next.Origin)},
		{
			field: runFieldIdempotencyKey,
			same:  strings.TrimSpace(current.IdempotencyKey) == strings.TrimSpace(next.IdempotencyKey),
		},
		{field: runFieldNetworkSpec, same: current.NetworkSpecSnapshot() == next.NetworkSpecSnapshot()},
		{
			field: runFieldDesignationGroupID,
			same: strings.TrimSpace(current.DesignationGroupID) ==
				strings.TrimSpace(next.DesignationGroupID),
		},
		{field: runFieldNetworkWake, same: sameRunNetworkWakeCorrelation(current, next)},
	}
	for _, check := range checks {
		if !check.same {
			return fmt.Errorf("%w: %s", ErrImmutableField, check.field)
		}
	}
	return nil
}

func sameRunNetworkWakeCorrelation(current Run, next Run) bool {
	currentWakeID, currentTargetSessionID, currentOwnerKey := current.NetworkWakeCorrelation()
	nextWakeID, nextTargetSessionID, nextOwnerKey := next.NetworkWakeCorrelation()
	return strings.TrimSpace(currentWakeID) == strings.TrimSpace(nextWakeID) &&
		strings.TrimSpace(currentTargetSessionID) == strings.TrimSpace(nextTargetSessionID) &&
		strings.TrimSpace(currentOwnerKey) == strings.TrimSpace(nextOwnerKey)
}
