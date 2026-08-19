package loop

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) finishIdleGenerationPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	topology controlTopology,
	gateEvaluator gate.GateEvaluator,
	plan task.CoordinatorCompletionPlan,
	advancedOutputs []GenerationOutput,
	gateEvaluations *gateEvaluationCollector,
	history GenerationHistory,
) (task.CoordinatorCompletionPlan, error) {
	graph := resolved.Definition.Graph
	if causes := gateEvaluations.routeCauses(); routeActionForCauses(causes) != "" {
		failed, ok := firstFailedGenerationOutput(advancedOutputs)
		if !ok {
			return task.CoordinatorCompletionPlan{}, fmt.Errorf(
				"%w: route-causing gate did not produce a failed output",
				ErrValidation,
			)
		}
		return r.buildFailedGenerationPlan(
			ctx,
			taskRun,
			run,
			generation,
			resolved.Definition,
			effective,
			plan,
			advancedOutputs,
			failed,
			false,
			plan.RunStops,
			gateEvaluations,
		)
	}
	if failed := selectFailedOutputControlAware(graph, topology, advancedOutputs); failed != nil {
		return r.buildFailedGenerationPlan(
			ctx, taskRun, run, generation, resolved.Definition, effective, plan,
			advancedOutputs, *failed, false, plan.RunStops, gateEvaluations,
		)
	}
	if allGenerationOutputsSucceededControlAware(graph, topology, advancedOutputs) {
		return r.finishSucceededGenerationPlan(
			ctx,
			taskRun,
			run,
			generation,
			resolved,
			effective,
			topology,
			gateEvaluator,
			plan,
			advancedOutputs,
			history,
		)
	}
	parkedPlan, parked, err := r.applyParkedDependencyAttention(
		ctx, run, graph, topology, advancedOutputs, plan,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if parked {
		return parkedPlan, nil
	}
	plan.Terminal = noReadyNodesTerminal()
	return plan, nil
}
