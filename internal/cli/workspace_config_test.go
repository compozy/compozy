package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/modelprovider"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/spf13/cobra"
)

var cliWorkingDirMu sync.Mutex

func TestApplyWorkspaceDefaultsLoadsNearestWorkspaceConfig(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
access_mode = "default"
timeout = "5m"
add_dirs = ["../shared", "../docs"]

[start]
include_completed = true
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("include-completed", false, "include completed")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if mustEvalSymlinksCLITest(t, state.workspaceRoot) != mustEvalSymlinksCLITest(t, root) {
		t.Fatalf("unexpected workspace root\nwant: %q\ngot:  %q", root, state.workspaceRoot)
	}
	if state.ide != "claude" {
		t.Fatalf("unexpected ide default: %q", state.ide)
	}
	if state.accessMode != "default" {
		t.Fatalf("unexpected access mode default: %q", state.accessMode)
	}
	if state.timeout != "5m" {
		t.Fatalf("unexpected timeout default: %q", state.timeout)
	}
	if !state.includeCompleted {
		t.Fatalf("expected includeCompleted=true")
	}
	resolvedRoot := mustEvalSymlinksCLITest(t, root)
	wantDirs := []string{
		filepath.Join(filepath.Dir(resolvedRoot), "shared"),
		filepath.Join(filepath.Dir(resolvedRoot), "docs"),
	}
	if !reflect.DeepEqual(state.addDirs, wantDirs) {
		t.Fatalf("unexpected addDirs\nwant: %#v\ngot:  %#v", wantDirs, state.addDirs)
	}

	globalConfigPath := filepath.Join(homeDir, ".compozy", "config.toml")
	if _, err := os.Stat(globalConfigPath); !os.IsNotExist(err) {
		t.Fatalf("expected isolated global config to be absent, got stat err=%v", err)
	}
}

func TestApplyWorkspaceDefaultsDoesNotOverrideChangedFlags(t *testing.T) {
	isolateCLIConfigHome(t)
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"

[fix_reviews]
batch_size = 4
`)

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)
	cmd.Flags().Int("batch-size", 1, "batch size")

	chdirCLITest(t, startDir)

	if err := cmd.Flags().Set("ide", "gemini"); err != nil {
		t.Fatalf("set ide: %v", err)
	}
	if err := cmd.Flags().Set("batch-size", "2"); err != nil {
		t.Fatalf("set batch-size: %v", err)
	}
	state.batchSize = 2

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.ide != "gemini" {
		t.Fatalf("expected explicit ide flag to win, got %q", state.ide)
	}
	if state.batchSize != 2 {
		t.Fatalf("expected explicit batch-size flag to win, got %d", state.batchSize)
	}
}

func TestApplyWorkspaceDefaultsCanDisableAutomaticRetries(t *testing.T) {
	isolateCLIConfigHome(t)
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
max_retries = 0
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)

	chdirCLITest(t, startDir)

	if state.maxRetries != defaultMaxRetries {
		t.Fatalf("unexpected built-in retry default before config: %d", state.maxRetries)
	}
	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.maxRetries != 0 {
		t.Fatalf("expected workspace config to disable automatic retries, got %d", state.maxRetries)
	}
}

func TestApplyWorkspaceDefaultsUsesExecOverridesOverDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
model = "sonnet"
output_format = "text"

[exec]
ide = "codex"
model = "gpt-5.4"
output_format = "raw-json"
verbose = true
`)

	state := newCommandState(commandKindExec, core.ModeExec)
	cmd := newTestCommand(state)
	cmd.Flags().String("format", "", "output format")
	cmd.Flags().Bool("verbose", false, "verbose logging")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.ide != "codex" {
		t.Fatalf("expected exec.ide to override defaults.ide, got %q", state.ide)
	}
	if state.model != "gpt-5.4" {
		t.Fatalf("expected exec.model to override defaults.model, got %q", state.model)
	}
	if state.outputFormat != "raw-json" {
		t.Fatalf("expected exec.output_format to override defaults.output_format, got %q", state.outputFormat)
	}
	if !state.verbose {
		t.Fatal("expected exec.verbose to enable verbose logging")
	}
}

