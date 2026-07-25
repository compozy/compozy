package runshared

import (
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

// The model fallback in the ACP client keys off ModelExplicit, so a dropped
// mapping here would silently downgrade an explicitly pinned model instead of
// failing. Nothing downstream can catch that, so assert the wiring directly.
func TestNewConfigCarriesExplicitModelFlag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		explicit model.ExplicitRuntimeFlags
		want     bool
	}{
		{
			name:     "Should mark the model explicit when the user pinned it",
			explicit: model.ExplicitRuntimeFlags{Model: true},
			want:     true,
		},
		{
			name:     "Should leave an inherited model correctable",
			explicit: model.ExplicitRuntimeFlags{},
			want:     false,
		},
		{
			name:     "Should not confuse other explicit runtime flags with the model",
			explicit: model.ExplicitRuntimeFlags{IDE: true, ReasoningEffort: true, AccessMode: true},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := NewConfig(&model.RuntimeConfig{
				IDE:             model.IDECodex,
				Model:           "opus",
				ExplicitRuntime: tt.explicit,
			}, model.RunArtifacts{})
			if cfg == nil {
				t.Fatal("NewConfig() = nil")
			}
			if cfg.ModelExplicit != tt.want {
				t.Fatalf("NewConfig().ModelExplicit = %v, want %v", cfg.ModelExplicit, tt.want)
			}
		})
	}
}

func TestJobCodeFileLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		codeFiles []string
		want      string
	}{
		{
			name:      "empty",
			codeFiles: nil,
			want:      "",
		},
		{
			name:      "single file",
			codeFiles: []string{"task_01"},
			want:      "task_01",
		},
		{
			name:      "multiple files",
			codeFiles: []string{"task_01", "task_02", "task_03"},
			want:      "task_01, task_02, task_03",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			j := Job{CodeFiles: append([]string(nil), tt.codeFiles...)}
			if got := j.CodeFileLabel(); got != tt.want {
				t.Fatalf("codeFileLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
