package loop

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) buildPendingRequeuePlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	graph dsl.Graph,
	currentOutputs []GenerationOutput,
) (task.CoordinatorCompletionPlan, bool, error) {
	if r.controls == nil {
		return task.CoordinatorCompletionPlan{}, false, nil
	}
	controls, err := r.controls.ListNodeControls(ctx, run.WorkspaceID, run.ID)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, fmt.Errorf("list node controls for requeue: %w", err)
	}
	nextGeneration := run.Generation + 1
	nodeID, pending, err := pendingRequeueNode(controls, nextGeneration)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	if !pending {
		return task.CoordinatorCompletionPlan{}, false, nil
	}
	if terminal := iterationCapTerminal(run, nextGeneration); terminal != nil {
		plan := coordinatorFinisherPlan(run, run.Generation, currentOutputs, nil)
		plan.Terminal = terminal
		return plan, true, nil
	}

	intent := GenerationIntent{
		Generation:       int64(nextGeneration),
		ParentGeneration: int64(run.Generation),
		Origin:           OriginRequeue,
	}
	if denied, deniedPlan := r.dispatchGenerationPre(ctx, taskRun, run, intent); denied {
		deniedPlan.Snapshot = coordinatorFinisherPlan(run, run.Generation, currentOutputs, nil).Snapshot
		return deniedPlan, true, nil
	}

	rerun := make(map[generationOutputKey]struct{})
	for _, output := range currentOutputs {
		if output.NodeID != string(nodeID) {
			continue
		}
		rerun[generationOutputKey{nodeID: output.NodeID, itemIndex: output.ItemIndex}] = struct{}{}
	}
	if len(rerun) == 0 {
		return task.CoordinatorCompletionPlan{}, false, fmt.Errorf(
			"%w: requeued node %q has no generation output",
			ErrValidation,
			nodeID,
		)
	}
	addTransitiveDependents(graph, currentOutputs, rerun)
	nextOutputs := reattemptGenerationOutputs(graph, currentOutputs, rerun, nextGeneration)
	plan, err := buildNextGenerationCoordinatorPlan(
		taskRun,
		run,
		run.Generation,
		nextGeneration,
		graph,
		false,
		currentOutputs,
		nextOutputs,
		nil,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	if err := applyGenerationIntent(&plan, intent); err != nil {
		return task.CoordinatorCompletionPlan{}, false, err
	}
	r.dispatchGenerationPost(ctx, taskRun, run, intent)
	return plan, true, nil
}
