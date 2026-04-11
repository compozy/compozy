package cli

import (
	"testing"

	core "github.com/compozy/compozy/internal/core"
)

func TestBuildConfigStartAlwaysEnablesExecutableExtensions(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if !cfg.EnableExecutableExtensions {
		t.Fatal("expected start config to enable executable extensions")
	}
}

func TestBuildConfigFixReviewsAlwaysEnablesExecutableExtensions(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if !cfg.EnableExecutableExtensions {
		t.Fatal("expected fix-reviews config to enable executable extensions")
	}
}

func TestBuildConfigExecDefaultsExtensionsDisabled(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindExec, core.ModeExec)

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.EnableExecutableExtensions {
		t.Fatal("expected exec config to keep executable extensions disabled by default")
	}
}

func TestBuildConfigExecExtensionsFlagEnablesExecutableExtensions(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindExec, core.ModeExec)
	state.extensionsEnabled = true

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if !cfg.EnableExecutableExtensions {
		t.Fatal("expected exec config to enable executable extensions when flag is set")
	}
}

func TestNewExecCommandRegistersExtensionsFlag(t *testing.T) {
	t.Parallel()

	cmd := newExecCommandWithDefaults(nil, defaultCommandStateDefaults())
	flag := cmd.Flags().Lookup("extensions")
	if flag == nil {
		t.Fatal("expected exec command to register --extensions")
	}
	if flag.DefValue != "false" {
		t.Fatalf("expected --extensions default false, got %q", flag.DefValue)
	}
}
