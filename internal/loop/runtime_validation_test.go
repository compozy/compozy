package loop_test

import (
	"context"
	"strings"
	"testing"

	loop "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
)

func TestValidateResolvedRuntimeShouldHonorCatalogValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unknown provider with task identity", func(t *testing.T) {
		t.Parallel()

		_, err := loop.ValidateResolvedRuntime(
			context.Background(), runtimeCatalogForTest{}, "task_07",
			loop.ResolvedRuntime{Runtime: loop.RuntimeSpec{Provider: "flarp"}},
		)
		assertRuntimeValidationItem(t, err, loop.RuntimeValidationItem{
			TaskID: "task_07", Field: "provider", Value: "flarp", Reason: "unknown_provider",
		})
	})

	t.Run("Should propagate a catalog model error", func(t *testing.T) {
		t.Parallel()

		_, err := loop.ValidateResolvedRuntime(
			context.Background(), runtimeCatalogForTest{}, "task_08",
			loop.ResolvedRuntime{Runtime: loop.RuntimeSpec{Provider: "cursor", Model: "unknown"}},
		)
		assertRuntimeValidationItem(t, err, loop.RuntimeValidationItem{
			TaskID: "task_08", Field: "model", Value: "unknown", Reason: "unknown_model",
		})
	})

	t.Run("Should preserve a slash model accepted by the catalog", func(t *testing.T) {
		t.Parallel()

		got, err := loop.ValidateResolvedRuntime(
			context.Background(), runtimeCatalogForTest{}, "task_09",
			loop.ResolvedRuntime{Runtime: loop.RuntimeSpec{
				Provider: "openrouter", Model: "anthropic/claude-opus-4-7",
			}},
		)
		if err != nil {
			t.Fatalf("ValidateResolvedRuntime() error = %v", err)
		}
		if got.Runtime.Provider != "openrouter" || got.Runtime.Model != "anthropic/claude-opus-4-7" {
			t.Fatalf("ValidateResolvedRuntime() = %#v, want canonical slash model", got)
		}
	})

	t.Run("Should canonicalize aliases before catalog lookup", func(t *testing.T) {
		t.Parallel()

		got, err := loop.ValidateResolvedRuntime(
			context.Background(), runtimeCatalogForTest{}, "task_10",
			loop.ResolvedRuntime{Runtime: loop.RuntimeSpec{Provider: "z-ai", Model: "glm-5"}},
		)
		if err != nil {
			t.Fatalf("ValidateResolvedRuntime() error = %v", err)
		}
		if got.Runtime.Provider != "zai" {
			t.Fatalf("provider = %q, want canonical zai", got.Runtime.Provider)
		}
	})

	t.Run("Should allow an empty triple before agent resolution", func(t *testing.T) {
		t.Parallel()

		got, err := loop.ValidateResolvedRuntime(
			context.Background(), nil, "task_11", loop.ResolvedRuntime{},
		)
		if err != nil {
			t.Fatalf("ValidateResolvedRuntime(empty) error = %v", err)
		}
		assertResolvedRuntime(t, got, loop.RuntimeSpec{}, loop.RuntimeProvenance{})
	})
}

