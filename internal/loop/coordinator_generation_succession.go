package loop

import (
	"context"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) finishSucceededGenerationPlan(
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
	history GenerationHistory,
) (task.CoordinatorCompletionPlan, error) {
	conditionRun, conditionHistory, err := pendingBestConditionContext(
		run,
		history,
		advancedOutputs,
		plan.Snapshot,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	stopWhen, hasStopWhen, err := evaluateContractStopWhen(
		ctx, conditionRun, generation, resolved, topology, advancedOutputs, conditionHistory,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if hasStopWhen && !stopWhen {
		return r.buildStopWhenNextGenerationPlan(
			ctx, taskRun, run, generation, resolved.Definition.Graph,
			gateEvaluator != nil, plan, advancedOutputs,
		)
	}
	doneEvaluation, err := evaluateDefinitionOfDone(
		ctx, conditionRun, generation, resolved, effective, topology, gateEvaluator,
		r.store, r.runtimeCatalog, advancedOutputs, conditionHistory,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if doneEvaluation.gate != nil {
		evaluations := &gateEvaluationCollector{}
		evaluations.record(doneEvaluation.gate.runtime, 0, doneEvaluation.gate.verdict)
		if err := applyGateEvaluationIntents(&plan, run, generation, evaluations); err != nil {
			return task.CoordinatorCompletionPlan{}, err
		}
		if doneEvaluation.gate.verdict.Route.Action == gate.RouteNextGeneration {
			return r.buildGateSuccessionPlan(
				ctx, taskRun, run, generation, resolved.Definition.Graph,
				gateEvaluator != nil, plan, advancedOutputs, evaluations.routeCauses(),
			)
		}
	}
	plan.Terminal = doneEvaluation.terminal
	return plan, nil
}

func pendingBestConditionContext(
	run Run,
	history GenerationHistory,
	outputs []GenerationOutput,
	snapshot task.GenerationSnapshot,
) (Run, GenerationHistory, error) {
	payload, err := GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return Run{}, GenerationHistory{}, err
	}
	if payload.BestUpdate != nil {
		run.BestGeneration = new(payload.BestUpdate.Generation)
		run.BestScore = new(payload.BestUpdate.Score)
	}
	if run.BestGeneration == nil || run.BestScore == nil ||
		*run.BestGeneration != int64(snapshot.Generation) {
		return run, history, nil
	}
	best, err := projectBestGeneration(run, outputs)
	if err != nil {
		return Run{}, GenerationHistory{}, err
	}
	history.Best = &best
	return run, history, nil
}

func (r *CoordinatorRunner) buildStopWhenNextGenerationPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
	graph dsl.Graph,
	gatesEnabled bool,
	plan task.CoordinatorCompletionPlan,
	advancedOutputs []GenerationOutput,
) (task.CoordinatorCompletionPlan, error) {
	nextGeneration := generation + 1
	if terminal := iterationCapTerminal(run, nextGeneration); terminal != nil {
		plan.Terminal = terminal
		return plan, nil
	}
	intent := GenerationIntent{
		Generation:       int64(nextGeneration),
		ParentGeneration: int64(generation),
		Origin:           OriginStopWhen,
	}
	if denied, deniedPlan := r.dispatchGenerationPre(ctx, taskRun, run, intent); denied {
		deniedPlan.Snapshot = plan.Snapshot
		return deniedPlan, nil
	}
	nextPlan, err := buildFreshGenerationCoordinatorPlan(
		taskRun, run, generation, nextGeneration, graph, gatesEnabled,
		advancedOutputs, plan.RunStops,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	nextPlan.Snapshot = plan.Snapshot
	if err := applyGenerationIntent(&nextPlan, intent); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	r.dispatchGenerationPost(ctx, taskRun, run, intent)
	return nextPlan, nil
}

func (r *CoordinatorRunner) buildFailedGenerationPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
	def dsl.Definition,
	effective EffectiveConfig,
	plan task.CoordinatorCompletionPlan,
	normalized []GenerationOutput,
	failed GenerationOutput,
	live bool,
	loopStops []task.CoordinatorStopSpec,
	evaluations *gateEvaluationCollector,
) (task.CoordinatorCompletionPlan, error) {
	if !live {
		if evaluations == nil || len(evaluations.routeCauses()) == 0 {
			loaded, err := r.loadPersistedRouteCauses(ctx, run, generation, def.Graph, normalized)
			if err != nil {
				return task.CoordinatorCompletionPlan{}, err
			}
			evaluations = loaded
		}
		if causes := evaluations.routeCauses(); routeActionForCauses(causes) != "" {
			return r.buildGateSuccessionPlan(
				ctx, taskRun, run, generation, def.Graph, r.gateEvaluator != nil,
				plan, normalized, causes,
			)
		}
	}
	terminal, terminalErr := r.terminalForFailedGeneration(
		ctx, run, generation, effective.NoProgressWindow, def.Graph, normalized, failed,
	)
	if terminalErr != nil {
		return task.CoordinatorCompletionPlan{}, terminalErr
	}
	if live || terminal.Status != string(StatusFailed) {
		plan.Terminal = terminal
		return plan, nil
	}
	nextGeneration := generation + 1
	if terminal := iterationCapTerminal(run, nextGeneration); terminal != nil {
		plan.Terminal = terminal
		return plan, nil
	}
	intent := GenerationIntent{
		Generation:       int64(nextGeneration),
		ParentGeneration: int64(generation),
		Origin:           OriginReattempt,
	}
	if denied, deniedPlan := r.dispatchGenerationPre(ctx, taskRun, run, intent); denied {
		deniedPlan.Snapshot = plan.Snapshot
		return deniedPlan, nil
	}
	reattemptPlan, err := buildReattemptCoordinatorPlan(
		taskRun, run, generation, nextGeneration, def.Graph, normalized, loopStops,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	reattemptPlan.Snapshot = plan.Snapshot
	if err := applyGenerationIntent(&reattemptPlan, intent); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	r.dispatchGenerationPost(ctx, taskRun, run, intent)
	return reattemptPlan, nil
}
