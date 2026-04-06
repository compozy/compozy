package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/setup"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/spf13/cobra"
)

type commandKind string

const (
	commandKindFetchReviews commandKind = "fetch-reviews"
	commandKindFixReviews   commandKind = "fix-reviews"
	commandKindExec         commandKind = "exec"
	commandKindArchive      commandKind = "archive"
	commandKindStart        commandKind = "start"
	commandKindSync         commandKind = "sync"
)

type commandState struct {
	workspaceRoot          string
	projectConfig          workspace.ProjectConfig
	kind                   commandKind
	mode                   core.Mode
	pr                     string
	name                   string
	provider               string
	round                  int
	reviewsDir             string
	tasksDir               string
	dryRun                 bool
	autoCommit             bool
	concurrent             int
	batchSize              int
	ide                    string
	model                  string
	force                  bool
	skipValidation         bool
	addDirs                []string
	tailLines              int
	reasoningEffort        string
	accessMode             string
	outputFormat           string
	verbose                bool
	tui                    bool
	persist                bool
	runID                  string
	promptText             string
	promptFile             string
	readPromptStdin        bool
	resolvedPromptText     string
	includeCompleted       bool
	includeResolved        bool
	timeout                string
	maxRetries             int
	retryBackoffMultiplier float64
	isInteractive          func() bool
	collectForm            func(*cobra.Command, *commandState) error
	listBundledSkills      func() ([]setup.Skill, error)
	verifyBundledSkills    func(setup.VerifyConfig) (setup.VerifyResult, error)
	installBundledSkills   func(setup.InstallConfig) (*setup.Result, error)
	confirmSkillRefresh    func(*cobra.Command, skillRefreshPrompt) (bool, error)
	fetchReviewsFn         func(context.Context, core.Config) (*core.FetchResult, error)
	runWorkflow            func(context.Context, core.Config) error
}

type commandStateDefaults struct {
	isInteractive        func() bool
	collectForm          func(*cobra.Command, *commandState) error
	listBundledSkills    func() ([]setup.Skill, error)
	verifyBundledSkills  func(setup.VerifyConfig) (setup.VerifyResult, error)
	installBundledSkills func(setup.InstallConfig) (*setup.Result, error)
	confirmSkillRefresh  func(*cobra.Command, skillRefreshPrompt) (bool, error)
}

var validateRootDispatcher = kernel.ValidateDefaultRegistry

func defaultCommandStateDefaults() commandStateDefaults {
	return commandStateDefaults{
		isInteractive:        isInteractiveTerminal,
		collectForm:          collectFormParams,
		listBundledSkills:    setup.ListBundledSkills,
		verifyBundledSkills:  setup.VerifyBundledSkills,
		installBundledSkills: setup.InstallBundledSkills,
		confirmSkillRefresh:  confirmSkillRefreshPrompt,
	}
}

func (defaults commandStateDefaults) withFallbacks() commandStateDefaults {
	builtin := defaultCommandStateDefaults()
	if defaults.isInteractive == nil {
		defaults.isInteractive = builtin.isInteractive
	}
	if defaults.collectForm == nil {
		defaults.collectForm = builtin.collectForm
	}
	if defaults.listBundledSkills == nil {
		defaults.listBundledSkills = builtin.listBundledSkills
	}
	if defaults.verifyBundledSkills == nil {
		defaults.verifyBundledSkills = builtin.verifyBundledSkills
	}
	if defaults.installBundledSkills == nil {
		defaults.installBundledSkills = builtin.installBundledSkills
	}
	if defaults.confirmSkillRefresh == nil {
		defaults.confirmSkillRefresh = builtin.confirmSkillRefresh
	}
	return defaults
}

func newRootDispatcher() *kernel.Dispatcher {
	deps := kernel.KernelDeps{
		Logger:        slog.Default(),
		EventBus:      events.New[events.Event](0),
		Workspace:     bestEffortRootWorkspaceContext(),
		AgentRegistry: agent.DefaultRegistry(),
	}

	dispatcher := kernel.BuildDefault(deps)
	if err := validateRootDispatcher(dispatcher); err != nil {
		panic(err)
	}
	return dispatcher
}

func bestEffortRootWorkspaceContext() workspace.Context {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	workspaceCtx, err := resolveWorkspaceContext(ctx)
	if err != nil {
		return workspace.Context{}
	}
	return workspaceCtx
}

// NewRootCommand returns the reusable compozy Cobra command.
func NewRootCommand() *cobra.Command {
	return newRootCommandWithDefaults(newRootDispatcher(), defaultCommandStateDefaults())
}

