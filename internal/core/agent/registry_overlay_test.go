package agent

import (
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestActivateOverlayRegistersDeclarativeRuntimeSpec(t *testing.T) {
	restore, err := ActivateOverlay([]OverlayEntry{
		{
			Name:    "ext-adapter",
			Command: "mock-acp --serve",
			Metadata: map[string]string{
				"display_name":      "Mock ACP",
				"default_model":     "mock-model",
				"agent_name":        "codex",
				"supports_add_dirs": "true",
			},
		},
	})
	if err != nil {
		t.Fatalf("activate ACP overlay: %v", err)
	}
	defer restore()

	if err := ValidateRuntimeConfig(&model.RuntimeConfig{
		Mode:                   model.ExecutionModePRDTasks,
		IDE:                    "ext-adapter",
		OutputFormat:           model.OutputFormatText,
		BatchSize:              1,
		MaxRetries:             0,
		RetryBackoffMultiplier: 1.5,
	}); err != nil {
		t.Fatalf("validate runtime config with overlay IDE: %v", err)
	}

	spec, err := lookupAgentSpec("ext-adapter")
	if err != nil {
		t.Fatalf("lookup overlay spec: %v", err)
	}
	if spec.Command != "mock-acp" {
		t.Fatalf("unexpected overlay command: %q", spec.Command)
	}
	if len(spec.FixedArgs) != 1 || spec.FixedArgs[0] != "--serve" {
		t.Fatalf("unexpected overlay fixed args: %#v", spec.FixedArgs)
	}
	if spec.SetupAgentName != "codex" {
		t.Fatalf("unexpected setup agent name: %q", spec.SetupAgentName)
	}
	if got := DisplayName("ext-adapter"); got != "Mock ACP" {
		t.Fatalf("unexpected overlay display name: %q", got)
	}
	if got, err := SetupAgentName("ext-adapter"); err != nil || got != "codex" {
		t.Fatalf("unexpected overlay setup agent mapping: got %q err=%v", got, err)
	}
	if got, err := ResolveRuntimeModel("ext-adapter", ""); err != nil || got != "mock-model" {
		t.Fatalf("unexpected overlay runtime model: got %q err=%v", got, err)
	}
}
