package loop

import (
	"encoding/json"
	"testing"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
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
			EnabledChecks: json.RawMessage(`{"project_check":{"enabled":false}}`),
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
			EnabledChecks: json.RawMessage(`{"project_check":{"command":"go test ./internal/loop"}}`),
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