func newRootCommandWithDefaults(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	root := &cobra.Command{
		Use:          "compozy",
		Short:        "Run AI review remediation and PRD task workflows",
		SilenceUsage: true,
		Long: `Compozy manages review rounds and PRD execution workflows.

Project-level defaults can be stored in .compozy/config.toml. Explicit CLI flags
always override values loaded from the workspace config.

Use explicit workflow subcommands:
  compozy setup         Install bundled public skills for supported agents
  compozy migrate       Convert legacy workflow artifacts to frontmatter
  compozy validate-tasks Validate task metadata under .compozy/tasks/<name>
  compozy sync          Refresh task workflow metadata files
  compozy archive       Move fully completed workflows into .compozy/tasks/_archived/
  compozy fetch-reviews Fetch provider review comments into .compozy/tasks/<name>/reviews-NNN/
  compozy fix-reviews   Process review issue files from a specific review round
  compozy exec          Execute one ad hoc prompt through the shared ACP runtime
  compozy start         Execute PRD task files`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(
		newSetupCommand(dispatcher),
		newMigrateCommand(dispatcher),
		newValidateTasksCommand(dispatcher),
		newSyncCommand(dispatcher),
		newArchiveCommand(dispatcher),
		newFetchReviewsCommandWithDefaults(dispatcher, defaults),
		newFixReviewsCommandWithDefaults(dispatcher, defaults),
		newExecCommandWithDefaults(dispatcher, defaults),
		newStartCommandWithDefaults(dispatcher, defaults),
	)
	return root
}

func newFetchReviewsCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	return newFetchReviewsCommandWithDefaults(dispatcher, defaultCommandStateDefaults())
}

func newFetchReviewsCommandWithDefaults(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindFetchReviews, core.ModePRReview, defaults)
	state.fetchReviewsFn = newFetchReviewsRunner(dispatcher)
	cmd := &cobra.Command{
		Use:          "fetch-reviews",
		Short:        "Fetch provider review comments into a PRD review round",
		SilenceUsage: true,
		Long: "Fetch review comments from a provider and write them into .compozy/tasks/<name>/reviews-NNN/.\n\n" +
			"When --provider is omitted, Compozy can load its default from .compozy/config.toml.",
		Example: `  compozy fetch-reviews --provider coderabbit --pr 259 --name my-feature
  compozy fetch-reviews --provider coderabbit --pr 259 --name my-feature --round 2
  compozy fetch-reviews`,
		RunE: state.fetchReviews,
	}

	cmd.Flags().StringVar(&state.provider, "provider", "", "Review provider name (for example: coderabbit)")
	cmd.Flags().StringVar(&state.pr, "pr", "", "Pull request number")
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number (default: next available round)")
	return cmd
}

func newFixReviewsCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	return newFixReviewsCommandWithDefaults(dispatcher, defaultCommandStateDefaults())
}

func newFixReviewsCommandWithDefaults(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindFixReviews, core.ModePRReview, defaults)
	state.runWorkflow = newRunWorkflow(dispatcher)
	cmd := &cobra.Command{
		Use:          "fix-reviews",
		Short:        "Process review issue files from a PRD review round",
		SilenceUsage: true,
		Long: `Process review issue markdown files from .compozy/tasks/<name>/reviews-NNN/ and run the configured AI agent
to remediate review feedback.

Most runtime defaults can be supplied by .compozy/config.toml.`,
		Example: `  compozy fix-reviews --name my-feature --ide codex --concurrent 2 --batch-size 3
  compozy fix-reviews --name my-feature --round 2
  compozy fix-reviews --reviews-dir .compozy/tasks/my-feature/reviews-001
  compozy fix-reviews`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state, commonFlagOptions{includeConcurrent: true})
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number (default: latest existing round)")
	cmd.Flags().
		StringVar(
			&state.reviewsDir,
			"reviews-dir",
			"",
			"Path to a review round directory (.compozy/tasks/<name>/reviews-NNN)",
		)
	cmd.Flags().
		IntVar(&state.batchSize, "batch-size", 1, "Number of file groups to batch together (default: 1 for no batching)")
	cmd.Flags().BoolVar(&state.includeResolved, "include-resolved", false, "Include already-resolved review issues")
	return cmd
}

func newStartCommand() *cobra.Command {
	return newStartCommandWithDefaults(nil, defaultCommandStateDefaults())
}

