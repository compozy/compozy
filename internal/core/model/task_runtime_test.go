package model_test

import (
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestApplyTaskRuntimePreservesFieldSelectionProvenance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		runtime  model.TaskRuntime
		selected model.ExplicitRuntimeFlags
		want     model.ExplicitRuntimeFlags
	}{
		{
			name: "Should preserve a same-value model selection",
			runtime: model.TaskRuntime{
				IDE:             model.IDECodex,
				Model:           "gpt-5.5",
				ReasoningEffort: "medium",
			},
			selected: model.ExplicitRuntimeFlags{Model: true},
			want:     model.ExplicitRuntimeFlags{Model: true},
		},
		{
			name: "Should not pin an untouched model when selecting another field",
			runtime: model.TaskRuntime{
				IDE:             model.IDECursor,
				Model:           "gpt-5.5",
				ReasoningEffort: "medium",
			},
			selected: model.ExplicitRuntimeFlags{IDE: true},
			want:     model.ExplicitRuntimeFlags{IDE: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &model.RuntimeConfig{
				IDE:             model.IDECodex,
				Model:           "gpt-5.5",
				ReasoningEffort: "medium",
			}

			model.ApplyTaskRuntime(cfg, tt.runtime, tt.selected)

			if cfg.ExplicitRuntime != tt.want {
				t.Fatalf("ApplyTaskRuntime() explicit flags = %#v, want %#v", cfg.ExplicitRuntime, tt.want)
			}
		})
	}
}
