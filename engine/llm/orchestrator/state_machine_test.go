package orchestrator

import (
	"bytes"
	"context"
	"strings"
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
		assertTransition(ctx, t, machine, loopCtx, EventStartLoop, StateAwaitLLM)
		assertTransition(ctx, t, machine, loopCtx, EventLLMResponse, StateEvaluateResponse)
		assertTransition(ctx, t, machine, loopCtx, EventResponseNoTool, StateHandleCompletion)
		assertTransition(ctx, t, machine, loopCtx, EventCompletionSuccess, StateFinalize)
	})

	t.Run("ShouldAllowToolExecutionAndFailurePaths", func(t *testing.T) {
		ctx := context.TODO()
		loopCtx := &LoopContext{}
		machine := newLoopFSM(ctx, &stubLoopDeps{}, loopCtx)
		assertTransition(ctx, t, machine, loopCtx, EventStartLoop, StateAwaitLLM)
		assertTransition(ctx, t, machine, loopCtx, EventLLMResponse, StateEvaluateResponse)
		assertTransition(ctx, t, machine, loopCtx, EventResponseWithTools, StateProcessTools)
		assertTransition(ctx, t, machine, loopCtx, EventToolsExecuted, StateUpdateBudgets)
		assertTransition(ctx, t, machine, loopCtx, EventBudgetExceeded, StateTerminateError)
		loopCtx = &LoopContext{}
		machine = newLoopFSM(ctx, &stubLoopDeps{}, loopCtx)
		assertTransition(ctx, t, machine, loopCtx, EventStartLoop, StateAwaitLLM)
		assertTransition(ctx, t, machine, loopCtx, EventLLMResponse, StateEvaluateResponse)
		assertTransition(ctx, t, machine, loopCtx, EventResponseWithTools, StateProcessTools)
		assertTransition(ctx, t, machine, loopCtx, EventToolsExecuted, StateUpdateBudgets)
		assertTransition(ctx, t, machine, loopCtx, EventBudgetOK, StateAwaitLLM)
		assertTransition(ctx, t, machine, loopCtx, EventLLMResponse, StateEvaluateResponse)
		assertTransition(ctx, t, machine, loopCtx, EventResponseNoTool, StateHandleCompletion)
		assertTransition(ctx, t, machine, loopCtx, EventCompletionRetry, StateAwaitLLM)
		assertTransition(ctx, t, machine, loopCtx, EventFailure, StateTerminateError)
	})
}

func TestNewLoopFSM_LogsTransitions(t *testing.T) {
	var logBuf bytes.Buffer
	cfg := logger.TestConfig()
	cfg.Level = logger.DebugLevel
	cfg.Output = &logBuf
	log := logger.NewLogger(cfg)
	ctx := logger.ContextWithLogger(context.TODO(), log)
	loopCtx := &LoopContext{}
	machine := newLoopFSM(ctx, &stubLoopDeps{}, loopCtx)
	require.NoError(t, machine.Event(ctx, EventStartLoop, loopCtx))
	require.NoError(t, machine.Event(ctx, EventLLMResponse, loopCtx))
	logs := stripANSI(logBuf.String())
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

func assertTransition(
	ctx context.Context,
	t *testing.T,
	machine *fsm.FSM,
	loopCtx *LoopContext,
	event string,
	expectedState string,
) {
	t.Helper()
	err := machine.Event(ctx, event, loopCtx)
	require.NoError(t, err)
	require.Equal(t, expectedState, machine.Current())
}

// stripANSI removes ANSI escape sequences from the input string.
// It handles standard SGR (color) codes used by the logger.
func stripANSI(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	escaping := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaping {
			if ch == 'm' {
				escaping = false
			}
			continue
		}
		if ch == 0x1b {
			escaping = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}
