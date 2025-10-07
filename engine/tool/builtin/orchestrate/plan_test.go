package orchestrate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanValidate(t *testing.T) {
	t.Run("Should validate complex plan", func(t *testing.T) {
		plan := Plan{
			ID:       "plan-alpha",
			Bindings: map[string]any{"input": "value"},
			Steps: []Step{
				{
					ID:     "step_agent",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent: &AgentStep{
						AgentID:   "agent.primary",
						ActionID:  "action.summary",
						ResultKey: "summary",
						With:      map[string]any{"topic": "{{bindings.input}}"},
					},
					Transitions: StepTransitions{
						AllowedEvents:    []StepEvent{StepEventStepSuccess, StepEventStepFailed},
						DefaultNext:      "step_parallel",
						FailureBranchIDs: []string{"step_parallel"},
					},
				},
				{
					ID:     "step_parallel",
					Type:   StepTypeParallel,
					Status: StepStatusPending,
					Parallel: &ParallelStep{
						Steps: []AgentStep{
							{AgentID: "agent.a", ResultKey: "branch_a"},
							{AgentID: "agent.b", ResultKey: "branch_b"},
						},
						MaxConcurrency: 2,
						MergeStrategy:  MergeStrategyCollect,
						ResultKey:      "parallel_result",
						Bindings:       map[string]any{"mode": "fast"},
					},
					Transitions: StepTransitions{
						AllowedEvents: []StepEvent{StepEventStepSuccess},
					},
				},
			},
		}
		err := plan.Validate()
		require.NoError(t, err)
	})
	t.Run("Should reject duplicate result keys", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{
					ID:     "step_one",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{AgentID: "agent.one", ResultKey: "dup"},
				},
				{
					ID:     "step_two",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{AgentID: "agent.two", ResultKey: "dup"},
				},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "result_key")
	})
	t.Run("Should reject agent step without agent id", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{
					ID:     "invalid_agent",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{},
				},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "agent_id")
	})
	t.Run("Should reject parallel step without children", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{
					ID:     "parallel_empty",
					Type:   StepTypeParallel,
					Status: StepStatusPending,
					Parallel: &ParallelStep{
						Steps: []AgentStep{},
					},
				},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "steps must contain at least one agent step")
	})
	t.Run("Should reject duplicate step IDs", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{ID: "dup", Type: StepTypeAgent, Status: StepStatusPending, Agent: &AgentStep{AgentID: "one"}},
				{ID: "dup", Type: StepTypeAgent, Status: StepStatusPending, Agent: &AgentStep{AgentID: "two"}},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "duplicates")
	})
	t.Run("Should reject step referencing itself as default next", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{
					ID:     "self",
					Type:   StepTypeAgent,
					Status: StepStatusPending,
					Agent:  &AgentStep{AgentID: "self"},
					Transitions: StepTransitions{
						DefaultNext: "self",
					},
				},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "default_next_step must not reference the current step")
	})
	t.Run("Should reject step with conflicting agent and parallel payload", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{
					ID:       "conflict",
					Type:     StepTypeParallel,
					Status:   StepStatusPending,
					Agent:    &AgentStep{AgentID: "agent"},
					Parallel: &ParallelStep{Steps: []AgentStep{{AgentID: "child"}}},
				},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "agent must be omitted")
	})
	t.Run("Should reject invalid status value", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{ID: "status", Type: StepTypeAgent, Status: StepStatus("unknown"), Agent: &AgentStep{AgentID: "a"}},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "status \"unknown\" is invalid")
	})
	t.Run("Should reject missing status", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{ID: "nostatus", Type: StepTypeAgent, Agent: &AgentStep{AgentID: "a"}},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "status is required")
	})
	t.Run("Should reject parallel step with negative max concurrency", func(t *testing.T) {
		plan := Plan{
			Steps: []Step{
				{
					ID:     "parallel",
					Type:   StepTypeParallel,
					Status: StepStatusPending,
					Parallel: &ParallelStep{
						Steps:          []AgentStep{{AgentID: "one"}},
						MaxConcurrency: -1,
					},
				},
			},
		}
		err := plan.Validate()
		require.Error(t, err)
		assert.ErrorContains(t, err, "max_concurrency must be non-negative")
	})
}