func TestApplyWorkspaceDefaultsUsesStartPresentationOverrides(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
output_format = "text"

[start]
output_format = "json"
tui = false
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.outputFormat != "json" {
		t.Fatalf("expected start.output_format to override defaults.output_format, got %q", state.outputFormat)
	}
	if state.tui {
		t.Fatal("expected start.tui to disable the workflow TUI")
	}
}

func TestApplyWorkspaceDefaultsKeepsConfiguredTaskRuntimeRulesAndBuildConfigAppendsCLIOverrides(t *testing.T) {
	isolateCLIConfigHome(t)

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[start]
[[start.task_runtime_rules]]
type = "frontend"
ide = "claude"
model = "sonnet"
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Var(
		newTaskRuntimeFlagValue(&state.executionTaskRuntimeRules),
		"task-runtime",
		"task runtime",
	)

	if err := cmd.Flags().Set("task-runtime", "id=task_01,model=gpt-5.4-mini"); err != nil {
		t.Fatalf("set task-runtime flag: %v", err)
	}

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if len(state.configuredTaskRuntimeRules) != 1 {
		t.Fatalf("unexpected configured task runtime rules: %#v", state.configuredTaskRuntimeRules)
	}

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if len(cfg.TaskRuntimeRules) != 2 {
		t.Fatalf("unexpected merged task runtime rules: %#v", cfg.TaskRuntimeRules)
	}
	if cfg.TaskRuntimeRules[0].Type == nil || *cfg.TaskRuntimeRules[0].Type != "frontend" {
		t.Fatalf("expected config type rule first, got %#v", cfg.TaskRuntimeRules[0])
	}
	if cfg.TaskRuntimeRules[1].ID == nil || *cfg.TaskRuntimeRules[1].ID != "task_01" {
		t.Fatalf("expected CLI id rule to append after config, got %#v", cfg.TaskRuntimeRules[1])
	}
}

func TestApplyWorkspaceDefaultsUsesFixReviewsPresentationOverrides(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
output_format = "text"

[fix_reviews]
output_format = "raw-json"
tui = false
`)

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.outputFormat != "raw-json" {
		t.Fatalf("expected fix_reviews.output_format to override defaults.output_format, got %q", state.outputFormat)
	}
	if state.tui {
		t.Fatal("expected fix_reviews.tui to disable the workflow TUI")
	}
}

func TestApplyWorkspaceDefaultsFetchReviewsNitpicks(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		configContent string
		wantNitpicks  bool
	}{
		{
			name:         "keep reviews fetch review-body comments enabled when config is absent",
			wantNitpicks: true,
		},
		{
			name: "disable reviews fetch review-body comments from workspace config",
			configContent: `
[fetch_reviews]
nitpicks = false
`,
			wantNitpicks: false,
		},
		{
			name: "enable reviews fetch review-body comments from workspace config",
			configContent: `
[fetch_reviews]
nitpicks = true
`,
			wantNitpicks: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("Should "+tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			startDir := filepath.Join(root, "pkg", "feature")
			if err := os.MkdirAll(startDir, 0o755); err != nil {
				t.Fatalf("mkdir start dir: %v", err)
			}
			if strings.TrimSpace(tc.configContent) != "" {
				writeCLIWorkspaceConfig(t, root, tc.configContent)
			}

			state := newCommandState(commandKindFetchReviews, core.ModePRReview)
			cmd := &cobra.Command{Use: "reviews fetch"}
			cmd.Flags().String("provider", "", "provider")

			chdirCLITest(t, startDir)

			if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
				t.Fatalf("apply workspace defaults: %v", err)
			}
			if state.nitpicks != tc.wantNitpicks {
				t.Fatalf("unexpected nitpicks setting: got %t, want %t", state.nitpicks, tc.wantNitpicks)
			}
		})
	}
}

func TestApplyWorkspaceDefaultsPreservesExplicitExecFormatFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
output_format = "text"

[exec]
output_format = "json"
`)

	state := newCommandState(commandKindExec, core.ModeExec)
	cmd := newTestCommand(state)
	cmd.Flags().String("format", "", "output format")

	chdirCLITest(t, startDir)

	if err := cmd.Flags().Set("format", "text"); err != nil {
		t.Fatalf("set format: %v", err)
	}
	state.outputFormat = "text"

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.outputFormat != "text" {
		t.Fatalf("expected explicit format flag to win, got %q", state.outputFormat)
	}
}

