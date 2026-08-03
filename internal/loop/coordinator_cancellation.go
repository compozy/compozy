package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/task"
)

type coordinatorCancellationState struct {
	outputs         []GenerationOutput
	attempts        []NodeAttempt
	events          []GenerationLifecycleEventIntent
	controls        []NodeControlMutation
	active          bool
	waitingDelivery bool
	live            bool
}

func (r *CoordinatorRunner) prepareCoordinatorCancellation(
	ctx context.Context,
	run Run,
	outputs []GenerationOutput,
) (coordinatorCancellationState, error) {
	state := coordinatorCancellationState{outputs: cloneGenerationOutputs(outputs)}
	controls, err := r.listNodeControls(ctx, run)
	if err != nil {
		return coordinatorCancellationState{}, err
	}
	activeControls := make([]NodeControl, 0, len(controls))
	for _, control := range controls {
		switch control.CancelState {
		case CancelStateRequested, CancelStateDelivering:
			state.active = true
			state.waitingDelivery = true
		case CancelStateDraining:
			state.active = true
			activeControls = append(activeControls, control)
		}
	}
	if state.waitingDelivery || len(activeControls) == 0 {
		return state, nil
	}
	attempts, err := r.listNodeAttempts(ctx, run)
	if err != nil {
		return coordinatorCancellationState{}, err
	}
	for _, control := range activeControls {
		closed, closeAttempts, closeEvents, closeErr := r.closeDrainedNodeCancellation(
			ctx,
			run,
			control,
			state.outputs,
			attempts,
		)
		if closeErr != nil {
			return coordinatorCancellationState{}, closeErr
		}
		if !closed {
			state.live = true
			continue
		}
		state.attempts = append(state.attempts, closeAttempts...)
		state.events = append(state.events, closeEvents...)
		attempts = append(attempts, closeAttempts...)
		state.controls = append(state.controls, NodeControlMutation{
			Kind:             NodeControlMutationCancel,
			NodeID:           control.NodeID,
			ExpectedRevision: control.Revision,
			ExpectExisting:   true,
			CancelState:      CancelStateCanceled,
			At:               r.now().UTC(),
		})
	}
	return state, nil
}

func (r *CoordinatorRunner) closeDrainedNodeCancellation(
	ctx context.Context,
	run Run,
	control NodeControl,
	outputs []GenerationOutput,
	recorded []NodeAttempt,
) (bool, []NodeAttempt, []GenerationLifecycleEventIntent, error) {
	indexes := make([]int, 0)
	for index := range outputs {
		if NodeID(outputs[index].NodeID) == control.NodeID && cancellationOutputLive(outputs[index].Status) {
			indexes = append(indexes, index)
		}
	}
	for _, index := range indexes {
		drained, err := r.cancellationOutputDrained(ctx, outputs[index])
		if err != nil {
			return false, nil, nil, err
		}
		if !drained {
			return false, nil, nil, nil
		}
	}
	attempts := make([]NodeAttempt, 0, len(indexes))
	events := make([]GenerationLifecycleEventIntent, 0, len(indexes))
	for _, index := range indexes {
		attempt, err := r.canceledNodeAttempt(ctx, run, control, outputs[index])
		if err != nil {
			return false, nil, nil, err
		}
		expectedEpoch := outputs[index].Epoch
		outputs[index].ExpectedEpoch = &expectedEpoch
		outputs[index].Status = generationOutputCanceled
		outputs[index].NextAttemptAt = nil
		if cancellationAttemptRecorded(recorded, attempt) {
			continue
		}
		failure := classifiedFailureFromAttempt(attempt)
		attempts = append(attempts, attempt)
		events = append(events, GenerationLifecycleEventIntent{
			Kind:        GenerationLifecycleEventNodeCanceled,
			NodeID:      string(attempt.NodeID),
			ItemIndex:   attempt.ItemIndex,
			Attempt:     attempt.Attempt,
			Failure:     &failure,
			Disposition: AttemptCanceled,
		})
	}
	return true, attempts, events, nil
}

func (r *CoordinatorRunner) cancellationOutputDrained(
	ctx context.Context,
	output GenerationOutput,
) (bool, error) {
	switch output.Status {
	case generationOutputPending, generationOutputRetrying, generationOutputWaiting,
		generationOutputPaused, generationOutputAwaitingChild:
		return true, nil
	}
	taskRunID := strings.TrimSpace(output.TaskRunID)
	if taskRunID == "" {
		return false, nil
	}
	run, err := r.taskRuns.GetTaskRun(ctx, taskRunID)
	if err != nil {
		return false, fmt.Errorf("loop: read canceling task run %q: %w", taskRunID, err)
	}
	return task.IsTerminalRunStatus(run.Status), nil
}

