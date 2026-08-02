package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func attachCoordinatorEffectIntents(
	run Run,
	resolved *ResolvedDefinition,
	plan *task.CoordinatorCompletionPlan,
) error {
	if resolved == nil || plan == nil {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return err
	}
	for _, attempt := range payload.Attempts {
		node, found := graphNode(resolved.Definition.Graph, attempt.NodeID)
		if !found {
			return fmt.Errorf("%w: effect attempt references unknown node %q", ErrValidation, attempt.NodeID)
		}
		if err := attachAttemptEffectIntents(run, node, attempt, &payload); err != nil {
			return err
		}
	}
	if err := attachPauseEventEffects(
		run, resolved.Definition.Graph, max(1, plan.Snapshot.Generation), &payload,
	); err != nil {
		return err
	}
	if plan.Terminal != nil {
		if err := attachBoundaryEffectIntents(
			run,
			resolved.Definition.Contract,
			Status(plan.Terminal.Status),
			max(1, plan.Snapshot.Generation),
			&payload,
		); err != nil {
			return err
		}
	}
	if err := attachBoundaryEffectIntents(
		run,
		resolved.Definition.Contract,
		StatusExhausted,
		max(1, plan.Snapshot.Generation),
		&payload,
	); err != nil {
		return err
	}
	plan.Snapshot.Payload = payload
	return nil
}

func attachBoundaryEffectIntents(
	run Run,
	contract dsl.Contract,
	status Status,
	generation int,
	payload *GenerationSnapshotPayload,
) error {
	if payload == nil {
		return nil
	}
	effects, trigger := terminalEffectSpecs(contract, status)
	if len(effects) == 0 {
		return nil
	}
	if payload.BoundaryEffects == nil {
		payload.BoundaryEffects = make(map[Status][]RenderedEffectIntent)
	}
	if _, exists := payload.BoundaryEffects[status]; exists {
		return nil
	}
	intents, err := RenderEffectIntents(effects, EffectContextRequest{
		Run: &run, Trigger: trigger, Generation: generation,
	})
	if err != nil {
		return err
	}
	payload.BoundaryEffects[status] = intents
	return nil
}

func attachAttemptEffectIntents(
	run Run,
	node dsl.Node,
	attempt NodeAttempt,
	payload *GenerationSnapshotPayload,
) error {
	if payload == nil {
		return nil
	}
	if hasNodePauseEvent(*payload, attempt) {
		return nil
	}
	switch attempt.Disposition {
	case AttemptRetried:
		return attachRetryEffectIntents(run, node, attempt, payload)
	case AttemptSucceeded:
		return appendNodeEffectEvent(run, node.OnSuccess, EffectTriggerOnSuccess, attempt, payload)
	case AttemptRouted, AttemptAbsorbed, AttemptEscalated:
		if err := appendNodeEffectEvent(
			run, nodeErrorEffects(node), EffectTriggerOnError, attempt, payload,
		); err != nil {
			return err
		}
		if attempt.FailureClass != nil && *attempt.FailureClass == FailureAttemptTimeout {
			return appendNodeEffectEvent(run, node.OnTimeout, EffectTriggerOnTimeout, attempt, payload)
		}
	case AttemptQuarantined:
		return attachQuarantineEffectIntents(run, node, attempt, payload)
	case AttemptCanceled:
		return appendNodeEffectEvent(run, node.OnCancel, EffectTriggerOnCancel, attempt, payload)
	}
	return nil
}

func hasNodePauseEvent(payload GenerationSnapshotPayload, attempt NodeAttempt) bool {
	for _, event := range payload.Events {
		if event.Kind == GenerationLifecycleEventNodePaused && event.NodeID == string(attempt.NodeID) &&
			event.ItemIndex == attempt.ItemIndex && event.Attempt == attempt.Attempt {
			return true
		}
	}
	return false
}

func attachPauseEventEffects(
	run Run,
	graph dsl.Graph,
	generation int,
	payload *GenerationSnapshotPayload,
) error {
	if payload == nil {
		return nil
	}
	for index := range payload.Events {
		event := &payload.Events[index]
		if event.Kind != GenerationLifecycleEventNodePaused &&
			event.Kind != GenerationLifecycleEventNodeWaitStarted {
			continue
		}
		node, found := graphNode(graph, dsl.NodeID(event.NodeID))
		if !found {
			return fmt.Errorf("%w: pause effect event references unknown node %q", ErrValidation, event.NodeID)
		}
		if len(node.OnPause) == 0 || len(event.Effects) > 0 {
			continue
		}
		intents, err := RenderEffectIntents(node.OnPause, EffectContextRequest{
			Run: &run, Trigger: EffectTriggerOnPause, Generation: generation,
			NodeID: dsl.NodeID(event.NodeID), ItemIndex: event.ItemIndex, Attempt: event.Attempt,
		})
		if err != nil {
			return err
		}
		event.Effects = append(event.Effects, intents...)
	}
	return nil
}