func TestPrepareWorkspaceContextBootstrapsExtensionProvidersBeforeWorkspaceValidation(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	supportsAddDirs := true
	t.Setenv("HOME", homeDir)

	startDir := filepath.Join(workspaceRoot, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, workspaceRoot, `
[defaults]
ide = "ext-adapter"
model = "ext-model"

[fetch_reviews]
provider = "ext-review"
`)

	manifest := bootstrapManifestFixture("provider-ext")
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityProvidersRegister}
	manifest.Providers.IDE = []extensions.ProviderEntry{{
		Name:            "ext-adapter",
		Command:         "mock-acp",
		FixedArgs:       []string{"serve"},
		DisplayName:     "Mock ACP",
		DefaultModel:    "ext-model",
		SetupAgentName:  "codex",
		SupportsAddDirs: &supportsAddDirs,
	}}
	manifest.Providers.Review = []extensions.ProviderEntry{{
		Name:        "ext-review",
		Command:     "coderabbit",
		DisplayName: "Extension Review",
	}}
	manifest.Providers.Model = []extensions.ProviderEntry{{
		Name:        "ext-model",
		Target:      "openai/gpt-5.4",
		DisplayName: "Extension Model",
	}}
	extensionDir := filepath.Join(workspaceRoot, ".compozy", "extensions", "provider-ext")
	writeBootstrapManifestJSON(t, extensionDir, manifest)
	enableBootstrapWorkspaceExtension(t, homeDir, workspaceRoot, "provider-ext")

	chdirCLITest(t, startDir)

	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	cmd := &cobra.Command{Use: "reviews fetch"}
	cmd.Flags().String("ide", "", "")
	cmd.Flags().String("model", "", "")
	cmd.Flags().String("provider", "", "")

	_, cleanup, err := state.prepareWorkspaceContext(context.Background(), cmd)
	if err != nil {
		t.Fatalf("prepareWorkspaceContext() error = %v", err)
	}
	defer cleanup()

	if state.ide != "ext-adapter" {
		t.Fatalf("state.ide = %q, want %q", state.ide, "ext-adapter")
	}
	if state.model != "ext-model" {
		t.Fatalf("state.model = %q, want %q", state.model, "ext-model")
	}
	if state.provider != "ext-review" {
		t.Fatalf("state.provider = %q, want %q", state.provider, "ext-review")
	}
	if got := modelprovider.ResolveAlias(state.model); got != "openai/gpt-5.4" {
		t.Fatalf("ResolveAlias(%q) = %q, want %q", state.model, got, "openai/gpt-5.4")
	}
	if err := agent.ValidateRuntimeConfig(&model.RuntimeConfig{
		Mode:                   model.ExecutionModePRReview,
		IDE:                    state.ide,
		OutputFormat:           model.OutputFormatText,
		BatchSize:              1,
		MaxRetries:             0,
		RetryBackoffMultiplier: 1.5,
	}); err != nil {
		t.Fatalf("ValidateRuntimeConfig() error = %v", err)
	}
}

