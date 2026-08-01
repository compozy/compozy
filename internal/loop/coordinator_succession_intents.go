package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/task"
)

func applyGateEvaluationIntents(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	evaluations *gateEvaluationCollector,
) error {
	if plan == nil || evaluations == nil || len(evaluations.values) == 0 {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return err
	}
	verdicts, bestUpdate, err := evaluations.intents(run, generation)
	if err != nil {
		return err
	}
	knownVerdicts := make(map[gateEvaluationKey]struct{}, len(payload.Verdicts)+len(verdicts))
	for _, verdict := range payload.Verdicts {
		knownVerdicts[gateEvaluationKey{gateID: verdict.GateID, itemIndex: verdict.ItemIndex}] = struct{}{}
	}
	for _, verdict := range verdicts {
		key := gateEvaluationKey{gateID: verdict.GateID, itemIndex: verdict.ItemIndex}
		if _, exists := knownVerdicts[key]; exists {
			return fmt.Errorf(
				"%w: duplicate verdict intent for gate %q item %d",
				ErrValidation,
				verdict.GateID,
				verdict.ItemIndex,
			)
		}
		knownVerdicts[key] = struct{}{}
		payload.Verdicts = append(payload.Verdicts, verdict)
	}
	if bestUpdate != nil {
		if payload.BestUpdate != nil {
			return fmt.Errorf("%w: multiple metric best updates in one generation", ErrValidation)
		}
		payload.BestUpdate = bestUpdate
	}
	bestGeneration := run.BestGeneration
	if payload.BestUpdate != nil {
		bestGeneration = new(payload.BestUpdate.Generation)
	}
	for _, evaluation := range evaluations.ordered() {
		payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
			Kind:      GenerationLifecycleEventGateVerdict,
			GateID:    evaluation.runtime.ID,
			ItemIndex: evaluation.itemIndex,
			Route:     evaluation.verdict.Route.Action,
			Reason: firstNonEmpty(
				evaluation.verdict.Route.ReasonCode,
				string(evaluation.verdict.Outcome),
			),
			BestGeneration: cloneInt64(bestGeneration),
		})
	}
	plan.Snapshot.Payload = payload
	return nil
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	return new(*value)
}

func applyGenerationIntent(
	plan *task.CoordinatorCompletionPlan,
	intent GenerationIntent,
) error {
	if plan == nil || plan.PostReserveSnapshot == nil {
		return fmt.Errorf("%w: next-generation snapshot is required", ErrValidation)
	}
	if err := intent.Validate(); err != nil {
		return err
	}
	if int64(plan.PostReserveSnapshot.Generation) != intent.Generation {
		return fmt.Errorf("%w: next-generation snapshot does not match provenance", ErrValidation)
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.PostReserveSnapshot.Payload)
	if err != nil {
		return err
	}
	payload.GenerationProvenance = &intent
	payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
		Kind: GenerationLifecycleEventGenerationStarted,
	})
	plan.PostReserveSnapshot.Payload = payload
	return nil
}
