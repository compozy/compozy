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
		switch transition.Phase {
		case PhaseDownloading:
			return EventDownloadProgress, nil
		case PhaseVerifying:
			return EventVerifyCompleted, nil
		case PhaseSwapping:
			return EventSwapCompleted, nil
		case PhaseRestarting:
			return EventRestartCompleted, nil
		case PhaseHealthChecking:
			return EventHealthCompleted, nil
		case PhaseFinalized:
			return EventFinalized, nil
		case PhaseRolledBack:
			return EventRolledBack, nil
		case PhaseStaged:
			return EventAppStaged, nil
		case PhaseApplying:
			return EventAppApplying, nil
		case PhaseInstallerHandoff:
			return EventAppInstallerHandoff, nil
		case PhaseVerified:
			return EventAppVerified, nil
		case PhaseFailed:
			if transition.Target == TargetApp {
				return EventAppFailed, nil
			}
			return EventFinalized, nil
		case PhaseRestarted:
			return EventRestartCompleted, nil
		}
	}
	return "", fmt.Errorf("update: transition %q has no canonical event", transition.Kind)
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
		return "failed"
	}
	return "ok"
}
