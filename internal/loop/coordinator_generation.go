package loop

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/task"
)

type generationFinisherState struct {
	outputs         []GenerationOutput
	attempts        []NodeAttempt
	controls        []NodeControlMutation
	events          []GenerationLifecycleEventIntent
	topology        controlTopology
	controlTerminal *task.CoordinatorTerminal
	loopStops       []task.CoordinatorStopSpec
	live            bool
}

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
	state, err := r.prepareGenerationFinisherState(ctx, run, generation, resolved, effective, outputs)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	failed := selectFailedOutput(state.outputs)
	plan := coordinatorFinisherPlan(run, generation, state.outputs, state.loopStops)
	if err := appendAttemptsToGenerationSnapshot(&plan.Snapshot, state.attempts); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := appendControlsToGenerationSnapshot(&plan.Snapshot, state.controls); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := appendLifecycleEventsToGenerationSnapshot(&plan.Snapshot, state.events); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	if err := appendRetryScheduleIntents(&plan, state.outputs, state.attempts); err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan.GenerationInFlight = state.live
	if state.controlTerminal != nil {
		return r.buildGoalControlFinisherPlan(
			ctx, taskRun, run, generation, resolved, effective, state.topology,
			fanOutWidth, plan, state.outputs, state.live, state.controlTerminal,
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
			state.outputs,
			*failed,
			state.live,
			state.loopStops,
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
		state.topology,
		r.gateEvaluator,
		fanOutWidth,
		r.watchRuntime(),
		r.watchEventsRuntime(),
		plan,
		state.outputs,
		state.live,
	)
}

func (r *CoordinatorRunner) prepareGenerationFinisherState(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	effective EffectiveConfig,
	outputs []GenerationOutput,
) (generationFinisherState, error) {
	topology := newControlTopology(resolved.Definition.Graph)
	normalized, _, controlTerminal, live, loopStops, err := r.refreshGenerationOutputs(
		ctx, run, generation, resolved.Definition.Graph, topology, outputs,
	)
	if err != nil {
		return generationFinisherState{}, err
	}
	normalized, attempts, controls, events, lifecycleLive, err := r.applyNodeLifecyclePrecedence(
		ctx, run, generation, resolved, effective, normalized,
	)
	if err != nil {
		return generationFinisherState{}, err
	}
	return generationFinisherState{
		outputs: normalized, attempts: attempts, controls: controls, events: events, topology: topology,
		controlTerminal: controlTerminal, loopStops: loopStops, live: live || lifecycleLive,
	}, nil
}

func appendControlsToGenerationSnapshot(snapshot *task.GenerationSnapshot, controls []NodeControlMutation) error {
	if snapshot == nil || len(controls) == 0 {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return err
	}
	payload.Controls = append(payload.Controls, controls...)
	snapshot.Payload = payload
	return nil
}

func appendRetryScheduleIntents(
	plan *task.CoordinatorCompletionPlan,
	outputs []GenerationOutput,
	attempts []NodeAttempt,
) error {
	if plan == nil {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return err
	}
	for _, attempt := range attempts {
		if attempt.Disposition != AttemptRetried || attempt.NextAttemptAt == nil || attempt.FailureClass == nil {
			continue
		}
		output, found := generationOutputByAttempt(outputs, attempt)
		if !found {
			return fmt.Errorf(
				"%w: retry output %s/%d/a%d is missing",
				ErrValidation,
				attempt.NodeID,
				attempt.ItemIndex,
				attempt.Attempt,
			)
		}
		cell := RetryDueCell{
			LoopRunID: planRetryRunID(plan), Generation: plan.Snapshot.Generation,
			NodeID: attempt.NodeID, ItemIndex: attempt.ItemIndex, Attempt: attempt.Attempt,
			Epoch: output.Epoch, NextAttemptAt: attempt.NextAttemptAt.UTC(),
		}
		payload.Events = append(payload.Events, GenerationLifecycleEventIntent{
			Kind: GenerationLifecycleEventNodeRetryScheduled, NodeID: string(attempt.NodeID),
			ItemIndex: attempt.ItemIndex, Attempt: attempt.Attempt + 1, IssuedEpoch: output.Epoch,
			NextAttemptAt: cloneTimePointer(attempt.NextAttemptAt), FailureClass: *attempt.FailureClass,
		})
		plan.PostCommitTimers = append(plan.PostCommitTimers, task.CoordinatorTimerSpec{
			LoopRunID: string(cell.LoopRunID), Generation: cell.Generation, NodeID: string(cell.NodeID),
			ItemIndex: cell.ItemIndex, Attempt: cell.Attempt, IssuedEpoch: cell.Epoch,
			FireAt: cell.NextAttemptAt, IdempotencyKey: RetryWakeIdempotencyKey(cell),
		})
	}
	plan.Snapshot.Payload = payload
	return nil
}