func newStartCommandWithDefaults(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindStart, core.ModePRDTasks, defaults)
	state.runWorkflow = newRunWorkflow(dispatcher)
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Execute PRD task files from a PRD directory",
		SilenceUsage: true,
		Long: `Execute task markdown files from a PRD workflow directory and dispatch them to the configured
AI agent one task at a time.

Most runtime defaults can be supplied by .compozy/config.toml.`,
		Example: `  compozy start --name multi-repo --tasks-dir .compozy/tasks/multi-repo --ide claude
  compozy start`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().StringVar(&state.name, "name", "", "Task workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Path to tasks directory (.compozy/tasks/<name>)")
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "Include completed tasks")
	cmd.Flags().BoolVar(
		&state.skipValidation,
		"skip-validation",
		false,
		"Skip task metadata preflight; use only when tasks were validated separately",
	)
	cmd.Flags().BoolVar(
		&state.force,
		"force",
		false,
		"Continue after task metadata validation fails in non-interactive mode",
	)
	return cmd
}

func newExecCommandWithDefaults(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindExec, core.ModeExec, defaults)
	state.runWorkflow = newRunWorkflow(dispatcher)
	cmd := &cobra.Command{
		Use:          "exec [prompt]",
		Short:        "Execute one ad hoc prompt through the shared ACP runtime",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Execute a single ad hoc prompt using the shared Compozy planning and ACP execution pipeline.

Provide the prompt as one positional argument, with --prompt-file, or via stdin. By default the
command is headless and ephemeral: text mode writes only the final assistant response to stdout and
json mode streams lean JSONL events to stdout, while raw-json preserves the full event stream.
Operational runtime logs stay silent unless you opt into --verbose. Use --tui to open the
interactive TUI and --persist to save resumable artifacts under
.compozy/runs/<run-id>/. Use --run-id to resume a previously persisted exec session.`,
		Example: `  compozy exec "Summarize the current repository changes"
  compozy exec --prompt-file prompt.md
  cat prompt.md | compozy exec --format json
  compozy exec --format raw-json "Inspect every streamed event"
  compozy exec --persist "Review the latest changes"
  compozy exec --run-id exec-20260405-120000-000000000 "Continue from the previous session"`,
		RunE: state.exec,
	}

	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().StringVar(&state.promptFile, "prompt-file", "", "Path to a file containing the prompt text")
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		string(core.OutputFormatText),
		"Output format: text, json, or raw-json",
	)
	cmd.Flags().BoolVar(&state.verbose, "verbose", false, "Emit operational runtime logs to stderr during exec")
	cmd.Flags().BoolVar(&state.tui, "tui", false, "Open the interactive TUI instead of using headless stdout output")
	cmd.Flags().BoolVar(&state.persist, "persist", false, "Persist exec artifacts under .compozy/runs/<run-id>/")
	cmd.Flags().StringVar(&state.runID, "run-id", "", "Resume a previously persisted exec session by run id")
	return cmd
}

type migrateCommandState struct {
	workspaceRoot string
	projectConfig workspace.ProjectConfig
	rootDir       string
	name          string
	tasksDir      string
	reviewsDir    string
	dryRun        bool
	migrateFn     func(context.Context, core.MigrationConfig) (*core.MigrationResult, error)
}

type syncCommandState struct {
	workspaceRoot string
	rootDir       string
	name          string
	tasksDir      string
	syncFn        func(context.Context, core.SyncConfig) (*core.SyncResult, error)
}

type archiveCommandState struct {
	workspaceRoot string
	rootDir       string
	name          string
	tasksDir      string
	archiveFn     func(context.Context, core.ArchiveConfig) (*core.ArchiveResult, error)
}

func newMigrateCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	state := &migrateCommandState{
		migrateFn: newMigrateRunner(dispatcher),
	}
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Migrate legacy workflow artifacts to frontmatter",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Convert legacy XML-tagged workflow artifacts under .compozy/tasks into Markdown frontmatter.

By default, the command scans the whole project workflow root recursively.`,
		Example: `  compozy migrate
  compozy migrate --dry-run
  compozy migrate --name my-feature
  compozy migrate --reviews-dir .compozy/tasks/my-feature/reviews-001`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.rootDir, "root-dir", "", "Workflow root to scan (default: .compozy/tasks)")
	cmd.Flags().StringVar(&state.name, "name", "", "Restrict migration to one workflow name under the workflow root")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Restrict migration to one task workflow directory")
	cmd.Flags().StringVar(&state.reviewsDir, "reviews-dir", "", "Restrict migration to one review round directory")
	cmd.Flags().BoolVar(&state.dryRun, "dry-run", false, "Plan migrations without writing files")
	return cmd
}

func newSyncCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	state := &syncCommandState{
		syncFn: newSyncRunner(dispatcher),
	}
	cmd := &cobra.Command{
		Use:          "sync",
		Short:        "Refresh task workflow metadata files",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Refresh task workflow _meta.md files under .compozy/tasks.

By default, the command scans the whole workflow root and creates missing task metadata files.`,
		Example: `  compozy sync
  compozy sync --name my-feature
  compozy sync --tasks-dir .compozy/tasks/my-feature`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.rootDir, "root-dir", "", "Workflow root to scan (default: .compozy/tasks)")
	cmd.Flags().StringVar(&state.name, "name", "", "Restrict sync to one workflow name under the workflow root")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Restrict sync to one task workflow directory")
	return cmd
}

func newArchiveCommand(dispatcher *kernel.Dispatcher) *cobra.Command {
	state := &archiveCommandState{
		archiveFn: newArchiveRunner(dispatcher),
	}
	cmd := &cobra.Command{
		Use:          "archive",
		Short:        "Move fully completed workflows into the archive root",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Archive fully completed workflows under .compozy/tasks by moving them into
.compozy/tasks/_archived/<timestamp>-<name>.

Eligible workflows must already have task _meta.md present, all task files completed, and all
review round _meta.md files fully resolved when review rounds exist.`,
		Example: `  compozy archive
  compozy archive --name my-feature
  compozy archive --tasks-dir .compozy/tasks/my-feature`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.rootDir, "root-dir", "", "Workflow root to scan (default: .compozy/tasks)")
	cmd.Flags().StringVar(&state.name, "name", "", "Restrict archiving to one workflow name under the workflow root")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Restrict archiving to one task workflow directory")
	return cmd
}

func newCommandState(kind commandKind, mode core.Mode) *commandState {
	return newCommandStateWithDefaults(kind, mode, defaultCommandStateDefaults())
}

func newCommandStateWithDefaults(kind commandKind, mode core.Mode, defaults commandStateDefaults) *commandState {
	defaults = defaults.withFallbacks()

	return &commandState{
		kind:                 kind,
		mode:                 mode,
		isInteractive:        defaults.isInteractive,
		collectForm:          defaults.collectForm,
		listBundledSkills:    defaults.listBundledSkills,
		verifyBundledSkills:  defaults.verifyBundledSkills,
		installBundledSkills: defaults.installBundledSkills,
		confirmSkillRefresh:  defaults.confirmSkillRefresh,
		fetchReviewsFn:       core.FetchReviews,
		runWorkflow:          core.Run,
	}
}

func newRunWorkflow(dispatcher *kernel.Dispatcher) func(context.Context, core.Config) error {
	if dispatcher == nil {
		return core.Run
	}

	return func(ctx context.Context, cfg core.Config) error {
		_, err := kernel.Dispatch[commands.RunStartCommand, commands.RunStartResult](
			ctx,
			dispatcher,
			commands.RunStartFromConfig(cfg),
		)
		return err
	}
}

