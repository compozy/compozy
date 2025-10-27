package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/sethvargo/go-retry"
	"github.com/spf13/cobra"
	"github.com/tidwall/pretty"
)

const (
	unknownFileName            = "unknown"
	ideCodex                   = "codex"
	ideClaude                  = "claude"
	ideDroid                   = "droid"
	defaultCodexModel          = "gpt-5-codex"
	defaultClaudeModel         = "claude-sonnet-4-5-20250929"
	thinkPromptMedium          = "Think hard through problems carefully before acting. Balance speed with thoroughness."
	thinkPromptLow             = "Think concisely and act quickly. Prefer direct solutions."
	thinkPromptHighDescription = "Ultrathink deeply and comprehensively before taking action. " +
		"Consider edge cases, alternatives, and long-term implications. Show your reasoning process."
	modeCodeReview = "pr-review"
	modePRDTasks   = "prd-tasks"
)

type executionMode string

const (
	ExecutionModePRReview executionMode = modeCodeReview
	ExecutionModePRDTasks executionMode = modePRDTasks
)

// Port of scripts/solve-pr-issues.ts with concurrency and non-streamed logging.
//
// Usage:
//   go run scripts/solve-pr-issues.go --pr 259
//   [--issues-dir ai-docs/<num>/issues] [--dry-run]
//   [--concurrent 4] [--batch-size 3] [--ide claude|codex|droid] [--model gpt-5-codex]
//   [--tail-lines 5] [--reasoning-effort medium] [--grouped]
//
// Behavior:
// - Scans issue markdown files under the issues dir, groups by the "**File:**`path:line`" header.
// - Optionally writes grouped summaries to issues/grouped/<safe>.md (with --grouped flag).
// - Generates prompts to .tmp/codex-prompts/pr-<PR>/.
// - Batches multiple file groups together (controlled by --batch-size) for processing.
// - Invokes the specified IDE tool (codex, claude, or droid) once per batch, feeding the generated prompt via stdin.
// - By default, only writes process output to log files; does not stream to current stdout/stderr.
// - Supports parallel execution with --concurrent N (default 1).
// - Configure log tail lines shown in UI with --tail-lines (default: 5).
// - Configure reasoning effort for codex/claude/droid with --reasoning-effort
//   (default: medium, options: low/medium/high).

type cliArgs struct {
	pr                     string
	issuesDir              string
	dryRun                 bool
	concurrent             int
	batchSize              int
	ide                    string
	model                  string
	grouped                bool
	tailLines              int
	reasoningEffort        string
	mode                   string
	includeCompleted       bool
	timeout                time.Duration
	maxRetries             int
	retryBackoffMultiplier float64
}

type issueEntry struct {
	name     string
	absPath  string
	content  string
	codeFile string // repository-relative code file or "__unknown__:<filename>"
}

type taskEntry struct {
	content      string
	status       string
	domain       string
	taskType     string
	scope        string
	complexity   string
	dependencies []string
}

func main() {
	setupFlags()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "solve-pr-issues",
	Short: "Solve PR issues by processing issue files and running IDE tools",
	Long: `Port of scripts/solve-pr-issues.ts with concurrency and non-streamed logging.

Usage:
  # Interactive mode (recommended for beginners):
  solve-pr-issues --form

  # Traditional CLI mode:
  solve-pr-issues --pr 259
  [--issues-dir ai-docs/<num>/issues] [--dry-run]
  [--concurrent 4] [--batch-size 3] [--ide claude|codex|droid] [--model gpt-5-codex]
  [--tail-lines 5] [--reasoning-effort medium] [--grouped]

  # Hybrid mode (mix flags with form):
  solve-pr-issues --form --pr 259 --ide codex

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
- Configure log tail lines shown in UI with --tail-lines (default: 5).
- Configure reasoning effort for codex/claude/droid with --reasoning-effort
  (default: medium, options: low/medium/high).`,
	RunE: runSolveIssues,
}

var (
	pr                     string
	issuesDir              string
	dryRun                 bool
	concurrent             int
	batchSize              int
	ide                    string
	model                  string
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

var _ = buildZenMCPGuidance

func setupFlags() {
	rootCmd.Flags().StringVar(&pr, "pr", "", "Pull request number")
	rootCmd.Flags().StringVar(&issuesDir, "issues-dir", "", "Path to issues directory (ai-docs/reviews-pr-<PR>/issues)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only generate prompts; do not run IDE tool")
	rootCmd.Flags().IntVar(&concurrent, "concurrent", 1, "Number of batches to process in parallel")
	rootCmd.Flags().
		IntVar(&batchSize, "batch-size", 1, "Number of file groups to batch together (default: 1 for no batching)")
	rootCmd.Flags().StringVar(&ide, "ide", ideCodex, "IDE tool to use: claude, codex, or droid")
	rootCmd.Flags().StringVar(
		&model,
		"model",
		"",
		"Model to use (default: gpt-5-codex for codex/droid, sonnet for claude)",
	)
	rootCmd.Flags().BoolVar(&grouped, "grouped", false, "Generate grouped issue summaries in issues/grouped/ directory")
	rootCmd.Flags().IntVar(&tailLines, "tail-lines", 30, "Number of log lines to show in UI for each job")
	rootCmd.Flags().StringVar(
		&reasoningEffort,
		"reasoning-effort",
		"medium",
		"Reasoning effort for codex/claude/droid (low, medium, high)",
	)
	rootCmd.Flags().BoolVar(&useForm, "form", false, "Use interactive form to collect parameters")
	rootCmd.Flags().StringVar(
		&mode, "mode", modeCodeReview,
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
		3,
		"Maximum number of retry attempts on timeout (0 = no retry, default: 3)",
	)
	rootCmd.Flags().Float64Var(
		&retryBackoffMultiplier,
		"retry-backoff-multiplier",
		2.0,
		"Timeout multiplier for each retry attempt (default: 2.0 = 2x timeout on each retry)",
	)

	// Note: PR is usually required, but we handle this dynamically in runSolveIssues
}

// collectFormParams shows interactive form to collect parameters
func collectFormParams(cmd *cobra.Command) error {
	fmt.Println("\n🎯 Interactive Parameter Collection")
	inputs := newFormInputs()
	builder := newFormBuilder(cmd)
	inputs.register(builder)
	if err := builder.build().Run(); err != nil {
		return fmt.Errorf("form canceled or error: %w", err)
	}
	inputs.apply(cmd)
	fmt.Println("\n✅ Parameters collected successfully!")
	return nil
}

type formInputs struct {
	pr              string
	issuesDir       string
	concurrent      string
	batchSize       string
	ide             string
	model           string
	tailLines       string
	reasoningEffort string
	mode            string
	timeout         string
}

func newFormInputs() *formInputs {
	return &formInputs{}
}

// register wires the interactive fields into the provided builder.
func (fi *formInputs) register(builder *formBuilder) {
	builder.addModeField(&fi.mode)
	builder.addPRField(&fi.pr)
	builder.addOptionalPathField("issues-dir", &fi.issuesDir)
	builder.addConcurrentField(&fi.concurrent)
	builder.addBatchSizeField(&fi.batchSize)
	builder.addIDEField(&fi.ide)
	builder.addModelField(&fi.model)
	builder.addTailLinesField(&fi.tailLines)
	builder.addReasoningEffortField(&fi.reasoningEffort)
	builder.addTimeoutField(&fi.timeout)
	builder.addConfirmField(
		"dry-run",
		"Dry Run?",
		"Only generate prompts without running IDE tool",
		&dryRun,
	)
	builder.addConfirmField(
		"grouped",
		"Generate Grouped Summaries?",
		"Create grouped issue summaries in issues/grouped/",
		&grouped,
	)
	builder.addIncludeCompletedField(&includeCompleted)
}

// apply updates CLI flags and globals with collected form values.
func (fi *formInputs) apply(cmd *cobra.Command) {
	applyStringInput(cmd, "mode", fi.mode, func(val string) { mode = val })
	applyStringInput(cmd, "pr", fi.pr, func(val string) { pr = val })
	applyStringInput(cmd, "issues-dir", fi.issuesDir, func(val string) { issuesDir = val })
	applyIntInput(cmd, "concurrent", fi.concurrent, func(val int) { concurrent = val })
	applyIntInput(cmd, "batch-size", fi.batchSize, func(val int) { batchSize = val })
	applyStringInput(cmd, "ide", fi.ide, func(val string) { ide = val })
	applyStringInput(cmd, "model", fi.model, func(val string) { model = val })
	applyIntInput(cmd, "tail-lines", fi.tailLines, func(val int) { tailLines = val })
	applyStringInput(cmd, "reasoning-effort", fi.reasoningEffort, func(val string) {
		reasoningEffort = val
	})
	applyStringInput(cmd, "timeout", fi.timeout, func(val string) { timeout = val })
}

type formBuilder struct {
	cmd    *cobra.Command
	fields []huh.Field
}

func newFormBuilder(cmd *cobra.Command) *formBuilder {
	return &formBuilder{cmd: cmd}
}

// build assembles the final form with the configured fields.
func (fb *formBuilder) build() *huh.Form {
	return huh.NewForm(huh.NewGroup(fb.fields...)).WithTheme(huh.ThemeCharm())
}

func (fb *formBuilder) addField(flag string, build func() huh.Field) {
	if fb.cmd.Flags().Changed(flag) {
		return
	}
	field := build()
	if field != nil {
		fb.fields = append(fb.fields, field)
	}
}

func (fb *formBuilder) addModeField(target *string) {
	fb.addField("mode", func() huh.Field {
		return huh.NewSelect[string]().
			Title("Execution Mode").
			Description("Choose what to process").
			Options(
				huh.NewOption("PR Review Issues (CodeRabbit)", modeCodeReview),
				huh.NewOption("PRD Task Files", modePRDTasks),
			).
			Value(target)
	})
}

func (fb *formBuilder) addPRField(target *string) {
	fb.addField("pr", func() huh.Field {
		title := "PR Number"
		placeholder := "259"
		description := "Required: Pull request number or identifier to process"
		errorMsg := "PR number is required"
		if mode == modePRDTasks {
			title = "Task Identifier"
			placeholder = "multi-repo"
			description = "Required: Task name/identifier (e.g., 'multi-repo' for tasks/prd-multi-repo)"
			errorMsg = "Task identifier is required"
		}
		return huh.NewInput().
			Title(title).
			Placeholder(placeholder).
			Description(description).
			Value(target).
			Validate(func(str string) error {
				if str == "" {
					return errors.New(errorMsg)
				}
				return nil
			})
	})
}

func (fb *formBuilder) addOptionalPathField(flag string, target *string) {
	fb.addField(flag, func() huh.Field {
		title := "Issues Directory (optional)"
		placeholder := "ai-docs/reviews-pr-<PR>/issues"
		description := "Leave empty to auto-generate from PR number"
		if mode == modePRDTasks {
			title = "Tasks Directory (optional)"
			placeholder = "tasks/prd-<name>"
			description = "Leave empty to auto-generate from task identifier"
		}
		return huh.NewInput().
			Title(title).
			Placeholder(placeholder).
			Description(description).
			Value(target)
	})
}

func (fb *formBuilder) addConcurrentField(target *string) {
	fb.addField("concurrent", func() huh.Field {
		return numericInput(
			"Concurrent Jobs",
			"1",
			"Number of batches to process in parallel (1-10)",
			target,
			1,
			10,
			true,
		)
	})
}

func (fb *formBuilder) addBatchSizeField(target *string) {
	fb.addField("batch-size", func() huh.Field {
		if mode == modePRDTasks {
			*target = "1"
			return nil
		}
		return numericInput(
			"Batch Size",
			"1",
			"Number of file groups per batch (1-20)",
			target,
			1,
			20,
			true,
		)
	})
}

func (fb *formBuilder) addIDEField(target *string) {
	fb.addField("ide", func() huh.Field {
		return huh.NewSelect[string]().
			Title("IDE Tool").
			Description("Choose which IDE tool to use").
			Options(
				huh.NewOption("Codex (recommended)", ideCodex),
				huh.NewOption("Claude", ideClaude),
				huh.NewOption("Droid", ideDroid),
			).
			Value(target)
	})
}

func (fb *formBuilder) addModelField(target *string) {
	fb.addField("model", func() huh.Field {
		return huh.NewInput().
			Title("Model (optional)").
			Placeholder("auto").
			Description("Specific model to use (default: gpt-5-codex for codex/droid, sonnet for claude)").
			Value(target)
	})
}

func (fb *formBuilder) addTailLinesField(target *string) {
	fb.addField("tail-lines", func() huh.Field {
		return numericInput(
			"Log Tail Lines",
			"5",
			"Number of log lines to show in UI (1-100)",
			target,
			1,
			100,
			true,
		)
	})
}

func (fb *formBuilder) addReasoningEffortField(target *string) {
	fb.addField("reasoning-effort", func() huh.Field {
		return huh.NewSelect[string]().
			Title("Reasoning Effort").
			Description("Model reasoning effort level (applies to Codex, Claude, and Droid)").
			Options(
				huh.NewOption("Low", "low"),
				huh.NewOption("Medium (recommended)", "medium"),
				huh.NewOption("High", "high"),
			).
			Value(target)
	})
}

func (fb *formBuilder) addTimeoutField(target *string) {
	fb.addField("timeout", func() huh.Field {
		return huh.NewInput().
			Title("Activity Timeout").
			Placeholder("10m").
			Description("Cancel job if no output received within this period (e.g., 5m, 30s)").
			Value(target).
			Validate(func(str string) error {
				if str == "" {
					return nil
				}
				_, err := time.ParseDuration(str)
				if err != nil {
					return errors.New("invalid duration format (e.g., 5m, 30s, 1h)")
				}
				return nil
			})
	})
}

func (fb *formBuilder) addConfirmField(flag, title, description string, target *bool) {
	fb.addField(flag, func() huh.Field {
		return huh.NewConfirm().
			Title(title).
			Description(description).
			Value(target)
	})
}

func (fb *formBuilder) addIncludeCompletedField(target *bool) {
	fb.addField("include-completed", func() huh.Field {
		if mode != modePRDTasks {
			return nil
		}
		return huh.NewConfirm().
			Title("Include Completed Tasks?").
			Description("Process tasks marked as completed").
			Value(target)
	})
}

func numericInput(
	title string,
	placeholder string,
	description string,
	target *string,
	minVal int,
	maxVal int,
	allowEmpty bool,
) huh.Field {
	return huh.NewInput().
		Title(title).
		Placeholder(placeholder).
		Description(description).
		Value(target).
		Validate(func(str string) error {
			if str == "" {
				if allowEmpty {
					return nil
				}
				return errors.New("value is required")
			}
			val, err := strconv.Atoi(str)
			if err != nil {
				return errors.New("must be a number")
			}
			if val < minVal || val > maxVal {
				return fmt.Errorf("must be between %d and %d", minVal, maxVal)
			}
			return nil
		})
}

func applyStringInput(cmd *cobra.Command, flagName, value string, setter func(string)) {
	if cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	setter(value)
}

func applyIntInput(cmd *cobra.Command, flagName, value string, setter func(int)) {
	if cmd.Flags().Changed(flagName) || value == "" {
		return
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		return
	}
	setter(val)
}

func runSolveIssues(cmd *cobra.Command, _ []string) error {
	if err := maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}
	if err := ensurePRProvided(); err != nil {
		return err
	}
	args := buildCLIArgs()
	if err := args.validate(); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return executeSolveIssues(ctx, args)
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

func buildCLIArgs() *cliArgs {
	timeoutDuration := 10 * time.Minute
	if timeout != "" {
		if parsed, err := time.ParseDuration(timeout); err == nil {
			timeoutDuration = parsed
		}
	}
	return &cliArgs{
		pr:                     pr,
		issuesDir:              issuesDir,
		dryRun:                 dryRun,
		concurrent:             concurrent,
		batchSize:              batchSize,
		ide:                    ide,
		model:                  model,
		grouped:                grouped,
		tailLines:              tailLines,
		reasoningEffort:        reasoningEffort,
		mode:                   mode,
		includeCompleted:       includeCompleted,
		timeout:                timeoutDuration,
		maxRetries:             maxRetries,
		retryBackoffMultiplier: retryBackoffMultiplier,
	}
}

func (c *cliArgs) validate() error {
	if c.mode != modeCodeReview && c.mode != modePRDTasks {
		return fmt.Errorf(
			"invalid --mode value '%s': must be '%s' or '%s'",
			c.mode,
			modeCodeReview,
			modePRDTasks,
		)
	}
	if c.ide != ideClaude && c.ide != ideCodex && c.ide != ideDroid {
		return fmt.Errorf(
			"invalid --ide value '%s': must be '%s', '%s', or '%s'",
			c.ide,
			ideClaude,
			ideCodex,
			ideDroid,
		)
	}
	if c.mode == modePRDTasks && c.batchSize != 1 {
		return fmt.Errorf(
			"batch size must be 1 for prd-tasks mode (got %d)",
			c.batchSize,
		)
	}
	return nil
}

var errNoIssues = errors.New("no issues to process")

func executeSolveIssues(ctx context.Context, args *cliArgs) error {
	prepared, err := prepareSolveIssues(args)
	if err != nil {
		if errors.Is(err, errNoIssues) {
			return nil
		}
		return err
	}
	failed, failures, total, shutdownErr := executeJobsWithGracefulShutdown(ctx, prepared.jobs, args)
	summarizeResults(failed, failures, total)
	if shutdownErr != nil {
		fmt.Fprintf(os.Stderr, "\nShutdown interrupted: %v\n", shutdownErr)
		return shutdownErr
	}
	if len(failures) > 0 {
		return errors.New("one or more groups failed; see logs above")
	}
	return nil
}

type solvePreparation struct {
	jobs              []job
	issuesDir         string
	resolvedPr        string
	issuesDirPath     string
	groupedSummarized bool
}

func validateAndFilterEntries(entries []issueEntry, mode executionMode) ([]issueEntry, error) {
	if len(entries) == 0 {
		if mode == ExecutionModePRDTasks {
			fmt.Println("No task files found.")
		} else {
			fmt.Println("No issue files found.")
		}
		return nil, errNoIssues
	}
	if mode == ExecutionModePRReview {
		entries = filterUnresolved(entries)
		if len(entries) == 0 {
			fmt.Println("All issues are already resolved. Nothing to do.")
			return nil, errNoIssues
		}
	}
	return entries, nil
}

func prepareSolveIssues(args *cliArgs) (*solvePreparation, error) {
	prep := &solvePreparation{}
	var err error
	prep.resolvedPr, prep.issuesDir, prep.issuesDirPath, err = resolveInputs(args)
	if err != nil {
		return nil, err
	}
	if err := ensureCLI(args); err != nil {
		return nil, err
	}
	entries, err := readIssueEntries(prep.issuesDirPath, executionMode(args.mode), args.includeCompleted)
	if err != nil {
		return nil, err
	}
	entries, err = validateAndFilterEntries(entries, executionMode(args.mode))
	if err != nil {
		return nil, err
	}
	groups := groupIssues(entries)
	if args.grouped {
		if err := writeSummaries(prep.issuesDirPath, groups); err != nil {
			return nil, err
		}
		prep.groupedSummarized = true
	}
	promptRoot, err := initPromptRoot(prep.resolvedPr)
	if err != nil {
		return nil, err
	}
	prep.jobs, err = prepareJobs(
		prep.resolvedPr,
		groups,
		promptRoot,
		args.batchSize,
		args.grouped,
		executionMode(args.mode),
	)
	if err != nil {
		return nil, err
	}
	return prep, nil
}

type job struct {
	codeFiles     []string                // Multiple files in this batch
	groups        map[string][]issueEntry // Map of codeFile -> issues
	safeName      string
	prompt        []byte
	outPromptPath string
	outLog        string
	errLog        string
}

type failInfo struct {
	codeFile string
	exitCode int
	outLog   string
	errLog   string
	err      error
}

func resolveInputs(args *cliArgs) (string, string, string, error) {
	pr := args.pr
	issuesDir := args.issuesDir
	if pr == "" && issuesDir == "" {
		return "", "", "", errors.New("missing required flags: either --pr or --issues-dir must be provided")
	}
	var err error
	if pr == "" && issuesDir != "" {
		pr, err = inferPrFromIssuesDir(issuesDir)
		if err != nil {
			return "", "", "", err
		}
	}
	if issuesDir == "" {
		if args.mode == modePRDTasks {
			issuesDir = fmt.Sprintf("tasks/prd-%s", pr)
		} else {
			issuesDir = fmt.Sprintf("ai-docs/reviews-pr-%s/issues", pr)
		}
	}
	resolvedIssuesDir, err := filepath.Abs(issuesDir)
	if err != nil {
		return "", "", "", fmt.Errorf("resolve issues dir: %w", err)
	}
	if st, statErr := os.Stat(resolvedIssuesDir); statErr != nil || !st.IsDir() {
		return "", "", "", fmt.Errorf("issues directory not found: %s", resolvedIssuesDir)
	}
	return pr, issuesDir, resolvedIssuesDir, nil
}

func ensureCLI(args *cliArgs) error {
	if args.dryRun {
		return nil
	}
	if err := assertIDEExists(args.ide); err != nil {
		return err
	}
	if err := assertExecSupported(args.ide); err != nil {
		return err
	}
	return nil
}

func writeSummaries(resolvedIssuesDir string, groups map[string][]issueEntry) error {
	groupedDir := filepath.Join(resolvedIssuesDir, "grouped")
	if err := os.MkdirAll(groupedDir, 0o755); err != nil {
		return fmt.Errorf("mkdir grouped dir: %w", err)
	}
	if err := writeGroupedSummaries(groupedDir, groups); err != nil {
		return err
	}
	return nil
}

func initPromptRoot(pr string) (string, error) {
	promptRoot, err := filepath.Abs(filepath.Join(".tmp", "codex-prompts", fmt.Sprintf("pr-%s", pr)))
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(promptRoot, 0o755); err != nil {
		return "", fmt.Errorf("mkdir prompt root: %w", err)
	}
	return promptRoot, nil
}

func prepareJobs(
	pr string,
	groups map[string][]issueEntry,
	promptRoot string,
	batchSize int,
	grouped bool,
	mode executionMode,
) ([]job, error) {
	if mode == ExecutionModePRDTasks {
		batchSize = 1
		grouped = false
	}
	allIssues := flattenAndSortIssues(groups)
	if batchSize <= 0 {
		batchSize = 1
	}
	issueBatches := createIssueBatches(allIssues, batchSize)
	jobs := make([]job, 0, len(issueBatches))
	for batchIdx, batchIssues := range issueBatches {
		jb, err := buildBatchJob(pr, promptRoot, grouped, batchIdx, batchIssues, mode)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, jb)
	}
	return jobs, nil
}

