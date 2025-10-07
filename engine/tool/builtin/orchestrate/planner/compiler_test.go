package planner_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/tool/builtin/orchestrate/planner"
	toolcontext "github.com/compozy/compozy/engine/tool/context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closableAdapter struct {
	*llmadapter.TestAdapter
}

func newClosableAdapter() *closableAdapter {
	return &closableAdapter{llmadapter.NewTestAdapter()}
}

func (c *closableAdapter) Close() error {
	return nil
}

func newTestCompiler(t *testing.T, adapter llmadapter.LLMClient, opts ...func(*planner.Options)) *planner.Compiler {
	t.Helper()
	options := planner.Options{
		Client: adapter,
	}
	for _, apply := range opts {
		apply(&options)
	}
	compiler, err := planner.NewCompiler(options)
	require.NoError(t, err)
	return compiler
}

func TestCompilerCompile_ShouldUseStructuredPlan(t *testing.T) {
	t.Run("Should merge bindings and normalize status", func(t *testing.T) {
		adapter := newClosableAdapter()
		compiler := newTestCompiler(t, adapter)
		rawPlan := map[string]any{
			"id": "structured-plan",
			"bindings": map[string]any{
				"default": "value",
			},
			"steps": []any{
				map[string]any{
					"id":     "alpha",
					"type":   "agent",
					"status": "success",
					"agent": map[string]any{
						"agent_id":   "agent.one",
						"result_key": "alpha_out",
					},
				},
			},
		}
		bindings := map[string]any{"default": "override", "extra": 42}
		result, err := compiler.Compile(context.Background(), planner.CompileInput{
			Plan:     rawPlan,
			Bindings: bindings,
		})
		require.NoError(t, err)
		assert.Equal(t, "structured-plan", result.ID)
		require.Len(t, result.Steps, 1)
		assert.Equal(t, "alpha", result.Steps[0].ID)
		assert.Equal(t, "pending", string(result.Steps[0].Status))
		require.NotNil(t, result.Bindings)
		assert.Equal(t, 2, len(result.Bindings))
		assert.Equal(t, 42, result.Bindings["extra"])
		assert.Equal(t, "override", result.Bindings["default"])
	})
}

func TestCompilerCompile_ShouldCallPlannerAgent(t *testing.T) {
	t.Run("Should produce plan from prompt", func(t *testing.T) {
		adapter := newClosableAdapter()
		planJSON := map[string]any{
			"id": "prompt-plan",
			"steps": []any{
				map[string]any{
					"id":     "step_1",
					"type":   "agent",
					"status": "pending",
					"agent": map[string]any{
						"agent_id":   "agent.summary",
						"result_key": "summary",
					},
				},
			},
		}
		bytes, err := json.Marshal(planJSON)
		require.NoError(t, err)
		adapter.SetResponse(string(bytes))
		compiler := newTestCompiler(t, adapter)
		result, err := compiler.Compile(context.Background(), planner.CompileInput{
			Prompt: "Summarize the report using agent.summary",
		})
		require.NoError(t, err)
		assert.Equal(t, "prompt-plan", result.ID)
		require.Len(t, result.Steps, 1)
		assert.Equal(t, "pending", string(result.Steps[0].Status))
		lastCall := adapter.GetLastCall()
		require.NotNil(t, lastCall)
		assert.Equal(t, 0.0, lastCall.Options.Temperature)
		assert.Equal(t, "none", lastCall.Options.ToolChoice)
	})
}

func TestCompilerCompile_ShouldRespectDisableFlags(t *testing.T) {
	t.Run("Should fail when planner disabled in options", func(t *testing.T) {
		adapter := newClosableAdapter()
		compiler := newTestCompiler(t, adapter, func(opts *planner.Options) {
			opts.Disabled = true
		})
		_, err := compiler.Compile(context.Background(), planner.CompileInput{
			Prompt: "Plan this request",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, planner.ErrPlannerDisabled))
	})
	t.Run("Should fail when disable flag provided in input", func(t *testing.T) {
		adapter := newClosableAdapter()
		compiler := newTestCompiler(t, adapter)
		_, err := compiler.Compile(context.Background(), planner.CompileInput{
			Prompt:         "Plan this request",
			DisablePlanner: true,
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, planner.ErrPlannerDisabled))
	})
}

func TestCompilerCompile_ShouldEnforceRecursionGuard(t *testing.T) {
	adapter := newClosableAdapter()
	compiler := newTestCompiler(t, adapter)
	ctx := toolcontext.DisablePlannerTools(context.Background())
	_, err := compiler.Compile(ctx, planner.CompileInput{
		Prompt: "Plan recursion",
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, planner.ErrPlannerRecursion))
}

func TestCompilerCompile_ShouldHandleInvalidPlannerResponse(t *testing.T) {
	t.Run("Should error on invalid JSON", func(t *testing.T) {
		adapter := newClosableAdapter()
		adapter.SetResponse("not-json")
		compiler := newTestCompiler(t, adapter)
		_, err := compiler.Compile(context.Background(), planner.CompileInput{
			Prompt: "Plan invalid response",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, planner.ErrInvalidPlan))
	})
	t.Run("Should enforce max steps", func(t *testing.T) {
		adapter := newClosableAdapter()
		planJSON := map[string]any{
			"id": "oversized",
			"steps": []any{
				map[string]any{
					"id":     "step_1",
					"type":   "agent",
					"status": "pending",
					"agent":  map[string]any{"agent_id": "agent.one"},
				},
				map[string]any{
					"id":     "step_2",
					"type":   "agent",
					"status": "pending",
					"agent":  map[string]any{"agent_id": "agent.two"},
				},
			},
		}
		bytes, err := json.Marshal(planJSON)
		require.NoError(t, err)
		adapter.SetResponse(string(bytes))
		compiler := newTestCompiler(t, adapter, func(opts *planner.Options) {
			opts.MaxSteps = 1
		})
		_, err = compiler.Compile(context.Background(), planner.CompileInput{
			Prompt: "Plan with many steps",
		})
		require.Error(t, err)
		assert.True(t, errors.Is(err, planner.ErrInvalidPlan))
	})
}