func TestValidateDefinitionRuntimeShouldEnforceStaticRuntimeContract(t *testing.T) {
	t.Parallel()

	t.Run("Should accept the complete reasoning vocabulary", func(t *testing.T) {
		t.Parallel()

		for _, reasoning := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
			t.Run("Should accept "+reasoning, func(t *testing.T) {
				t.Parallel()
				definition := dsl.Definition{Contract: dsl.Contract{RuntimeRules: []dsl.RuntimeRule{{
					Match:   dsl.RuntimeMatch{Type: "frontend"},
					Runtime: dsl.RuntimeSpec{Reasoning: reasoning},
				}}}}
				if err := loop.ValidateDefinitionRuntime(context.Background(), nil, definition); err != nil {
					t.Fatalf("ValidateDefinitionRuntime(%q) error = %v", reasoning, err)
				}
			})
		}
	})

	t.Run("Should reject unsupported reasoning", func(t *testing.T) {
		t.Parallel()

		definition := dsl.Definition{Contract: dsl.Contract{RuntimeDefaults: &dsl.RuntimeDefaults{
			Worker: dsl.RuntimeSpec{Reasoning: "ultra"},
		}}}
		err := loop.ValidateDefinitionRuntime(context.Background(), nil, definition)
		assertRuntimeValidationItem(t, err, loop.RuntimeValidationItem{
			Field: "reasoning", Value: "ultra", Reason: "unsupported_reasoning",
		})
	})

	t.Run("Should reject an empty matcher instead of matching every item", func(t *testing.T) {
		t.Parallel()

		definition := dsl.Definition{Contract: dsl.Contract{RuntimeRules: []dsl.RuntimeRule{{
			Runtime: dsl.RuntimeSpec{Model: "opus"},
		}}}}
		err := loop.ValidateDefinitionRuntime(context.Background(), nil, definition)
		assertRuntimeValidationItem(t, err, loop.RuntimeValidationItem{
			Field: "runtime_rules[0].match", Reason: "selector_required",
		})
	})

	t.Run("Should reject every retired runtime key with migration guidance", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			definition dsl.Definition
			field      string
		}{
			{
				name: "Should reject model defaults",
				definition: dsl.Definition{Contract: dsl.Contract{
					Extra: map[string]any{"model_defaults": map[string]any{"worker": "opus"}},
				}},
				field: "model_defaults",
			},
			{
				name: "Should reject scalar node model",
				definition: dsl.Definition{Graph: dsl.Graph{Nodes: []dsl.Node{{
					ID: "work", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{"agent": "worker", "prompt": "Work", "model": "opus"},
				}}}},
				field: "graph.nodes.work.params.model",
			},
			{
				name: "Should reject scalar criterion model",
				definition: dsl.Definition{Contract: dsl.Contract{Verification: []dsl.GateCriterion{{
					ID: "judge", Type: dsl.CriterionAgentJudge, Extra: map[string]any{"model": "opus"},
				}}}},
				field: "contract.verification[0].model",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				err := loop.ValidateDefinitionRuntime(context.Background(), nil, tc.definition)
				assertRuntimeValidationItem(t, err, loop.RuntimeValidationItem{
					Field: tc.field, Reason: "retired_key",
				})
				if err == nil || !strings.Contains(err.Error(), "MIGRATION_GUIDE.md#per-task-runtime-selection") {
					t.Fatalf("ValidateDefinitionRuntime() error = %v, want migration guide", err)
				}
			})
		}
	})
}

type runtimeCatalogForTest struct{}

func (runtimeCatalogForTest) CanonicalProvider(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "z-ai" {
		return "zai"
	}
	return provider
}

func (runtimeCatalogForTest) ValidateRuntime(_ context.Context, runtime loop.RuntimeSpec) error {
	switch runtime.Provider {
	case "cursor":
		if runtime.Model != "grok-4.5[effort=high,fast=true]" {
			return loop.NewRuntimeValidationError(loop.RuntimeValidationItem{
				Field: "model", Value: runtime.Model, Reason: "unknown_model",
			})
		}
	case "openrouter", "zai":
		return nil
	default:
		return loop.NewRuntimeValidationError(loop.RuntimeValidationItem{
			Field: "provider", Value: runtime.Provider, Reason: "unknown_provider",
		})
	}
	return nil
}

func assertRuntimeValidationItem(t *testing.T, err error, want loop.RuntimeValidationItem) {
	t.Helper()
	validation, ok := loop.AsRuntimeValidationError(err)
	if !ok || len(validation.Items) != 1 {
		t.Fatalf("runtime validation error = %v, want one structured item", err)
	}
	got := validation.Items[0]
	if got.TaskID != want.TaskID || got.Field != want.Field || got.Reason != want.Reason {
		t.Fatalf("runtime validation item = %#v, want %#v", got, want)
	}
	if want.Value != "" && got.Value != want.Value {
		t.Fatalf("runtime validation value = %q, want %q", got.Value, want.Value)
	}
}