// buildBatchJob converts a batch of issues into an executable job definition.
func buildBatchJob(
	pr string,
	promptRoot string,
	grouped bool,
	batchIdx int,
	batchIssues []issueEntry,
	mode executionMode,
) (job, error) {
	batchGroups, batchFiles := groupIssuesByCodeFile(batchIssues)
	safeName := determineBatchName(batchIdx, batchFiles, mode)
	promptStr := buildBatchedIssuesPrompt(buildBatchedIssuesParams{
		PR:          pr,
		BatchGroups: batchGroups,
		Grouped:     grouped,
		Mode:        mode,
	})
	outPromptPath, outLog, errLog, err := writeBatchArtifacts(promptRoot, safeName, promptStr)
	if err != nil {
		return job{}, err
	}
	return job{
		codeFiles:     batchFiles,
		groups:        batchGroups,
		safeName:      safeName,
		prompt:        []byte(promptStr),
		outPromptPath: outPromptPath,
		outLog:        outLog,
		errLog:        errLog,
	}, nil
}

// determineBatchName picks a human-readable name for the generated batch artifacts.
func determineBatchName(batchIdx int, batchFiles []string, mode executionMode) string {
	if mode == ExecutionModePRDTasks {
		if len(batchFiles) == 1 {
			return safeFileName(batchFiles[0])
		}
		return fmt.Sprintf("task_%03d", batchIdx+1)
	}
	if len(batchFiles) == 1 {
		filename := batchFiles[0]
		if strings.HasPrefix(filename, "__unknown__") {
			filename = unknownFileName
		}
		return safeFileName(filename)
	}
	return fmt.Sprintf("batch_%03d", batchIdx+1)
}

// writeBatchArtifacts persists prompt and log files for a generated job.
func writeBatchArtifacts(promptRoot, safeName, promptStr string) (string, string, string, error) {
	outPromptPath := filepath.Join(promptRoot, fmt.Sprintf("%s.prompt.md", safeName))
	if err := os.WriteFile(outPromptPath, []byte(promptStr), 0o600); err != nil {
		return "", "", "", fmt.Errorf("write prompt: %w", err)
	}
	outLog := filepath.Join(promptRoot, fmt.Sprintf("%s.out.log", safeName))
	errLog := filepath.Join(promptRoot, fmt.Sprintf("%s.err.log", safeName))
	return outPromptPath, outLog, errLog, nil
}

func flattenAndSortIssues(groups map[string][]issueEntry) []issueEntry {
	allIssues := make([]issueEntry, 0)
	for _, items := range groups {
		allIssues = append(allIssues, items...)
	}
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].name < allIssues[j].name
	})
	return allIssues
}

func createIssueBatches(allIssues []issueEntry, batchSize int) [][]issueEntry {
	batches := make([][]issueEntry, 0)
	for i := 0; i < len(allIssues); i += batchSize {
		end := i + batchSize
		if end > len(allIssues) {
			end = len(allIssues)
		}
		batches = append(batches, allIssues[i:end])
	}
	return batches
}

func groupIssuesByCodeFile(issues []issueEntry) (map[string][]issueEntry, []string) {
	batchGroups := make(map[string][]issueEntry)
	for _, issue := range issues {
		batchGroups[issue.codeFile] = append(batchGroups[issue.codeFile], issue)
	}
	batchFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		batchFiles = append(batchFiles, codeFile)
	}
	sort.Strings(batchFiles)
	return batchGroups, batchFiles
}

// executeJobsWithGracefulShutdown executes jobs with proper graceful shutdown handling
func executeJobsWithGracefulShutdown(ctx context.Context, jobs []job, args *cliArgs) (int32, []failInfo, int, error) {
	execCtx, err := newJobExecutionContext(ctx, jobs, args)
	if err != nil {
		total := len(jobs)
		return 0, []failInfo{{err: err}}, total, nil
	}
	defer execCtx.cleanup()
	_, cancelJobs := execCtx.launchWorkers(ctx)
	defer cancelJobs()
	done := execCtx.waitChannel()
	return execCtx.awaitCompletion(ctx, done, cancelJobs)
}

type jobExecutionContext struct {
	args           *cliArgs
	jobs           []job
	total          int
	cwd            string
	uiCh           chan uiMsg
	uiProg         *tea.Program
	sem            chan struct{}
	aggregateUsage TokenUsage
	aggregateMu    sync.Mutex
	failed         int32
	failures       []failInfo
	failuresMu     sync.Mutex
	completed      int32
	wg             sync.WaitGroup
}

func newJobExecutionContext(ctx context.Context, jobs []job, args *cliArgs) (*jobExecutionContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	execCtx := &jobExecutionContext{
		args:  args,
		jobs:  jobs,
		total: len(jobs),
		cwd:   cwd,
		sem:   make(chan struct{}, maxInt(1, args.concurrent)),
	}
	execCtx.uiCh, execCtx.uiProg = setupUI(ctx, jobs, args, !args.dryRun, &execCtx.aggregateUsage, &execCtx.aggregateMu)
	return execCtx, nil
}

