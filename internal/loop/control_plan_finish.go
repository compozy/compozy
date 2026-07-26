package loop

import (
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/task"
)

func finishInitialControlPlan(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	gateEvaluator gate.GateEvaluator,
	outputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
) (task.CoordinatorCompletionPlan, error) {
	graph := resolved.Definition.Graph
	postReserveOutputs := cloneGenerationOutputs(outputs)
	if err := appendReadyNodeRunsControlAware(
		plan,
		run,
		generation,
		resolved,
		topology,
		gateEvaluator != nil,
		postReserveOutputs,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if len(plan.NodeRuns) > 0 {
		plan.PostReserveSnapshot = generationSnapshotWithOutputs(
			run.ID,
			generation,
			postReserveOutputs,
			outputBlobs,
		)
		return *plan, nil
	}
	if allGenerationOutputsSucceededControlAware(graph, topology, outputs) {
		plan.Terminal = &task.CoordinatorTerminal{
			Status: string(StatusDone),
			Cause:  string(TransitionCauseContract),
		}
		return *plan, nil
	}
	plan.Terminal = noReadyNodesTerminal()
	return *plan, nil
}

func generationSnapshotPayload(
	outputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
) GenerationSnapshotPayload {
	return GenerationSnapshotPayload{Outputs: outputs, OutputBlobs: outputBlobs}
}

func generationSnapshotWithOutputs(
	runID RunID,
	generation int,
	outputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
) *task.GenerationSnapshot {
	return &task.GenerationSnapshot{
		LoopRunID:  string(runID),
		Generation: generation,
		Payload:    generationSnapshotPayload(outputs, outputBlobs),
	}
}
