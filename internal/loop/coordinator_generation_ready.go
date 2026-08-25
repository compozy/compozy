package loop

import (
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func appendReadyNodeRunsToPlan(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	gateEvaluator gate.GateEvaluator,
	advancedOutputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
	scheduledAt time.Time,
) (bool, error) {
	postReserveOutputs := cloneGenerationOutputs(sortedGenerationOutputs(advancedOutputs))
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
		return false, err
	}
	if err := appendCoordinatorArtifactsForOutputs(
		plan,
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		gateEvaluator != nil,
		postReserveOutputs,
	); err != nil {
		return false, err
	}
	if len(plan.NodeRuns) == 0 {
		return false, nil
	}
	postReserveOutputs = generationOutputsExpectCurrentEpoch(postReserveOutputs)
	plan.PostReserveSnapshot = generationSnapshotWithOutputs(
		run.ID,
		generation,
		postReserveOutputs,
		outputBlobs,
	)
	return true, nil
}