func (j *jobExecutionContext) cleanup() {
	if j.uiProg != nil {
		close(j.uiCh)
		time.Sleep(80 * time.Millisecond)
		j.uiProg.Quit()
	}
}

func (j *jobExecutionContext) launchWorkers(ctx context.Context) (context.Context, context.CancelFunc) {
	jobCtx, cancel := context.WithCancel(ctx)
	for idx := range j.jobs {
		jb := &j.jobs[idx]
		j.wg.Add(1)
		j.sem <- struct{}{}
		go j.executeJob(jobCtx, idx, jb)
	}
	return jobCtx, cancel
}

func (j *jobExecutionContext) executeJob(jobCtx context.Context, index int, jb *job) {
	defer func() {
		<-j.sem
		j.wg.Done()
		atomic.AddInt32(&j.completed, 1)
	}()
	runOneJob(
		jobCtx,
		j.args,
		index,
		jb,
		j.cwd,
		j.uiCh,
		&j.failed,
		&j.failuresMu,
		&j.failures,
		&j.aggregateUsage,
		&j.aggregateMu,
	)
}

func (j *jobExecutionContext) waitChannel() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		j.wg.Wait()
		close(done)
	}()
	return done
}

func (j *jobExecutionContext) awaitCompletion(
	ctx context.Context,
	done <-chan struct{},
	cancelJobs context.CancelFunc,
) (int32, []failInfo, int, error) {
	select {
	case <-done:
		j.reportAggregateUsage()
		return j.failed, j.failures, j.total, nil
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "\nReceived shutdown signal, canceling remaining jobs...\n")
		cancelJobs()
		return j.awaitShutdownAfterCancel(done)
	}
}

func (j *jobExecutionContext) reportAggregateUsage() {
	if j.args.ide != ideClaude {
		return
	}
	j.aggregateMu.Lock()
	defer j.aggregateMu.Unlock()
	printAggregateTokenUsage(&j.aggregateUsage)
}

func (j *jobExecutionContext) awaitShutdownAfterCancel(done <-chan struct{}) (int32, []failInfo, int, error) {
	shutdownTimeout := 30 * time.Second
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	select {
	case <-done:
		fmt.Fprintf(os.Stderr, "All jobs completed gracefully within %v\n", shutdownTimeout)
		j.reportAggregateUsage()
		return j.failed, j.failures, j.total, nil
	case <-shutdownCtx.Done():
		fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded (%v), forcing exit\n", shutdownTimeout)
		return j.failed, j.failures, j.total, fmt.Errorf("shutdown timeout exceeded")
	}
}

func setupUI(
	ctx context.Context,
	jobs []job,
	_ *cliArgs,
	enabled bool,
	_ *TokenUsage,
	_ *sync.Mutex,
) (chan uiMsg, *tea.Program) {
	if !enabled {
		return nil, nil
	}
	total := len(jobs)
	uiCh := make(chan uiMsg, total*4)
	mdl := newUIModel(total)
	mdl.setEventSource(uiCh)
	prog := tea.NewProgram(mdl, tea.WithAltScreen())
	go func() {
		if _, runErr := prog.Run(); runErr != nil {
			fmt.Fprintf(os.Stderr, "UI program error: %v\n", runErr)
		}
	}()
	for idx := range jobs {
		jb := &jobs[idx]
		totalIssues := 0
		for _, items := range jb.groups {
			totalIssues += len(items)
		}
		codeFileLabel := strings.Join(jb.codeFiles, ", ")
		if len(jb.codeFiles) > 3 {
			codeFileLabel = fmt.Sprintf("%s and %d more", strings.Join(jb.codeFiles[:3], ", "), len(jb.codeFiles)-3)
		}
		uiCh <- jobQueuedMsg{
			Index:     idx,
			CodeFile:  codeFileLabel,
			CodeFiles: jb.codeFiles,
			Issues:    totalIssues,
			SafeName:  jb.safeName,
			OutLog:    jb.outLog,
			ErrLog:    jb.errLog,
		}
	}
	go func() {
		<-ctx.Done()
		prog.Quit()
	}()
	return uiCh, prog
}

func runOneJob(
	ctx context.Context,
	args *cliArgs,
	index int,
	j *job,
	cwd string,
	uiCh chan uiMsg,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) {
	useUI := uiCh != nil
	if ctx.Err() != nil {
		if useUI {
			uiCh <- jobFinishedMsg{Index: index, Success: false, ExitCode: -1}
		}
		return
	}
	notifyJobStart(useUI, uiCh, index, j, args.ide, args.model, args.reasoningEffort)
	if args.dryRun {
		if useUI {
			uiCh <- jobFinishedMsg{Index: index, Success: true, ExitCode: 0}
		}
		return
	}
	executeJobWithRetry(
		ctx, args, j, cwd, useUI, uiCh, index,
		failed, failuresMu, failures, aggregateUsage, aggregateMu,
	)
}

func executeJobWithRetry(
	ctx context.Context,
	args *cliArgs,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) {
	currentTimeout := args.timeout
	attempt := 0
	maxRetries := uint64(0)
	if args.maxRetries > 0 {
		// #nosec G115 - maxRetries is validated to be non-negative and reasonable
		maxRetries = uint64(args.maxRetries)
	}
	backoff := retry.WithMaxRetries(maxRetries, retry.NewConstant(1*time.Millisecond))
	err := retry.Do(ctx, backoff, func(retryCtx context.Context) error {
		attempt++
		currentTimeout = calculateRetryTimeout(
			currentTimeout,
			attempt,
			args.retryBackoffMultiplier,
			args.maxRetries,
			index,
			j,
			useUI,
		)
		return executeJobAttempt(
			retryCtx, args, j, cwd, useUI, uiCh, index, currentTimeout,
			failed, failuresMu, failures, aggregateUsage, aggregateMu, attempt,
		)
	})
	logRetryCompletion(err, attempt, index, j, useUI)
}

func calculateRetryTimeout(
	currentTimeout time.Duration,
	attempt int,
	multiplier float64,
	maxRetries int,
	index int,
	j *job,
	useUI bool,
) time.Duration {
	if attempt > 1 {
		currentTimeout = time.Duration(float64(currentTimeout) * multiplier)
		if !useUI {
			fmt.Fprintf(
				os.Stderr,
				"\n🔄 Retry attempt %d/%d for job %d (%s) with timeout %v\n",
				attempt-1,
				maxRetries,
				index+1,
				strings.Join(j.codeFiles, ", "),
				currentTimeout,
			)
		}
	}
	return currentTimeout
}

func executeJobAttempt(
	ctx context.Context,
	args *cliArgs,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	currentTimeout time.Duration,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	attempt int,
) error {
	argsWithTimeout := *args
	argsWithTimeout.timeout = currentTimeout
	if useUI && attempt > 1 {
		uiCh <- jobStartedMsg{Index: index}
	}
	success, exitCode := executeJobWithTimeoutAndResult(
		ctx, &argsWithTimeout, j, cwd, useUI, uiCh,
		index, failed, failuresMu, failures, aggregateUsage, aggregateMu,
	)
	if !success && exitCode == -2 {
		return retry.RetryableError(fmt.Errorf("timeout"))
	}
	return nil
}

func logRetryCompletion(err error, attempt int, index int, j *job, useUI bool) {
	if err != nil && attempt > 1 && !useUI {
		fmt.Fprintf(
			os.Stderr,
			"\n❌ Job %d (%s) failed after %d retry attempts\n",
			index+1,
			strings.Join(j.codeFiles, ", "),
			attempt-1,
		)
	}
}

func executeJobWithTimeoutAndResult(
	ctx context.Context,
	args *cliArgs,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) (bool, int) {
	cmd, outF, errF, monitor := setupCommandExecution(
		ctx, args, j, cwd, useUI, uiCh, index, aggregateUsage, aggregateMu,
	)
	if cmd == nil {
		return false, -1
	}
	return executeCommandAndHandleResultWithStatus(
		ctx, args.timeout, monitor, cmd, outF, errF, j,
		index, useUI, uiCh, failed, failuresMu, failures,
	)
}

func notifyJobStart(useUI bool, uiCh chan uiMsg, index int, j *job, ide string, model string, reasoningEffort string) {
	if useUI {
		uiCh <- jobStartedMsg{Index: index}
		return
	}
	shellCmdStr := buildShellCommandString(ide, model, reasoningEffort)
	ideName := getIDEName(ide)
	totalIssues := countTotalIssues(j)
	codeFileLabel := formatCodeFileLabel(j.codeFiles)
	fmt.Printf(
		"\n=== Running %s (non-interactive) for batch: %s (%d issues)\n$ %s\n",
		ideName,
		codeFileLabel,
		totalIssues,
		shellCmdStr,
	)
}

func buildShellCommandString(ide string, model string, reasoningEffort string) string {
	switch ide {
	case ideCodex:
		return buildCodexCommand(model, reasoningEffort)
	case ideClaude:
		return buildClaudeCommand(model, reasoningEffort)
	case ideDroid:
		return buildDroidCommand(model, reasoningEffort)
	default:
		return ""
	}
}

func buildCodexCommand(model string, reasoningEffort string) string {
	modelToUse := defaultCodexModel
	if model != "" && model != defaultCodexModel {
		modelToUse = model
	}
	return fmt.Sprintf("codex --full-auto -m %s -c model_reasoning_effort=%s exec -", modelToUse, reasoningEffort)
}

func buildClaudeCommand(model string, reasoningEffort string) string {
	thinkPrompt := getThinkPrompt(reasoningEffort)
	modelToUse := defaultClaudeModel
	if model != "" && model != defaultClaudeModel {
		modelToUse = model
	}
	return fmt.Sprintf(
		"claude --print --output-format stream-json --verbose --model %s "+
			"--dangerously-skip-permissions --permission-mode bypassPermissions "+
			"--append-system-prompt %q",
		modelToUse,
		thinkPrompt,
	)
}

func buildDroidCommand(model string, reasoningEffort string) string {
	base := fmt.Sprintf("droid exec --auto medium --reasoning-effort %s", reasoningEffort)
	if model != "" && model != defaultCodexModel {
		return fmt.Sprintf("%s --model %s --file -", base, model)
	}
	if model == defaultCodexModel {
		return fmt.Sprintf("%s --model %s --file -", base, defaultCodexModel)
	}
	return fmt.Sprintf("%s --file -", base)
}

func getThinkPrompt(reasoningEffort string) string {
	switch reasoningEffort {
	case "low":
		return thinkPromptLow
	case "high":
		return thinkPromptHighDescription
	default:
		return thinkPromptMedium
	}
}

func getIDEName(ide string) string {
	switch ide {
	case ideCodex:
		return "Codex"
	case ideClaude:
		return "Claude"
	case ideDroid:
		return "Droid"
	default:
		return ""
	}
}

func countTotalIssues(j *job) int {
	total := 0
	for _, items := range j.groups {
		total += len(items)
	}
	return total
}

func formatCodeFileLabel(codeFiles []string) string {
	label := strings.Join(codeFiles, ", ")
	if len(codeFiles) > 1 {
		return fmt.Sprintf("%d files: %s", len(codeFiles), label)
	}
	return label
}

func createIDECommand(ctx context.Context, args *cliArgs) *exec.Cmd {
	model := args.model
	if model == "" {
		model = defaultModelForIDE(args.ide)
	}
	switch args.ide {
	case ideCodex:
		return codexCommand(ctx, model, args.reasoningEffort)
	case ideClaude:
		return claudeCommand(ctx, model, args.reasoningEffort)
	case ideDroid:
		return droidCommand(ctx, model, args.reasoningEffort)
	default:
		return nil
	}
}

// defaultModelForIDE resolves the implicit model selection for each IDE.
func defaultModelForIDE(ide string) string {
	switch ide {
	case ideCodex, ideDroid:
		return defaultCodexModel
	case ideClaude:
		return defaultClaudeModel
	default:
		return ""
	}
}

// codexCommand builds the Codex CLI invocation with optional model override.
func codexCommand(ctx context.Context, model, reasoning string) *exec.Cmd {
	args := []string{"--full-auto"}
	if model != "" {
		args = append(args, "-m", model)
	}
	args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%s", reasoning), "exec", "-")
	return exec.CommandContext(ctx, ideCodex, args...)
}

// claudeCommand prepares the Claude CLI invocation using the reasoning preset.
func claudeCommand(ctx context.Context, model, reasoning string) *exec.Cmd {
	prompt := claudePromptForEffort(reasoning)
	return exec.CommandContext(
		ctx,
		ideClaude,
		"--print",
		"--output-format", "stream-json",
		"--verbose",
		"--model", model,
		"--permission-mode", "bypassPermissions",
		"--dangerously-skip-permissions",
		"--append-system-prompt", prompt,
	)
}

// claudePromptForEffort maps reasoning presets to system prompts.
func claudePromptForEffort(reasoning string) string {
	switch reasoning {
	case "low":
		return thinkPromptLow
	case "high":
		return thinkPromptHighDescription
	case "medium":
		return thinkPromptMedium
	default:
		return thinkPromptMedium
	}
}

// droidCommand composes the Droid CLI invocation with appropriate switches.
func droidCommand(ctx context.Context, model, reasoning string) *exec.Cmd {
	droidArgs := []string{
		"exec",
		"--auto", "medium",
		"--reasoning-effort", reasoning,
	}
	if model != "" {
		droidArgs = append(droidArgs, "--model", model)
	}
	droidArgs = append(droidArgs, "--file", "-")
	return exec.CommandContext(ctx, ideDroid, droidArgs...)
}

