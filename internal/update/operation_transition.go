package update

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// TransitionKind identifies one journal mutation.
type TransitionKind string

const (
	TransitionPhase        TransitionKind = "phase"
	TransitionProgress     TransitionKind = "progress"
	TransitionWaitForApp   TransitionKind = "wait-for-app"
	TransitionAcquireLease TransitionKind = "acquire-lease"
	TransitionRenewLease   TransitionKind = "renew-lease"
	TransitionRecover      TransitionKind = "recover"
	TransitionCancel       TransitionKind = "cancel"
)

// Transition is the declarative input to the sole operation mutation API.
type Transition struct {
	Kind       TransitionKind
	Actor      Actor
	Target     Target
	Phase      OperationPhase
	Percent    int
	Holder     *Holder
	BackupPath string
	LastError  string
	Outcome    string
}

// Transition applies one fenced compare-and-swap mutation to the live journal.
func (s *OperationStore) Transition(
	ctx context.Context,
	operationID string,
	executorGeneration string,
	expectedRevision int64,
	transition Transition,
) (*Operation, error) {
	if strings.TrimSpace(operationID) == "" || expectedRevision < 1 {
		return nil, errors.New("update: operation id and expected revision are required")
	}
	if !transition.Actor.valid() {
		return nil, fmt.Errorf("update: invalid transition actor %q", transition.Actor)
	}

	var result *Operation
	err := s.withLock(ctx, func() error {
		operation, err := s.readUnlocked()
		if err != nil {
			return err
		}
		if operation == nil || operation.ID != operationID {
			return ErrOperationNotFound
		}
		if operation.Revision != expectedRevision {
			return fmt.Errorf("%w: have %d, expected %d", ErrOperationConflict, operation.Revision, expectedRevision)
		}
		if err := validateExecutorFence(operation, executorGeneration, transition, s.now(), s.holderLive); err != nil {
			if transition.Kind == TransitionCancel {
				s.events.EmitUpdateEvent(
					ctx,
					eventFromOperation(
						EventOperationCancelDeclined, operation, operation.ActiveTarget, transition.Actor, "declined", s.now(),
					),
				)
				return err
			}
			return err
		}

		updated := cloneOperation(operation)
		if err := applyTransition(updated, transition, s.now()); err != nil {
			return err
		}
		updated.Revision++
		updated.UpdatedAt = s.now()
		if err := validateOperation(updated); err != nil {
			return err
		}
		if isTerminalOperation(updated) {
			if err := s.archiveAndRemoveUnlocked(updated); err != nil {
				return err
			}
		} else if err := s.replaceUnlocked(updated); err != nil {
			return err
		}
		if transition.Kind == TransitionRecover && operation.Holder != nil &&
			!s.holderLive(operation.Holder, updated.UpdatedAt) {
			s.events.EmitUpdateEvent(
				ctx,
				eventFromOperation(
					EventLeaseExpired,
					operation,
					operation.ActiveTarget,
					transition.Actor,
					"expired",
					updated.UpdatedAt,
				),
			)
		}
		eventName, err := eventNameForTransition(transition, updated)
		if err != nil {
			return err
		}
		s.events.EmitUpdateEvent(
			ctx,
			eventFromOperation(
				eventName, updated, transition.Target, transition.Actor, transitionOutcome(transition), updated.UpdatedAt,
			),
		)
		result = cloneOperation(updated)
		return nil
	})
	return result, err
}

// Fence proves that a caller still owns the exact revision immediately before an irreversible side effect.
func (s *OperationStore) Fence(
	ctx context.Context,
	operationID string,
	executorGeneration string,
	expectedRevision int64,
) error {
	return s.withLock(ctx, func() error {
		operation, err := s.readUnlocked()
		if err != nil {
			return err
		}
		if operation == nil || operation.ID != operationID {
			return ErrOperationNotFound
		}
		if operation.Revision != expectedRevision {
			return ErrOperationConflict
		}
		if operation.Holder == nil || operation.Holder.ExecutorGeneration != strings.TrimSpace(executorGeneration) ||
			!s.holderLive(operation.Holder, s.now()) {
			return ErrExecutorFenced
		}
		return nil
	})
}

