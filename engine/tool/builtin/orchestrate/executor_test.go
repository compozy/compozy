package orchestrate

import (
	"context"
	"sync"
	"testing"
	"time"

	agentexec "github.com/compozy/compozy/engine/agent/exec"
	"github.com/compozy/compozy/engine/core"
	toolcontext "github.com/compozy/compozy/engine/tool/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRunner struct {
	execFunc func(context.Context, agentexec.ExecuteRequest) (*agentexec.ExecuteResult, error)
}

func (s *stubRunner) Execute(ctx context.Context, req agentexec.ExecuteRequest) (*agentexec.ExecuteResult, error) {
	if s.execFunc != nil {
		return s.execFunc(ctx, req)
	}
	return &agentexec.ExecuteResult{ExecID: core.MustNewID()}, nil
}

func TestEngine_Run(t *testing.T) {
	t.Run("Should execute sequential agent steps", func(t *testing.T) {
		runner := &stubRunner{}
		executionOrder := make([]string, 0, 2)
		runner.execFunc = func(_ context.Context, req agentexec.ExecuteRequest) (*agentexec.ExecuteResult, error) {
			executionOrder = append(executionOrder, req.AgentID)
			output := &core.Output{"agent": req.AgentID}
			return &agentexec.ExecuteResult{ExecID: core.MustNewID(), Output: output}, nil
		}
		plan := &Plan{
			Steps: []Step{
				{
					ID:     "step_1",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{AgentID: "agent.one", ResultKey: "one"},
				},
				{
					ID:     "step_2",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{AgentID: "agent.two", ResultKey: "two"},
				},
			},
		}
		engine := NewEngine(runner, Limits{MaxSteps: 5, MaxParallel: 3})
		results, err := engine.Run(context.Background(), plan)
		require.NoError(t, err)
		require.Len(t, results, 2)
		assert.Equal(t, []string{"agent.one", "agent.two"}, executionOrder)
		assert.Equal(t, "step_1", results[0].StepID)
		assert.Equal(t, StepStatusSuccess, results[0].Status)
		assert.NotZero(t, results[0].ExecID)
		require.NotNil(t, results[0].Output)
		assert.Equal(t, "agent.one", results[0].Output.Prop("agent"))
	})

	t.Run("Should respect parallel concurrency limits", func(t *testing.T) {
		runner := &stubRunner{}
		var mu sync.Mutex
		current := 0
		peak := 0
		runner.execFunc = func(_ context.Context, req agentexec.ExecuteRequest) (*agentexec.ExecuteResult, error) {
			_ = req
			mu.Lock()
			current++
			if current > peak {
				peak = current
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			current--
			mu.Unlock()
			return &agentexec.ExecuteResult{ExecID: core.MustNewID(), Output: &core.Output{"ok": true}}, nil
		}
		plan := &Plan{
			Steps: []Step{
				{
					ID:     "parallel",
					Type:   StepTypeParallel,
					Status: StepStatusPending,
					Parallel: &ParallelStep{Steps: []AgentStep{
						{AgentID: "agent.a"},
						{AgentID: "agent.b"},
						{AgentID: "agent.c"},
					}},
				},
			},
		}
		engine := NewEngine(runner, Limits{MaxParallel: 2})
		results, err := engine.Run(context.Background(), plan)
		require.NoError(t, err)
		t.Logf("results=%+v", results)
		require.Len(t, results, 1)
		assert.Equal(t, StepTypeParallel, results[0].Type)
		assert.Len(t, results[0].Children, 3)
		assert.LessOrEqual(t, peak, 2)
	})

	t.Run("Should return error when context canceled", func(t *testing.T) {
		runner := &stubRunner{}
		runner.execFunc = func(ctx context.Context, req agentexec.ExecuteRequest) (*agentexec.ExecuteResult, error) {
			_ = req
			<-ctx.Done()
			return nil, ctx.Err()
		}
		plan := &Plan{
			Steps: []Step{
				{
					ID:     "cancel",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{AgentID: "agent.cancel"},
				},
			},
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		engine := NewEngine(runner, Limits{})
		results, err := engine.Run(ctx, plan)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		t.Logf("results=%+v", results)
		require.Len(t, results, 1)
		assert.Equal(t, StepStatusFailed, results[0].Status)
		assert.Equal(t, "cancel", results[0].StepID)
		assert.ErrorIs(t, results[0].Error, context.Canceled)
	})

	t.Run("Should block when max depth exceeded", func(t *testing.T) {
		runner := &stubRunner{}
		plan := &Plan{
			Steps: []Step{
				{ID: "step", Type: StepTypeAgent, Status: StepStatusPending, Agent: &AgentStep{AgentID: "agent.depth"}},
			},
		}
		ctx := toolcontext.IncrementAgentOrchestratorDepth(context.Background())
		engine := NewEngine(runner, Limits{MaxDepth: 1})
		results, err := engine.Run(ctx, plan)
		require.Error(t, err)
		assert.ErrorIs(t, err, errMaxDepthExceeded)
		require.Len(t, results, 0)
	})
}