func newFetchReviewsRunner(
	dispatcher *kernel.Dispatcher,
) func(context.Context, core.Config) (*core.FetchResult, error) {
	if dispatcher == nil {
		return core.FetchReviews
	}

	return func(ctx context.Context, cfg core.Config) (*core.FetchResult, error) {
		result, err := kernel.Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
			ctx,
			dispatcher,
			commands.ReviewsFetchFromConfig(cfg),
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

func newMigrateRunner(
	dispatcher *kernel.Dispatcher,
) func(context.Context, core.MigrationConfig) (*core.MigrationResult, error) {
	if dispatcher == nil {
		return core.Migrate
	}

	return func(ctx context.Context, cfg core.MigrationConfig) (*core.MigrationResult, error) {
		typedCommand := commands.WorkspaceMigrateFromConfig(core.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			Name:          cfg.Name,
			TasksDir:      cfg.TasksDir,
			ReviewsDir:    cfg.ReviewsDir,
			DryRun:        cfg.DryRun,
		})
		typedCommand.RootDir = cfg.RootDir

		result, err := kernel.Dispatch[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
			ctx,
			dispatcher,
			typedCommand,
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

func newSyncRunner(dispatcher *kernel.Dispatcher) func(context.Context, core.SyncConfig) (*core.SyncResult, error) {
	if dispatcher == nil {
		return core.Sync
	}

	return func(ctx context.Context, cfg core.SyncConfig) (*core.SyncResult, error) {
		typedCommand := commands.WorkflowSyncFromConfig(core.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			Name:          cfg.Name,
			TasksDir:      cfg.TasksDir,
		})
		typedCommand.RootDir = cfg.RootDir

		result, err := kernel.Dispatch[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
			ctx,
			dispatcher,
			typedCommand,
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

func newArchiveRunner(
	dispatcher *kernel.Dispatcher,
) func(context.Context, core.ArchiveConfig) (*core.ArchiveResult, error) {
	if dispatcher == nil {
		return core.Archive
	}

	return func(ctx context.Context, cfg core.ArchiveConfig) (*core.ArchiveResult, error) {
		typedCommand := commands.WorkflowArchiveFromConfig(core.Config{
			WorkspaceRoot: cfg.WorkspaceRoot,
			Name:          cfg.Name,
			TasksDir:      cfg.TasksDir,
		})
		typedCommand.RootDir = cfg.RootDir

		result, err := kernel.Dispatch[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
			ctx,
			dispatcher,
			typedCommand,
		)
		if err != nil {
			return nil, err
		}
		return result.Result, nil
	}
}

type commonFlagOptions struct {
	includeConcurrent bool
}

func addCommonFlags(cmd *cobra.Command, state *commandState, opts commonFlagOptions) {
	cmd.Flags().BoolVar(&state.dryRun, "dry-run", false, "Only generate prompts; do not run IDE tool")
	cmd.Flags().BoolVar(
		&state.autoCommit,
		"auto-commit",
		false,
		"Include automatic commit instructions at task/batch completion",
	)
	if opts.includeConcurrent {
		cmd.Flags().IntVar(&state.concurrent, "concurrent", 1, "Number of batches to process in parallel")
	}
	cmd.Flags().StringVar(
		&state.ide,
		"ide",
		string(core.IDECodex),
		"ACP runtime to use: claude, codex, copilot, cursor-agent, droid, gemini, opencode, or pi "+
			"(requires the matching ACP adapter, ACP-capable CLI, or supported launcher such as npx)",
	)
	cmd.Flags().StringVar(
		&state.model,
		"model",
		"",
		"Model to use (per-IDE defaults: codex/droid=gpt-5.4, claude=opus, copilot=claude-sonnet-4.6, "+
			"cursor-agent=composer-1, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro)",
	)
	cmd.Flags().StringSliceVar(
		&state.addDirs,
		"add-dir",
		nil,
		"Additional directory to allow for ACP runtimes that support extra writable roots "+
			"(currently claude and codex; repeatable or comma-separated)",
	)
	cmd.Flags().IntVar(
		&state.tailLines,
		"tail-lines",
		0,
		"Maximum number of log lines to retain in UI per job (0 = full history)",
	)
	cmd.Flags().StringVar(
		&state.reasoningEffort,
		"reasoning-effort",
		"medium",
		"Reasoning effort for runtimes that support bootstrap reasoning flags, such as droid (low, medium, high, xhigh)",
	)
	cmd.Flags().StringVar(
		&state.accessMode,
		"access-mode",
		core.AccessModeFull,
		"Runtime access policy: default keeps native safeguards; "+
			"full requests the most permissive mode Compozy can configure",
	)
	cmd.Flags().StringVar(
		&state.timeout,
		"timeout",
		"10m",
		"Activity timeout duration (e.g., 5m, 30s). Job canceled if no output received within this period.",
	)
	cmd.Flags().IntVar(
		&state.maxRetries,
		"max-retries",
		0,
		"Retry execution-stage ACP failures or timeouts up to N times before marking them failed",
	)
	cmd.Flags().Float64Var(
		&state.retryBackoffMultiplier,
		"retry-backoff-multiplier",
		1.5,
		"Multiplier applied to the next activity timeout after each retry",
	)
}

func (s *commandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return fmt.Errorf("apply workspace defaults for %s: %w", cmd.Name(), err)
	}
	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}

	cfg, err := s.buildConfig()
	if err != nil {
		return s.handleExecError(cmd, err)
	}
	if err := s.applyPersistedExecConfig(cmd, &cfg); err != nil {
		return s.handleExecError(cmd, err)
	}
	if err := cfg.Validate(); err != nil {
		return s.handleExecError(cmd, err)
	}

	if err := s.runPrepared(ctx, cmd, cfg); err != nil {
		return s.handleExecError(cmd, err)
	}
	return nil
}

func (s *commandState) exec(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return s.handleExecError(cmd, fmt.Errorf("apply workspace defaults for %s: %w", cmd.Name(), err))
	}
	if err := s.resolveExecPromptSource(cmd, args); err != nil {
		return s.handleExecError(cmd, err)
	}

	cfg, err := s.buildConfig()
	if err != nil {
		return s.handleExecError(cmd, err)
	}
	if err := s.applyPersistedExecConfig(cmd, &cfg); err != nil {
		return s.handleExecError(cmd, err)
	}
	if err := cfg.Validate(); err != nil {
		return s.handleExecError(cmd, err)
	}

	if err := s.runPrepared(ctx, cmd, cfg); err != nil {
		return s.handleExecError(cmd, err)
	}
	return nil
}

func (s *commandState) fetchReviews(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return fmt.Errorf("apply workspace defaults for %s: %w", cmd.Name(), err)
	}
	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}

	cfg, err := s.buildConfig()
	if err != nil {
		return err
	}

	fetchReviewsFn := s.fetchReviewsFn
	if fetchReviewsFn == nil {
		fetchReviewsFn = core.FetchReviews
	}

	result, err := fetchReviewsFn(ctx, cfg)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(
		cmd.OutOrStdout(),
		"Fetched %d review issues from %s for PR %s into %s (round %03d)\n",
		result.Total,
		result.Provider,
		result.PR,
		result.ReviewsDir,
		result.Round,
	)
	return nil
}

func (s *migrateCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	migrateFn := s.migrateFn
	if migrateFn == nil {
		migrateFn = core.Migrate
	}

	result, err := migrateFn(ctx, core.MigrationConfig{
		WorkspaceRoot: s.workspaceRoot,
		RootDir:       s.rootDir,
		Name:          s.name,
		TasksDir:      s.tasksDir,
		ReviewsDir:    s.reviewsDir,
		DryRun:        s.dryRun,
	})
	if result != nil {
		const summaryFormat = "Migrate target: %s\n" +
			"Dry run: %t\n" +
			"Scanned: %d\n" +
			"Migrated: %d\n" +
			"V1->V2 migrated: %d\n" +
			"Already frontmatter: %d\n" +
			"Skipped: %d\n" +
			"Invalid: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.DryRun,
			result.FilesScanned,
			result.FilesMigrated,
			result.V1ToV2Migrated,
			result.FilesAlreadyFrontmatter,
			result.FilesSkipped,
			result.FilesInvalid,
		)
		if len(result.UnmappedTypeFiles) > 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Unmapped type files: %d\n", len(result.UnmappedTypeFiles))
			for _, path := range result.UnmappedTypeFiles {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", path)
			}

			registry, regErr := taskTypeRegistryFromConfig(s.projectConfig)
			if regErr == nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nFix prompt:\n%s\n", migrationFixPrompt(result, registry))
			}
		}
	}
	return err
}

