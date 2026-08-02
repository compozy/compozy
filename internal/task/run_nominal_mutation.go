package task

import (
	"strings"
	"time"
)

// NominalRunMutationResult records the exact source and committed run snapshots
// for one non-terminal lifecycle mutation.
type NominalRunMutationResult struct {
	Previous Run
	Run      Run
}

// RunDirectExecutionAdmissionMutation authorizes one exact queued worker run
// to enter tokenless direct execution under a trusted actor.
type RunDirectExecutionAdmissionMutation struct {
	fence     RunMutationFence
	actor     ActorContext
	claimedAt time.Time
}

// NewRunDirectExecutionAdmissionMutation captures the queued run and actor
// that may own its direct execution lifecycle.
func NewRunDirectExecutionAdmissionMutation(
	previous Run,
	actor ActorContext,
	claimedAt time.Time,
) RunDirectExecutionAdmissionMutation {
	return RunDirectExecutionAdmissionMutation{
		fence:     NewRunMutationFence(previous),
		actor:     actor,
		claimedAt: claimedAt,
	}
}

// Fence reports the exact queued snapshot required by the command.
func (m RunDirectExecutionAdmissionMutation) Fence() RunMutationFence { return m.fence }

// Actor reports the trusted direct executor captured by the command.
func (m RunDirectExecutionAdmissionMutation) Actor() ActorContext { return m.actor }

// ClaimedAt reports when direct ownership was admitted.
func (m RunDirectExecutionAdmissionMutation) ClaimedAt() time.Time { return m.claimedAt }

// RunStartingMutation authorizes claimed -> starting against one exact run
// ownership snapshot.
type RunStartingMutation struct {
	fence RunMutationFence
}

// NewRunStartingMutation captures the claimed run that may enter starting.
func NewRunStartingMutation(previous Run) RunStartingMutation {
	return RunStartingMutation{fence: NewRunMutationFence(previous)}
}

// Fence reports the exact source snapshot required by the command.
func (m RunStartingMutation) Fence() RunMutationFence { return m.fence }

// RunSessionBindingMutation authorizes one exact claimed or starting run to
// bind the session returned by the runtime bridge.
type RunSessionBindingMutation struct {
	fence     RunMutationFence
	sessionID string
}

// NewRunSessionBindingMutation captures the source run and requested session.
func NewRunSessionBindingMutation(previous Run, sessionID string) RunSessionBindingMutation {
	return RunSessionBindingMutation{
		fence:     NewRunMutationFence(previous),
		sessionID: strings.TrimSpace(sessionID),
	}
}

// Fence reports the exact source snapshot required by the command.
func (m RunSessionBindingMutation) Fence() RunMutationFence { return m.fence }

// SessionID reports the normalized session binding requested by the command.
func (m RunSessionBindingMutation) SessionID() string { return m.sessionID }

// RunRunningMutation authorizes starting -> running against one exact run
// ownership snapshot.
type RunRunningMutation struct {
	fence     RunMutationFence
	startedAt time.Time
}

// NewRunRunningMutation captures the starting run and its execution timestamp.
func NewRunRunningMutation(previous Run, startedAt time.Time) RunRunningMutation {
	return RunRunningMutation{fence: NewRunMutationFence(previous), startedAt: startedAt}
}

// Fence reports the exact source snapshot required by the command.
func (m RunRunningMutation) Fence() RunMutationFence { return m.fence }

// StartedAt reports the requested execution timestamp.
func (m RunRunningMutation) StartedAt() time.Time { return m.startedAt }

// RunBootRecoveryMutation authorizes one daemon recovery of an exact
// task-anchored run. Only the non-terminal requeue and mark-running actions are
// represented here; failure continues through the terminal mutation boundary.
type RunBootRecoveryMutation struct {
	fence     RunMutationFence
	action    RunBootRecoveryAction
	startedAt time.Time
}

// NewRunBootRecoveryMutation captures the source run and recovery action.
func NewRunBootRecoveryMutation(
	previous Run,
	action RunBootRecoveryAction,
	startedAt time.Time,
) RunBootRecoveryMutation {
	return RunBootRecoveryMutation{
		fence:     NewRunMutationFence(previous),
		action:    action.Normalize(),
		startedAt: startedAt,
	}
}

// Fence reports the exact source snapshot required by the command.
func (m RunBootRecoveryMutation) Fence() RunMutationFence { return m.fence }

// Action reports the requested daemon recovery action.
func (m RunBootRecoveryMutation) Action() RunBootRecoveryAction { return m.action }

// StartedAt reports the execution timestamp used by mark-running recovery.
func (m RunBootRecoveryMutation) StartedAt() time.Time { return m.startedAt }

// NetworkWakeBootRecoveryMutation authorizes one daemon-owned recovery of an
// exact taskless network-wake run and carries the actor needed by its durable
// network lifecycle ledger entry.
type NetworkWakeBootRecoveryMutation struct {
	fence       RunMutationFence
	recovery    RunBootRecovery
	failureText string
	actor       ActorContext
	now         time.Time
}

// NewNetworkWakeBootRecoveryMutation captures an exact taskless wake recovery.
func NewNetworkWakeBootRecoveryMutation(
	previous Run,
	recovery RunBootRecovery,
	actor ActorContext,
	now time.Time,
) NetworkWakeBootRecoveryMutation {
	return NetworkWakeBootRecoveryMutation{
		fence:       NewRunMutationFence(previous),
		recovery:    recovery,
		failureText: runBootRecoveryError(previous, recovery),
		actor:       actor,
		now:         now,
	}
}

// Fence reports the exact source snapshot required by the command.
func (m NetworkWakeBootRecoveryMutation) Fence() RunMutationFence { return m.fence }

// Recovery reports the normalized recovery decision.
func (m NetworkWakeBootRecoveryMutation) Recovery() RunBootRecovery { return m.recovery }

// FailureText reports the sanitized terminal diagnostic for fail recovery.
func (m NetworkWakeBootRecoveryMutation) FailureText() string { return m.failureText }

// Actor reports the trusted daemon actor that owns the recovery.
func (m NetworkWakeBootRecoveryMutation) Actor() ActorContext { return m.actor }

// Now reports the recovery timestamp.
func (m NetworkWakeBootRecoveryMutation) Now() time.Time { return m.now }
