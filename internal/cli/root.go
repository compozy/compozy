package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	core "github.com/compozy/looper/internal/looper"
	"github.com/spf13/cobra"
)

var setupFlagsOnce sync.Once

// NewRootCommand returns the reusable looper Cobra command.
func NewRootCommand() *cobra.Command {
	setupFlagsOnce.Do(setupFlags)
	return rootCmd
}

var rootCmd = &cobra.Command{
	Use:   "looper",
	Short: "Run AI issue and PRD task loops from markdown inputs",
	Long: `Looper processes PR review issue markdown files and PRD task markdown files.

Usage:
  # Interactive mode (recommended for beginners):
  looper --form

  # Traditional CLI mode:
  looper --pr 259
  [--issues-dir ai-docs/<num>/issues] [--dry-run]
  [--concurrent 4] [--batch-size 3] [--ide claude|codex|droid] [--model gpt-5.4]
  [--add-dir ../shared] [--tail-lines 5] [--reasoning-effort medium] [--grouped]

  # Hybrid mode (mix flags with form):
  looper --form --pr 259 --ide codex

Interactive Form (--form):
- Beautiful terminal UI for parameter collection
- Smart field detection (only asks for unset parameters)
- Real-time input validation with helpful errors
- Mix CLI flags with interactive prompts

Behavior:
- Scans issue markdown files under the issues dir, groups by the "**File:** path:line header.
- Optionally writes grouped summaries to issues/grouped/<safe>.md (with --grouped flag).
- Generates prompts to .tmp/codex-prompts/pr-<PR>/.
- Batches multiple file groups together (controlled by --batch-size) for processing.
- Invokes the specified IDE tool (codex, claude, or droid) once per batch, feeding the generated prompt via stdin.
- By default, only writes process output to log files; does not stream to current stdout/stderr.
- Supports parallel execution with --concurrent N (default 1).
- Pass extra writable directories to Codex or Claude with repeated --add-dir values.
- Configure log tail lines shown in UI with --tail-lines (default: 5).
- Configure reasoning effort for codex/claude/droid with --reasoning-effort
  (default: medium, options: low/medium/high/xhigh).`,
	RunE: runSolveIssues,
}

var (
	pr                     string
	issuesDir              string
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
	mode                   string
	includeCompleted       bool
	timeout                string
	maxRetries             int
	retryBackoffMultiplier float64
)

func setupFlags() {
	rootCmd.Flags().StringVar(&pr, "pr", "", "Pull request number")
	rootCmd.Flags().StringVar(&issuesDir, "issues-dir", "", "Path to issues directory (ai-docs/reviews-pr-<PR>/issues)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only generate prompts; do not run IDE tool")
	rootCmd.Flags().BoolVar(
		&autoCommit,
		"auto-commit",
		false,
		"Include automatic commit instructions at task/batch completion",
	)
	rootCmd.Flags().IntVar(&concurrent, "concurrent", 1, "Number of batches to process in parallel")
	rootCmd.Flags().
		IntVar(&batchSize, "batch-size", 1, "Number of file groups to batch together (default: 1 for no batching)")
	rootCmd.Flags().StringVar(&ide, "ide", string(core.IDECodex), "IDE tool to use: claude, codex, cursor, or droid")
	rootCmd.Flags().StringVar(
		&model,
		"model",
		"",
		"Model to use (default: gpt-5.4 for codex/droid, opus for claude, composer-1 for cursor)",
	)
	rootCmd.Flags().StringSliceVar(
		&addDirs,
		"add-dir",
		nil,
		"Additional directory to allow for Codex and Claude (repeatable or comma-separated)",
	)
	rootCmd.Flags().BoolVar(&grouped, "grouped", false, "Generate grouped issue summaries in issues/grouped/ directory")
	rootCmd.Flags().IntVar(&tailLines, "tail-lines", 30, "Number of log lines to show in UI for each job")
	rootCmd.Flags().StringVar(
		&reasoningEffort,
		"reasoning-effort",
		"medium",
		"Reasoning effort for codex/claude/droid (low, medium, high, xhigh)",
	)
	rootCmd.Flags().BoolVar(&useForm, "form", false, "Use interactive form to collect parameters")
	rootCmd.Flags().StringVar(
		&mode, "mode", string(core.ModePRReview),
		"Execution mode: pr-review (CodeRabbit issues) or prd-tasks (PRD task files)",
	)
	rootCmd.Flags().
		BoolVar(&includeCompleted, "include-completed", false, "Include completed tasks (only applies to prd-tasks mode)")
	rootCmd.Flags().StringVar(
		&timeout,
		"timeout",
		"10m",
		"Activity timeout duration (e.g., 5m, 30s). Job canceled if no output received within this period.",
	)
	rootCmd.Flags().IntVar(
		&maxRetries,
		"max-retries",
		0,
		"Retry failed or timed-out jobs up to N times before marking them failed",
	)
	rootCmd.Flags().Float64Var(
		&retryBackoffMultiplier,
		"retry-backoff-multiplier",
		1.5,
		"Multiplier applied to activity timeout after each retry",
	)
}

func runSolveIssues(cmd *cobra.Command, _ []string) error {
	if err := maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}
	if err := ensurePRProvided(); err != nil {
		return err
	}

	cfg, err := buildConfig()
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

func maybeCollectInteractiveParams(cmd *cobra.Command) error {
	if !useForm {
		return nil
	}
	if err := collectFormParams(cmd); err != nil {
		return fmt.Errorf("interactive form failed: %w", err)
	}
	return nil
}

func ensurePRProvided() error {
	if pr != "" {
		return nil
	}
	return errors.New("PR number is required (use --pr or --form)")
}

func buildConfig() (core.Config, error) {
	timeoutDuration := time.Duration(0)
	if timeout != "" {
		parsed, err := time.ParseDuration(timeout)
		if err != nil {
			return core.Config{}, fmt.Errorf("parse timeout: %w", err)
		}
		timeoutDuration = parsed
	}

	return core.Config{
		PR:                     pr,
		IssuesDir:              issuesDir,
		DryRun:                 dryRun,
		AutoCommit:             autoCommit,
		Concurrent:             concurrent,
		BatchSize:              batchSize,
		IDE:                    core.IDE(ide),
		Model:                  model,
		AddDirs:                core.NormalizeAddDirs(addDirs),
		Grouped:                grouped,
		TailLines:              tailLines,
		ReasoningEffort:        reasoningEffort,
		Mode:                   core.Mode(mode),
		IncludeCompleted:       includeCompleted,
		Timeout:                timeoutDuration,
		MaxRetries:             maxRetries,
		RetryBackoffMultiplier: retryBackoffMultiplier,
	}, nil
}

func buildCLIArgs() core.Config {
	timeoutDuration := time.Duration(0)
	if timeout != "" {
		if parsed, err := time.ParseDuration(timeout); err == nil {
			timeoutDuration = parsed
		}
	}
	return core.Config{
		PR:                     pr,
		IssuesDir:              issuesDir,
		DryRun:                 dryRun,
		AutoCommit:             autoCommit,
		Concurrent:             concurrent,
		BatchSize:              batchSize,
		IDE:                    core.IDE(ide),
		Model:                  model,
		AddDirs:                core.NormalizeAddDirs(addDirs),
		Grouped:                grouped,
		TailLines:              tailLines,
		ReasoningEffort:        reasoningEffort,
		Mode:                   core.Mode(mode),
		IncludeCompleted:       includeCompleted,
		Timeout:                timeoutDuration,
		MaxRetries:             maxRetries,
		RetryBackoffMultiplier: retryBackoffMultiplier,
	}
}
