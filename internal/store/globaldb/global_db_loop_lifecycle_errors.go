package globaldb

import (
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

const (
	nodeLifecycleStateActive       = "active"
	nodeLifecycleStatePaused       = "paused"
	nodeLifecycleStateQuarantined  = "quarantined"
	nodeLifecycleStateAttention    = "attention"
	loopLifecycleStateIterationCap = "iteration_cap_reached"
	nodeLifecycleTransitionPause   = "pause"
	nodeLifecycleTransitionResume  = "resume"
	nodeLifecycleTransitionCancel  = "cancel"
	nodeLifecycleTransitionRequeue = "requeue"
)

func loopLifecycleReasonError(
	code looppkg.ReasonCode,
	cause error,
	actualState string,
	allowedTransitions []string,
	winner *looppkg.ControlProvenance,
) error {
	meta := map[string]string{
		looppkg.ReasonMetaActualState:        strings.TrimSpace(actualState),
		looppkg.ReasonMetaAllowedTransitions: strings.Join(allowedTransitions, ","),
	}
	if winner != nil {
		meta[looppkg.ReasonMetaWinnerActorKind] = strings.TrimSpace(winner.ActorKind)
		meta[looppkg.ReasonMetaWinnerActorID] = strings.TrimSpace(winner.ActorID)
		meta[looppkg.ReasonMetaWinnerReason] = strings.TrimSpace(winner.Reason)
		if !winner.RequestedAt.IsZero() {
			meta[looppkg.ReasonMetaWinnerRequestedAt] = winner.RequestedAt.UTC().Format(time.RFC3339Nano)
		}
	}
	return &looppkg.ReasonError{Code: code, Err: cause, Meta: meta}
}

func nodeLifecycleActualState(control looppkg.NodeControl, found bool) string {
	if !found {
		return nodeLifecycleStateActive
	}
	if control.CancelState != looppkg.CancelStateNone {
		return string(control.CancelState)
	}
	if control.Paused {
		return nodeLifecycleStatePaused
	}
	if control.Quarantined {
		return nodeLifecycleStateQuarantined
	}
	if strings.TrimSpace(control.AttentionFlag) != "" {
		return nodeLifecycleStateAttention
	}
	return nodeLifecycleStateActive
}

func nodeLifecycleAllowedTransitions(actualState string) []string {
	switch actualState {
	case nodeLifecycleStatePaused:
		return []string{
			nodeLifecycleTransitionResume,
			nodeLifecycleTransitionCancel,
		}
	case nodeLifecycleStateQuarantined:
		return []string{
			nodeLifecycleTransitionRequeue,
			nodeLifecycleTransitionCancel,
		}
	case string(looppkg.CancelStateCanceled):
		return []string{}
	case nodeLifecycleStateActive, nodeLifecycleStateAttention:
		return []string{
			nodeLifecycleTransitionPause,
			nodeLifecycleTransitionCancel,
		}
	default:
		return []string{}
	}
}

func nodeLifecycleWinner(control looppkg.NodeControl) *looppkg.ControlProvenance {
	if control.CancelProvenance != nil {
		return control.CancelProvenance
	}
	return control.PauseProvenance
}

func runLifecycleWinner(run looppkg.Run) *looppkg.ControlProvenance {
	if strings.TrimSpace(run.ControlActor.Ref) == "" && run.ControlRequestedAt.IsZero() {
		return nil
	}
	return &looppkg.ControlProvenance{
		ActorKind:   string(run.ControlActor.Kind),
		ActorID:     run.ControlActor.Ref,
		RequestedAt: run.ControlRequestedAt,
	}
}
