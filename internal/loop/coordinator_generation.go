package loop

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

func (r *CoordinatorRunner) buildGenerationFinisherPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	fanOutWidth int,
	outputs []GenerationOutput,
) (task.CoordinatorCompletionPlan, error) {
	def := resolved.Definition
	graph := def.Graph
	topology := newControlTopology(graph)
	normalized, failed, controlTerminal, live, loopStops, err := r.refreshGenerationOutputs(
		ctx,
		run,
		generation,
		graph,
		topology,
		outputs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan := coordinatorFinisherPlan(run, generation, normalized, loopStops)
	plan.GenerationInFlight = live
	if controlTerminal != nil {
		return r.buildGoalControlFinisherPlan(
			ctx, taskRun, run, generation, resolved, effective, topology,
			fanOutWidth, plan, normalized, live, controlTerminal,
		)
	}
	if failed != nil {
		return r.buildFailedGenerationPlan(
			ctx,
			taskRun,
			run,
			generation,
			def,
			effective,
			plan,
			normalized,
			*failed,
			live,
			loopStops,
			nil,
		)
	}
	return r.buildLiveGenerationPlan(
		ctx,
		taskRun,
		run,
		generation,
		resolved,
		effective,
		topology,
		r.gateEvaluator,
		fanOutWidth,
		r.watchRuntime(),
		r.watchEventsRuntime(),
		plan,
		normalized,
		live,
	)
}

func (r *CoordinatorRunner) buildGoalControlFinisherPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	topology controlTopology,
	fanOutWidth int,
	plan task.CoordinatorCompletionPlan,
	normalized []GenerationOutput,
	live bool,
	controlTerminal *task.CoordinatorTerminal,
) (task.CoordinatorCompletionPlan, error) {
	goalPlan, err := r.buildLiveGenerationPlan(
		ctx,
		taskRun,
		run,
		generation,
		resolved,
		effective,
		topology,
		r.gateEvaluator,
		fanOutWidth,
		r.watchRuntime(),
		r.watchEventsRuntime(),
		plan,
		normalized,
		live,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	goalPlan.Terminal = controlTerminal
	goalPlan.Yield = false
	goalPlan.GenerationInFlight = goalPlan.GenerationInFlight || len(goalPlan.NodeRuns) > 0
	return goalPlan, nil
}

func (r *CoordinatorRunner) buildLiveGenerationPlan(
	ctx context.Context,
	taskRun task.Run,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	topology controlTopology,
	gateEvaluator gate.GateEvaluator,
	fanOutWidth int,
	watchRuntime coordinatorWatchRuntime,
	watchEventsRuntime coordinatorWatchEventsRuntime,
	plan task.CoordinatorCompletionPlan,
	normalized []GenerationOutput,
	live bool,
) (task.CoordinatorCompletionPlan, error) {
	history, err := r.readGenerationHistory(ctx, run, generation)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	advancedOutputs := cloneGenerationOutputs(normalized)
	outputBlobs := []GenerationOutputBlob{}
	gateEvaluations := &gateEvaluationCollector{}
	terminal, err := advanceControlNodes(
		&controlEvalContext{
			ctx:                ctx,
			run:                run,
			generation:         generation,
			resolved:           resolved,
			topology:           topology,
			effective:          effective,
			gateEvaluator:      gateEvaluator,
			gateDecisions:      r.store,
			runtimeCatalog:     r.runtimeCatalog,
			fanOutWidth:        fanOutWidth,
			watchRuntime:       watchRuntime,
			watchEventsRuntime: watchEventsRuntime,
			gateEvaluations:    gateEvaluations,
			history:            history,
		},
		&plan,
		&advancedOutputs,
		&outputBlobs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan.Snapshot.Payload = generationSnapshotPayload(
		sortedGenerationOutputs(advancedOutputs),
		outputBlobs,
	)
	if err := applyGateEvaluationIntents(&plan, run, generation, gateEvaluations); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if terminal != nil {
		plan.Terminal = terminal
	}
	if terminal != nil || plan.Yield {
		return plan, nil
	}
	return r.finishLiveGenerationPlan(
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
		outputBlobs,
		live,
		gateEvaluations,
		history,
	)
}

func (r *CoordinatorRunner) finishLiveGenerationPlan(
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
	outputBlobs []GenerationOutputBlob,
	live bool,
	gateEvaluations *gateEvaluationCollector,
	history GenerationHistory,
) (task.CoordinatorCompletionPlan, error) {
	graph := resolved.Definition.Graph
	hasReadyRuns, err := appendReadyNodeRunsToPlan(
		&plan,
		run,
		generation,
		resolved,
		topology,
		gateEvaluator,
		advancedOutputs,
		outputBlobs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if hasReadyRuns {
		return plan, nil
	}
	if live {
		plan.Yield = true
		return plan, nil
	}
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
	plan.Terminal = noReadyNodesTerminal()
	return plan, nil
}

func appendReadyNodeRunsToPlan(
	plan *task.CoordinatorCompletionPlan,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	gateEvaluator gate.GateEvaluator,
	advancedOutputs []GenerationOutput,
	outputBlobs []GenerationOutputBlob,
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
	); err != nil {
		return false, err
	}
	if len(plan.NodeRuns) == 0 {
		return false, nil
	}
	plan.PostReserveSnapshot = generationSnapshotWithOutputs(
		run.ID,
		generation,
		postReserveOutputs,
		outputBlobs,
	)
	return true, nil
}

func iterationCapTerminal(run Run, generation int) *task.CoordinatorTerminal {
	if run.IterationCap <= 0 || generation <= run.IterationCap {
		return nil
	}
	return &task.CoordinatorTerminal{
		Status:     string(StatusExhausted),
		Cause:      string(TransitionCauseIterationCap),
		ReasonCode: "iteration_cap_exceeded",
	}
}

func cloneGenerationOutputs(outputs []GenerationOutput) []GenerationOutput {
	cloned := make([]GenerationOutput, len(outputs))
	copy(cloned, outputs)
	return cloned
}

func coordinatorFinisherPlan(
	run Run,
	generation int,
	outputs []GenerationOutput,
	loopStops []task.CoordinatorStopSpec,
) task.CoordinatorCompletionPlan {
	return task.CoordinatorCompletionPlan{
		RunStops: loopStops,
		Snapshot: task.GenerationSnapshot{
			LoopRunID:  string(run.ID),
			Generation: generation,
			Payload:    GenerationSnapshotPayload{Outputs: outputs},
		},
	}
}
