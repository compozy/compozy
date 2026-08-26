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
	stopWhen, err := evaluateContractStopWhen(
		ctx, conditionRun, generation, resolved, topology, advancedOutputs, conditionHistory,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	predicateEvaluations := &gateEvaluationCollector{}
	predicateEvaluations.recordPredicate(stopWhen.diagnostics...)
	if err := applyGateEvaluationIntents(&plan, run, generation, predicateEvaluations); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if stopWhen.terminal != nil {
		plan.Terminal = stopWhen.terminal
		return plan, nil
	}
	if stopWhen.present && !stopWhen.stop {
		return r.buildStopWhenNextGenerationPlan(
			ctx, taskRun, run, generation, resolved.Definition.Graph,
			gateEvaluator != nil, plan, advancedOutputs,
		)
	}
	doneEvaluation, err := evaluateDefinitionOfDone(
		ctx, conditionRun, generation, resolved, effective, topology, gateEvaluator,
		r.store, r.controls, r.runtimeCatalog, advancedOutputs, conditionHistory, r.now().UTC(),
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if doneEvaluation.gate != nil {
		evaluations := &gateEvaluationCollector{}
		evaluations.recordWithControl(
			doneEvaluation.gate.runtime,
			0,
			doneEvaluation.gate.verdict,
			doneEvaluation.gate.control,
			doneEvaluation.gate.at,
		)
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
	if deferTargetUnavailableFailure(&plan, failed, live) {
		return plan, nil
	}
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
	if normalizeReattemptStrategy(run.ReattemptStrategy) == ReattemptHalt {
		plan.Terminal = terminal
		return plan, nil
	}
	repeatedNodes, err := r.repeatedFailureNodes(ctx, run, generation, normalized)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if len(repeatedNodes) > 0 {
		plan, normalized, err = r.applyQuarantineToPlan(
			ctx, run, generation, def.Graph, plan, normalized, repeatedNodes,
		)
		if err != nil {
			return task.CoordinatorCompletionPlan{}, err
		}
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

func exactTargetUnavailableFailure(output GenerationOutput) bool {
	failure := classifyGenerationOutputFailure(output, task.Run{})
	return failure.Class == FailureTargetUnavailable && failure.Code == targetUnavailableReasonCode
}

func deferTargetUnavailableFailure(
	plan *task.CoordinatorCompletionPlan,
	failed GenerationOutput,
	live bool,
) bool {
	if !live || !exactTargetUnavailableFailure(failed) {
		return false
	}
	plan.GenerationInFlight = true
	plan.Yield = true
	return true
}