func attachQuarantineEffectIntents(
	run Run,
	node dsl.Node,
	attempt NodeAttempt,
	payload *GenerationSnapshotPayload,
) error {
	if len(node.OnQuarantine) == 0 {
		return nil
	}
	failure := effectFailureFromAttempt(attempt)
	for index := range payload.Events {
		event := &payload.Events[index]
		if event.Kind != GenerationLifecycleEventNodeQuarantined ||
			event.NodeID != string(attempt.NodeID) || event.ItemIndex != attempt.ItemIndex {
			continue
		}
		intents, err := RenderEffectIntents(node.OnQuarantine, EffectContextRequest{
			Run: &run, Trigger: EffectTriggerOnQuarantine, Generation: attempt.Generation,
			NodeID: attempt.NodeID, ItemIndex: attempt.ItemIndex, Attempt: attempt.Attempt,
			Disposition: attempt.Disposition, Failure: failure, Quarantine: event.QuarantineEntry,
		})
		if err != nil {
			return err
		}
		event.Effects = append(event.Effects, intents...)
		return nil
	}
	return fmt.Errorf("%w: on_quarantine effect has no quarantine lifecycle event", ErrValidation)
}

func attachRetryEffectIntents(
	run Run,
	node dsl.Node,
	attempt NodeAttempt,
	payload *GenerationSnapshotPayload,
) error {
	if len(node.OnRetry) == 0 {
		return nil
	}
	failure := effectFailureFromAttempt(attempt)
	intents, err := RenderEffectIntents(node.OnRetry, EffectContextRequest{
		Run: &run, Trigger: EffectTriggerOnRetry, Generation: attempt.Generation,
		NodeID: attempt.NodeID, ItemIndex: attempt.ItemIndex, Attempt: attempt.Attempt + 1,
		NextAttemptAt: attempt.NextAttemptAt, Disposition: attempt.Disposition, Failure: failure,
	})
	if err != nil {
		return err
	}
	for index := range payload.Events {
		event := &payload.Events[index]
		matches := event.Kind == GenerationLifecycleEventNodeRetryScheduled &&
			event.NodeID == string(attempt.NodeID) && event.ItemIndex == attempt.ItemIndex &&
			event.Attempt == attempt.Attempt+1
		if matches {
			event.Effects = append(event.Effects, intents...)
			return nil
		}
	}
	return fmt.Errorf("%w: on_retry effect has no retry lifecycle event", ErrValidation)
}

func appendNodeEffectEvent(
	run Run,
	effects []dsl.EffectSpec,
	trigger EffectTrigger,
	attempt NodeAttempt,
	payload *GenerationSnapshotPayload,
) error {
	if len(effects) == 0 {
		return nil
	}
	failure := effectFailureFromAttempt(attempt)
	intents, err := RenderEffectIntents(effects, EffectContextRequest{
		Run: &run, Trigger: trigger, Generation: attempt.Generation, NodeID: attempt.NodeID,
		ItemIndex: attempt.ItemIndex, Attempt: attempt.Attempt, NextAttemptAt: attempt.NextAttemptAt,
		Disposition: attempt.Disposition, Failure: failure,
	})
	if err != nil {
		return err
	}
	kind := GenerationLifecycleEventNodeSucceeded
	if trigger == EffectTriggerOnCancel {
		kind = GenerationLifecycleEventNodeCanceled
	} else if trigger != EffectTriggerOnSuccess {
		kind = GenerationLifecycleEventNodeFailed
	}
	for index := range payload.Events {
		event := &payload.Events[index]
		if event.Kind == kind && event.NodeID == string(attempt.NodeID) &&
			event.ItemIndex == attempt.ItemIndex && event.Attempt == attempt.Attempt &&
			event.Disposition == attempt.Disposition {
			event.Effects = append(event.Effects, intents...)
			return nil
		}
	}
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: kind, NodeID: string(attempt.NodeID), ItemIndex: attempt.ItemIndex,
		Attempt: attempt.Attempt, Failure: failure, Disposition: attempt.Disposition,
		Effects: intents,
	})
	return nil
}

func nodeErrorEffects(node dsl.Node) []dsl.EffectSpec {
	if node.OnError == nil {
		return nil
	}
	return node.OnError.Effects
}

func effectFailureFromAttempt(attempt NodeAttempt) *ClassifiedFailure {
	if attempt.FailureClass == nil {
		return nil
	}
	failure := classifiedFailureFromAttempt(attempt)
	return &failure
}

func terminalEffectSpecs(contract dsl.Contract, status Status) ([]dsl.EffectSpec, EffectTrigger) {
	if contract.ContractLifecycleState == nil {
		return nil, ""
	}
	switch status {
	case StatusDone:
		return contract.OnDone, EffectTriggerOnDone
	case StatusNoOp:
		return contract.OnNoOp, EffectTriggerOnNoOp
	case StatusBlocked:
		return contract.OnBlocked, EffectTriggerOnBlocked
	case StatusFailed:
		return contract.OnFailed, EffectTriggerOnFailed
	case StatusExhausted:
		return contract.OnExhausted, EffectTriggerOnExhausted
	case StatusStalled:
		return contract.OnStalled, EffectTriggerOnStalled
	case StatusCanceled:
		return contract.OnCanceled, EffectTriggerOnCanceled
	default:
		return nil, ""
	}
}
