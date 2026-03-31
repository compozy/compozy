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

type commandKind string

const (
	commandKindFetchReviews commandKind = "fetch-reviews"
	commandKindFixReviews   commandKind = "fix-reviews"
	commandKindStart        commandKind = "start"
)

type commandState struct {
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
	useForm                bool
	includeCompleted       bool
	includeResolved        bool
	timeout                string
	maxRetries             int
	retryBackoffMultiplier float64
}

// NewRootCommand returns the reusable looper Cobra command.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "looper",
		Short:        "Run AI review remediation and PRD task workflows",
		SilenceUsage: true,
		Long: `Looper manages review rounds and PRD execution workflows.

Use explicit workflow subcommands:
  looper setup         Install bundled public skills for supported agents
  looper fetch-reviews Fetch provider review comments into tasks/<name>/reviews-NNN/
  looper fix-reviews   Process review issue files from a specific review round
  looper start         Execute PRD task files`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.AddCommand(newSetupCommand(), newFetchReviewsCommand(), newFixReviewsCommand(), newStartCommand())
	return root
}

func newFetchReviewsCommand() *cobra.Command {
	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	cmd := &cobra.Command{
		Use:          "fetch-reviews",
		Short:        "Fetch provider review comments into a PRD review round",
		SilenceUsage: true,
		Long:         "Fetch review comments from a provider and write them into tasks/<name>/reviews-NNN/.",
		Example: `  looper fetch-reviews --provider coderabbit --pr 259 --name my-feature
  looper fetch-reviews --provider coderabbit --pr 259 --name my-feature --round 2
  looper fetch-reviews --form`,
		RunE: state.fetchReviews,
	}

	cmd.Flags().StringVar(&state.provider, "provider", "", "Review provider name (for example: coderabbit)")
	cmd.Flags().StringVar(&state.pr, "pr", "", "Pull request number")
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number (default: next available round)")
	cmd.Flags().BoolVar(&state.useForm, "form", false, "Use interactive form to collect parameters")
	return cmd
}

func newFixReviewsCommand() *cobra.Command {
	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := &cobra.Command{
		Use:          "fix-reviews",
		Short:        "Process review issue files from a PRD review round",
		SilenceUsage: true,
		Long: `Process review issue markdown files from tasks/<name>/reviews-NNN/ and run the configured AI agent
to remediate review feedback.`,
		Example: `  looper fix-reviews --name my-feature --ide codex --concurrent 2 --batch-size 3 --grouped
  looper fix-reviews --name my-feature --round 2
  looper fix-reviews --reviews-dir tasks/my-feature/reviews-001`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state, commonFlagOptions{includeConcurrent: true})
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number (default: latest existing round)")
	cmd.Flags().
		StringVar(&state.reviewsDir, "reviews-dir", "", "Path to a review round directory (tasks/<name>/reviews-NNN)")
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
AI agent one task at a time.`,
		Example: `  looper start --name multi-repo --tasks-dir tasks/multi-repo --ide claude
  looper start --form --name multi-repo`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().StringVar(&state.name, "name", "", "Task workflow name (used for tasks/<name>)")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Path to tasks directory (tasks/<name>)")
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "Include completed tasks")
	return cmd
}

func newCommandState(kind commandKind, mode core.Mode) *commandState {
	return &commandState{
		kind: kind,
		mode: mode,
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

func (s *commandState) fetchReviews(cmd *cobra.Command, _ []string) error {
	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}

	cfg, err := s.buildConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