func validateExecutorFence(
	operation *Operation,
	executorGeneration string,
	transition Transition,
	now time.Time,
	holderLive func(*Holder, time.Time) bool,
) error {
	if transition.Kind == TransitionAcquireLease || transition.Kind == TransitionRecover {
		if operation.Holder == nil || !holderLive(operation.Holder, now) {
			return nil
		}
		return ErrExecutorFenced
	}
	if transition.Kind == TransitionCancel {
		if !cancelAllowedForPhase(operation) {
			return ErrOperationNotCancelable
		}
		if operation.Holder == nil || !holderLive(operation.Holder, now) {
			return nil
		}
		return ErrOperationNotCancelable
	}
	if operation.Holder == nil || operation.Holder.ExecutorGeneration != strings.TrimSpace(executorGeneration) ||
		!holderLive(operation.Holder, now) {
		return ErrExecutorFenced
	}
	return nil
}

func cancelAllowedForPhase(operation *Operation) bool {
	if operation == nil {
		return false
	}
	switch operation.ActiveTarget {
	case TargetRuntime:
		return operation.Runtime != nil && slices.Contains(
			[]OperationPhase{PhasePending, PhaseDownloading, PhaseVerifying},
			operation.Runtime.Phase,
		)
	case TargetApp:
		return operation.App != nil && slices.Contains(
			[]OperationPhase{PhasePending, PhaseStaged},
			operation.App.Phase,
		)
	case "":
		return operation.Waiting == WaitingForApp && operation.App != nil && operation.App.Phase == PhaseStaged
	default:
		return false
	}
}

func applyTransition(operation *Operation, transition Transition, now time.Time) error {
	switch transition.Kind {
	case TransitionPhase:
		if err := applyPhaseTransition(operation, transition); err != nil {
			return err
		}
	case TransitionProgress:
		if transition.Target != operation.ActiveTarget || transition.Percent < 0 || transition.Percent > 100 {
			return errors.New("update: invalid progress transition")
		}
		operation.Percent = transition.Percent
	case TransitionWaitForApp:
		if operation.App == nil || operation.App.Phase != PhaseStaged || !runtimeAllowsApp(operation.Runtime) {
			return errors.New("update: operation cannot wait for the app in its current phase")
		}
		operation.ActiveTarget = ""
		operation.Percent = -1
		operation.Holder = nil
		operation.Waiting = WaitingForApp
	case TransitionAcquireLease, TransitionRecover:
		if transition.Holder == nil {
			return errors.New("update: lease transition requires a holder")
		}
		operation.Holder = cloneHolder(transition.Holder)
		operation.Waiting = WaitingNone
		if transition.Target.valid() {
			operation.ActiveTarget = transition.Target
		}
	case TransitionRenewLease:
		if transition.Holder == nil || operation.Holder == nil ||
			transition.Holder.ExecutorGeneration != operation.Holder.ExecutorGeneration ||
			!transition.Holder.LeaseExpiresAt.After(now) {
			return errors.New("update: invalid lease renewal")
		}
		operation.Holder = cloneHolder(transition.Holder)
	case TransitionCancel:
		operation.Holder = nil
		operation.Waiting = WaitingNone
		operation.Outcome = "canceled"
		operation.LastError = ""
	default:
		return fmt.Errorf("update: invalid transition kind %q", transition.Kind)
	}
	if transition.BackupPath != "" && operation.Runtime != nil {
		operation.Runtime.BackupPath = strings.TrimSpace(transition.BackupPath)
	}
	if transition.LastError != "" {
		operation.LastError = strings.TrimSpace(transition.LastError)
	}
	if transition.Outcome != "" {
		operation.Outcome = strings.TrimSpace(transition.Outcome)
	}
	return nil
}

func applyPhaseTransition(operation *Operation, transition Transition) error {
	if transition.Target != operation.ActiveTarget || !transition.Phase.validFor(transition.Target) {
		return errors.New("update: phase transition does not match the active target")
	}
	current := phaseForTarget(operation, transition.Target)
	if !allowedPhaseTransition(transition.Target, current, transition.Phase) {
		return fmt.Errorf("update: phase transition %s -> %s is not allowed", current, transition.Phase)
	}
	setPhaseForTarget(operation, transition.Target, transition.Phase)
	if transition.Target == TargetRuntime && transition.Phase == PhaseFinalized && operation.App != nil {
		operation.ActiveTarget = TargetApp
	}
	operation.Percent = transition.Percent
	if operation.Percent < -1 || operation.Percent > 100 {
		return errors.New("update: transition percent is outside -1..100")
	}
	return nil
}

