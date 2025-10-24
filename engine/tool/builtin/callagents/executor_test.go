package callagents

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteAgentsParallel(t *testing.T) {
	t.Parallel()

	t.Run("Should respect max concurrency and return results", func(t *testing.T) {
		t.Parallel()
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		maxConcurrent := int64(2)
		exec := &recordingExecutor{
			delay:          10 * time.Millisecond,
			maxConcurrent:  maxConcurrent,
			resultsFactory: successResultFactory(),
		}
		plans := []agentPlan{
			{index: 0, request: toolenv.AgentRequest{AgentID: "A"}, userConfig: AgentExecutionRequest{AgentID: "A"}},
			{index: 1, request: toolenv.AgentRequest{AgentID: "B"}, userConfig: AgentExecutionRequest{AgentID: "B"}},
			{index: 2, request: toolenv.AgentRequest{AgentID: "C"}, userConfig: AgentExecutionRequest{AgentID: "C"}},
		}
		env := &stubEnvironment{executor: exec}
		results := executeAgentsParallel(ctx, env, plans, int(maxConcurrent))
		require.Len(t, results, len(plans))
		assert.LessOrEqual(t, exec.maxObserved.Load(), maxConcurrent)
		for _, res := range results {
			assert.True(t, res.Success)
			assert.NotEmpty(t, res.ExecID)
		}
	})

	t.Run("Should preserve input order and surface errors", func(t *testing.T) {
		t.Parallel()
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		errStub := builtin.Internal(errors.New("boom"), nil)
		exec := &recordingExecutor{
			resultsFactory: func(req toolenv.AgentRequest, _ int) (*toolenv.AgentResult, error) {
				if req.AgentID == "beta" {
					return nil, errStub
				}
				output := core.Output{"agent_id": req.AgentID}
				return &toolenv.AgentResult{
					ExecID: core.MustNewID(),
					Output: &output,
				}, nil
			},
		}
		plans := []agentPlan{
			{
				index:      0,
				request:    toolenv.AgentRequest{AgentID: "alpha"},
				userConfig: AgentExecutionRequest{AgentID: "alpha"},
			},
			{
				index:      1,
				request:    toolenv.AgentRequest{AgentID: "beta"},
				userConfig: AgentExecutionRequest{AgentID: "beta"},
			},
			{
				index:      2,
				request:    toolenv.AgentRequest{AgentID: "gamma"},
				userConfig: AgentExecutionRequest{AgentID: "gamma"},
			},
		}
		env := &stubEnvironment{executor: exec}
		results := executeAgentsParallel(ctx, env, plans, 3)
		require.Len(t, results, len(plans))
		assert.True(t, results[0].Success)
		assert.False(t, results[1].Success)
		assert.Equal(t, errStub.Code, results[1].Error.Code)
		assert.True(t, results[2].Success)
		assert.Equal(t, "alpha", results[0].AgentID)
		assert.Equal(t, "beta", results[1].AgentID)
		assert.Equal(t, "gamma", results[2].AgentID)
	})
}

type recordingExecutor struct {
	delay          time.Duration
	maxConcurrent  int64
	current        int64
	maxObserved    atomic.Int64
	resultsFactory func(toolenv.AgentRequest, int) (*toolenv.AgentResult, error)
	mu             sync.Mutex
	callIndex      int
}

func (r *recordingExecutor) AgentExecutor() toolenv.AgentExecutor { return r }

func (r *recordingExecutor) ExecuteAgent(_ context.Context, req toolenv.AgentRequest) (*toolenv.AgentResult, error) {
	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	current := atomic.AddInt64(&r.current, 1)
	for {
		observed := r.maxObserved.Load()
		if current <= observed {
			break
		}
		if r.maxObserved.CompareAndSwap(observed, current) {
			break
		}
	}
	defer atomic.AddInt64(&r.current, -1)
	r.mu.Lock()
	index := r.callIndex
	r.callIndex++
	r.mu.Unlock()
	return r.resultsFactory(req, index)
}

func successResultFactory() func(toolenv.AgentRequest, int) (*toolenv.AgentResult, error) {
	return func(req toolenv.AgentRequest, _ int) (*toolenv.AgentResult, error) {
		output := core.Output{"agent_id": req.AgentID}
		return &toolenv.AgentResult{
			ExecID: core.MustNewID(),
			Output: &output,
		}, nil
	}
}
