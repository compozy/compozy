package commands

import (
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
)

func TestRunStartFromConfigMapsLegacyRuntimeFields(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := RunStartFromConfig(cfg)

	assertRuntimeFields(t, cmd.RuntimeFields, cfg)
	if cmd.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("unexpected mode: %q", cmd.Mode)
	}
	if cmd.IDE != model.IDECodex {
		t.Fatalf("unexpected ide: %q", cmd.IDE)
	}
	if cmd.Model != "gpt-5.4" {
		t.Fatalf("unexpected model: %q", cmd.Model)
	}
	if cmd.Name != "demo" {
		t.Fatalf("unexpected name: %q", cmd.Name)
	}
}

func TestRunStartCommandRuntimeConfigUsesSharedConversion(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := RunStartFromConfig(cfg)
	runtimeCfg := cmd.RuntimeConfig()

	if runtimeCfg.WorkspaceRoot != cfg.WorkspaceRoot {
		t.Fatalf("unexpected workspace root: %q", runtimeCfg.WorkspaceRoot)
	}
	if runtimeCfg.IDE != model.IDECodex {
		t.Fatalf("unexpected ide: %q", runtimeCfg.IDE)
	}
}

func TestWorkflowPrepareFromConfigMapsLegacyRuntimeFields(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := WorkflowPrepareFromConfig(cfg)

	assertRuntimeFields(t, cmd.RuntimeFields, cfg)
}

func TestWorkflowPrepareCommandRuntimeConfigUsesSharedConversion(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := WorkflowPrepareFromConfig(cfg)
	runtimeCfg := cmd.RuntimeConfig()

	if runtimeCfg.Name != cfg.Name {
		t.Fatalf("unexpected name: %q", runtimeCfg.Name)
	}
	if runtimeCfg.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("unexpected mode: %q", runtimeCfg.Mode)
	}
}

func TestReviewsFetchFromConfigMapsLegacyFields(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := ReviewsFetchFromConfig(cfg)

	if cmd.WorkspaceRoot != cfg.WorkspaceRoot {
		t.Fatalf("unexpected workspace root: %q", cmd.WorkspaceRoot)
	}
	if cmd.Name != cfg.Name {
		t.Fatalf("unexpected name: %q", cmd.Name)
	}
	if cmd.Round != cfg.Round {
		t.Fatalf("unexpected round: %d", cmd.Round)
	}
	if cmd.Provider != cfg.Provider {
		t.Fatalf("unexpected provider: %q", cmd.Provider)
	}
	if cmd.PR != cfg.PR {
		t.Fatalf("unexpected pr: %q", cmd.PR)
	}
}

func TestReviewsFetchCommandCoreConfigMapsFields(t *testing.T) {
	t.Parallel()

	cmd := ReviewsFetchCommand{
		WorkspaceRoot: "/workspace",
		Name:          "demo",
		Round:         3,
		Provider:      "coderabbit",
		PR:            "259",
	}
	cfg := cmd.CoreConfig()

	if cfg.WorkspaceRoot != cmd.WorkspaceRoot || cfg.Name != cmd.Name || cfg.Round != cmd.Round {
		t.Fatalf("unexpected fetch config: %#v", cfg)
	}
}

func TestWorkspaceMigrateFromConfigMapsLegacyFields(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := WorkspaceMigrateFromConfig(cfg)

	if cmd.WorkspaceRoot != cfg.WorkspaceRoot {
		t.Fatalf("unexpected workspace root: %q", cmd.WorkspaceRoot)
	}
	if cmd.Name != cfg.Name {
		t.Fatalf("unexpected name: %q", cmd.Name)
	}
	if cmd.TasksDir != cfg.TasksDir {
		t.Fatalf("unexpected tasks dir: %q", cmd.TasksDir)
	}
	if cmd.ReviewsDir != cfg.ReviewsDir {
		t.Fatalf("unexpected reviews dir: %q", cmd.ReviewsDir)
	}
	if !cmd.DryRun {
		t.Fatal("expected dry run to pass through")
	}
}

func TestWorkspaceMigrateCommandCoreConfigMapsFields(t *testing.T) {
	t.Parallel()

	cmd := WorkspaceMigrateCommand{
		WorkspaceRoot: "/workspace",
		RootDir:       "/workspace/.compozy/tasks",
		Name:          "demo",
		TasksDir:      "/workspace/.compozy/tasks/demo",
		ReviewsDir:    "/workspace/.compozy/tasks/demo/reviews-001",
		DryRun:        true,
	}
	cfg := cmd.CoreConfig()

	if cfg.WorkspaceRoot != cmd.WorkspaceRoot || cfg.RootDir != cmd.RootDir || cfg.ReviewsDir != cmd.ReviewsDir {
		t.Fatalf("unexpected migrate config: %#v", cfg)
	}
}

