package loop

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/gate"
)

func TestApplyEffectiveGateConfigShouldFilterRuntimeCriteria(t *testing.T) {
	t.Run("Should treat disabled command and disabled human criteria as empty", func(t *testing.T) {
		t.Parallel()

		runtimeGate := gate.Gate{
			ID: "verify",
			Criteria: []dsl.GateCriterion{
				{
					ID:     "project_check",
					Type:   dsl.CriterionCommand,
					Check:  "make verify",
					Expect: "exit_zero",
				},
				{
					ID:     "approval",
					Type:   dsl.CriterionHuman,
					Prompt: "Approve?",
				},
			},
		}
		effective := EffectiveConfig{
			EnabledChecks: new(EffectiveChecksJSON(`{"project_check":{"enabled":false}}`)),
		}

		filtered, empty, err := applyEffectiveGateConfig(runtimeGate, effective)
		if err != nil {
			t.Fatalf("applyEffectiveGateConfig() error = %v", err)
		}
		if !empty {
			t.Fatalf("empty = false, want true with disabled command and human gate off")
		}
		if len(filtered.Criteria) != 0 {
			t.Fatalf("criteria = %#v, want none", filtered.Criteria)
		}
	})

	t.Run("Should override a command criterion from enabled_checks_json", func(t *testing.T) {
		t.Parallel()

		runtimeGate := gate.Gate{
			ID: "verify",
			Criteria: []dsl.GateCriterion{{
				ID:     "project_check",
				Type:   dsl.CriterionCommand,
				Check:  "make verify",
				Expect: "exit_zero",
			}},
		}
		effective := EffectiveConfig{
			EnabledChecks: new(EffectiveChecksJSON(`{"project_check":{"command":"go test ./internal/loop"}}`)),
		}

		filtered, empty, err := applyEffectiveGateConfig(runtimeGate, effective)
		if err != nil {
			t.Fatalf("applyEffectiveGateConfig() error = %v", err)
		}
		if empty {
			t.Fatal("empty = true, want overridden command criterion")
		}
		if got, want := filtered.Criteria[0].Check, "go test ./internal/loop"; got != want {
			t.Fatalf("criterion check = %q, want %q", got, want)
		}
	})
}

func TestJudgeRuntimeShouldValidateWithoutTaskRules(t *testing.T) {
	t.Parallel()

	t.Run("Should validate the criterion merged over judge defaults", func(t *testing.T) {
		t.Parallel()

		factory := judgeRuntimeCatalogFactoryForTest{catalog: judgeRuntimeCatalogForTest{}}
		err := validateJudgeGateRuntimes(
			context.Background(), factory, "ws-judge",
			RuntimeSpec{Provider: "codex", Model: "default-model", Reasoning: "high"},
			[]dsl.GateCriterion{{
				ID: "judge", Type: dsl.CriterionAgentJudge,
				Runtime: dsl.RuntimeSpec{Provider: "flarp", Model: "criterion-model"},
			}},
		)
		validation, ok := AsRuntimeValidationError(err)
		if !ok || len(validation.Items) != 1 || validation.Items[0].Field != "provider" ||
			validation.Items[0].Value != "flarp" {
			t.Fatalf("validateJudgeGateRuntimes() error = %v, want unknown provider", err)
		}
	})

	t.Run("Should expose only judge defaults to the gate evaluator", func(t *testing.T) {
		t.Parallel()

		effective := EffectiveConfig{
			RuntimeDefaults: RuntimeDefaults{Judge: RuntimeSpec{
				Provider: "codex", Model: "judge-model", Reasoning: "high",
			}},
			RuntimeRules: []RuntimeRule{{
				Match: RuntimeMatch{Type: "frontend"}, Runtime: RuntimeSpec{Model: "config-task-model"},
			}},
			RunRuntimeRules: []RuntimeRule{{
				Match: RuntimeMatch{ID: "task_01"}, Runtime: RuntimeSpec{Model: "run-task-model"},
			}},
		}
		input, err := runtimeGateInput(
			Run{ID: "run-judge", WorkspaceID: "ws-judge"}, 1,
			&ResolvedDefinition{Definition: dsl.Definition{}}, effective,
			gate.PlacementInBody, nil,
		)
		if err != nil {
			t.Fatalf("runtimeGateInput() error = %v", err)
		}
		if input.JudgeRuntime.Provider != "codex" || input.JudgeRuntime.Model != "judge-model" ||
			input.JudgeRuntime.Reasoning != "high" {
			t.Fatalf("JudgeRuntime = %#v, want only runtime_defaults.judge", input.JudgeRuntime)
		}
	})
}

func TestRenderGateCriterionShouldMaterializeNestedInputsOnce(t *testing.T) {
	t.Run("Should preserve direct JSON types and literal braces from input values", func(t *testing.T) {
		t.Parallel()

		criterion, err := renderGateCriterion("quality", dsl.GateCriterion{
			ID: "extension", Type: dsl.CriterionExtension, Tool: "ext__quality__gate",
			Inputs: map[string]any{
				"payload": "{{ .inputs.payload }}",
				"label":   "value={{ .inputs.literal }}",
			},
		}, map[string]any{"inputs": map[string]any{
			"payload": map[string]any{"completed": true},
			"literal": "{{ .inputs.slug }}",
		}})
		if err != nil {
			t.Fatalf("renderGateCriterion() error = %v", err)
		}
		payload, ok := criterion.Inputs["payload"].(map[string]any)
		if !ok || payload["completed"] != true {
			t.Fatalf("rendered payload = %#v, want typed JSON object", criterion.Inputs["payload"])
		}
		if got, want := criterion.Inputs["label"], "value={{ .inputs.slug }}"; got != want {
			t.Fatalf("rendered label = %#v, want %q", got, want)
		}
	})
}

type judgeRuntimeCatalogFactoryForTest struct {
	catalog RuntimeCatalog
}

func (f judgeRuntimeCatalogFactoryForTest) ForWorkspace(context.Context, WorkspaceID) (RuntimeCatalog, error) {
	if f.catalog == nil {
		return nil, errors.New("catalog unavailable")
	}
	return f.catalog, nil
}

type judgeRuntimeCatalogForTest struct{}

func (judgeRuntimeCatalogForTest) CanonicalProvider(provider string) string {
	return provider
}

func (judgeRuntimeCatalogForTest) ValidateRuntime(_ context.Context, runtime RuntimeSpec) error {
	if runtime.Provider == "flarp" {
		return NewRuntimeValidationError(RuntimeValidationItem{
			Field: "provider", Value: runtime.Provider, Reason: "unknown_provider",
		})
	}
	return nil
}