func TestNewFormInputsFromStatePreservesResolvedDefaults(t *testing.T) {
	t.Parallel()

	state := &commandState{
		workflowIdentity: workflowIdentity{
			name:     "demo",
			tasksDir: "/tmp/demo/.compozy/tasks/demo",
		},
		runtimeConfig: runtimeConfig{
			ide:              "claude",
			model:            "sonnet",
			addDirs:          []string{"../shared", "../docs"},
			reasoningEffort:  "high",
			includeCompleted: true,
			autoCommit:       true,
		},
	}

	inputs := newFormInputsFromState(state)

	if inputs.name != "demo" || inputs.ide != "claude" || inputs.model != "sonnet" {
		t.Fatalf("unexpected string inputs: %#v", inputs)
	}
	if inputs.addDirs != "../shared, ../docs" {
		t.Fatalf("unexpected addDirs input: %q", inputs.addDirs)
	}
	if !inputs.includeCompleted || !inputs.autoCommit {
		t.Fatalf("expected boolean defaults to be preserved: %#v", inputs)
	}
}

func TestApplyWorkspaceDefaultsLoadsGlobalConfigWhenWorkspaceConfigIsMissing(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIGlobalConfig(t, homeDir, `
[defaults]
ide = "claude"
model = "sonnet"

[start]
include_completed = true
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("include-completed", false, "include completed")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.ide != "claude" {
		t.Fatalf("expected global ide to apply, got %q", state.ide)
	}
	if state.model != "sonnet" {
		t.Fatalf("expected global model to apply, got %q", state.model)
	}
	if !state.includeCompleted {
		t.Fatal("expected global start.include_completed to apply")
	}
}

func TestApplyWorkspaceDefaultsMergesWorkspaceOverGlobal(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIGlobalConfig(t, homeDir, `
[defaults]
ide = "claude"
model = "sonnet"
access_mode = "default"

[start]
include_completed = false
`)
	writeCLIWorkspaceConfig(t, root, `
[defaults]
model = "gpt-5.4"

[start]
include_completed = true
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("include-completed", false, "include completed")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.ide != "claude" {
		t.Fatalf("expected global defaults.ide fallback, got %q", state.ide)
	}
	if state.model != "gpt-5.4" {
		t.Fatalf("expected workspace defaults.model to override global, got %q", state.model)
	}
	if state.accessMode != "default" {
		t.Fatalf("expected global access_mode fallback, got %q", state.accessMode)
	}
	if !state.includeCompleted {
		t.Fatal("expected workspace start.include_completed to override global")
	}
}

func TestApplyWorkspaceDefaultsUsesWorkspaceAttachModeOverGlobalRunsDefault(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIGlobalConfig(t, homeDir, `
[runs]
default_attach_mode = "stream"
`)
	writeCLIWorkspaceConfig(t, root, `
[runs]
default_attach_mode = "detach"
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := &cobra.Command{Use: "tasks run"}
	cmd.Flags().StringVar(&state.attachMode, "attach", attachModeAuto, "attach mode")
	cmd.Flags().Bool("ui", false, "ui mode")
	cmd.Flags().Bool("stream", false, "stream mode")
	cmd.Flags().Bool("detach", false, "detach mode")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.attachMode != attachModeDetach {
		t.Fatalf("expected workspace runs.default_attach_mode to override global, got %q", state.attachMode)
	}
}

func TestApplyWorkspaceDefaultsPreservesExplicitAttachFlagOverConfig(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIGlobalConfig(t, homeDir, `
[runs]
default_attach_mode = "stream"
`)
	writeCLIWorkspaceConfig(t, root, `
[runs]
default_attach_mode = "detach"
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := &cobra.Command{Use: "tasks run"}
	cmd.Flags().StringVar(&state.attachMode, "attach", attachModeAuto, "attach mode")
	cmd.Flags().Bool("ui", false, "ui mode")
	cmd.Flags().Bool("stream", false, "stream mode")
	cmd.Flags().Bool("detach", false, "detach mode")

	chdirCLITest(t, startDir)

	if err := cmd.Flags().Set("attach", "ui"); err != nil {
		t.Fatalf("set attach: %v", err)
	}
	state.attachMode = attachModeUI

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.attachMode != attachModeUI {
		t.Fatalf("expected explicit --attach flag to win, got %q", state.attachMode)
	}
}

func TestApplyWorkspaceDefaultsKeepsWorkspaceDefaultsAheadOfGlobalStartOverrides(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIGlobalConfig(t, homeDir, `
[defaults]
output_format = "json"

[start]
output_format = "raw-json"
tui = false
`)
	writeCLIWorkspaceConfig(t, root, `
[defaults]
output_format = "text"
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.outputFormat != "text" {
		t.Fatalf(
			"expected workspace defaults.output_format to beat global start.output_format, got %q",
			state.outputFormat,
		)
	}
	if state.tui {
		t.Fatal("expected global start.tui to remain applied")
	}
}

func TestApplyWorkspaceDefaultsKeepsWorkspaceDefaultsAheadOfGlobalExecOverrides(t *testing.T) {
	root := t.TempDir()
	homeDir := isolateCLIConfigHome(t)
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIGlobalConfig(t, homeDir, `
[defaults]
model = "sonnet"

[exec]
model = "gpt-5.4"
verbose = true
`)
	writeCLIWorkspaceConfig(t, root, `
[defaults]
model = "o4-mini"
`)

	state := newCommandState(commandKindExec, core.ModeExec)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("verbose", false, "verbose logging")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}
	if state.model != "o4-mini" {
		t.Fatalf("expected workspace defaults.model to beat global exec.model, got %q", state.model)
	}
	if !state.verbose {
		t.Fatal("expected global exec.verbose to remain applied")
	}
}

func TestNewFormInputsFromStateQuotesAddDirsContainingCommas(t *testing.T) {
	t.Parallel()

	state := &commandState{
		runtimeConfig: runtimeConfig{
			addDirs: []string{"../docs,archive", "../shared"},
		},
	}

	inputs := newFormInputsFromState(state)
	if inputs.addDirs != "\"../docs,archive\", ../shared" {
		t.Fatalf("unexpected addDirs input: %q", inputs.addDirs)
	}
}

func TestApplyConfigHandlesSupportedTypes(t *testing.T) {
	t.Parallel()

	newCommand := func() *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("string", "", "")
		cmd.Flags().Int("int", 0, "")
		cmd.Flags().Float64("float", 0, "")
		cmd.Flags().Bool("bool", false, "")
		cmd.Flags().StringSlice("slice", nil, "")
		return cmd
	}

	cases := []struct {
		name string
		run  func(t *testing.T, cmd *cobra.Command)
	}{
		{
			name: "Should apply string config values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				value := "claude"
				var got string
				applyConfig(cmd, "string", &value, func(applied string) { got = applied })
				if got != value {
					t.Fatalf("unexpected string config value: %q", got)
				}
			},
		},
		{
			name: "Should apply integer config values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				value := 3
				var got int
				applyConfig(cmd, "int", &value, func(applied int) { got = applied })
				if got != value {
					t.Fatalf("unexpected int config value: %d", got)
				}
			},
		},
		{
			name: "Should apply float config values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				value := 2.5
				var got float64
				applyConfig(cmd, "float", &value, func(applied float64) { got = applied })
				if got != value {
					t.Fatalf("unexpected float config value: %v", got)
				}
			},
		},
		{
			name: "Should apply boolean config values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				value := true
				var got bool
				applyConfig(cmd, "bool", &value, func(applied bool) { got = applied })
				if got != value {
					t.Fatalf("unexpected bool config value: %t", got)
				}
			},
		},
		{
			name: "Should clone string slice config values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				value := []string{"../shared", "../docs"}
				var got []string
				applyConfig(cmd, "slice", &value, func(applied []string) { got = applied }, slices.Clone[[]string])
				if !reflect.DeepEqual(got, value) {
					t.Fatalf("unexpected string slice config: %#v", got)
				}

				got[0] = "../changed"
				if value[0] != "../shared" {
					t.Fatalf("applyConfig should clone []string values, got source %#v", value)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newCommand())
		})
	}
}

func TestApplyInputHandlesSupportedTypes(t *testing.T) {
	t.Parallel()

	newCommand := func() *cobra.Command {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("name", "", "")
		cmd.Flags().String("round", "", "")
		cmd.Flags().Bool("dry-run", false, "")
		cmd.Flags().String("add-dir", "", "")
		return cmd
	}

	cases := []struct {
		name string
		run  func(t *testing.T, cmd *cobra.Command)
	}{
		{
			name: "Should apply string input values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				var got string
				applyInput(cmd, "name", "demo", passThroughInput[string], func(value string) { got = value })
				if got != "demo" {
					t.Fatalf("unexpected name input: %q", got)
				}
			},
		},
		{
			name: "Should apply integer input values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				var got int
				applyInput(cmd, "round", "7", parseIntInput, func(value int) { got = value })
				if got != 7 {
					t.Fatalf("unexpected round input: %d", got)
				}
			},
		},
		{
			name: "Should apply boolean input values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				var got bool
				applyInput(cmd, "dry-run", true, passThroughInput[bool], func(value bool) { got = value })
				if !got {
					t.Fatal("expected dry-run input to be applied")
				}
			},
		},
		{
			name: "Should parse string slice input values",
			run: func(t *testing.T, cmd *cobra.Command) {
				t.Helper()

				var got []string
				applyInput(
					cmd,
					"add-dir",
					"../shared, ../docs",
					parseStringSliceInput,
					func(value []string) { got = value },
				)
				if !reflect.DeepEqual(got, []string{"../shared", "../docs"}) {
					t.Fatalf("unexpected add-dir input: %#v", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newCommand())
		})
	}
}

func TestSimpleCommandBaseLoadWorkspaceRootWorksForSimpleCommands(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
`)
	chdirCLITest(t, startDir)

	migrateState := &migrateCommandState{}
	syncState := &syncCommandState{}
	archiveState := &archiveCommandState{}
	cases := []struct {
		name string
		base *simpleCommandBase
	}{
		{name: "Should migrate workspace", base: &migrateState.simpleCommandBase},
		{name: "Should sync workspace", base: &syncState.simpleCommandBase},
		{name: "Should archive workspace", base: &archiveState.simpleCommandBase},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.base.loadWorkspaceRoot(context.Background()); err != nil {
				t.Fatalf("load workspace root: %v", err)
			}
			if mustEvalSymlinksCLITest(t, tc.base.workspaceRoot) != mustEvalSymlinksCLITest(t, root) {
				t.Fatalf("unexpected workspace root for %s: %q", tc.name, tc.base.workspaceRoot)
			}
		})
	}
}