func setupCommandIO(
	cmd *exec.Cmd,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	tailLines int,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) (*os.File, *os.File, *activityMonitor, error) {
	configureCommandEnvironment(cmd, cwd, j.prompt)
	outF, err := createLogFile(j.outLog, "out")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create out log: %w", err)
	}
	errF, err := createLogFile(j.errLog, "err")
	if err != nil {
		outF.Close()
		return nil, nil, nil, fmt.Errorf("create err log: %w", err)
	}
	monitor := newActivityMonitor()
	outTap, errTap := buildCommandTaps(
		outF,
		errF,
		tailLines,
		useUI,
		uiCh,
		index,
		ideType,
		aggregateUsage,
		aggregateMu,
		monitor,
	)
	cmd.Stdout = outTap
	cmd.Stderr = errTap
	return outF, errF, monitor, nil
}

// configureCommandEnvironment applies working directory, stdin, and color env vars.
func configureCommandEnvironment(cmd *exec.Cmd, cwd string, prompt []byte) {
	cmd.Dir = cwd
	cmd.Stdin = bytes.NewReader(prompt)
	cmd.Env = append(os.Environ(),
		"FORCE_COLOR=1",
		"CLICOLOR_FORCE=1",
		"TERM=xterm-256color",
	)
}

// buildCommandTaps configures stdout/stderr writers according to UI settings.
func buildCommandTaps(
	outF, errF *os.File,
	tailLines int,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	monitor *activityMonitor,
) (io.Writer, io.Writer) {
	outRing := newLineRing(tailLines)
	errRing := newLineRing(tailLines)
	if useUI {
		return buildUITaps(outF, errF, outRing, errRing, uiCh, index, ideType, aggregateUsage, aggregateMu, monitor)
	}
	return buildCLITaps(outF, errF, ideType, aggregateUsage, aggregateMu, monitor)
}

// buildUITaps creates stdout/stderr writers when the interactive UI is enabled.
func buildUITaps(
	outF, errF *os.File,
	outRing, errRing *lineRing,
	uiCh chan uiMsg,
	index int,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	monitor *activityMonitor,
) (io.Writer, io.Writer) {
	uiTap := newUILogTap(index, false, outRing, errRing, uiCh, monitor)
	var outTap io.Writer
	if ideType == ideClaude {
		usageCallback := func(usage TokenUsage) {
			if uiCh != nil {
				uiCh <- tokenUsageUpdateMsg{Index: index, Usage: usage}
			}
			if aggregateUsage != nil && aggregateMu != nil {
				aggregateMu.Lock()
				aggregateUsage.Add(usage)
				aggregateMu.Unlock()
			}
		}
		outTap = io.MultiWriter(outF, newJSONFormatterWithCallbackAndMonitor(uiTap, usageCallback, monitor))
	} else {
		outTap = io.MultiWriter(outF, uiTap)
	}
	errTap := io.MultiWriter(errF, newUILogTap(index, true, outRing, errRing, uiCh, monitor))
	return outTap, errTap
}

// buildCLITaps creates stdout/stderr writers for non-UI execution.
func buildCLITaps(
	outF, errF *os.File,
	ideType string,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
	monitor *activityMonitor,
) (io.Writer, io.Writer) {
	if ideType == ideClaude {
		usageCallback := func(usage TokenUsage) {
			if aggregateUsage != nil && aggregateMu != nil {
				aggregateMu.Lock()
				aggregateUsage.Add(usage)
				aggregateMu.Unlock()
			}
		}
		return io.MultiWriter(
				outF,
				newJSONFormatterWithCallbackAndMonitor(os.Stdout, usageCallback, monitor),
			), io.MultiWriter(
				errF,
				os.Stderr,
			)
	}
	return io.MultiWriter(outF, os.Stdout), io.MultiWriter(errF, os.Stderr)
}

func setupCommandExecution(
	ctx context.Context,
	args *cliArgs,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
	aggregateUsage *TokenUsage,
	aggregateMu *sync.Mutex,
) (*exec.Cmd, *os.File, *os.File, *activityMonitor) {
	cmd := createIDECommand(ctx, args)
	if cmd == nil {
		return nil, nil, nil, nil
	}
	outF, errF, monitor, err := setupCommandIO(
		cmd,
		j,
		cwd,
		useUI,
		uiCh,
		index,
		args.tailLines,
		args.ide,
		aggregateUsage,
		aggregateMu,
	)
	if err != nil {
		recordFailureWithContext(nil, j, nil, err, -1)
		return nil, nil, nil, nil
	}
	return cmd, outF, errF, monitor
}

func createLogFile(path, _ string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func executeCommandAndHandleResultWithStatus(
	ctx context.Context,
	timeout time.Duration,
	monitor *activityMonitor,
	cmd *exec.Cmd,
	outF *os.File,
	errF *os.File,
	j *job,
	index int,
	useUI bool,
	uiCh chan uiMsg,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
) (bool, int) {
	defer func() {
		if outF != nil {
			outF.Close()
		}
		if errF != nil {
			errF.Close()
		}
	}()
	cmdDone := make(chan error, 1)
	go func() {
		cmdDone <- cmd.Run()
	}()
	activityTimeout := startActivityWatchdog(ctx, monitor, timeout, cmdDone)
	type result struct {
		success  bool
		exitCode int
	}
	resultCh := make(chan result, 1)
	select {
	case err := <-cmdDone:
		success, exitCode := handleCommandCompletionWithResult(
			err, j, index, useUI, uiCh, failed, failuresMu, failures,
		)
		resultCh <- result{success, exitCode}
	case <-activityTimeout:
		handleActivityTimeout(ctx, cmd, cmdDone, j, index, useUI, uiCh, failed, failuresMu, failures, timeout)
		resultCh <- result{false, -2}
	case <-ctx.Done():
		handleCommandCancellation(ctx, cmd, cmdDone, j, index, useUI, uiCh, failed, failuresMu, failures)
		resultCh <- result{false, -1}
	}
	res := <-resultCh
	return res.success, res.exitCode
}

func startActivityWatchdog(
	ctx context.Context,
	monitor *activityMonitor,
	timeout time.Duration,
	cmdDone <-chan error,
) <-chan struct{} {
	activityTimeout := make(chan struct{})
	if monitor != nil && timeout > 0 {
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if monitor.timeSinceLastActivity() > timeout {
						close(activityTimeout)
						return
					}
				case <-cmdDone:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	return activityTimeout
}

func handleCommandCompletionWithResult(
	err error,
	j *job,
	index int,
	useUI bool,
	uiCh chan uiMsg,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
) (bool, int) {
	if err != nil {
		ec := exitCodeOf(err)
		atomic.AddInt32(failed, 1)
		codeFileLabel := strings.Join(j.codeFiles, ", ")
		recordFailure(
			failuresMu,
			failures,
			failInfo{codeFile: codeFileLabel, exitCode: ec, outLog: j.outLog, errLog: j.errLog, err: err},
		)
		if useUI {
			uiCh <- jobFinishedMsg{Index: index, Success: false, ExitCode: ec}
		}
		return false, ec
	}
	if useUI {
		uiCh <- jobFinishedMsg{Index: index, Success: true, ExitCode: 0}
	}
	return true, 0
}

func handleCommandCancellation(
	_ context.Context,
	cmd *exec.Cmd,
	cmdDone <-chan error,
	j *job,
	index int,
	useUI bool,
	uiCh chan uiMsg,
	_ *int32,
	_ *sync.Mutex,
	_ *[]failInfo,
) {
	fmt.Fprintf(
		os.Stderr,
		"\nCanceling job %d (%s) due to shutdown signal\n",
		index+1,
		strings.Join(j.codeFiles, ", "),
	)
	if cmd.Process != nil {
		// NOTE: Attempt graceful termination before force killing spawned commands.
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send SIGTERM to process: %v\n", err)
		}

		select {
		case <-cmdDone:
			fmt.Fprintf(os.Stderr, "Job %d terminated gracefully\n", index+1)
		case <-time.After(5 * time.Second):
			// NOTE: Escalate to SIGKILL if the process ignores our grace period.
			fmt.Fprintf(os.Stderr, "Job %d did not terminate gracefully, force killing...\n", index+1)
			if err := cmd.Process.Kill(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
			}
		}
	}
	if useUI {
		uiCh <- jobFinishedMsg{Index: index, Success: false, ExitCode: -1}
	}
}

func handleActivityTimeout(
	_ context.Context,
	cmd *exec.Cmd,
	cmdDone <-chan error,
	j *job,
	index int,
	useUI bool,
	uiCh chan uiMsg,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
	timeout time.Duration,
) {
	logTimeoutMessage(index, j, timeout, useUI)
	atomic.AddInt32(failed, 1)
	terminateTimedOutProcess(cmd, cmdDone, index, useUI)
	recordTimeoutFailure(j, timeout, failuresMu, failures)
	if useUI {
		uiCh <- jobFinishedMsg{Index: index, Success: false, ExitCode: -2}
	}
}

func logTimeoutMessage(index int, j *job, timeout time.Duration, useUI bool) {
	if !useUI {
		fmt.Fprintf(
			os.Stderr,
			"\nJob %d (%s) timed out after %v of inactivity\n",
			index+1,
			strings.Join(j.codeFiles, ", "),
			timeout,
		)
	}
}

func terminateTimedOutProcess(cmd *exec.Cmd, cmdDone <-chan error, index int, useUI bool) {
	if cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		if !useUI {
			fmt.Fprintf(os.Stderr, "Failed to send SIGTERM to process: %v\n", err)
		}
	}
	waitForProcessTermination(cmdDone, cmd, index, useUI)
}

func waitForProcessTermination(cmdDone <-chan error, cmd *exec.Cmd, index int, useUI bool) {
	select {
	case <-cmdDone:
		if !useUI {
			fmt.Fprintf(os.Stderr, "Job %d terminated gracefully after timeout\n", index+1)
		}
	case <-time.After(5 * time.Second):
		forceKillProcess(cmd, index, useUI)
	}
}

func forceKillProcess(cmd *exec.Cmd, index int, useUI bool) {
	if !useUI {
		fmt.Fprintf(os.Stderr, "Job %d did not terminate gracefully, force killing...\n", index+1)
	}
	if err := cmd.Process.Kill(); err != nil {
		if !useUI {
			fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
		}
	}
}

func recordTimeoutFailure(j *job, timeout time.Duration, failuresMu *sync.Mutex, failures *[]failInfo) {
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	timeoutErr := fmt.Errorf("activity timeout: no output received for %v", timeout)
	recordFailure(
		failuresMu,
		failures,
		failInfo{
			codeFile: codeFileLabel,
			exitCode: -2,
			outLog:   j.outLog,
			errLog:   j.errLog,
			err:      timeoutErr,
		},
	)
}

func recordFailureWithContext(
	failuresMu *sync.Mutex,
	j *job,
	failures *[]failInfo,
	err error,
	exitCode int,
) {
	codeFileLabel := strings.Join(j.codeFiles, ", ")
	recordFailure(failuresMu, failures, failInfo{
		codeFile: codeFileLabel,
		exitCode: exitCode,
		outLog:   j.outLog,
		errLog:   j.errLog,
		err:      err,
	})
}

func recordFailure(mu *sync.Mutex, list *[]failInfo, f failInfo) {
	mu.Lock()
	*list = append(*list, f)
	mu.Unlock()
}

func printAggregateTokenUsage(usage *TokenUsage) {
	if usage == nil || usage.Total() == 0 {
		return // No token usage to report
	}
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Claude API Token Usage (Aggregate across all jobs)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Input Tokens:          %s\n", formatNumber(usage.InputTokens))
	if usage.CacheReadTokens > 0 {
		fmt.Printf("  Cache Read Tokens:     %s\n", formatNumber(usage.CacheReadTokens))
	}
	if usage.CacheCreationTokens > 0 {
		fmt.Printf("  Cache Creation Tokens: %s\n", formatNumber(usage.CacheCreationTokens))
	}
	fmt.Printf("  Output Tokens:         %s\n", formatNumber(usage.OutputTokens))
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("  Total Tokens:          %s\n", formatNumber(usage.Total()))
	fmt.Println(strings.Repeat("=", 60))
}

func summarizeResults(failed int32, failures []failInfo, total int) {
	fmt.Printf(
		"\nExecution Summary:\n- Total Groups: %d\n- Success: %d\n- Failed: %d\n",
		total,
		total-int(failed),
		int(failed),
	)
	if len(failures) == 0 {
		return
	}
	fmt.Println("\nFailures:")
	for _, f := range failures {
		fmt.Printf(
			"- Group: %s\n  - Exit Code: %d\n  - Logs: %s (out), %s (err)\n",
			f.codeFile,
			f.exitCode,
			f.outLog,
			f.errLog,
		)
	}
}

// --- UI (Bubble Tea + Lipgloss) ---
type jobState int

const (
	jobPending jobState = iota
	jobRunning
	jobSuccess
	jobFailed
)

const (
	sidebarWidthRatio      = 0.25
	sidebarMinWidth        = 30
	sidebarMaxWidth        = 50
	mainMinWidth           = 60
	minContentHeight       = 10
	sidebarChromeWidth     = 4 // border (2) + horizontal padding (2)
	sidebarChromeHeight    = 2 // rounded border top + bottom
	mainHorizontalPadding  = 2 // padding applied in renderMainContent
	logViewportMinHeight   = 6
	sidebarViewportMinRows = 5
	headerSectionHeight    = 3 // header line + top/bottom margins
	helpSectionHeight      = 2 // help line + bottom margin
	separatorSectionHeight = 1
	chromeHeight           = headerSectionHeight + helpSectionHeight + separatorSectionHeight
)

type uiJob struct {
	codeFile    string
	codeFiles   []string // Multiple files in batch
	issues      int
	safeName    string
	outLog      string
	errLog      string
	state       jobState
	exitCode    int
	lastOut     []string
	lastErr     []string
	startedAt   time.Time
	completedAt time.Time
	duration    time.Duration
	tokenUsage  *TokenUsage // Claude API token usage (nil for non-Claude IDEs)
}

type tickMsg struct{}

