package loop

import (
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
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
	scheduledAt time.Time,
) (task.CoordinatorCompletionPlan, error) {
	graph := resolved.Definition.Graph
	postReserveOutputs := cloneGenerationOutputs(outputs)
	if err := appendCoordinatorArtifactsForOutputs(
		plan,
		run,
		generation,
		graph,
		topology,
		gateEvaluator != nil,
		postReserveOutputs,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := appendReadyNodeRunsControlAware(
		plan,
		run,
		generation,
		resolved,
		topology,
		gateEvaluator != nil,
		postReserveOutputs,
		scheduledAt,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if len(plan.NodeRuns) > 0 {
		postReserveOutputs = generationOutputsExpectCurrentEpoch(postReserveOutputs)
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
	if generationOutputsWaiting(outputs) {
		plan.Yield = true
		plan.GenerationInFlight = true
		return *plan, nil
	}
	plan.Terminal = noReadyNodesTerminal()
	return *plan, nil
}

func generationOutputsWaiting(outputs []GenerationOutput) bool {
	for _, output := range outputs {
		if output.Status == generationOutputWaiting {
			return true
		}
	}
	return false
}

func generationSnapshotPayload(
	outputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
) GenerationSnapshotPayload {
	return GenerationSnapshotPayload{Outputs: outputs, OutputBlobs: outputBlobs}
}

func generationSnapshotPayloadPreservingIntents(
	existing any,
	outputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
) (GenerationSnapshotPayload, error) {
	payload, err := GenerationSnapshotPayloadFrom(existing)
	if err != nil {
		return GenerationSnapshotPayload{}, err
	}
	payload.Outputs = outputs
	payload.OutputBlobs = outputBlobs
	return payload, nil
}

func generationOutputsExpectCurrentEpoch(outputs []GenerationOutput) []GenerationOutput {
	for index := range outputs {
		expectedEpoch := outputs[index].Epoch
		outputs[index].ExpectedEpoch = &expectedEpoch
	}
	return outputs
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