func migrationFixPrompt(result *core.MigrationResult, registry *tasks.TypeRegistry) string {
	report := tasks.Report{
		TasksDir: migrationTasksDir(result),
		Issues:   make([]tasks.Issue, 0, len(result.UnmappedTypeFiles)),
	}
	for _, path := range result.UnmappedTypeFiles {
		report.Issues = append(report.Issues, tasks.Issue{
			Path:    path,
			Field:   "type",
			Message: fmt.Sprintf(`type value is unmapped; must be one of: %s`, strings.Join(registry.Values(), ", ")),
		})
	}
	return tasks.FixPrompt(report, registry)
}

func migrationTasksDir(result *core.MigrationResult) string {
	if result == nil {
		return ""
	}
	if len(result.UnmappedTypeFiles) == 0 {
		return result.Target
	}
	return filepath.Dir(result.UnmappedTypeFiles[0])
}

func (s *syncCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	syncFn := s.syncFn
	if syncFn == nil {
		syncFn = core.Sync
	}

	result, err := syncFn(ctx, core.SyncConfig{
		WorkspaceRoot: s.workspaceRoot,
		RootDir:       s.rootDir,
		Name:          s.name,
		TasksDir:      s.tasksDir,
	})
	if result != nil {
		const summaryFormat = "Sync target: %s\n" +
			"Workflows scanned: %d\n" +
			"Meta created: %d\n" +
			"Meta updated: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.WorkflowsScanned,
			result.MetaCreated,
			result.MetaUpdated,
		)
	}
	return err
}

func (s *archiveCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	archiveFn := s.archiveFn
	if archiveFn == nil {
		archiveFn = core.Archive
	}

	result, err := archiveFn(ctx, core.ArchiveConfig{
		WorkspaceRoot: s.workspaceRoot,
		RootDir:       s.rootDir,
		Name:          s.name,
		TasksDir:      s.tasksDir,
	})
	if result != nil {
		const summaryFormat = "Archive target: %s\n" +
			"Archive root: %s\n" +
			"Workflows scanned: %d\n" +
			"Archived: %d\n" +
			"Skipped: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.ArchiveRoot,
			result.WorkflowsScanned,
			result.Archived,
			result.Skipped,
		)
	}
	return err
}