type jobQueuedMsg struct {
	Index     int
	CodeFile  string
	CodeFiles []string
	Issues    int
	SafeName  string
	OutLog    string
	ErrLog    string
}
type jobStartedMsg struct{ Index int }
type jobFinishedMsg struct {
	Index    int
	Success  bool
	ExitCode int
}
type jobLogUpdateMsg struct {
	Index int
	Out   []string
	Err   []string
}
type drainMsg struct{}
type tokenUsageUpdateMsg struct {
	Index int
	Usage TokenUsage
}

// TokenUsage tracks Claude API token consumption
type TokenUsage struct {
	InputTokens         int
	CacheCreationTokens int
	CacheReadTokens     int
	OutputTokens        int
	Ephemeral5mTokens   int
	Ephemeral1hTokens   int
}

// Add accumulates usage from another TokenUsage
func (u *TokenUsage) Add(other TokenUsage) {
	u.InputTokens += other.InputTokens
	u.CacheCreationTokens += other.CacheCreationTokens
	u.CacheReadTokens += other.CacheReadTokens
	u.OutputTokens += other.OutputTokens
	u.Ephemeral5mTokens += other.Ephemeral5mTokens
	u.Ephemeral1hTokens += other.Ephemeral1hTokens
}

// Total returns total tokens used (input + output, excluding cache metrics)
func (u *TokenUsage) Total() int {
	return u.InputTokens + u.OutputTokens
}

// ClaudeMessage represents a parsed Claude stream-json message
type ClaudeMessage struct {
	Type    string `json:"type"`
	Message struct {
		Role    string `json:"role"`
		Content []struct {
			Type    string `json:"type"`
			Text    string `json:"text"`
			Content string `json:"content"`
		} `json:"content"`
		Usage struct {
			InputTokens         int `json:"input_tokens"`
			CacheCreationTokens int `json:"cache_creation_input_tokens"`
			CacheReadTokens     int `json:"cache_read_input_tokens"`
			OutputTokens        int `json:"output_tokens"`
			CacheCreation       struct {
				Ephemeral5mTokens int `json:"ephemeral_5m_input_tokens"`
				Ephemeral1hTokens int `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

type uiModel struct {
	jobs            []uiJob
	total           int
	completed       int
	failed          int
	frame           int
	events          <-chan uiMsg
	onQuit          func()
	viewport        viewport.Model
	sidebarViewport viewport.Model
	selectedJob     int
	width           int
	height          int
	sidebarWidth    int
	mainWidth       int
	contentHeight   int
}

type uiMsg any

func newUIModel(total int) *uiModel {
	vp := viewport.New(80, 24)        // Increased initial height
	sidebarVp := viewport.New(30, 24) // Increased initial height
	defaultWidth := 120
	defaultHeight := 40
	initialSidebarWidth := int(float64(defaultWidth) * sidebarWidthRatio)
	if initialSidebarWidth < sidebarMinWidth {
		initialSidebarWidth = sidebarMinWidth
	}
	if initialSidebarWidth > sidebarMaxWidth {
		initialSidebarWidth = sidebarMaxWidth
	}
	initialMainWidth := defaultWidth - initialSidebarWidth
	if initialMainWidth < mainMinWidth {
		initialMainWidth = mainMinWidth
	}
	initialContentHeight := defaultHeight - chromeHeight
	if initialContentHeight < minContentHeight {
		initialContentHeight = minContentHeight
	}
	return &uiModel{
		total:           total,
		viewport:        vp,
		sidebarViewport: sidebarVp,
		selectedJob:     0,
		width:           defaultWidth,
		height:          defaultHeight,
		sidebarWidth:    initialSidebarWidth,
		mainWidth:       initialMainWidth,
		contentHeight:   initialContentHeight,
	}
}

func (m *uiModel) setEventSource(ch <-chan uiMsg) { m.events = ch }

func (m *uiModel) Init() tea.Cmd {
	return tea.Batch(m.waitEvent(), m.tick())
}

func (m *uiModel) waitEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	return func() tea.Msg {
		if ev, ok := <-m.events; ok {
			return ev
		}
		return drainMsg{}
	}
}

func (m *uiModel) tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case tea.KeyMsg:
		cmd := m.handleKey(v)
		return m, cmd
	case tea.WindowSizeMsg:
		m.handleWindowSize(v)
		return m, nil
	case tickMsg:
		cmd := m.handleTick()
		return m, cmd
	case jobQueuedMsg:
		cmd := m.handleJobQueued(&v)
		return m, cmd
	case jobStartedMsg:
		cmd := m.handleJobStarted(v)
		return m, cmd
	case jobFinishedMsg:
		cmd := m.handleJobFinished(v)
		return m, cmd
	case jobLogUpdateMsg:
		cmd := m.handleJobLogUpdate(v)
		return m, cmd
	case tokenUsageUpdateMsg:
		cmd := m.handleTokenUsageUpdate(v)
		return m, cmd
	case drainMsg:
		return m, nil
	default:
		return m, nil
	}
}

func (m *uiModel) handleKey(v tea.KeyMsg) tea.Cmd {
	switch v.String() {
	case "ctrl+c", "q":
		if m.onQuit != nil {
			m.onQuit()
		}
		return tea.Quit
	case "up", "k":
		if m.selectedJob > 0 {
			m.selectedJob--
		}
		return nil
	case "down", "j":
		if m.selectedJob < len(m.jobs)-1 {
			m.selectedJob++
		}
		return nil
	case "left", "h":
		m.viewport.ScrollUp(1)
		return nil
	case "right", "l":
		m.viewport.ScrollDown(1)
		return nil
	case "pgup", "b", "u":
		m.viewport.HalfPageUp()
		return nil
	case "pgdown", "f", "d":
		m.viewport.HalfPageDown()
		return nil
	case "home":
		m.viewport.GotoTop()
		return nil
	case "end":
		m.viewport.GotoBottom()
		return nil
	default:
		return m.waitEvent()
	}
}

func (m *uiModel) handleWindowSize(v tea.WindowSizeMsg) {
	m.width = v.Width
	m.height = v.Height
	sidebarWidth, mainWidth := m.computePaneWidths(v.Width)
	contentHeight := m.computeContentHeight(v.Height)
	m.configureViewports(sidebarWidth, mainWidth, contentHeight)
	m.sidebarWidth = sidebarWidth
	m.mainWidth = mainWidth
	m.contentHeight = contentHeight
}

func (m *uiModel) computePaneWidths(totalWidth int) (int, int) {
	sidebar := m.initialSidebarWidth(totalWidth)
	main := totalWidth - sidebar
	if main < mainMinWidth {
		main = mainMinWidth
		if main >= totalWidth {
			main = totalWidth - sidebarMinWidth
		}
		sidebar = totalWidth - main
		if sidebar < sidebarMinWidth {
			sidebar = sidebarMinWidth
			main = totalWidth - sidebar
		}
	}
	if main < 0 {
		main = 0
	}
	return sidebar, main
}

func (m *uiModel) initialSidebarWidth(totalWidth int) int {
	sidebar := int(float64(totalWidth) * sidebarWidthRatio)
	if sidebar < sidebarMinWidth {
		sidebar = sidebarMinWidth
	}
	if sidebar > sidebarMaxWidth {
		sidebar = sidebarMaxWidth
	}
	if sidebar >= totalWidth {
		sidebar = totalWidth / 2
	}
	return sidebar
}

func (m *uiModel) computeContentHeight(totalHeight int) int {
	content := totalHeight - chromeHeight
	if content < minContentHeight {
		return minContentHeight
	}
	return content
}

func (m *uiModel) configureViewports(sidebarWidth, mainWidth, contentHeight int) {
	sidebarViewportWidth := sidebarWidth - sidebarChromeWidth
	if sidebarViewportWidth < 10 {
		sidebarViewportWidth = 10
	}
	sidebarViewportHeight := contentHeight - sidebarChromeHeight
	if sidebarViewportHeight < sidebarViewportMinRows {
		sidebarViewportHeight = sidebarViewportMinRows
	}
	m.sidebarViewport.Width = sidebarViewportWidth
	if m.sidebarViewport.YOffset > sidebarViewportHeight {
		m.sidebarViewport.SetYOffset(sidebarViewportHeight)
	}
	m.sidebarViewport.Height = sidebarViewportHeight
	mainViewportWidth := mainWidth - mainHorizontalPadding
	if mainViewportWidth < 10 {
		mainViewportWidth = 10
	}
	m.viewport.Width = mainViewportWidth
	if contentHeight < logViewportMinHeight {
		m.viewport.Height = logViewportMinHeight
	} else {
		m.viewport.Height = contentHeight
	}
}

func (m *uiModel) refreshViewportContent() {
	if len(m.jobs) == 0 {
		m.viewport.SetContent("")
		return
	}
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		m.selectedJob = 0
	}
	m.updateViewportForJob(&m.jobs[m.selectedJob])
}

func (m *uiModel) selectNextRunningJob() {
	for i := range m.jobs {
		if m.jobs[i].state == jobRunning {
			m.selectedJob = i
			return
		}
	}
	for i := range m.jobs {
		if m.jobs[i].state == jobPending {
			m.selectedJob = i
			return
		}
	}
}

func (m *uiModel) handleTick() tea.Cmd {
	m.frame++
	if m.completed+m.failed < m.total {
		return m.tick()
	}
	return nil
}

func (m *uiModel) handleJobQueued(v *jobQueuedMsg) tea.Cmd {
	if v.Index >= len(m.jobs) {
		grow := v.Index - len(m.jobs) + 1
		m.jobs = append(m.jobs, make([]uiJob, grow)...)
	}
	m.jobs[v.Index] = uiJob{
		codeFile:  v.CodeFile,
		codeFiles: v.CodeFiles,
		issues:    v.Issues,
		safeName:  v.SafeName,
		outLog:    v.OutLog,
		errLog:    v.ErrLog,
		state:     jobPending,
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobStarted(v jobStartedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
		job.state = jobRunning
		if job.startedAt.IsZero() {
			job.startedAt = time.Now()
			job.duration = 0
		}
		m.selectedJob = v.Index
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobFinished(v jobFinishedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		job := &m.jobs[v.Index]
		if v.Success {
			job.state = jobSuccess
			m.completed++
		} else {
			job.state = jobFailed
			job.exitCode = v.ExitCode
			m.failed++
		}
		if !job.startedAt.IsZero() {
			job.completedAt = time.Now()
			job.duration = job.completedAt.Sub(job.startedAt)
		}
		m.selectNextRunningJob()
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobLogUpdate(v jobLogUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.jobs[v.Index].lastOut = v.Out
		m.jobs[v.Index].lastErr = v.Err
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleTokenUsageUpdate(v tokenUsageUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		if m.jobs[v.Index].tokenUsage == nil {
			m.jobs[v.Index].tokenUsage = &TokenUsage{}
		}
		m.jobs[v.Index].tokenUsage.Add(v.Usage)
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *uiModel) View() string {
	header, headerStyle := m.renderHeader()
	helpText, helpStyle := m.renderHelp()
	separator := m.renderSeparator()
	splitView := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderSidebar(),
		m.renderMainContent(),
	)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerStyle.Render(header),
		helpStyle.Render(helpText),
		separator,
		splitView,
	)
}

// renderHeader returns the header content and styling based on job progress.
func (m *uiModel) renderHeader() (string, lipgloss.Style) {
	complete := m.completed+m.failed >= m.total
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		MarginTop(1).
		MarginBottom(1)
	if !complete {
		msg := fmt.Sprintf("Processing Jobs: %d/%d completed, %d failed", m.completed, m.total, m.failed)
		return msg, style
	}
	if m.failed > 0 {
		style = style.Foreground(lipgloss.Color("220"))
		return fmt.Sprintf("✓ All Jobs Complete: %d/%d succeeded, %d failed", m.completed, m.total, m.failed), style
	}
	style = style.Foreground(lipgloss.Color("42"))
	return fmt.Sprintf("✓ All Jobs Complete: %d/%d succeeded!", m.completed, m.total), style
}

// renderHelp returns the help text and style depending on job completion status.
func (m *uiModel) renderHelp() (string, lipgloss.Style) {
	complete := m.completed+m.failed >= m.total
	text := "↑↓/jk navigate • pgup/pgdn scroll logs • q quit"
	if complete {
		text = "↑↓/jk navigate • pgup/pgdn scroll logs • Press 'q' to exit"
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		MarginBottom(1)
	return text, style
}

// renderSeparator draws a horizontal separator sized to the current viewport width.
func (m *uiModel) renderSeparator() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Render(strings.Repeat("─", m.width))
}

func (m *uiModel) renderSidebar() string {
	sidebarWidth := m.sidebarWidth
	if sidebarWidth <= 0 {
		sidebarWidth = int(float64(m.width) * sidebarWidthRatio)
		if sidebarWidth < sidebarMinWidth {
			sidebarWidth = sidebarMinWidth
		}
		if sidebarWidth > sidebarMaxWidth {
			sidebarWidth = sidebarMaxWidth
		}
	}
	contentHeight := m.contentHeight
	if contentHeight < minContentHeight {
		contentHeight = minContentHeight
	}
	var items []string
	for i := range m.jobs {
		item := m.renderSidebarItem(&m.jobs[i], i == m.selectedJob)
		items = append(items, item)
	}
	m.sidebarViewport.SetContent(strings.Join(items, "\n"))
	if m.selectedJob >= 0 && m.selectedJob < len(m.jobs) {
		lineOffset := m.selectedJob * 3
		if lineOffset > m.sidebarViewport.YOffset+m.sidebarViewport.Height-3 {
			m.sidebarViewport.SetYOffset(lineOffset - m.sidebarViewport.Height + 3)
		} else if lineOffset < m.sidebarViewport.YOffset {
			m.sidebarViewport.SetYOffset(lineOffset)
		}
	}
	sidebar := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(contentHeight).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(0, 1). // Reduced vertical padding
		Render(m.sidebarViewport.View())
	return sidebar
}

func (m *uiModel) renderSidebarItem(job *uiJob, selected bool) string {
	var icon string
	var color lipgloss.Color
	switch job.state {
	case jobPending:
		icon = "⏸"
		color = lipgloss.Color("245")
	case jobRunning:
		icon = spinnerFrames[m.frame%len(spinnerFrames)]
		color = lipgloss.Color("220")
	case jobSuccess:
		icon = "✓"
		color = lipgloss.Color("42")
	case jobFailed:
		icon = "✗"
		color = lipgloss.Color("196")
	}
	style := lipgloss.NewStyle().Foreground(color)
	if selected {
		style = style.Bold(true).Background(lipgloss.Color("235")).Foreground(lipgloss.Color("255"))
	}
	line1 := fmt.Sprintf("%s %s", icon, job.safeName)
	line2 := fmt.Sprintf("  %d file(s), %d issue(s)", len(job.codeFiles), job.issues)
	if selected {
		line1 = "► " + line1
	} else {
		line1 = "  " + line1
	}
	return style.Render(line1) + "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(line2)
}

func (m *uiModel) renderMainContent() string {
	if m.selectedJob < 0 || m.selectedJob >= len(m.jobs) {
		emptyMsg := "Select a job from the sidebar"
		return lipgloss.NewStyle().
			Padding(2).
			Foreground(lipgloss.Color("245")).
			Render(emptyMsg)
	}
	job := &m.jobs[m.selectedJob]
	mainWidth, contentHeight := m.mainDimensions()
	metaBlock := m.buildMetaBlock(job)
	logsHeader := m.renderLogsHeader()
	m.viewport.Height = m.availableLogHeight(contentHeight, metaBlock, logsHeader)
	m.updateViewportForJob(job)
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		metaBlock,
		logsHeader,
		m.viewport.View(),
	)
	return lipgloss.NewStyle().
		Width(mainWidth).
		Height(contentHeight).
		Padding(0, 1).
		Render(body)
}

// buildMetaBlock assembles the summary sections shown above the log viewport.
func (m *uiModel) buildMetaBlock(job *uiJob) string {
	sections := []string{m.renderMainHeader(job)}
	if fileList := strings.TrimSpace(m.renderMainFileList(job)); fileList != "" {
		sections = append(sections, fileList)
	}
	sections = append(sections, m.renderMainStatus(job), m.renderRuntime(job))
	if usage := strings.TrimSpace(m.renderTokenUsage(job)); usage != "" {
		sections = append(sections, usage)
	}
	if paths := strings.TrimSpace(m.renderLogPaths(job)); paths != "" {
		sections = append(sections, paths)
	}
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// availableLogHeight determines the viewport height available for streaming logs.
func (m *uiModel) availableLogHeight(contentHeight int, metaBlock, logsHeader string) int {
	usedHeight := lipgloss.Height(metaBlock) + lipgloss.Height(logsHeader)
	available := contentHeight - usedHeight
	if available < logViewportMinHeight {
		return logViewportMinHeight
	}
	return available
}

func (m *uiModel) mainDimensions() (int, int) {
	contentHeight := m.contentHeight
	if contentHeight < minContentHeight {
		contentHeight = minContentHeight
	}
	mainWidth := m.mainWidth
	if mainWidth <= 0 {
		mainWidth = int(float64(m.width) * (1 - sidebarWidthRatio))
	}
	if mainWidth < mainMinWidth {
		mainWidth = mainMinWidth
	}
	return mainWidth, contentHeight
}

func (m *uiModel) renderMainHeader(job *uiJob) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		MarginBottom(1).
		Render(fmt.Sprintf("Batch: %s", job.safeName))
}

func (m *uiModel) renderMainFileList(job *uiJob) string {
	if len(job.codeFiles) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Files:\n")
	for _, file := range job.codeFiles {
		b.WriteString("  • ")
		b.WriteString(file)
		b.WriteString("\n")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		MarginBottom(1).
		Render(b.String())
}

func (m *uiModel) renderMainStatus(job *uiJob) string {
	statusLabel := m.getStateLabel(job.state)
	if job.state == jobFailed && job.exitCode != 0 {
		statusLabel = fmt.Sprintf("%s (exit %d)", statusLabel, job.exitCode)
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("81")).
		MarginBottom(1).
		Render(fmt.Sprintf(
			"Issues: %d  |  Status: %s",
			job.issues,
			statusLabel,
		))
}

func (m *uiModel) renderLogsHeader() string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		MarginBottom(1).
		Render("Live Logs:")
}

func (m *uiModel) renderRuntime(job *uiJob) string {
	var label string
	var duration time.Duration
	switch job.state {
	case jobRunning:
		label = "Runtime"
		if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	case jobSuccess:
		label = "Completed in"
		if job.duration > 0 {
			duration = job.duration
		} else if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	case jobFailed:
		label = "Ran for"
		if job.duration > 0 {
			duration = job.duration
		} else if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	default:
		label = "Runtime"
		if !job.startedAt.IsZero() {
			duration = time.Since(job.startedAt)
		}
	}
	var rendered string
	if duration <= 0 {
		rendered = fmt.Sprintf("%s: --:--", label)
	} else {
		rendered = fmt.Sprintf("%s: %s", label, formatDuration(duration))
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("117")).
		MarginBottom(1).
		Render(rendered)
}

func (m *uiModel) renderLogPaths(job *uiJob) string {
	var lines []string
	if job.outLog != "" {
		lines = append(lines, fmt.Sprintf("  • stdout: %s", job.outLog))
	}
	if job.errLog != "" {
		lines = append(lines, fmt.Sprintf("  • stderr: %s", job.errLog))
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		MarginBottom(1).
		Render("Log Files:\n" + strings.Join(lines, "\n"))
}

func (m *uiModel) renderTokenUsage(job *uiJob) string {
	if job.tokenUsage == nil {
		return "" // No token usage data (not using Claude or no data yet)
	}
	usage := job.tokenUsage
	var lines []string
	lines = append(lines,
		"Token Usage (Claude API):",
		fmt.Sprintf("  Input:          %s tokens", formatNumber(usage.InputTokens)))
	if usage.CacheReadTokens > 0 {
		lines = append(lines, fmt.Sprintf("  Cache Reads:    %s tokens", formatNumber(usage.CacheReadTokens)))
	}
	if usage.CacheCreationTokens > 0 {
		lines = append(lines, fmt.Sprintf("  Cache Creation: %s tokens", formatNumber(usage.CacheCreationTokens)))
	}
	lines = append(lines,
		fmt.Sprintf("  Output:         %s tokens", formatNumber(usage.OutputTokens)),
		fmt.Sprintf("  Total:          %s tokens", formatNumber(usage.Total())))
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")).
		MarginBottom(1).
		Render(strings.Join(lines, "\n"))
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

// formatNumber formats an integer with comma separators for thousands
func formatNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var result strings.Builder
	mod := len(s) % 3
	if mod > 0 {
		result.WriteString(s[:mod])
		if len(s) > mod {
			result.WriteString(",")
		}
	}
	for i := mod; i < len(s); i += 3 {
		if i > mod {
			result.WriteString(",")
		}
		result.WriteString(s[i : i+3])
	}
	return result.String()
}

func (m *uiModel) updateViewportForJob(job *uiJob) {
	var content strings.Builder
	if len(job.lastOut) > 0 {
		for _, line := range job.lastOut {
			if line != "" {
				content.WriteString(line)
				content.WriteString("\n")
			}
		}
	}
	if len(job.lastErr) > 0 {
		stderrLabel := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render("[stderr]")
		content.WriteString(stderrLabel)
		content.WriteString("\n")
		for _, line := range job.lastErr {
			if line != "" {
				content.WriteString(line)
				content.WriteString("\n")
			}
		}
	}
	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom() // Auto-scroll to latest
	if len(job.lastOut) == 0 && len(job.lastErr) == 0 {
		m.viewport.GotoTop()
	}
}

func (m *uiModel) getStateLabel(state jobState) string {
	switch state {
	case jobPending:
		return "Pending"
	case jobRunning:
		return "Running"
	case jobSuccess:
		return "Success ✓"
	case jobFailed:
		return "Failed ✗"
	default:
		return "Unknown"
	}
}

func assertIDEExists(ide string) error {
	if _, err := exec.LookPath(ide); err != nil {
		return fmt.Errorf("%s CLI not found on PATH", ide)
	}
	return nil
}

func assertExecSupported(ide string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	switch ide {
	case ideCodex:
		cmd = exec.CommandContext(ctx, ideCodex, "exec", "-h")
	case ideClaude:
		cmd = exec.CommandContext(ctx, ideClaude, "--help")
	case ideDroid:
		cmd = exec.CommandContext(ctx, ideDroid, "exec", "--help")
	default:
		return fmt.Errorf("unsupported IDE: %s", ide)
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s CLI does not appear to be properly installed or configured", ide)
	}
	return nil
}

func parseTaskFile(content string) taskEntry {
	task := taskEntry{content: content}
	statusRe := regexp.MustCompile(`(?m)^##\s*status:\s*(\w+)`)
	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		task.status = strings.TrimSpace(m[1])
	}
	contextStart := strings.Index(content, "<task_context>")
	contextEnd := strings.Index(content, "</task_context>")
	if contextStart > 0 && contextEnd > contextStart {
		contextBlock := content[contextStart : contextEnd+15]
		task.domain = extractXMLTag(contextBlock, "domain")
		task.taskType = extractXMLTag(contextBlock, "type")
		task.scope = extractXMLTag(contextBlock, "scope")
		task.complexity = extractXMLTag(contextBlock, "complexity")
		if deps := extractXMLTag(contextBlock, "dependencies"); deps != "none" {
			task.dependencies = strings.Split(deps, ",")
			for i := range task.dependencies {
				task.dependencies[i] = strings.TrimSpace(task.dependencies[i])
			}
		}
	}
	return task
}

func extractXMLTag(content, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`<%s>(.*?)</%s>`, tag, tag))
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func isTaskCompleted(task *taskEntry) bool {
	status := strings.ToLower(task.status)
	return status == "completed" || status == "done" || status == "finished"
}

func readIssueEntries(resolvedIssuesDir string, mode executionMode, includeCompleted bool) ([]issueEntry, error) {
	if mode == ExecutionModePRDTasks {
		return readTaskEntries(resolvedIssuesDir, includeCompleted)
	}
	return readCodeRabbitIssues(resolvedIssuesDir)
}

func readTaskEntries(tasksDir string, includeCompleted bool) ([]issueEntry, error) {
	entries := []issueEntry{}
	files, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if !f.Type().IsRegular() || !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		if !reTaskFile.MatchString(f.Name()) {
			continue
		}
		names = append(names, f.Name())
	}
	sort.Strings(names)
	for _, name := range names {
		absPath := filepath.Join(tasksDir, name)
		b, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}
		content := string(b)
		task := parseTaskFile(content)
		if !includeCompleted && isTaskCompleted(&task) {
			continue
		}
		entries = append(entries, issueEntry{
			name:     name,
			absPath:  absPath,
			content:  content,
			codeFile: strings.TrimSuffix(name, ".md"),
		})
	}
	return entries, nil
}

