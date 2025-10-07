package orchestrate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubExecutorDeps struct {
	planningResult  TransitionResult
	dispatchResult  TransitionResult
	awaitResult     TransitionResult
	mergingResult   TransitionResult
	completedResult TransitionResult
	failedResult    TransitionResult
	events          []string
}

func (s *stubExecutorDeps) record(event string) {
	s.events = append(s.events, event)
}

func (s *stubExecutorDeps) OnEnterPending(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("pending")
	_ = ctx
	_ = execCtx
	return TransitionResult{}
}

func (s *stubExecutorDeps) OnEnterPlanning(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("planning")
	_ = ctx
	_ = execCtx
	return s.planningResult
}

func (s *stubExecutorDeps) OnEnterDispatching(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("dispatching")
	_ = ctx
	_ = execCtx
	return s.dispatchResult
}

func (s *stubExecutorDeps) OnEnterAwaitingResults(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("awaiting")
	_ = ctx
	_ = execCtx
	return s.awaitResult
}

func (s *stubExecutorDeps) OnEnterMerging(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("merging")
	_ = ctx
	_ = execCtx
	return s.mergingResult
}

func (s *stubExecutorDeps) OnEnterCompleted(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("completed")
	_ = ctx
	_ = execCtx
	return s.completedResult
}

func (s *stubExecutorDeps) OnEnterFailed(ctx context.Context, execCtx *executorContext) TransitionResult {
	s.record("failed")
	_ = ctx
	_ = execCtx
	return s.failedResult
}

func (s *stubExecutorDeps) OnFailure(context.Context, *executorContext, string) {}

func TestExecutorFSM(t *testing.T) {
	t.Run("Should transition through success path", func(t *testing.T) {
		stub := &stubExecutorDeps{
			planningResult: TransitionResult{Event: EventPlannerFinished},
			dispatchResult: TransitionResult{Event: EventDispatchStep},
			awaitResult:    TransitionResult{Event: EventStepSucceeded},
			mergingResult:  TransitionResult{Event: EventParallelComplete},
		}
		machine := newExecutorFSM(context.Background(), stub, &executorContext{})
		require.Equal(t, StatePending, machine.Current())
		require.NoError(t, machine.Event(context.Background(), EventStartPlan, &executorContext{}))
		assert.Equal(t, StateCompleted, machine.Current())
		assert.Contains(t, stub.events, "planning")
		assert.Contains(t, stub.events, "completed")
	})

	t.Run("Should propagate validation failure", func(t *testing.T) {
		errValidate := errors.New("invalid plan")
		stub := &stubExecutorDeps{
			planningResult: TransitionResult{Event: EventValidationFailed, Err: errValidate},
		}
		machine := newExecutorFSM(context.Background(), stub, &executorContext{})
		require.NoError(t, machine.Event(context.Background(), EventStartPlan, &executorContext{}))
		assert.Equal(t, StateFailed, machine.Current())
		assert.Contains(t, stub.events, "failed")
	})
}
