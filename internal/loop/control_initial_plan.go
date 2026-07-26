package loop

import (
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

func newInitialControlCoordinatorPlan(
	run Run,
	generation int,
	graph dsl.Graph,
	topology controlTopology,
	gatesEnabled bool,
	outputs []GenerationOutput,
) (task.CoordinatorCompletionPlan, error) {
	plan := task.CoordinatorCompletionPlan{
		NodeTasks:    make([]task.CoordinatorTaskSpec, 0, len(graph.Nodes)),
		Dependencies: make([]task.CoordinatorDependencySpec, 0, len(graph.Edges)),
		NodeRuns:     make([]task.EnqueueSpec, 0, len(graph.Nodes)),
		Snapshot: task.GenerationSnapshot{
			LoopRunID:  string(run.ID),
			Generation: generation,
		},
	}
	if err := appendCoordinatorTasksForOutputs(
		&plan,
		run,
		generation,
		graph,
		topology,
		gatesEnabled,
		outputs,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := appendCoordinatorDependenciesForOutputs(
		&plan,
		run,
		generation,
		graph,
		topology,
		gatesEnabled,
		outputs,
	); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	return plan, nil
}