func readCodeRabbitIssues(resolvedIssuesDir string) ([]issueEntry, error) {
	entries := []issueEntry{}
	files, err := os.ReadDir(resolvedIssuesDir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(files))
	for _, f := range files {
		if !f.Type().IsRegular() {
			continue
		}
		if f.Name() == "_summary.md" {
			continue
		}
		if strings.HasSuffix(f.Name(), ".md") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		absPath := filepath.Join(resolvedIssuesDir, name)
		b, err := os.ReadFile(absPath)
		if err != nil {
			return nil, err
		}
		content := string(b)
		cf := extractCodeFileFromIssue(content)
		if cf == "" {
			cf = "__unknown__:" + name
		}
		entries = append(entries, issueEntry{name: name, absPath: absPath, content: content, codeFile: cf})
	}
	return entries, nil
}

// filterUnresolved removes issues already marked as resolved.
func filterUnresolved(all []issueEntry) []issueEntry {
	out := make([]issueEntry, 0, len(all))
	for _, e := range all {
		if !isIssueResolved(e.content) {
			out = append(out, e)
		}
	}
	return out
}

// isIssueResolved checks common markers used by our PR-review flow.
// Heuristics (case-insensitive):
// - A literal "RESOLVED ✓" anywhere in the file
// - A line starting with "Status: RESOLVED" or "State: RESOLVED"
// - A checked task list item like "- [x] resolved"
var (
	reResolvedStatus = regexp.MustCompile(`(?mi)^\s*(status|state)\s*:\s*resolved\b`)
	reResolvedTask   = regexp.MustCompile(`(?mi)^\s*-\s*\[(x|X)\]\s*resolved\b`)
	reTaskFile       = regexp.MustCompile(`^_task_\d+\.md$`)
)

func isIssueResolved(content string) bool {
	if strings.Contains(strings.ToUpper(content), "RESOLVED ✓") {
		return true
	}
	if reResolvedStatus.FindStringIndex(content) != nil {
		return true
	}
	if reResolvedTask.FindStringIndex(content) != nil {
		return true
	}
	return false
}

func groupIssues(entries []issueEntry) map[string][]issueEntry {
	groups := make(map[string][]issueEntry)
	for _, it := range entries {
		groups[it.codeFile] = append(groups[it.codeFile], it)
	}
	return groups
}