func (s *commandState) maybeCollectInteractiveParams(cmd *cobra.Command) error {
	if cmd.Flags().NFlag() > 0 {
		return nil
	}

	isInteractive := s.isInteractive
	if isInteractive == nil {
		isInteractive = isInteractiveTerminal
	}
	if !isInteractive() {
		return fmt.Errorf(
			"%s requires an interactive terminal when called without flags; pass flags explicitly",
			cmd.CommandPath(),
		)
	}

	collectForm := s.collectForm
	if collectForm == nil {
		collectForm = collectFormParams
	}
	if err := collectForm(cmd, s); err != nil {
		return fmt.Errorf("interactive form failed: %w", err)
	}
	return nil
}

func (s *commandState) buildConfig() (core.Config, error) {
	timeoutDuration := time.Duration(0)
	if s.timeout != "" {
		parsed, err := time.ParseDuration(s.timeout)
		if err != nil {
			return core.Config{}, fmt.Errorf("parse timeout: %w", err)
		}
		timeoutDuration = parsed
	}

	return core.Config{
		WorkspaceRoot:          s.workspaceRoot,
		Name:                   s.name,
		Round:                  s.round,
		Provider:               s.provider,
		PR:                     s.pr,
		ReviewsDir:             s.reviewsDir,
		TasksDir:               s.tasksDir,
		DryRun:                 s.dryRun,
		AutoCommit:             s.autoCommit,
		Concurrent:             s.concurrent,
		BatchSize:              s.batchSize,
		IDE:                    core.IDE(s.ide),
		Model:                  s.model,
		AddDirs:                core.NormalizeAddDirs(s.addDirs),
		TailLines:              s.tailLines,
		ReasoningEffort:        s.reasoningEffort,
		AccessMode:             s.accessMode,
		Mode:                   s.mode,
		OutputFormat:           core.OutputFormat(s.outputFormat),
		Verbose:                s.verbose,
		TUI:                    s.tui,
		Persist:                s.persist,
		RunID:                  s.runID,
		PromptText:             s.promptText,
		PromptFile:             s.promptFile,
		ReadPromptStdin:        s.readPromptStdin,
		ResolvedPromptText:     s.resolvedPromptText,
		IncludeCompleted:       s.includeCompleted,
		IncludeResolved:        s.includeResolved,
		Timeout:                timeoutDuration,
		MaxRetries:             s.maxRetries,
		RetryBackoffMultiplier: s.retryBackoffMultiplier,
	}, nil
}

func (s *commandState) applyPersistedExecConfig(cmd *cobra.Command, cfg *core.Config) error {
	if cfg == nil || strings.TrimSpace(s.runID) == "" {
		return nil
	}

	record, err := coreRun.LoadPersistedExecRun(s.workspaceRoot, s.runID)
	if err != nil {
		return err
	}
	cfg.Persist = true
	cfg.RunID = record.RunID
	if err := s.assertPersistedExecCompatibility(cmd, *cfg, record); err != nil {
		return err
	}

	cfg.WorkspaceRoot = record.WorkspaceRoot
	cfg.IDE = core.IDE(record.IDE)
	cfg.Model = record.Model
	cfg.ReasoningEffort = record.ReasoningEffort
	cfg.AccessMode = record.AccessMode
	cfg.AddDirs = core.NormalizeAddDirs(record.AddDirs)
	return nil
}

func (s *commandState) assertPersistedExecCompatibility(
	cmd *cobra.Command,
	cfg core.Config,
	record coreRun.PersistedExecRun,
) error {
	if cmd.Flags().Changed("ide") && string(cfg.IDE) != record.IDE {
		return fmt.Errorf("--run-id %q must continue with persisted --ide %q", record.RunID, record.IDE)
	}
	if cmd.Flags().Changed("model") && cfg.Model != record.Model {
		return fmt.Errorf("--run-id %q must continue with persisted --model %q", record.RunID, record.Model)
	}
	if cmd.Flags().Changed("reasoning-effort") && cfg.ReasoningEffort != record.ReasoningEffort {
		return fmt.Errorf(
			"--run-id %q must continue with persisted --reasoning-effort %q",
			record.RunID,
			record.ReasoningEffort,
		)
	}
	if cmd.Flags().Changed("access-mode") && cfg.AccessMode != record.AccessMode {
		return fmt.Errorf("--run-id %q must continue with persisted --access-mode %q", record.RunID, record.AccessMode)
	}
	if cmd.Flags().Changed("add-dir") &&
		!slices.Equal(core.NormalizeAddDirs(cfg.AddDirs), core.NormalizeAddDirs(record.AddDirs)) {
		return fmt.Errorf("--run-id %q must continue with persisted --add-dir values", record.RunID)
	}
	return nil
}