func TestResolveWorkspaceContextUsesNearestWorkspace(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, "")
	chdirCLITest(t, startDir)

	workspaceCtx, err := resolveWorkspaceContext(context.Background())
	if err != nil {
		t.Fatalf("resolveWorkspaceContext: %v", err)
	}
	if mustEvalSymlinksCLITest(t, workspaceCtx.Root) != mustEvalSymlinksCLITest(t, root) {
		t.Fatalf("unexpected workspace root: got %q want %q", workspaceCtx.Root, root)
	}
}

func TestCommandPathHandlesNilCommand(t *testing.T) {
	t.Parallel()

	if got := commandPath(nil); got != "" {
		t.Fatalf("expected empty command path for nil command, got %q", got)
	}
	cmd := &cobra.Command{Use: "tasks"}
	if got := commandPath(cmd); got != "tasks" {
		t.Fatalf("unexpected command path: %q", got)
	}
}

func TestApplySoundConfigCopiesConfiguredValues(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	enabled := true
	onCompleted := "done.wav"
	onFailed := "failed.wav"

	applySoundConfig(state, workspace.SoundConfig{
		Enabled:     &enabled,
		OnCompleted: &onCompleted,
		OnFailed:    &onFailed,
	})

	if !state.soundEnabled {
		t.Fatal("expected soundEnabled to be copied from config")
	}
	if state.soundOnCompleted != "done.wav" {
		t.Fatalf("unexpected soundOnCompleted: %q", state.soundOnCompleted)
	}
	if state.soundOnFailed != "failed.wav" {
		t.Fatalf("unexpected soundOnFailed: %q", state.soundOnFailed)
	}
}

