package orchestrator

import (
	"bytes"
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/looplab/fsm"
	"github.com/stretchr/testify/require"
)

type stubLoopDeps struct{}

func (s *stubLoopDeps) OnEnterInit(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterAwaitLLM(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterEvaluateResponse(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterProcessTools(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterUpdateBudgets(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterHandleCompletion(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterFinalize(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnEnterTerminateError(_ context.Context, _ *LoopContext) transitionResult {
	return transitionResult{}
}

func (s *stubLoopDeps) OnFailure(_ context.Context, _ *LoopContext, _ string) {}

func TestNewLoopFSM(t *testing.T) {
	t.Run("ShouldConfigureStateMachineWithExpectedTransitions", func(t *testing.T) {
		ctx := context.TODO()
		loopCtx := &LoopContext{}
		machine := newLoopFSM(ctx, &stubLoopDeps{}, loopCtx)
		require.Equal(t, StateInit, machine.Current())
		storedCtx, ok := machine.Metadata(loopContextKey)
		require.True(t, ok)
		require.Same(t, loopCtx, storedCtx.(*LoopContext))
		assertTransition(t, machine, EventStartLoop, StateAwaitLLM)
		assertTransition(t, machine, EventLLMResponse, StateEvaluateResponse)
		assertTransition(t, machine, EventResponseNoTool, StateHandleCompletion)
		assertTransition(t, machine, EventCompletionSuccess, StateFinalize)
	})

	t.Run("ShouldAllowToolExecutionAndFailurePaths", func(t *testing.T) {
		ctx := context.TODO()
		machine := newLoopFSM(ctx, &stubLoopDeps{}, &LoopContext{})
		assertTransition(t, machine, EventStartLoop, StateAwaitLLM)
		assertTransition(t, machine, EventLLMResponse, StateEvaluateResponse)
		assertTransition(t, machine, EventResponseWithTools, StateProcessTools)
		assertTransition(t, machine, EventToolsExecuted, StateUpdateBudgets)
		assertTransition(t, machine, EventBudgetExceeded, StateTerminateError)
		machine = newLoopFSM(ctx, &stubLoopDeps{}, &LoopContext{})
		assertTransition(t, machine, EventStartLoop, StateAwaitLLM)
		assertTransition(t, machine, EventLLMResponse, StateEvaluateResponse)
		assertTransition(t, machine, EventResponseWithTools, StateProcessTools)
		assertTransition(t, machine, EventToolsExecuted, StateUpdateBudgets)
		assertTransition(t, machine, EventBudgetOK, StateAwaitLLM)
		assertTransition(t, machine, EventLLMResponse, StateEvaluateResponse)
		assertTransition(t, machine, EventResponseNoTool, StateHandleCompletion)
		assertTransition(t, machine, EventCompletionRetry, StateAwaitLLM)
		assertTransition(t, machine, EventFailure, StateTerminateError)
	})
}

func TestNewLoopFSM_LogsTransitions(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.NewLogger(&logger.Config{Level: logger.DebugLevel, Output: &logBuf, TimeFormat: "15:04:05"})
	ctx := logger.ContextWithLogger(context.TODO(), log)
	loopCtx := &LoopContext{}
	machine := newLoopFSM(ctx, &stubLoopDeps{}, loopCtx)
	require.NoError(t, machine.Event(ctx, EventStartLoop))
	require.NoError(t, machine.Event(ctx, EventLLMResponse))
	logs := logBuf.String()
	require.Contains(t, logs, "FSM transition start")
	require.Contains(t, logs, "event=start_loop")
	require.Contains(t, logs, "from_state=init")
	require.Contains(t, logs, "to_state=await_llm")
	require.Contains(t, logs, "FSM state entered")
	require.Contains(t, logs, "state=await_llm")
	require.Contains(t, logs, "FSM transition complete")
	// second transition should log evaluate_response entry as well
	require.Contains(t, logs, "event=llm_response")
	require.Contains(t, logs, "state=evaluate_response")
}

func assertTransition(t *testing.T, machine *fsm.FSM, event string, expectedState string) {
	t.Helper()
	err := machine.Event(context.TODO(), event)
	require.NoError(t, err)
	require.Equal(t, expectedState, machine.Current())
}