func (s *commandState) handleExecError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	if isExecJSONOutputFormatFlag(s.outputFormat) && !coreRun.IsExecErrorReported(err) {
		cmd.SilenceErrors = true
		if root := cmd.Root(); root != nil {
			root.SilenceErrors = true
		}
		if emitErr := coreRun.WriteExecJSONFailure(cmd.OutOrStdout(), strings.TrimSpace(s.runID), err); emitErr != nil {
			return errors.Join(err, emitErr)
		}
	}
	return err
}

func (s *commandState) resolveExecPromptSource(cmd *cobra.Command, args []string) error {
	s.promptText = ""
	s.readPromptStdin = false
	s.resolvedPromptText = ""

	positionalPrompt := ""
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		positionalPrompt = args[0]
	}
	promptFile := strings.TrimSpace(s.promptFile)

	sourceCount := 0
	if positionalPrompt != "" {
		sourceCount++
	}
	if promptFile != "" {
		sourceCount++
	}

	if sourceCount > 1 {
		return fmt.Errorf(
			"%s accepts only one prompt source at a time: positional prompt, --prompt-file, or stdin",
			cmd.CommandPath(),
		)
	}

	switch {
	case positionalPrompt != "":
		s.promptText = positionalPrompt
		s.resolvedPromptText = positionalPrompt
		return nil
	case promptFile != "":
		content, err := os.ReadFile(promptFile)
		if err != nil {
			return fmt.Errorf("read prompt file %s: %w", promptFile, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			return fmt.Errorf("prompt file %s is empty", promptFile)
		}
		s.promptFile = promptFile
		s.resolvedPromptText = string(content)
		return nil
	default:
		stdinPrompt, hasStdinPrompt, err := readPromptFromCommandInput(cmd.InOrStdin())
		if err != nil {
			return err
		}
		if !hasStdinPrompt {
			return fmt.Errorf(
				"%s requires exactly one prompt source: positional prompt, --prompt-file, or non-empty stdin",
				cmd.CommandPath(),
			)
		}
		s.readPromptStdin = true
		s.resolvedPromptText = stdinPrompt
		return nil
	}
}

func readPromptFromCommandInput(reader io.Reader) (string, bool, error) {
	if reader == nil {
		return "", false, nil
	}

	if file, ok := reader.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return "", false, fmt.Errorf("inspect stdin: %w", err)
		}
		if info.Mode()&os.ModeCharDevice != 0 {
			return "", false, nil
		}
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return "", false, fmt.Errorf("read stdin prompt: %w", err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return "", false, nil
	}
	return string(content), true, nil
}

func (s *commandState) runPrepared(ctx context.Context, cmd *cobra.Command, cfg core.Config) error {
	if err := s.preflightBundledSkills(cmd, cfg); err != nil {
		return err
	}
	if err := s.preflightTaskMetadata(ctx, cmd, cfg); err != nil {
		return err
	}

	runWorkflow := s.runWorkflow
	if runWorkflow == nil {
		runWorkflow = core.Run
	}
	return runWorkflow(ctx, cfg)
}

func (s *commandState) preflightTaskMetadata(ctx context.Context, cmd *cobra.Command, cfg core.Config) error {
	if s.kind != commandKindStart || cfg.Mode != core.ModePRDTasks {
		return nil
	}

	preflightCfg := coreRun.PreflightConfig{
		Force:          s.force,
		SkipValidation: s.skipValidation,
		IsInteractive:  s.isInteractive,
		Stderr:         cmd.ErrOrStderr(),
		Logger:         slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), nil)),
	}
	if !s.skipValidation {
		registry, err := taskTypeRegistryFromConfig(s.projectConfig)
		if err != nil {
			return fmt.Errorf("resolve task type registry: %w", err)
		}
		resolvedTasksDir, err := resolveTaskWorkflowDir(s.workspaceRoot, cfg.Name, cfg.TasksDir)
		if err != nil {
			return err
		}
		preflightCfg.TasksDir = resolvedTasksDir
		preflightCfg.Registry = registry
	}

	decision, err := coreRun.PreflightCheckConfig(ctx, preflightCfg)
	if err != nil {
		return err
	}
	if decision == coreRun.PreflightAborted {
		return withExitCode(1, fmt.Errorf("task validation failed"))
	}
	return nil
}

func isExecJSONOutputFormatFlag(value string) bool {
	switch strings.TrimSpace(value) {
	case string(core.OutputFormatJSON), string(core.OutputFormatRawJSON):
		return true
	default:
		return false
	}
}