func writeGroupedSummaries(groupedDir string, groups map[string][]issueEntry) error {
	for codeFile, items := range groups {
		safeName := safeFileName(func() string {
			if strings.HasPrefix(codeFile, "__unknown__") {
				return unknownFileName
			}
			return codeFile
		}())
		groupFile := filepath.Join(groupedDir, fmt.Sprintf("%s.md", safeName))
		header := fmt.Sprintf("# Issue Group for %s\n\n", func() string {
			if strings.HasPrefix(codeFile, "__unknown__") {
				return "(unknown file)"
			}
			return codeFile
		}())
		var sb strings.Builder
		sb.WriteString(header)
		sb.WriteString(buildGroupedResolutionChecklist(items))
		sb.WriteString("## Included Issues\n\n")
		for _, it := range items {
			sb.WriteString("- ")
			sb.WriteString(it.name)
			sb.WriteString("\n")
		}
		for _, it := range items {
			sb.WriteString("\n---\n\n## ")
			sb.WriteString(it.name)
			sb.WriteString("\n\n")
			sb.WriteString(it.content)
		}
		sb.WriteString("\n")
		if err := os.WriteFile(groupFile, []byte(sb.String()), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func buildGroupedResolutionChecklist(items []issueEntry) string {
	var checklist strings.Builder
	checklist.WriteString("## Resolution Checklist\n\n")
	checklist.WriteString(
		"> ⚠️ This grouped issue contains multiple unresolved review tasks for the same source file.\n",
	)
	checklist.WriteString("> Resolve **every** task below before treating this file as complete.\n")
	checklist.WriteString(
		"> After resolving a task, update the original issue file with " + "`RESOLVED ✓`" + " and run any provided gh command.\n\n",
	)
	for _, it := range items {
		checklist.WriteString("- [ ] Resolve `")
		checklist.WriteString(it.name)
		checklist.WriteString("` (source issue: `")
		checklist.WriteString(normalizeForPrompt(it.absPath))
		checklist.WriteString("`)\n")
		checklist.WriteString(
			"      - Apply the requested code changes and update the issue status to " + "`RESOLVED ✓`" + ".\n",
		)
		checklist.WriteString("      - Run the review thread command if a Thread ID is provided.\n")
	}
	checklist.WriteString(
		"- [ ] Document the fixes in this grouped file and tick every checklist item above.\n\n",
	)
	return checklist.String()
}

func normalizeForPrompt(absPath string) string {
	var err error
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return absPath // fallback to original path if abs fails
	}
	cwd, err := os.Getwd()
	if err != nil {
		return absPath // fallback to original path if cwd fails
	}
	cwd = filepath.Clean(cwd)
	absPath = filepath.Clean(absPath)
	pref := cwd + string(os.PathSeparator)
	if strings.HasPrefix(absPath, pref) {
		return absPath[len(pref):]
	}
	return absPath
}

func inferPrFromIssuesDir(dir string) (string, error) {
	re := regexp.MustCompile(`reviews-pr-(\d+)`)
	m := re.FindStringSubmatch(dir)
	if len(m) < 2 {
		return "", errors.New("unable to infer PR number from issues dir")
	}
	return m[1], nil
}

func extractCodeFileFromIssue(content string) string {
	re := regexp.MustCompile(`\*\*File:\*\*\s*` + "`" + `([^` + "`" + `]+)` + "`")
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	raw := strings.TrimSpace(m[1])
	if idx := strings.LastIndex(raw, ":"); idx != -1 {
		tail := raw[idx+1:]
		if tail != "" && isAllDigits(tail) {
			raw = strings.TrimSpace(raw[:idx])
		}
	}
	return raw
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func sanitizePath(p string) string {
	b := make([]rune, 0, len(p))
	for _, r := range p {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' ||
			r == '-' {
			b = append(b, r)
		} else {
			b = append(b, '_')
		}
	}
	return string(b)
}

func safeFileName(p string) string {
	norm := strings.ReplaceAll(p, "\\", "/")
	base := sanitizePath(norm)
	sum := sha256.Sum256([]byte(norm))
	h := hex.EncodeToString(sum[:])[:6]
	return fmt.Sprintf("%s-%s", base, h)
}

// Prompt builders

type buildBatchedIssuesParams struct {
	PR          string
	BatchGroups map[string][]issueEntry
	Grouped     bool
	Mode        executionMode
}

func buildBatchedIssuesPrompt(p buildBatchedIssuesParams) string {
	if p.Mode == ExecutionModePRDTasks {
		return buildPRDTasksPrompt(p)
	}
	return buildCodeReviewPrompt(p)
}

func buildPRDTasksPrompt(p buildBatchedIssuesParams) string {
	var taskEntry issueEntry
	for _, items := range p.BatchGroups {
		if len(items) > 0 {
			taskEntry = items[0]
			break
		}
	}
	return buildPRDTaskPrompt(taskEntry)
}

func buildCodeReviewPrompt(p buildBatchedIssuesParams) string {
	codeFiles := sortCodeFiles(p.BatchGroups)
	helperCommands := buildHelperCommands(p.PR, p.BatchGroups)
	header := buildBatchHeader(p.PR, codeFiles, p.BatchGroups)
	critical := buildBatchCritical(p.PR, codeFiles, p.Grouped)
	batchNotice := buildBatchNotice(codeFiles)
	issueGroups := buildIssueGroups(p.BatchGroups)
	task := buildBatchTask(p.PR, codeFiles, p.Grouped)
	testingReqs := buildTestingRequirements()
	checklist := buildBatchChecklist(p.PR, p.BatchGroups, p.Grouped)
	composed := strings.Join([]string{helperCommands, header, critical, batchNotice,
		issueGroups, task, testingReqs, checklist}, "\n\n")
	return composed
}

func buildPRDTaskPrompt(task issueEntry) string {
	taskData := parseTaskFile(task.content)
	prdDir := filepath.Dir(task.absPath)
	tasksFile := filepath.Join(prdDir, "_tasks.md")
	header := fmt.Sprintf("# Implementation Task: %s\n\n", task.name)
	contextSection := buildTaskContextSection(&taskData)
	criticalSection := buildCriticalExecutionSection()
	specSection := fmt.Sprintf("## Task Specification\n\n%s\n\n", task.content)
	implSection := buildImplementationInstructionsSection(prdDir)
	completionSection := buildCompletionCriteriaSection(task.absPath, tasksFile, task.name)
	return header + contextSection + criticalSection + specSection + implSection + completionSection
}

func buildTaskContextSection(taskData *taskEntry) string {
	var sb strings.Builder
	sb.WriteString("## Task Context\n\n")
	sb.WriteString(fmt.Sprintf("- **Domain**: %s\n", taskData.domain))
	sb.WriteString(fmt.Sprintf("- **Type**: %s\n", taskData.taskType))
	sb.WriteString(fmt.Sprintf("- **Scope**: %s\n", taskData.scope))
	sb.WriteString(fmt.Sprintf("- **Complexity**: %s\n", taskData.complexity))
	if len(taskData.dependencies) > 0 {
		sb.WriteString(fmt.Sprintf("- **Dependencies**: %s\n", strings.Join(taskData.dependencies, ", ")))
	}
	sb.WriteString("\n")
	return sb.String()
}

func buildCriticalExecutionSection() string {
	var sb strings.Builder
	sb.WriteString("<CRITICAL>\n")
	sb.WriteString("**EXECUTION MODE: ONE-SHOT DIRECT IMPLEMENTATION**\n\n")
	sb.WriteString("You MUST complete this task in ONE continuous execution from beginning to end:\n\n")
	sb.WriteString("- **NO ASKING**: Do not ask for clarification, confirmation, or approval\n")
	sb.WriteString("- **NO PLANNING MODE**: Execute directly without presenting plans\n")
	sb.WriteString("- **NO PARTIAL WORK**: Complete ALL requirements, subtasks, and deliverables\n")
	sb.WriteString("- **FOLLOW ALL STANDARDS**: Adhere to ALL project rules in `.cursor/rules/`\n")
	sb.WriteString("- **BEST PRACTICES ONLY**: No workarounds, hacks, or shortcuts\n")
	sb.WriteString("- **PROPER SOLUTIONS**: Implement production-ready, maintainable code\n\n")
	sb.WriteString("**VALIDATION REQUIREMENTS**:\n")
	sb.WriteString("- All tests MUST pass (`make test`)\n")
	sb.WriteString("- All linting MUST pass (`make lint`)\n")
	sb.WriteString("- All type checking MUST pass (`make typecheck`)\n")
	sb.WriteString("- All subtasks MUST be marked complete\n")
	sb.WriteString("- Task status MUST be updated to 'completed'\n\n")
	sb.WriteString("⚠️  **WORK WILL BE INVALIDATED** if:\n")
	sb.WriteString("- Any requirement is incomplete\n")
	sb.WriteString("- Tests/linting/typecheck fail\n")
	sb.WriteString("- Project standards are violated\n")
	sb.WriteString("- Workarounds are used instead of proper solutions\n")
	sb.WriteString("- Task completion steps are skipped\n")
	sb.WriteString("</CRITICAL>\n\n")
	return sb.String()
}

func buildImplementationInstructionsSection(prdDir string) string {
	var sb strings.Builder
	sb.WriteString("## Implementation Instructions\n\n")
	sb.WriteString("<critical>\n")
	sb.WriteString("**MANDATORY READ BEFORE STARTING**:\n")
	sb.WriteString("- @.cursor/rules/critical-validation.mdc\n")
	sb.WriteString(fmt.Sprintf("- All documents from this PRD directory: `%s`\n", prdDir))
	sb.WriteString("  - Especially review `_techspec.md` and `_tasks.md` for full context\n")
	sb.WriteString("</critical>\n\n")
	return sb.String()
}

func buildCompletionCriteriaSection(taskAbsPath, tasksFile, taskName string) string {
	var sb strings.Builder
	sb.WriteString("## Completion Criteria\n\n")
	sb.WriteString("After implementation, you MUST complete ALL of the following steps:\n\n")
	sb.WriteString("1. **Verify Implementation**:\n")
	sb.WriteString("   - All subtasks in this task file are completed\n")
	sb.WriteString("   - All deliverables specified are produced\n")
	sb.WriteString("   - All tests pass: `make test`\n")
	sb.WriteString("   - Code passes linting: `make lint`\n\n")
	sb.WriteString("2. **Mark Subtasks Complete**:\n")
	sb.WriteString(fmt.Sprintf("   - In `%s`, check all `[ ]` boxes to `[x]` for completed subtasks\n\n", taskAbsPath))
	sb.WriteString("3. **Update Task Status**:\n")
	sb.WriteString(fmt.Sprintf("   - In `%s`, change the status line from:\n", taskAbsPath))
	sb.WriteString("     ```\n")
	sb.WriteString("     ## status: pending\n")
	sb.WriteString("     ```\n")
	sb.WriteString("     to:\n")
	sb.WriteString("     ```\n")
	sb.WriteString("     ## status: completed\n")
	sb.WriteString("     ```\n\n")
	sb.WriteString("4. **Update Master Tasks List**:\n")
	sb.WriteString(fmt.Sprintf("   - In `%s`, check the corresponding task checkbox for `%s`\n\n", tasksFile, taskName))
	sb.WriteString("5. **Commit Changes**:\n")
	sb.WriteString(
		fmt.Sprintf("   - Commit all changes with a descriptive message like: `feat: complete %s`\n\n", taskName),
	)
	sb.WriteString("<critical>\n")
	sb.WriteString("**DO NOT SKIP ANY COMPLETION STEPS**\n")
	sb.WriteString("Your task is NOT complete until ALL steps above are done, including:\n")
	sb.WriteString("- All subtask checkboxes marked\n")
	sb.WriteString("- Status changed to 'completed'\n")
	sb.WriteString("- Master tasks list updated\n")
	sb.WriteString("</critical>\n\n")
	return sb.String()
}

func buildHelperCommands(pr string, batchGroups map[string][]issueEntry) string {
	var issueNumbers []int
	for _, items := range batchGroups {
		for _, item := range items {
			parts := strings.SplitN(item.name, "-", 2)
			if len(parts) > 0 {
				if num := strings.TrimLeft(parts[0], "0"); num != "" {
					var issueNum int
					if _, err := fmt.Sscanf(num, "%d", &issueNum); err == nil {
						issueNumbers = append(issueNumbers, issueNum)
					}
				}
			}
		}
	}
	if len(issueNumbers) == 0 {
		return ""
	}
	sort.Ints(issueNumbers)
	minIssue := issueNumbers[0]
	maxIssue := issueNumbers[len(issueNumbers)-1]
	return fmt.Sprintf(`## Helper Commands

Before starting work on fixing issues, you can review them using:

`+"```bash"+`
# Read all issues in this batch (issues %d-%d)
scripts/read_pr_issues.sh --pr %s --type issue --from %d --to %d

# Read all issues for this PR
scripts/read_pr_issues.sh --pr %s --type issue --all
`+"```"+`

<critical>
- **YOU NEED** to fix ALL issues listed below from PR %s, and only finish when ALL are addressed
- This should be fixed in THE BEST WAY possible, not using workarounds
- **YOU MUST** follow project standards and rules from .cursor/rules, and ensure all parameters are addressed
- If, in the end, you don't have all issues addressed, your work will be **INVALIDATED**
- After making all the changes, you need to update the progress in all related issue files
- **MUST DO:** After resolving every issue run `+"`scripts/resolve_pr_issues.sh --pr-dir ai-docs/reviews-pr-%s --from %d --to %d`"+` so the script calls `+"`gh`"+` to close the review threads and refreshes the summary
</critical>`, minIssue, maxIssue, pr, minIssue, maxIssue, pr, pr, pr, minIssue, maxIssue)
}

func sortCodeFiles(batchGroups map[string][]issueEntry) []string {
	codeFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		codeFiles = append(codeFiles, codeFile)
	}
	sort.Strings(codeFiles)
	return codeFiles
}

func buildBatchHeader(pr string, codeFiles []string, batchGroups map[string][]issueEntry) string {
	totalIssues := 0
	for _, items := range batchGroups {
		totalIssues += len(items)
	}
	return fmt.Sprintf(`<arguments>
  <type>batched-issues</type>
  <pr>%s</pr>
  <files>%d</files>
  <total-issues>%d</total-issues>
</arguments>`, pr, len(codeFiles), totalIssues)
}

func buildBatchCritical(pr string, codeFiles []string, grouped bool) string {
	codeFileList := strings.Join(codeFiles, "\n  - ")
	progressFiles := fmt.Sprintf(`- Each included issue file under ai-docs/reviews-pr-%s/issues/`, pr)
	if grouped {
		progressFiles += fmt.Sprintf(`
  - The grouped files for each file in ai-docs/reviews-pr-%s/issues/grouped/`, pr)
	}
	return fmt.Sprintf(`
<critical>
- You MUST fix ALL issues listed below across MULTIPLE files in this batch.
- Implement proper solutions; do not use workarounds.
- Follow project standards in .cursor/rules.
- Files in this batch:
  - %s
- After making changes, update ONLY the progress files generated by pr-review for this PR:
%s
- MUST DO: If these are GitHub review issues, after resolving them you need to call the gh command to resolve each
  thread as per the instructions in the issue files (look for a "Thread ID:" line and use the provided gh command).
</critical>`, codeFileList, progressFiles)
}

func buildBatchNotice(codeFiles []string) string {
	return fmt.Sprintf(`
<important_batch_processing>
⚠️  BATCH PROCESSING MODE ⚠️

This batch contains issues from %d different files. You should:
- Address ALL issues across ALL files in this batch cohesively
- Consider interdependencies between files (e.g., shared types, utilities)
- Ensure changes are consistent across the codebase

Files in this batch: %s
</important_batch_processing>`, len(codeFiles), strings.Join(codeFiles, ", "))
}

func buildIssueGroups(batchGroups map[string][]issueEntry) string {
	allIssues := make([]issueEntry, 0)
	for _, items := range batchGroups {
		allIssues = append(allIssues, items...)
	}
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].name < allIssues[j].name
	})
	issuesByFile := make(map[string][]issueEntry)
	fileOrder := make([]string, 0)
	seenFiles := make(map[string]bool)
	for _, issue := range allIssues {
		if !seenFiles[issue.codeFile] {
			fileOrder = append(fileOrder, issue.codeFile)
			seenFiles[issue.codeFile] = true
		}
		issuesByFile[issue.codeFile] = append(issuesByFile[issue.codeFile], issue)
	}
	var issueGroupsBuilder strings.Builder
	issueGroupsBuilder.WriteString("\n<issues>\n")
	issueGroupsBuilder.WriteString(
		"Read and address ALL issues from the following files (in sequential issue order):\n\n",
	)
	for _, codeFile := range fileOrder {
		items := issuesByFile[codeFile]
		issueGroupsBuilder.WriteString(fmt.Sprintf("**%s** (%d issue%s):\n",
			codeFile, len(items), func() string {
				if len(items) == 1 {
					return ""
				}
				return "s"
			}()))
		for _, item := range items {
			relPath := normalizeForPrompt(item.absPath)
			issueGroupsBuilder.WriteString(fmt.Sprintf("  - %s\n", relPath))
		}
		issueGroupsBuilder.WriteString("\n")
	}
	issueGroupsBuilder.WriteString("</issues>")
	return issueGroupsBuilder.String()
}

