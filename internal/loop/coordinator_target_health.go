package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) accountActionTarget(
	ctx context.Context,
	run Run,
	node dsl.Node,
	input ActionExecutionInput,
	taskRun task.Run,
	failure *ClassifiedFailure,
	disposition AttemptDisposition,
) ([]GenerationLifecycleEventIntent, error) {
	if r == nil || r.targetHealth == nil {
		return nil, nil
	}
	observation, found := r.takeTargetProbe(taskRun.ID)
	if !found {
		key, bound, err := targetHealthKey(run.WorkspaceID, node, input)
		if err != nil {
			return nil, err
		}
		if !bound {
			return nil, nil
		}
		observation.key = key
	}
	before, hasStatus, err := targetHealthStatus(ctx, r.targetHealth, observation.key)
	if err != nil {
		return nil, err
	}
	if targetHealthRecordsFailure(failure, disposition) {
		reason := "loop target transport failure"
		if failure != nil && strings.TrimSpace(failure.Cause) != "" {
			reason = failure.Cause
		}
		if err := r.targetHealth.RecordFailure(
			ctx,
			observation.key,
			deadentity.FailurePermanent,
			reason,
		); err != nil {
			return nil, fmt.Errorf("loop: record target failure for %q: %w", observation.key.EntityID, err)
		}
	} else if targetHealthRecordsSuccess(failure, disposition) {
		if err := r.targetHealth.RecordSuccess(ctx, observation.key); err != nil {
			return nil, fmt.Errorf("loop: record target success for %q: %w", observation.key.EntityID, err)
		}
	}
	after, afterHasStatus, err := targetHealthStatus(ctx, r.targetHealth, observation.key)
	if err != nil {
		return nil, err
	}
	events := make([]GenerationLifecycleEventIntent, 0, 2)
	if observation.decision.Recovery {
		events = append(events, targetBreakerTransitionIntent(observation.key, targetBreakerStateHalfOpen))
		if targetHealthRecordsFailure(failure, disposition) {
			events = append(events, targetBreakerTransitionIntent(observation.key, targetBreakerStateOpen))
		}
	}
	if hasStatus && afterHasStatus && before.Dead != after.Dead {
		state := targetBreakerStateClosed
		if after.Dead {
			state = targetBreakerStateOpen
		}
		events = append(events, targetBreakerTransitionIntent(observation.key, state))
	}
	return events, nil
}

func (r *CoordinatorRunner) takeTargetProbe(taskRunID string) (targetProbeObservation, bool) {
	taskRunID = strings.TrimSpace(taskRunID)
	if r == nil || taskRunID == "" {
		return targetProbeObservation{}, false
	}
	value, ok := r.targetProbes.LoadAndDelete(taskRunID)
	if !ok {
		return targetProbeObservation{}, false
	}
	observation, ok := value.(targetProbeObservation)
	return observation, ok
}

func (r *CoordinatorRunner) discardTargetProbe(taskRunID string) {
	if r == nil {
		return
	}
	taskRunID = strings.TrimSpace(taskRunID)
	if taskRunID != "" {
		r.targetProbes.Delete(taskRunID)
	}
}

func (r *CoordinatorRunner) actionTargetFor(
	taskRunID string,
	run Run,
	node dsl.Node,
	input ActionExecutionInput,
) (string, error) {
	if r != nil {
		if value, ok := r.targetProbes.Load(strings.TrimSpace(taskRunID)); ok {
			if observation, valid := value.(targetProbeObservation); valid {
				_, target := targetIdentityFromKey(observation.key)
				return target, nil
			}
		}
	}
	key, bound, err := targetHealthKey(run.WorkspaceID, node, input)
	if err != nil || !bound {
		return "", err
	}
	_, target := targetIdentityFromKey(key)
	return target, nil
}

func targetHealthRecordsFailure(failure *ClassifiedFailure, disposition AttemptDisposition) bool {
	return failure != nil && failure.Class == FailureTransport &&
		disposition != AttemptRouted && disposition != AttemptAbsorbed
}

func targetHealthRecordsSuccess(failure *ClassifiedFailure, disposition AttemptDisposition) bool {
	if disposition == AttemptRouted || disposition == AttemptAbsorbed || failure == nil {
		return true
	}
	return failure.Class != FailureTransport && failure.Class != FailureTargetUnavailable
}

func targetHealthStatus(
	ctx context.Context,
	health TargetHealth,
	key store.DeadEntityKey,
) (deadentity.Status, bool, error) {
	reader, ok := health.(targetHealthStatusReader)
	if !ok {
		return deadentity.Status{}, false, nil
	}
	status, err := reader.Status(ctx, key)
	if err != nil {
		return deadentity.Status{}, false, fmt.Errorf("loop: read target health for %q: %w", key.EntityID, err)
	}
	return status, true, nil
}

func targetBreakerTransitionIntent(
	key store.DeadEntityKey,
	state string,
) GenerationLifecycleEventIntent {
	family, target := targetIdentityFromKey(key)
	return GenerationLifecycleEventIntent{
		Kind:         GenerationLifecycleEventTargetBreakerTransition,
		TargetFamily: family,
		Target:       target,
		BreakerState: state,
	}
}
