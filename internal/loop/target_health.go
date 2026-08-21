package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/deadentity"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
)

const targetUnavailableReasonCode = "target_unavailable"

// TargetHealth is the loop-owned admission and accounting seam over the
// daemon-global loop-target dead-entity service.
type TargetHealth interface {
	BeforeProbe(context.Context, store.DeadEntityKey) (deadentity.ProbeDecision, error)
	RecordFailure(context.Context, store.DeadEntityKey, deadentity.FailureClass, string) error
	RecordSuccess(context.Context, store.DeadEntityKey) error
}

type targetHealthStatusReader interface {
	Status(context.Context, store.DeadEntityKey) (deadentity.Status, error)
}

type targetProbeObservation struct {
	key      store.DeadEntityKey
	decision deadentity.ProbeDecision
}

func (r *CoordinatorRunner) admitActionTarget(
	ctx context.Context,
	taskRunID string,
	node dsl.Node,
	input ActionExecutionInput,
) error {
	if r == nil || r.targetHealth == nil {
		return nil
	}
	key, bound, err := targetHealthKey(input.WorkspaceID, node, input)
	if err != nil || !bound {
		return err
	}
	taskRunID = strings.TrimSpace(taskRunID)
	if taskRunID == "" {
		return fmt.Errorf("%w: target probe task run id is required", ErrValidation)
	}
	decision, err := r.targetHealth.BeforeProbe(ctx, key)
	if err != nil {
		return fmt.Errorf("loop: check target health for %q: %w", key.EntityID, err)
	}
	if !decision.Allowed {
		return targetUnavailableError(key, decision)
	}
	r.targetProbes.Store(taskRunID, targetProbeObservation{key: key, decision: decision})
	return nil
}

func targetHealthKey(
	workspaceID WorkspaceID,
	node dsl.Node,
	input ActionExecutionInput,
) (store.DeadEntityKey, bool, error) {
	family, target, bound, err := actionTargetIdentity(node, input)
	if err != nil || !bound {
		return store.DeadEntityKey{}, false, err
	}
	profileID := strings.TrimSpace(input.ToolScope.ProfileID)
	if profileID == "" {
		return store.DeadEntityKey{}, false, fmt.Errorf("%w: target health profile id is required", ErrValidation)
	}
	key := store.DeadEntityKey{
		ProfileID:   profileID,
		WorkspaceID: string(workspaceID),
		Kind:        store.DeadEntityKindLoopTarget,
		EntityID:    family + ":" + target,
	}.Normalize()
	if err := key.Validate(); err != nil {
		return store.DeadEntityKey{}, false, err
	}
	return key, true, nil
}

func actionTargetIdentity(node dsl.Node, input ActionExecutionInput) (string, string, bool, error) {
	kind := dsl.ActionKind(strings.TrimSpace(node.Kind))
	switch kind {
	case dsl.ActionRunAgent:
		params, err := actionParamsExcept(node, input, map[string]struct{}{outputSchemaParamKey: {}})
		if err != nil {
			return "", "", false, err
		}
		var spec dsl.RunAgentParams
		if err := dsl.NodeParams(params).Decode(&spec); err != nil {
			return "", "", false, fmt.Errorf("decode run-agent target: %w", err)
		}
		return normalizedActionTarget("run-agent", spec.Agent)
	case dsl.ActionRunLoop:
		params, err := actionParams(node, input)
		if err != nil {
			return "", "", false, err
		}
		var spec dsl.RunLoopParams
		if err := dsl.NodeParams(params).Decode(&spec); err != nil {
			return "", "", false, fmt.Errorf("decode run-loop target: %w", err)
		}
		return normalizedActionTarget("run-loop", spec.Loop)
	case dsl.ActionTransform, dsl.ActionGoal:
		return "", "", false, nil
	default:
		return normalizedActionTarget("toolcall", node.Kind)
	}
}

func normalizedActionTarget(family, target string) (string, string, bool, error) {
	family = strings.TrimSpace(family)
	target = strings.TrimSpace(target)
	if family == "" || target == "" {
		return "", "", false, fmt.Errorf("%w: target-bound action identity is incomplete", ErrValidation)
	}
	return family, target, true, nil
}

func targetUnavailableError(key store.DeadEntityKey, decision deadentity.ProbeDecision) error {
	cause := fmt.Sprintf("loop target %q is unavailable", key.EntityID)
	if reason := strings.TrimSpace(decision.Reason); reason != "" {
		cause += ": " + reason
	}
	return newSafeActionFailureError(
		errors.New("loop: target unavailable"),
		NewActionFailure(
			targetUnavailableReasonCode,
			cause,
			"Restore the target or wait for the recovery probe before requeueing the node.",
		),
	)
}

func targetIdentityFromKey(key store.DeadEntityKey) (family, target string) {
	family, target, _ = strings.Cut(strings.TrimSpace(key.EntityID), ":")
	return strings.TrimSpace(family), strings.TrimSpace(target)
}