func (r *CoordinatorRunner) canceledNodeAttempt(
	ctx context.Context,
	run Run,
	control NodeControl,
	output GenerationOutput,
) (NodeAttempt, error) {
	at := r.now().UTC()
	if control.CancelProvenance != nil && !control.CancelProvenance.RequestedAt.IsZero() {
		at = control.CancelProvenance.RequestedAt.UTC()
	}
	attemptNumber := max(1, output.Attempt)
	if output.Status == generationOutputRetrying {
		attemptNumber++
	}
	attempt := NodeAttempt{
		LoopRunID: run.ID, Generation: run.Generation, NodeID: control.NodeID,
		ItemIndex: output.ItemIndex, Attempt: attemptNumber, StartedAt: at, EndedAt: new(at),
	}
	if taskRunID := strings.TrimSpace(output.TaskRunID); taskRunID != "" && output.Status != generationOutputRetrying {
		taskRun, err := r.taskRuns.GetTaskRun(ctx, taskRunID)
		if err != nil {
			return NodeAttempt{}, fmt.Errorf("loop: read canceled task run %q: %w", taskRunID, err)
		}
		attempt = nodeAttemptForTaskRun(run, run.Generation, output, taskRun, at)
		attempt.Attempt = attemptNumber
	}
	failureClass := FailureCancellation
	attempt.FailureClass = &failureClass
	attempt.FailureCode = string(TransitionCauseOperatorCancel)
	attempt.Cause = string(TransitionCauseOperatorCancel)
	if control.CancelProvenance != nil && strings.TrimSpace(control.CancelProvenance.Reason) != "" {
		attempt.Cause = strings.TrimSpace(control.CancelProvenance.Reason)
	}
	attempt.Disposition = AttemptCanceled
	attempt.NextAttemptAt = nil
	return attempt, nil
}

func cancellationOutputLive(status string) bool {
	switch strings.TrimSpace(status) {
	case generationOutputPending,
		generationOutputEnqueued,
		generationOutputRunning,
		generationOutputRetrying,
		generationOutputWaiting,
		generationOutputPaused,
		generationOutputAwaitingChild,
		generationOutputControlPending,
		generationOutputAwaitingGoal:
		return true
	default:
		return false
	}
}

func cancellationAttemptRecorded(attempts []NodeAttempt, candidate NodeAttempt) bool {
	for _, attempt := range attempts {
		if attempt.Generation == candidate.Generation && attempt.NodeID == candidate.NodeID &&
			attempt.ItemIndex == candidate.ItemIndex && attempt.Attempt == candidate.Attempt {
			return true
		}
	}
	return false
}

func cancellationWaitCoordinatorPlan(run Run, outputs []GenerationOutput) task.CoordinatorCompletionPlan {
	plan := coordinatorFinisherPlan(run, max(1, run.Generation), outputs, nil)
	plan.Yield = true
	plan.GenerationInFlight = true
	plan.CancellationDrain = true
	return plan
}

func runCancellationCoordinatorPlan(
	run Run,
	state coordinatorCancellationState,
) (task.CoordinatorCompletionPlan, error) {
	plan := coordinatorFinisherPlan(run, max(1, run.Generation), state.outputs, nil)
	plan.CancellationDrain = true
	if state.live {
		plan.Yield = true
		plan.GenerationInFlight = true
	} else {
		plan.Terminal = &task.CoordinatorTerminal{
			Status: string(StatusCanceled),
			Cause:  string(TransitionCauseOperatorCancel),
		}
	}
	return appendCoordinatorCancellationState(plan, state)
}

func appendCoordinatorCancellationState(
	plan task.CoordinatorCompletionPlan,
	state coordinatorCancellationState,
) (task.CoordinatorCompletionPlan, error) {
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return task.CoordinatorCompletionPlan{}, err
	}
	payload.Attempts = append(payload.Attempts, state.attempts...)
	payload.Events = append(payload.Events, state.events...)
	payload.Controls = append(payload.Controls, state.controls...)
	plan.Snapshot.Payload = payload
	plan.CancellationDrain = state.active || plan.CancellationDrain
	return plan, nil
}