func buildBatchTask(pr string, codeFiles []string, grouped bool) string {
	taskText := fmt.Sprintf(`
<task>
- Resolve ALL issues above across ALL %d files in a cohesive set of changes.
- In each included issue file under ai-docs/reviews-pr-%s/issues,
  update the status section/checkbox to RESOLVED ✓ when addressed.`, len(codeFiles), pr)
	if grouped {
		groupedFiles := make([]string, len(codeFiles))
		for i, codeFile := range codeFiles {
			groupedFiles[i] = fmt.Sprintf("ai-docs/reviews-pr-%s/issues/grouped/%s.md", pr, safeFileName(codeFile))
		}
		taskText += fmt.Sprintf(`
- Update the grouped tracking files for each file in this batch:
  %s`, strings.Join(groupedFiles, "\n  "))
	}
	taskText += fmt.Sprintf(`
- If a GitHub review thread ID is present in any issue,
  resolve it using gh as per the command snippet included in that issue.
- If documentation updates are required, include them.
- For any included issue that is already solved (no code change required),
  you MUST still apply the progress updates above:
  - mark the specific issue file as RESOLVED ✓,
  - resolve its GitHub review thread via gh if a Thread ID is present.
</task>

<after_finish>
- **MUST COMMIT:** After fixing ALL issues in this batch and ensuring make lint && make test pass,
  commit the changes with a descriptive message that references the PR and fixed issues.
  Example: `+"`git commit -am \"fix: resolve PR #%s issues [batch]\"`"+`
  Note: Commit locally only - do NOT push. Multiple batches will be committed separately.
</after_finish>`, pr)
	return taskText
}

func buildTestingRequirements() string {
	return `
<testing_and_linting_requirements>
### For tests and linting

- **MUST** run ` + "`make lint`" + ` and ` + "`make test`" + ` before completing ANY subtask
- **YOU CAN ONLY** finish a task if ` + "`make lint`" + ` and ` + "`make test`" + ` are passing, your task should not finish before this
- **TIP:** Since our project is big, **YOU SHOULD** run ` + "`make test`" + ` and ` + "`make lint`" + ` just at the end before finishing the task; during development, use scoped commands:
  - **Tests:** ` + "`gotestsum --format pkgname -- -race -parallel=4 <scope>`" + ` (e.g., ` + "`./engine/agent`" + `)
  - **Linting:** ` + "`golangci-lint run --fix --allow-parallel-runners <scope>`" + ` (e.g., ` + "`./engine/agent/...`" + `)
  - **IF YOUR SCOPE** is ` + "`.../.`" + ` then you need to run ` + "`make test`" + ` and ` + "`make lint`" + `
</testing_and_linting_requirements>`
}

func buildZenMCPGuidance() string {
	return `
<critical>
### For complex/big tasks

- **YOU MUST** use Zen MCP (with Gemini 2.5 Pro) debug, refactor, analyze or tracer
  (depends on the task and what the user prompt says to do) complex flow **BEFORE INITIATE A TASK**
- **YOU MUST** use Zen MCP (with Gemini 2.5 Pro) codereview tool **AFTER FINISH A TASK**
- **YOU MUST ALWAYS** show all recommendations/issues from a Zen MCP review,
  does not matter if they are related to your task or not, you **NEED TO ALWAYS** show them

### For small/simple issues

- **DO NOT** use Zen MCP tools for small, straightforward fixes
  (e.g., typos, simple refactors, adding comments)
- **USE YOUR JUDGMENT:** If the issue is clear and the fix is obvious,
  proceed directly without Zen MCP overhead
- Reserve Zen MCP for:
  - Complex logic changes requiring deep analysis
  - Multi-file refactorings with interdependencies
  - Performance optimization requiring tracing
  - Security-sensitive code requiring thorough review
  - Architecture decisions
</critical>`
}

func buildBatchChecklist(pr string, batchGroups map[string][]issueEntry, grouped bool) string {
	allIssues := make([]issueEntry, 0)
	for _, items := range batchGroups {
		allIssues = append(allIssues, items...)
	}
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].name < allIssues[j].name
	})
	var checklistPaths []string
	if grouped {
		seenGrouped := make(map[string]bool)
		for _, issue := range allIssues {
			groupedPath := fmt.Sprintf("ai-docs/reviews-pr-%s/issues/grouped/%s.md", pr, safeFileName(issue.codeFile))
			if !seenGrouped[groupedPath] {
				checklistPaths = append(checklistPaths, groupedPath)
				seenGrouped[groupedPath] = true
			}
		}
	}
	for _, item := range allIssues {
		checklistPaths = append(checklistPaths, normalizeForPrompt(item.absPath))
	}
	var chk strings.Builder
	chk.WriteString("\n<checklist>\n  <title>Progress Files to Update</title>\n")
	for _, path := range checklistPaths {
		chk.WriteString("  <path>")
		chk.WriteString(path)
		chk.WriteString("</path>\n")
	}
	chk.WriteString("</checklist>\n")
	return chk.String()
}

func exitCodeOf(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(interface{ ExitStatus() int }); ok {
			return status.ExitStatus()
		}
		return exitErr.ExitCode()
	}
	return -1
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// lineRing keeps the last N lines in insertion order (oldest->newest on Snapshot).
type lineRing struct {
	mu    sync.Mutex
	capN  int
	lines []string
}

func newLineRing(n int) *lineRing {
	if n <= 0 {
		n = 1
	}
	return &lineRing{capN: n, lines: make([]string, 0, n)}
}

func (r *lineRing) appendLine(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s == "" {
		return
	}
	r.lines = append(r.lines, s)
	if len(r.lines) > r.capN {
		r.lines = r.lines[len(r.lines)-r.capN:]
	}
}

func (r *lineRing) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.lines))
	copy(out, r.lines)
	return out
}

// activityMonitor tracks the last time output was received from a process.
// It enables activity-based timeout detection, where a job is considered
// stuck if no output is received within the configured timeout period.
type activityMonitor struct {
	mu           sync.Mutex
	lastActivity time.Time
}

func newActivityMonitor() *activityMonitor {
	return &activityMonitor{
		lastActivity: time.Now(),
	}
}

func (a *activityMonitor) recordActivity() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastActivity = time.Now()
}

func (a *activityMonitor) timeSinceLastActivity() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Since(a.lastActivity)
}

// uiLogTap is an io.Writer that splits by newlines, appends to a ring buffer
// and emits UI updates with the newest snapshots.
type uiLogTap struct {
	idx             int
	isErr           bool
	out             *lineRing
	err             *lineRing
	ch              chan<- uiMsg
	buf             []byte
	activityMonitor *activityMonitor
}

func newUILogTap(
	idx int,
	isErr bool,
	outRing, errRing *lineRing,
	ch chan<- uiMsg,
	monitor *activityMonitor,
) *uiLogTap {
	return &uiLogTap{
		idx:             idx,
		isErr:           isErr,
		out:             outRing,
		err:             errRing,
		ch:              ch,
		buf:             make([]byte, 0, 1024),
		activityMonitor: monitor,
	}
}

func (t *uiLogTap) Write(p []byte) (int, error) {
	if len(p) > 0 && t.activityMonitor != nil {
		t.activityMonitor.recordActivity()
	}
	cleaned := bytes.ReplaceAll(p, []byte{'\r'}, []byte{'\n'})
	t.buf = append(t.buf, cleaned...)
	for {
		i := bytes.IndexByte(t.buf, '\n')
		if i < 0 {
			break
		}
		line := string(bytes.TrimRight(t.buf[:i], "\r\n"))
		if t.isErr {
			t.err.appendLine(line)
		} else {
			t.out.appendLine(line)
		}
		t.buf = t.buf[i+1:]
	}
	select {
	case t.ch <- jobLogUpdateMsg{Index: t.idx, Out: t.out.snapshot(), Err: t.err.snapshot()}:
	default:
	}
	return len(p), nil
}

// jsonFormatter wraps an io.Writer and formats JSON lines with pretty printing and colors.
// Non-JSON lines are passed through unchanged.
// For Claude messages, it can optionally parse and report token usage via callback.
type jsonFormatter struct {
	w               io.Writer
	buf             []byte
	usageCallback   func(TokenUsage) // Called when Claude usage data is parsed
	activityMonitor *activityMonitor
}

func newJSONFormatterWithCallbackAndMonitor(
	w io.Writer,
	callback func(TokenUsage),
	monitor *activityMonitor,
) *jsonFormatter {
	return &jsonFormatter{
		w:               w,
		buf:             make([]byte, 0, 4096),
		usageCallback:   callback,
		activityMonitor: monitor,
	}
}

func (f *jsonFormatter) Write(p []byte) (int, error) {
	if len(p) > 0 && f.activityMonitor != nil {
		f.activityMonitor.recordActivity()
	}
	f.buf = append(f.buf, p...)
	for {
		i := bytes.IndexByte(f.buf, '\n')
		if i < 0 {
			break
		}
		line := bytes.TrimRight(f.buf[:i], "\r\n")
		f.buf = f.buf[i+1:]
		formatted := f.formatLine(line)
		if _, err := f.w.Write(append(formatted, '\n')); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (f *jsonFormatter) formatLine(line []byte) []byte {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return line
	}
	if !json.Valid(line) {
		return line
	}
	var msg ClaudeMessage
	if err := json.Unmarshal(line, &msg); err == nil {
		if f.usageCallback != nil {
			f.tryParseUsage(&msg)
		}

		if formatted := f.formatClaudeMessage(&msg); formatted != nil {
			return formatted
		}
	}
	formatted := pretty.Color(pretty.Pretty(line), nil)
	return formatted
}

// formatClaudeMessage extracts and formats the readable content from a Claude message
func (f *jsonFormatter) formatClaudeMessage(msg *ClaudeMessage) []byte {
	switch msg.Type {
	case "user", "assistant":
		if len(msg.Message.Content) > 0 {
			var contentParts []string
			for _, content := range msg.Message.Content {
				if content.Type == "text" && content.Text != "" {
					contentParts = append(contentParts, content.Text)
				} else if content.Type == "tool_result" && content.Content != "" {
					contentParts = append(contentParts, content.Content)
				}
			}
			if len(contentParts) > 0 {
				return []byte(strings.Join(contentParts, "\n"))
			}
		}
	case "system":
		return []byte(fmt.Sprintf("[System: %s]", msg.Type))
	}
	return nil // Return nil to trigger fallback to raw JSON
}

// tryParseUsage attempts to extract token usage from a Claude message
func (f *jsonFormatter) tryParseUsage(msg *ClaudeMessage) {
	if msg.Type != "assistant" {
		return
	}
	usage := msg.Message.Usage
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return // No meaningful usage data
	}
	tokenUsage := TokenUsage{
		InputTokens:         usage.InputTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		OutputTokens:        usage.OutputTokens,
		Ephemeral5mTokens:   usage.CacheCreation.Ephemeral5mTokens,
		Ephemeral1hTokens:   usage.CacheCreation.Ephemeral1hTokens,
	}
	f.usageCallback(tokenUsage)
}