func TestWorkflowSyncFromConfigPassesThroughTasksDirAndDryRun(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := WorkflowSyncFromConfig(cfg)

	if cmd.WorkspaceRoot != cfg.WorkspaceRoot {
		t.Fatalf("unexpected workspace root: %q", cmd.WorkspaceRoot)
	}
	if cmd.Name != cfg.Name {
		t.Fatalf("unexpected name: %q", cmd.Name)
	}
	if cmd.TasksDir != cfg.TasksDir {
		t.Fatalf("unexpected tasks dir: %q", cmd.TasksDir)
	}
	if !cmd.DryRun {
		t.Fatal("expected dry run to pass through")
	}
}

func TestWorkflowSyncCommandCoreConfigMapsFields(t *testing.T) {
	t.Parallel()

	cmd := WorkflowSyncCommand{
		WorkspaceRoot: "/workspace",
		RootDir:       "/workspace/.compozy/tasks",
		Name:          "demo",
		TasksDir:      "/workspace/.compozy/tasks/demo",
	}
	cfg := cmd.CoreConfig()

	if cfg.WorkspaceRoot != cmd.WorkspaceRoot || cfg.RootDir != cmd.RootDir || cfg.TasksDir != cmd.TasksDir {
		t.Fatalf("unexpected sync config: %#v", cfg)
	}
}

func TestWorkflowArchiveFromConfigMapsLegacyFields(t *testing.T) {
	t.Parallel()

	cfg := testCoreConfig()
	cmd := WorkflowArchiveFromConfig(cfg)

	if cmd.WorkspaceRoot != cfg.WorkspaceRoot {
		t.Fatalf("unexpected workspace root: %q", cmd.WorkspaceRoot)
	}
	if cmd.Name != cfg.Name {
		t.Fatalf("unexpected name: %q", cmd.Name)
	}
	if cmd.TasksDir != cfg.TasksDir {
		t.Fatalf("unexpected tasks dir: %q", cmd.TasksDir)
	}
}

func TestWorkflowArchiveCommandCoreConfigMapsFields(t *testing.T) {
	t.Parallel()

	cmd := WorkflowArchiveCommand{
		WorkspaceRoot: "/workspace",
		RootDir:       "/workspace/.compozy/tasks",
		Name:          "demo",
		TasksDir:      "/workspace/.compozy/tasks/demo",
	}
	cfg := cmd.CoreConfig()

	if cfg.WorkspaceRoot != cmd.WorkspaceRoot || cfg.RootDir != cmd.RootDir || cfg.TasksDir != cmd.TasksDir {
		t.Fatalf("unexpected archive config: %#v", cfg)
	}
}

func TestRuntimeConfigNormalizesAddDirsAndAppliesDefaults(t *testing.T) {
	t.Parallel()

	fields := RuntimeFields{
		WorkspaceRoot: "/workspace",
		Name:          "demo",
		Mode:          model.ExecutionModePRDTasks,
		IDE:           model.IDECodex,
		AddDirs:       []string{" docs ", "docs", "", "src"},
	}

	cfg := fields.RuntimeConfig()
	if cfg.Concurrent != 1 {
		t.Fatalf("unexpected concurrent default: %d", cfg.Concurrent)
	}
	if cfg.BatchSize != 1 {
		t.Fatalf("unexpected batch size default: %d", cfg.BatchSize)
	}
	if cfg.Timeout != model.DefaultActivityTimeout {
		t.Fatalf("unexpected timeout default: %s", cfg.Timeout)
	}
	if len(cfg.AddDirs) != 2 || cfg.AddDirs[0] != "docs" || cfg.AddDirs[1] != "src" {
		t.Fatalf("unexpected add dirs: %#v", cfg.AddDirs)
	}
}

