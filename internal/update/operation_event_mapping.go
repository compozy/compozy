package update

import (
	"fmt"
	"strings"
	"time"
)

func eventNameForTransition(transition Transition, operation *Operation) (OperationEventName, error) {
	switch transition.Kind {
	case TransitionProgress:
		return EventDownloadProgress, nil
	case TransitionWaitForApp:
		return EventOperationWaiting, nil
	case TransitionAcquireLease:
		return EventLeaseAcquired, nil
	case TransitionRenewLease:
		return EventLeaseRenewed, nil
	case TransitionRecover:
		if operation.Holder != nil {
			return EventOperationRecovered, nil
		}
		return EventLeaseExpired, nil
	case TransitionCancel:
		return EventOperationCanceled, nil
	case TransitionPhase:
		return eventNameForPhase(transition)
	}
	return "", fmt.Errorf("update: transition %q has no canonical event", transition.Kind)
}

func eventNameForPhase(transition Transition) (OperationEventName, error) {
	if transition.Phase == PhaseFailed {
		if transition.Target == TargetApp {
			return EventAppFailed, nil
		}
		return EventFinalized, nil
	}
	eventName, ok := phaseEventNames[transition.Phase]
	if !ok {
		return "", fmt.Errorf("update: phase %q has no canonical event", transition.Phase)
	}
	return eventName, nil
}

var phaseEventNames = map[OperationPhase]OperationEventName{
	PhaseDownloading:      EventDownloadProgress,
	PhaseVerifying:        EventVerifyCompleted,
	PhaseSwapping:         EventSwapCompleted,
	PhaseRestarting:       EventRestartCompleted,
	PhaseHealthChecking:   EventHealthCompleted,
	PhaseFinalized:        EventFinalized,
	PhaseRolledBack:       EventRolledBack,
	PhaseStaged:           EventAppStaged,
	PhaseApplying:         EventAppApplying,
	PhaseInstallerHandoff: EventAppInstallerHandoff,
	PhaseVerified:         EventAppVerified,
	PhaseRestarted:        EventRestartCompleted,
}

func eventFromOperation(
	name OperationEventName,
	operation *Operation,
	target Target,
	actor Actor,
	outcome string,
	occurredAt time.Time,
) OperationEvent {
	event := OperationEvent{
		Name:        name,
		OperationID: operation.ID,
		Revision:    operation.Revision,
		Target:      target,
		Actor:       actor,
		Outcome:     strings.TrimSpace(outcome),
		OccurredAt:  occurredAt.UTC(),
	}
	if operation.Holder != nil {
		event.ExecutorGeneration = operation.Holder.ExecutorGeneration
	}
	if target == TargetRuntime && operation.Runtime != nil {
		event.InstallMethod = operation.Runtime.InstallMethod
		event.FromVersion = operation.Runtime.FromVersion
		event.ToVersion = operation.Runtime.ToVersion
	}
	if target == TargetApp && operation.App != nil {
		event.FromVersion = operation.App.FromVersion
		event.ToVersion = operation.App.ToVersion
	}
	return event
}

func transitionOutcome(transition Transition) string {
	if value := strings.TrimSpace(transition.Outcome); value != "" {
		return value
	}
	if transition.Phase == PhaseFailed || transition.Phase == PhaseRolledBack {
		return operationOutcomeFailed
	}
	return "ok"
}