func allowedPhaseTransition(target Target, current OperationPhase, next OperationPhase) bool {
	return slices.Contains(allowedPhaseTransitions[target][current], next)
}

var allowedPhaseTransitions = map[Target]map[OperationPhase][]OperationPhase{
	TargetRuntime: {
		PhasePending:        {PhaseDownloading, PhaseFailed},
		PhaseDownloading:    {PhaseDownloading, PhaseVerifying, PhaseFailed},
		PhaseVerifying:      {PhaseDownloading, PhaseVerifying, PhaseSwapping, PhaseFailed},
		PhaseSwapping:       {PhaseDownloading, PhaseRestarting, PhaseRolledBack, PhaseFailed},
		PhaseRestarting:     {PhaseHealthChecking, PhaseRolledBack, PhaseFailed},
		PhaseHealthChecking: {PhaseFinalized, PhaseRolledBack, PhaseFailed},
	},
	TargetApp: {
		PhasePending:          {PhaseStaged, PhaseFailed},
		PhaseStaged:           {PhaseApplying, PhaseFailed},
		PhaseApplying:         {PhaseInstallerHandoff, PhaseFailed},
		PhaseInstallerHandoff: {PhaseRestarted, PhaseFailed},
		PhaseRestarted:        {PhaseVerified, PhaseFailed},
	},
}

func phaseForTarget(operation *Operation, target Target) OperationPhase {
	if target == TargetRuntime && operation.Runtime != nil {
		return operation.Runtime.Phase
	}
	if target == TargetApp && operation.App != nil {
		return operation.App.Phase
	}
	return ""
}

func setPhaseForTarget(operation *Operation, target Target, phase OperationPhase) {
	if target == TargetRuntime && operation.Runtime != nil {
		operation.Runtime.Phase = phase
	}
	if target == TargetApp && operation.App != nil {
		operation.App.Phase = phase
	}
}

func canResumeOperation(
	operation *Operation,
	targets []Target,
	now time.Time,
	holderLive func(*Holder, time.Time) bool,
) bool {
	if !slices.Equal(operation.Targets, targets) {
		return false
	}
	return operation.Waiting == WaitingForApp && operation.Holder == nil ||
		operation.Holder != nil && !holderLive(operation.Holder, now)
}

func (s *OperationStore) resumeUnlocked(
	ctx context.Context,
	operation *Operation,
	request OperationRequest,
	result **Operation,
) error {
	resumed := cloneOperation(operation)
	resumed.Revision++
	resumed.RequestedBy = request.RequestedBy
	resumed.Holder = cloneHolder(&request.Holder)
	resumed.Waiting = WaitingNone
	resumed.UpdatedAt = s.now()
	if resumed.ActiveTarget == "" {
		resumed.ActiveTarget = nextIncompleteTarget(resumed)
	}
	if err := s.replaceUnlocked(resumed); err != nil {
		return err
	}
	s.events.EmitUpdateEvent(
		ctx,
		eventFromOperation(
			EventOperationResumed, resumed, resumed.ActiveTarget, request.RequestedBy, "resumed", resumed.UpdatedAt,
		),
	)
	*result = cloneOperation(resumed)
	return nil
}

func nextIncompleteTarget(operation *Operation) Target {
	if operation.Runtime != nil && !runtimeAllowsApp(operation.Runtime) {
		return TargetRuntime
	}
	if operation.App != nil && operation.App.Phase != PhaseVerified && operation.App.Phase != PhaseFailed {
		return TargetApp
	}
	return ""
}

func runtimeAllowsApp(runtime *RuntimeOperationState) bool {
	return runtime == nil || runtime.Phase == PhaseFinalized
}

func isTerminalOperation(operation *Operation) bool {
	if operation.Outcome == "canceled" {
		return true
	}
	if operation.Runtime != nil && (operation.Runtime.Phase == PhaseRolledBack || operation.Runtime.Phase == PhaseFailed) {
		return true
	}
	runtimeTerminal := operation.Runtime == nil || operation.Runtime.Phase == PhaseFinalized
	appTerminal := operation.App == nil || operation.App.Phase == PhaseVerified || operation.App.Phase == PhaseFailed
	return runtimeTerminal && appTerminal
}
