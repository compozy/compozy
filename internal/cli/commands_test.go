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
		t.Fatal("expected reviews fix config to enable executable extensions")
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

func TestBuildConfigFetchReviewsDefaultsReviewBodyCommentsEnabled(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFetchReviews, core.ModePRReview)

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if !cfg.Nitpicks {
		t.Fatal("expected reviews fetch config to enable CodeRabbit review-body comments by default")
	}
}

func TestNewExecCommandRegistersExtensionsFlag(t *testing.T) {
	t.Parallel()

	cmd := newExecCommandWithDefaults(defaultCommandStateDefaults())
	flag := cmd.Flags().Lookup("extensions")
	if flag == nil {
		t.Fatal("expected exec command to register --extensions")
	}
	if flag.DefValue != "false" {
		t.Fatalf("expected --extensions default false, got %q", flag.DefValue)
	}
}

func TestNewTasksRunCommandDefaultsAttachModeToAuto(t *testing.T) {
	t.Parallel()

	cmd := newTasksRunCommandWithDefaults(nil, defaultCommandStateDefaults())
	flag := cmd.Flags().Lookup("attach")
	if flag == nil {
		t.Fatal("expected --attach flag")
	}
	if flag.DefValue != attachModeAuto {
		t.Fatalf("expected --attach default %q, got %q", attachModeAuto, flag.DefValue)
	}
	if cmd.Flags().Lookup("tui") != nil {
		t.Fatal("expected tasks run to omit legacy --tui flag")
	}
}

func TestReviewsFixCommandDefaultsTUIToTrue(t *testing.T) {
	t.Parallel()

	cmd := newReviewsFixCommandWithDefaults(defaultCommandStateDefaults())
	flag := cmd.Flags().Lookup("tui")
	if flag == nil {
		t.Fatal("expected --tui flag")
	}
	if flag.DefValue != "true" {
		t.Fatalf("expected --tui default true, got %q", flag.DefValue)
	}
}
