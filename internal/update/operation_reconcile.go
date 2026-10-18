package update

import (
	"context"
	"slices"

	"github.com/compozy/compozy/internal/procutil"
)

func (s *OperationStore) reconcileUnlocked(ctx context.Context, operation *Operation) (*Operation, error) {
	if operation == nil {
		return nil, nil
	}
	now := s.now()
	if operation.Deadline.IsZero() || operation.Deadline.After(now) {
		return operation, nil
	}
	target := operation.ActiveTarget
	if target == "" && operation.Waiting == WaitingForApp {
		target = TargetApp
	}
	if !deadlineCanSettle(operation, target) {
		return operation, nil
	}
	// An applying process can still own an installer after its deadline or lease expires.
	if target == TargetApp && operation.App.Phase == PhaseApplying && operation.Holder != nil &&
		procutil.Alive(operation.Holder.PID) &&
		procutil.MatchesStartTime(operation.Holder.PID, operation.Holder.PIDStartTime) {
		return operation, nil
	}
	updated := cloneOperation(operation)
	updated.ActiveTarget = target
	transition := Transition{
		Kind:                 TransitionPhase,
		Actor:                ActorDaemon,
		Target:               target,
		Phase:                PhaseFailed,
		Percent:              -1,
		LastError:            "update operation deadline expired",
		Outcome:              string(StatusFailed),
		IncrementAppFailures: target == TargetApp,
	}
	if target == TargetApp {
		transition.LastError = "The app update did not reach the installer handoff before its deadline."
	}
	if err := applyTransition(updated, transition, now); err != nil {
		return nil, err
	}
	updated.Holder = nil
	updated.Waiting = WaitingNone
	updated.Revision++
	updated.UpdatedAt = now
	if err := validateOperation(updated); err != nil {
		return nil, err
	}
	if err := s.persistTransition(updated); err != nil {
		return nil, err
	}
	if err := s.emitTransitionEvents(ctx, operation, updated, transition); err != nil {
		return nil, err
	}
	return nil, nil
}

func deadlineCanSettle(operation *Operation, target Target) bool {
	// Runtime replacement and installer handoff require their own recovery owners.
	switch target {
	case TargetRuntime:
		return operation.Runtime != nil && slices.Contains(
			[]OperationPhase{PhasePending, PhaseDownloading, PhaseVerifying}, operation.Runtime.Phase,
		)
	case TargetApp:
		return operation.App != nil && slices.Contains(
			[]OperationPhase{PhasePending, PhaseStaged, PhaseApplying}, operation.App.Phase,
		)
	default:
		return false
	}
}
