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
	taskRun task.Run,
	failure *ClassifiedFailure,
	disposition AttemptDisposition,
) ([]GenerationLifecycleEventIntent, error) {
	if r == nil || r.targetHealth == nil {
		return nil, nil
	}
	observation, found := r.takeTargetProbe(taskRun.ID)
	if !found {
		key, ok := staticTargetHealthKey(run.WorkspaceID, node)
		if !ok {
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
	value, ok := r.targetProbes.LoadAndDelete(strings.TrimSpace(taskRunID))
	if !ok {
		return targetProbeObservation{}, false
	}
	observation, ok := value.(targetProbeObservation)
	return observation, ok
}

func (r *CoordinatorRunner) actionTargetFor(taskRunID string, run Run, node dsl.Node) string {
	if r != nil {
		if value, ok := r.targetProbes.Load(strings.TrimSpace(taskRunID)); ok {
			if observation, valid := value.(targetProbeObservation); valid {
				_, target := targetIdentityFromKey(observation.key)
				return target
			}
		}
	}
	key, ok := staticTargetHealthKey(run.WorkspaceID, node)
	if !ok {
		return ""
	}
	_, target := targetIdentityFromKey(key)
	return target
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

func staticTargetHealthKey(workspaceID WorkspaceID, node dsl.Node) (store.DeadEntityKey, bool) {
	family := "toolcall"
	target := strings.TrimSpace(node.Kind)
	switch dsl.ActionKind(strings.TrimSpace(node.Kind)) {
	case dsl.ActionRunAgent:
		family = string(dsl.ActionRunAgent)
		var params dsl.RunAgentParams
		if err := node.Params.Decode(&params); err != nil {
			return store.DeadEntityKey{}, false
		}
		target = strings.TrimSpace(params.Agent)
	case dsl.ActionRunLoop:
		family = "run-loop"
		var params dsl.RunLoopParams
		if err := node.Params.Decode(&params); err != nil {
			return store.DeadEntityKey{}, false
		}
		target = strings.TrimSpace(params.Loop)
	case dsl.ActionTransform, dsl.ActionGoal:
		return store.DeadEntityKey{}, false
	}
	if target == "" || strings.Contains(target, "{{") {
		return store.DeadEntityKey{}, false
	}
	key := store.DeadEntityKey{
		WorkspaceID: string(workspaceID),
		Kind:        store.DeadEntityKindLoopTarget,
		EntityID:    family + ":" + target,
	}.Normalize()
	return key, key.Validate() == nil
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