func TestBuildConfigMapsEmbeddedStateGroups(t *testing.T) {
	t.Parallel()

	state := &commandState{
		workspaceRoot: "/workspace",
		kind:          commandKindExec,
		mode:          core.ModeExec,
		workflowIdentity: workflowIdentity{
			name:       "demo",
			pr:         "259",
			provider:   "coderabbit",
			round:      4,
			nitpicks:   true,
			reviewsDir: "/workspace/.compozy/tasks/demo/reviews-004",
			tasksDir:   "/workspace/.compozy/tasks/demo",
		},
		runtimeConfig: runtimeConfig{
			dryRun:           true,
			autoCommit:       true,
			concurrent:       2,
			batchSize:        3,
			ide:              "codex",
			model:            "gpt-5.4",
			addDirs:          []string{"../shared", "../docs", "../shared"},
			tailLines:        40,
			reasoningEffort:  "high",
			accessMode:       core.AccessModeDefault,
			includeCompleted: true,
			includeResolved:  true,
		},
		execConfig: execConfig{
			outputFormat:       string(core.OutputFormatJSON),
			verbose:            true,
			tui:                true,
			persist:            true,
			runID:              "run-123",
			promptText:         "prompt",
			promptFile:         "prompt.md",
			readPromptStdin:    true,
			resolvedPromptText: "resolved prompt",
		},
		retryConfig: retryConfig{
			timeout:                "2m",
			maxRetries:             2,
			retryBackoffMultiplier: 2.0,
		},
	}

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}

	if cfg.Name != "demo" || cfg.PR != "259" || cfg.Provider != "coderabbit" || cfg.Round != 4 {
		t.Fatalf("unexpected identity config: %#v", cfg)
	}
	if !cfg.Nitpicks {
		t.Fatal("expected nitpicks to pass through buildConfig")
	}
	if cfg.IDE != core.IDECodex || cfg.Model != "gpt-5.4" || cfg.AccessMode != core.AccessModeDefault {
		t.Fatalf("unexpected runtime config: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.AddDirs, []string{"../shared", "../docs"}) {
		t.Fatalf("unexpected normalized add dirs: %#v", cfg.AddDirs)
	}
	if cfg.OutputFormat != core.OutputFormatJSON || cfg.RunID != "run-123" ||
		cfg.ResolvedPromptText != "resolved prompt" {
		t.Fatalf("unexpected exec config: %#v", cfg)
	}
	if cfg.Timeout.String() != "2m0s" || cfg.MaxRetries != 2 || cfg.RetryBackoffMultiplier != 2.0 {
		t.Fatalf("unexpected retry config: %#v", cfg)
	}
}

func writeCLIWorkspaceConfig(t *testing.T, workspaceRoot, content string) {
	t.Helper()

	configDir := filepath.Join(workspaceRoot, ".compozy")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeCLIGlobalConfig(t *testing.T, homeDir, content string) {
	t.Helper()

	configDir := filepath.Join(homeDir, ".compozy")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir global .compozy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}
}

func isolateCLIConfigHome(t *testing.T) string {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	return homeDir
}

func mustEvalSymlinksCLITest(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks for %s: %v", path, err)
	}
	return resolved
}

func chdirCLITest(t *testing.T, dir string) {
	t.Helper()

	cliWorkingDirMu.Lock()

	originalWD, err := os.Getwd()
	if err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("chdir: %v", err)
	}

	t.Cleanup(func() {
		defer cliWorkingDirMu.Unlock()
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
}