func generationOutputByAttempt(outputs []GenerationOutput, attempt NodeAttempt) (GenerationOutput, bool) {
	for _, output := range outputs {
		sameCell := output.NodeID == string(attempt.NodeID) && output.ItemIndex == attempt.ItemIndex
		if sameCell && output.Attempt == attempt.Attempt {
			return output, true
		}
	}
	return GenerationOutput{}, false
}

func planRetryRunID(plan *task.CoordinatorCompletionPlan) RunID {
	if plan == nil {
		return ""
	}
	return RunID(plan.Snapshot.LoopRunID)
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
	goalPlan.GenerationInFlight = live || len(goalPlan.NodeRuns) > 0
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
	applyWaitExpiryRoutes(resolved.Definition.Graph, topology, &advancedOutputs)
	outputBlobs := []GenerationOutputBlob{}
	gateEvaluations := &gateEvaluationCollector{}
	terminal, err := advanceControlNodes(
		r.liveControlEvalContext(
			ctx, run, generation, resolved, topology, effective, gateEvaluator, fanOutWidth,
			watchRuntime, watchEventsRuntime, gateEvaluations, history,
		),
		&plan,
		&advancedOutputs,
		&outputBlobs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	payload, err := generationSnapshotPayloadPreservingIntents(
		plan.Snapshot.Payload,
		sortedGenerationOutputs(advancedOutputs),
		outputBlobs,
	)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	plan.Snapshot.Payload = payload
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

func (r *CoordinatorRunner) liveControlEvalContext(
	ctx context.Context,
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	effective EffectiveConfig,
	gateEvaluator gate.GateEvaluator,
	fanOutWidth int,
	watchRuntime coordinatorWatchRuntime,
	watchEventsRuntime coordinatorWatchEventsRuntime,
	gateEvaluations *gateEvaluationCollector,
	history GenerationHistory,
) *controlEvalContext {
	return &controlEvalContext{
		ctx: ctx, run: run, generation: generation, resolved: resolved, topology: topology,
		effective: effective, gateEvaluator: gateEvaluator, gateDecisions: r.store,
		runtimeCatalog: r.runtimeCatalog, fanOutWidth: fanOutWidth, watchRuntime: watchRuntime,
		watchEventsRuntime: watchEventsRuntime, gateEvaluations: gateEvaluations, history: history,
		now: r.now().UTC(),
	}
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
	hasReadyRuns, err := appendReadyNodeRunsToPlan(
		&plan,
		run,
		generation,
		resolved,
		topology,
		gateEvaluator,
		advancedOutputs,
		outputBlobs,
		r.now().UTC(),
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
	return r.finishIdleGenerationPlan(
		ctx, taskRun, run, generation, resolved, effective, topology, gateEvaluator,
		plan, advancedOutputs, gateEvaluations, history,
	)
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

func appendAttemptsToGenerationSnapshot(
	snapshot *task.GenerationSnapshot,
	attempts []NodeAttempt,
) error {
	if snapshot == nil || len(attempts) == 0 {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return err
	}
	payload.Attempts = append(payload.Attempts, attempts...)
	snapshot.Payload = payload
	return nil
}

func appendLifecycleEventsToGenerationSnapshot(
	snapshot *task.GenerationSnapshot,
	events []GenerationLifecycleEventIntent,
) error {
	if snapshot == nil || len(events) == 0 {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(snapshot.Payload)
	if err != nil {
		return err
	}
	payload.Events = append(payload.Events, events...)
	snapshot.Payload = payload
	return nil
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