func assertRuntimeFields(t *testing.T, got RuntimeFields, want core.Config) {
	t.Helper()

	if got.WorkspaceRoot != want.WorkspaceRoot {
		t.Fatalf("unexpected workspace root: %q", got.WorkspaceRoot)
	}
	if got.Name != want.Name {
		t.Fatalf("unexpected name: %q", got.Name)
	}
	if got.Round != want.Round {
		t.Fatalf("unexpected round: %d", got.Round)
	}
	if got.Provider != want.Provider {
		t.Fatalf("unexpected provider: %q", got.Provider)
	}
	if got.PR != want.PR {
		t.Fatalf("unexpected pr: %q", got.PR)
	}
	if got.ReviewsDir != want.ReviewsDir {
		t.Fatalf("unexpected reviews dir: %q", got.ReviewsDir)
	}
	if got.TasksDir != want.TasksDir {
		t.Fatalf("unexpected tasks dir: %q", got.TasksDir)
	}
	if got.DryRun != want.DryRun {
		t.Fatalf("unexpected dry run: %v", got.DryRun)
	}
	if got.AutoCommit != want.AutoCommit {
		t.Fatalf("unexpected auto commit: %v", got.AutoCommit)
	}
	if got.Concurrent != want.Concurrent {
		t.Fatalf("unexpected concurrent: %d", got.Concurrent)
	}
	if got.BatchSize != want.BatchSize {
		t.Fatalf("unexpected batch size: %d", got.BatchSize)
	}
	if got.IDE != string(want.IDE) {
		t.Fatalf("unexpected ide: %q", got.IDE)
	}
	if got.Model != want.Model {
		t.Fatalf("unexpected model: %q", got.Model)
	}
	if len(got.AddDirs) != len(want.AddDirs) {
		t.Fatalf("unexpected add dir count: %d", len(got.AddDirs))
	}
	for idx := range want.AddDirs {
		if got.AddDirs[idx] != want.AddDirs[idx] {
			t.Fatalf("unexpected add dir %d: %q", idx, got.AddDirs[idx])
		}
	}
	if got.TailLines != want.TailLines {
		t.Fatalf("unexpected tail lines: %d", got.TailLines)
	}
	if got.ReasoningEffort != want.ReasoningEffort {
		t.Fatalf("unexpected reasoning effort: %q", got.ReasoningEffort)
	}
	if got.AccessMode != want.AccessMode {
		t.Fatalf("unexpected access mode: %q", got.AccessMode)
	}
	if got.Mode != model.ExecutionMode(want.Mode) {
		t.Fatalf("unexpected mode: %q", got.Mode)
	}
	if got.OutputFormat != model.OutputFormat(want.OutputFormat) {
		t.Fatalf("unexpected output format: %q", got.OutputFormat)
	}
	if got.Verbose != want.Verbose {
		t.Fatalf("unexpected verbose: %v", got.Verbose)
	}
	if got.TUI != want.TUI {
		t.Fatalf("unexpected tui: %v", got.TUI)
	}
	if got.Persist != want.Persist {
		t.Fatalf("unexpected persist: %v", got.Persist)
	}
	if got.RunID != want.RunID {
		t.Fatalf("unexpected run id: %q", got.RunID)
	}
	if got.PromptText != want.PromptText {
		t.Fatalf("unexpected prompt text: %q", got.PromptText)
	}
	if got.PromptFile != want.PromptFile {
		t.Fatalf("unexpected prompt file: %q", got.PromptFile)
	}
	if got.ReadPromptStdin != want.ReadPromptStdin {
		t.Fatalf("unexpected read prompt stdin: %v", got.ReadPromptStdin)
	}
	if got.ResolvedPromptText != want.ResolvedPromptText {
		t.Fatalf("unexpected resolved prompt text: %q", got.ResolvedPromptText)
	}
	if got.IncludeCompleted != want.IncludeCompleted {
		t.Fatalf("unexpected include completed: %v", got.IncludeCompleted)
	}
	if got.IncludeResolved != want.IncludeResolved {
		t.Fatalf("unexpected include resolved: %v", got.IncludeResolved)
	}
	if got.Timeout != want.Timeout {
		t.Fatalf("unexpected timeout: %s", got.Timeout)
	}
	if got.MaxRetries != want.MaxRetries {
		t.Fatalf("unexpected max retries: %d", got.MaxRetries)
	}
	if got.RetryBackoffMultiplier != want.RetryBackoffMultiplier {
		t.Fatalf("unexpected retry backoff multiplier: %f", got.RetryBackoffMultiplier)
	}
}

func testCoreConfig() core.Config {
	return core.Config{
		WorkspaceRoot:          "/workspace",
		Name:                   "demo",
		Round:                  7,
		Provider:               "coderabbit",
		PR:                     "259",
		ReviewsDir:             "/workspace/.compozy/tasks/demo/reviews-007",
		TasksDir:               "/workspace/.compozy/tasks/demo",
		DryRun:                 true,
		AutoCommit:             true,
		Concurrent:             2,
		BatchSize:              1,
		IDE:                    core.IDECodex,
		Model:                  "gpt-5.4",
		AddDirs:                []string{"docs", "src"},
		TailLines:              25,
		ReasoningEffort:        "high",
		AccessMode:             core.AccessModeFull,
		Mode:                   core.ModePRDTasks,
		OutputFormat:           core.OutputFormatText,
		Verbose:                true,
		TUI:                    true,
		Persist:                true,
		RunID:                  "run-123",
		PromptText:             "prompt text",
		PromptFile:             "prompt.md",
		ReadPromptStdin:        true,
		ResolvedPromptText:     "resolved prompt text",
		IncludeCompleted:       true,
		IncludeResolved:        true,
		Timeout:                90 * time.Second,
		MaxRetries:             4,
		RetryBackoffMultiplier: 2.5,
	}
}