func TestDecodePlanMap(t *testing.T) {
	t.Run("Should decode and validate plan map", func(t *testing.T) {
		raw := map[string]any{
			"id": "plan-from-map",
			"steps": []any{
				map[string]any{
					"id":     "alpha",
					"type":   "agent",
					"status": "pending",
					"agent": map[string]any{
						"agent_id":   "agent.alpha",
						"result_key": "alpha_out",
					},
				},
			},
		}
		plan, err := DecodePlanMap(raw)
		require.NoError(t, err)
		assert.Equal(t, "plan-from-map", plan.ID)
		require.Len(t, plan.Steps, 1)
		assert.Equal(t, "alpha_out", plan.Steps[0].Agent.ResultKey)
	})
	t.Run("Should fail decoding invalid type", func(t *testing.T) {
		raw := map[string]any{
			"steps": []any{
				map[string]any{
					"id":     "alpha",
					"type":   123,
					"status": "pending",
					"agent":  map[string]any{"agent_id": "agent.alpha"},
				},
			},
		}
		_, err := DecodePlanMap(raw)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to decode plan payload")
	})
	t.Run("Should fail validation on decoded plan", func(t *testing.T) {
		raw := map[string]any{
			"steps": []any{
				map[string]any{
					"id":     "beta",
					"type":   "agent",
					"status": "pending",
					"agent":  map[string]any{},
				},
			},
		}
		_, err := DecodePlanMap(raw)
		require.Error(t, err)
		assert.ErrorContains(t, err, "agent_id")
	})
	t.Run("Should fail when payload is nil", func(t *testing.T) {
		_, err := DecodePlanMap(nil)
		require.Error(t, err)
		assert.ErrorContains(t, err, "plan payload is nil")
	})
}

func TestPlanSchemaValidation(t *testing.T) {
	t.Run("Should validate plan using JSON schema", func(t *testing.T) {
		rawPlan := map[string]any{
			"id": "schema-plan",
			"steps": []any{
				map[string]any{
					"id":     "intro",
					"type":   "agent",
					"status": "pending",
					"agent": map[string]any{
						"agent_id":   "agent.schema",
						"result_key": "intro",
					},
				},
				map[string]any{
					"id":     "fanout",
					"type":   "parallel",
					"status": "pending",
					"parallel": map[string]any{
						"steps": []any{
							map[string]any{"agent_id": "agent.sub", "result_key": "sub"},
						},
						"merge_strategy": "collect",
					},
				},
			},
		}
		sc, err := PlanSchema()
		require.NoError(t, err)
		res, err := sc.Validate(context.Background(), rawPlan)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})
	t.Run("Should reject invalid plan via schema", func(t *testing.T) {
		rawPlan := map[string]any{
			"steps": []any{
				map[string]any{
					"id":     "bad",
					"type":   "agent",
					"status": "pending",
				},
			},
		}
		sc, err := PlanSchema()
		require.NoError(t, err)
		_, err = sc.Validate(context.Background(), rawPlan)
		require.Error(t, err)
	})
	t.Run("Should reject schema when agent step includes parallel payload", func(t *testing.T) {
		rawPlan := map[string]any{
			"steps": []any{
				map[string]any{
					"id":     "conflict",
					"type":   "agent",
					"status": "pending",
					"agent": map[string]any{
						"agent_id": "agent.a",
					},
					"parallel": map[string]any{
						"steps": []any{map[string]any{"agent_id": "agent.child"}},
					},
				},
			},
		}
		sc, err := PlanSchema()
		require.NoError(t, err)
		_, err = sc.Validate(context.Background(), rawPlan)
		require.Error(t, err)
	})
	t.Run("Should allow forward-compatible top-level fields", func(t *testing.T) {
		rawPlan := map[string]any{
			"id":      "forward",
			"version": 1,
			"steps": []any{
				map[string]any{
					"id":     "intro",
					"type":   "agent",
					"status": "pending",
					"agent": map[string]any{
						"agent_id": "agent.forward",
					},
				},
			},
		}
		sc, err := PlanSchema()
		require.NoError(t, err)
		res, err := sc.Validate(context.Background(), rawPlan)
		require.NoError(t, err)
		assert.NotNil(t, res)
	})
}

func BenchmarkDecodePlanMap(b *testing.B) {
	raw := map[string]any{
		"id": "bench-plan",
		"steps": []any{
			map[string]any{
				"id":     "bench",
				"type":   "agent",
				"status": "pending",
				"agent": map[string]any{
					"agent_id":   "agent.bench",
					"result_key": "bench_out",
					"with": map[string]any{
						"payload": "value",
					},
				},
			},
		},
	}
	payload, err := json.Marshal(raw)
	require.NoError(b, err)
	var payloadMap map[string]any
	require.NoError(b, json.Unmarshal(payload, &payloadMap))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := DecodePlanMap(payloadMap)
		if err != nil {
			b.Fatal(err)
		}
	}
}
