package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	core "github.com/compozy/looper/internal/looper"
	"github.com/spf13/cobra"
)

type commandState struct {
	mode                   core.Mode
	pr                     string
	name                   string
	issuesDir              string
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
	useForm                bool
	includeCompleted       bool
	timeout                string
	maxRetries             int
	retryBackoffMultiplier float64
}

// NewRootCommand returns the reusable looper Cobra command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "looper",
		Short:        "Run AI issue and PRD task loops from markdown inputs",
		SilenceUsage: true,
		Long: `Looper processes PR review issue markdown files and PRD task markdown files.

Use explicit workflow subcommands:
  looper fix-reviews   Process CodeRabbit review issues
  looper start         Execute PRD task files`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(newFixReviewsCommand(), newStartCommand())
	return root
}

func newFixReviewsCommand() *cobra.Command {
	state := newCommandState(core.ModePRReview)
	cmd := &cobra.Command{
		Use:          "fix-reviews",
		Short:        "Process PR review issues from markdown inputs",
		SilenceUsage: true,
		Long: `Process CodeRabbit review issue markdown files, batch them, and run the configured AI agent
to remediate review feedback.`,
		Example: `  looper fix-reviews --pr 259 --ide codex --concurrent 2 --batch-size 3 --grouped
  looper fix-reviews --form --pr 259`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state)
	cmd.Flags().StringVar(&state.pr, "pr", "", "Pull request number")
	cmd.Flags().StringVar(
		&state.issuesDir,
		"issues-dir",
		"",
		"Path to issues directory (ai-docs/reviews-pr-<PR>/issues)",
	)
	cmd.Flags().
		IntVar(&state.batchSize, "batch-size", 1, "Number of file groups to batch together (default: 1 for no batching)")
	cmd.Flags().BoolVar(
		&state.grouped,
		"grouped",
		false,
		"Generate grouped issue summaries in issues/grouped/ directory",
	)

	return cmd
}

func newStartCommand() *cobra.Command {
	state := newCommandState(core.ModePRDTasks)
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Execute PRD task files from a PRD directory",
		SilenceUsage: true,
		Long: `Execute task markdown files from a PRD workflow directory and dispatch them to the configured
AI agent one task at a time.`,
		Example: `  looper start --name multi-repo --tasks-dir tasks/prd-multi-repo --ide claude
  looper start --form --name multi-repo`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state)
	cmd.Flags().StringVar(&state.name, "name", "", "PRD task workflow name (used for tasks/prd-<name>)")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Path to PRD tasks directory (tasks/prd-<name>)")
	cmd.Flags().BoolVar(
		&state.includeCompleted,
		"include-completed",
		false,
		"Include completed tasks",
	)

	return cmd
}

func newCommandState(mode core.Mode) *commandState {
	return &commandState{mode: mode}
}

func addCommonFlags(cmd *cobra.Command, state *commandState) {
	cmd.Flags().BoolVar(&state.dryRun, "dry-run", false, "Only generate prompts; do not run IDE tool")
	cmd.Flags().BoolVar(
		&state.autoCommit,
		"auto-commit",
		false,
		"Include automatic commit instructions at task/batch completion",
	)
	cmd.Flags().IntVar(&state.concurrent, "concurrent", 1, "Number of batches to process in parallel")
	cmd.Flags().StringVar(
		&state.ide,
		"ide",
		string(core.IDECodex),
		"IDE tool to use: claude, codex, cursor, or droid",
	)
	cmd.Flags().StringVar(
		&state.model,
		"model",
		"",
		"Model to use (default: gpt-5.4 for codex/droid, opus for claude, composer-1 for cursor)",
	)
	cmd.Flags().StringSliceVar(
		&state.addDirs,
		"add-dir",
		nil,
		"Additional directory to allow for Codex and Claude (repeatable or comma-separated)",
	)
	cmd.Flags().IntVar(&state.tailLines, "tail-lines", 30, "Number of log lines to show in UI for each job")
	cmd.Flags().StringVar(
		&state.reasoningEffort,
		"reasoning-effort",
		"medium",
		"Reasoning effort for codex/claude/droid (low, medium, high, xhigh)",
	)
	cmd.Flags().BoolVar(&state.useForm, "form", false, "Use interactive form to collect parameters")
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return core.Run(ctx, cfg)
}

func (s *commandState) maybeCollectInteractiveParams(cmd *cobra.Command) error {
	if !s.useForm {
		return nil
	}
	if err := collectFormParams(cmd, s); err != nil {
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
		PR:                     s.identifierValue(),
		IssuesDir:              s.inputDirValue(),
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
		Timeout:                timeoutDuration,
		MaxRetries:             s.maxRetries,
		RetryBackoffMultiplier: s.retryBackoffMultiplier,
	}, nil
}

func (s *commandState) isTaskWorkflow() bool {
	return s.mode == core.ModePRDTasks
}

func (s *commandState) identifierFlagName() string {
	if s.isTaskWorkflow() {
		return "name"
	}
	return "pr"
}

func (s *commandState) inputDirFlagName() string {
	if s.isTaskWorkflow() {
		return "tasks-dir"
	}
	return "issues-dir"
}

func (s *commandState) identifierTitle() string {
	if s.isTaskWorkflow() {
		return "Task Name"
	}
	return "PR Number"
}

func (s *commandState) identifierPlaceholder() string {
	if s.isTaskWorkflow() {
		return "multi-repo"
	}
	return "259"
}

func (s *commandState) identifierDescription() string {
	if s.isTaskWorkflow() {
		return "Required: PRD workflow name (e.g., 'multi-repo' for tasks/prd-multi-repo)"
	}
	return "Required: Pull request number or identifier to process"
}

func (s *commandState) identifierRequiredMessage() string {
	if s.isTaskWorkflow() {
		return "Task name is required"
	}
	return "PR number is required"
}

func (s *commandState) inputDirTitle() string {
	if s.isTaskWorkflow() {
		return "Tasks Directory (optional)"
	}
	return "Issues Directory (optional)"
}

func (s *commandState) inputDirPlaceholder() string {
	if s.isTaskWorkflow() {
		return "tasks/prd-<name>"
	}
	return "ai-docs/reviews-pr-<PR>/issues"
}

func (s *commandState) inputDirDescription() string {
	if s.isTaskWorkflow() {
		return "Leave empty to auto-generate from task name"
	}
	return "Leave empty to auto-generate from PR number"
}

func (s *commandState) identifierValue() string {
	if s.isTaskWorkflow() {
		return s.name
	}
	return s.pr
}

func (s *commandState) setIdentifierValue(value string) {
	if s.isTaskWorkflow() {
		s.name = value
		return
	}
	s.pr = value
}

func (s *commandState) inputDirValue() string {
	if s.isTaskWorkflow() {
		return s.tasksDir
	}
	return s.issuesDir
}

func (s *commandState) setInputDirValue(value string) {
	if s.isTaskWorkflow() {
		s.tasksDir = value
		return
	}
	s.issuesDir = value
}
