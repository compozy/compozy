package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type commandKind string

const (
	commandKindFetchReviews commandKind = "fetch-reviews"
	commandKindFixReviews   commandKind = "fix-reviews"
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
	addDirs                []string
	grouped                bool
	tailLines              int
	reasoningEffort        string
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
	runWorkflow            func(context.Context, core.Config) error
}

// NewRootCommand returns the reusable compozy Cobra command.
func NewRootCommand() *cobra.Command {
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
  compozy sync          Refresh task workflow metadata files
  compozy archive       Move fully completed workflows into .compozy/tasks/_archived/
  compozy fetch-reviews Fetch provider review comments into .compozy/tasks/<name>/reviews-NNN/
  compozy fix-reviews   Process review issue files from a specific review round
  compozy start         Execute PRD task files`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(
		newSetupCommand(),
		newMigrateCommand(),
		newSyncCommand(),
		newArchiveCommand(),
		newFetchReviewsCommand(),
		newFixReviewsCommand(),
		newStartCommand(),
	)
	return root
}

func newFetchReviewsCommand() *cobra.Command {
	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
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

func newFixReviewsCommand() *cobra.Command {
	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := &cobra.Command{
		Use:          "fix-reviews",
		Short:        "Process review issue files from a PRD review round",
		SilenceUsage: true,
		Long: `Process review issue markdown files from .compozy/tasks/<name>/reviews-NNN/ and run the configured AI agent
to remediate review feedback.

Most runtime defaults can be supplied by .compozy/config.toml.`,
		Example: `  compozy fix-reviews --name my-feature --ide codex --concurrent 2 --batch-size 3 --grouped
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
	cmd.Flags().BoolVar(&state.grouped, "grouped", false, "Generate grouped issue summaries in reviews-NNN/grouped/")
	cmd.Flags().BoolVar(&state.includeResolved, "include-resolved", false, "Include already-resolved review issues")
	return cmd
}

func newStartCommand() *cobra.Command {
	state := newCommandState(commandKindStart, core.ModePRDTasks)
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
	return cmd
}

type migrateCommandState struct {
	workspaceRoot string
	rootDir       string
	name          string
	tasksDir      string
	reviewsDir    string
	dryRun        bool
}

type syncCommandState struct {
	workspaceRoot string
	rootDir       string
	name          string
	tasksDir      string
}

type archiveCommandState struct {
	workspaceRoot string
	rootDir       string
	name          string
	tasksDir      string
}

func newMigrateCommand() *cobra.Command {
	state := &migrateCommandState{}
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

func newSyncCommand() *cobra.Command {
	state := &syncCommandState{}
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

func newArchiveCommand() *cobra.Command {
	state := &archiveCommandState{}
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
	return &commandState{
		kind:                 kind,
		mode:                 mode,
		isInteractive:        isInteractiveTerminal,
		collectForm:          collectFormParams,
		listBundledSkills:    setup.ListBundledSkills,
		verifyBundledSkills:  setup.VerifyBundledSkills,
		installBundledSkills: setup.InstallBundledSkills,
		confirmSkillRefresh:  confirmSkillRefreshPrompt,
		runWorkflow:          core.Run,
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
		"ACP runtime to use: claude, codex, cursor-agent, droid, opencode, pi, or gemini "+
			"(requires the matching ACP adapter, ACP-capable CLI, or supported launcher such as npx)",
	)
	cmd.Flags().StringVar(
		&state.model,
		"model",
		"",
		"Model to use (per-IDE defaults: codex/droid=gpt-5.4, claude=opus, "+
			"cursor-agent=composer-1, opencode/pi=anthropic/claude-opus-4-6, gemini=gemini-2.5-pro)",
	)
	cmd.Flags().StringSliceVar(
		&state.addDirs,
		"add-dir",
		nil,
		"Additional directory to allow for ACP runtimes that support extra writable roots (repeatable or comma-separated)",
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
		&state.timeout,
		"timeout",
		"10m",
		"Activity timeout duration (e.g., 5m, 30s). Job canceled if no output received within this period.",
	)
	cmd.Flags().IntVar(
		&state.maxRetries,
		"max-retries",
		0,
		"Retry failed or timed-out jobs up to N times before marking them failed",
	)
	cmd.Flags().Float64Var(
		&state.retryBackoffMultiplier,
		"retry-backoff-multiplier",
		1.5,
		"Multiplier applied to activity timeout after each retry",
	)
}

func (s *commandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
	if err := cfg.Validate(); err != nil {
		return err
	}

	return s.runPrepared(ctx, cmd, cfg)
}

func (s *commandState) fetchReviews(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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

	result, err := core.FetchReviews(ctx, cfg)
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	result, err := core.Migrate(ctx, core.MigrationConfig{
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
			"Already frontmatter: %d\n" +
			"Skipped: %d\n" +
			"Invalid: %d\n" +
			"Grouped regenerated: %d\n"
		_, _ = fmt.Fprintf(
			cmd.OutOrStdout(),
			summaryFormat,
			result.Target,
			result.DryRun,
			result.FilesScanned,
			result.FilesMigrated,
			result.FilesAlreadyFrontmatter,
			result.FilesSkipped,
			result.FilesInvalid,
			result.GroupedRegenerated,
		)
	}
	return err
}

func (s *syncCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	result, err := core.Sync(ctx, core.SyncConfig{
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return fmt.Errorf("load workspace root for %s: %w", cmd.Name(), err)
	}

	result, err := core.Archive(ctx, core.ArchiveConfig{
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
		Grouped:                s.grouped,
		TailLines:              s.tailLines,
		ReasoningEffort:        s.reasoningEffort,
		Mode:                   s.mode,
		IncludeCompleted:       s.includeCompleted,
		IncludeResolved:        s.includeResolved,
		Timeout:                timeoutDuration,
		MaxRetries:             s.maxRetries,
		RetryBackoffMultiplier: s.retryBackoffMultiplier,
	}, nil
}

func (s *commandState) runPrepared(ctx context.Context, cmd *cobra.Command, cfg core.Config) error {
	if err := s.preflightBundledSkills(cmd, cfg); err != nil {
		return err
	}

	runWorkflow := s.runWorkflow
	if runWorkflow == nil {
		runWorkflow = core.Run
	}
	return runWorkflow(ctx, cfg)
}
