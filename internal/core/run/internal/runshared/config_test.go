package runshared

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

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

func TestNewConfigPropagatesCloseOnComplete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		closeOnComplete bool
	}{
		{name: "close_on_complete true", closeOnComplete: true},
		{name: "close_on_complete false", closeOnComplete: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			src := &model.RuntimeConfig{
				WorkspaceRoot:   "/workspace",
				CloseOnComplete: tt.closeOnComplete,
				Mode:            model.ExecutionModePRDTasks,
				IDE:             model.IDECodex,
				Timeout:         10 * time.Minute,
			}
			cfg := NewConfig(src, model.RunArtifacts{})
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if cfg.CloseOnComplete != tt.closeOnComplete {
				t.Fatalf("CloseOnComplete = %v, want %v", cfg.CloseOnComplete, tt.closeOnComplete)
			}
		})
	}
}

func TestNewConfigDefaultsCloseOnCompleteToFalse(t *testing.T) {
	t.Parallel()

	src := &model.RuntimeConfig{
		WorkspaceRoot: "/workspace",
		Mode:          model.ExecutionModePRDTasks,
		IDE:           model.IDECodex,
		Timeout:       10 * time.Minute,
	}
	cfg := NewConfig(src, model.RunArtifacts{})
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.CloseOnComplete {
		t.Fatal("expected CloseOnComplete=false by default")
	}
}

func TestNewConfigReturnsNilForNilSource(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(nil, model.RunArtifacts{})
	if cfg != nil {
		t.Fatal("expected nil config for nil source")
	}
}
