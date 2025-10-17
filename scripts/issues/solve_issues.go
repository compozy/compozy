package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const (
	unknownFileName = "unknown"
	ideCodex        = "codex"
	ideClaude       = "claude"
)

// Port of scripts/solve-pr-issues.ts with concurrency and non-streamed logging.
//
// Usage:
//   go run scripts/solve-pr-issues.go --pr 259
//   [--issues-dir ai-docs/<num>/issues] [--dry-run]
//   [--concurrent 4] [--batch-size 3] [--ide claude|codex]
//
// Behavior:
// - Scans issue markdown files under the issues dir, groups by the "**File:**`path:line`" header.
// - Writes grouped summaries to issues/grouped/<safe>.md and prompts to .tmp/codex-prompts/pr-<PR>/.
// - Batches multiple file groups together (controlled by --batch-size) for processing.
// - Invokes the specified IDE tool (codex or claude) once per batch, feeding the generated prompt via stdin.
// - By default, only writes process output to log files; does not stream to current stdout/stderr.
// - Supports parallel execution with --concurrent N (default 1).

type cliArgs struct {
	pr         string
	issuesDir  string
	dryRun     bool
	concurrent int
	batchSize  int
	ide        string
	model      string
}

type issueEntry struct {
	name     string
	absPath  string
	content  string
	codeFile string // repository-relative code file or "__unknown__:<filename>"
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
  solve-pr-issues --pr 259
  [--issues-dir ai-docs/<num>/issues] [--dry-run]
  [--concurrent 4] [--batch-size 3] [--ide claude|codex] [--model gpt-5]

Behavior:
- Scans issue markdown files under the issues dir, groups by the "**File:** path:line header.
- Writes grouped summaries to issues/grouped/<safe>.md and prompts to .tmp/codex-prompts/pr-<PR>/.
- Batches multiple file groups together (controlled by --batch-size) for processing.
- Invokes the specified IDE tool (codex or claude) once per batch, feeding the generated prompt via stdin.
- By default, only writes process output to log files; does not stream to current stdout/stderr.
- Supports parallel execution with --concurrent N (default 1).`,
	RunE: runSolveIssues,
}

var (
	pr         string
	issuesDir  string
	dryRun     bool
	concurrent int
	batchSize  int
	ide        string
	model      string
)

func setupFlags() {
	rootCmd.Flags().StringVar(&pr, "pr", "", "Pull request number")
	rootCmd.Flags().StringVar(&issuesDir, "issues-dir", "", "Path to issues directory (ai-docs/reviews-pr-<PR>/issues)")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Only generate prompts; do not run IDE tool")
	rootCmd.Flags().IntVar(&concurrent, "concurrent", 1, "Number of batches to process in parallel")
	rootCmd.Flags().
		IntVar(&batchSize, "batch-size", 1, "Number of file groups to batch together (default: 1 for no batching)")
	rootCmd.Flags().StringVar(&ide, "ide", ideCodex, "IDE tool to use: claude or codex")
	rootCmd.Flags().StringVar(&model, "model", "", "Model to use (default: gpt-5 for codex, auto for claude)")

	if err := rootCmd.MarkFlagRequired("pr"); err != nil {
		panic(fmt.Sprintf("failed to mark flag 'pr' as required: %v", err))
	}
}

func runSolveIssues(_ *cobra.Command, _ []string) error {
	cliArgs := cliArgs{
		pr:         pr,
		issuesDir:  issuesDir,
		dryRun:     dryRun,
		concurrent: concurrent,
		batchSize:  batchSize,
		ide:        ide,
		model:      model,
	}

	// Validate IDE parameter
	if ide != ideClaude && ide != ideCodex {
		return fmt.Errorf("invalid --ide value '%s': must be '%s' or '%s'", ide, ideClaude, ideCodex)
	}

	// Create signal-aware context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	resolvedPr, resolvedIssuesDir, resolvedIssuesDirPath, err := resolveInputs(&cliArgs)
	if err != nil {
		return err
	}
	if err := ensureCLI(&cliArgs); err != nil {
		return err
	}
	template := readTemplateSafely()
	entries, err := readIssueEntries(resolvedIssuesDirPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No issue files found.")
		return nil
	}
	entries = filterUnresolved(entries)
	if len(entries) == 0 {
		fmt.Println("All issues are already resolved. Nothing to do.")
		return nil
	}
	groups := groupIssues(entries)
	if err := writeSummaries(resolvedIssuesDirPath, groups); err != nil {
		return err
	}
	promptRoot, err := initPromptRoot(resolvedPr)
	if err != nil {
		return err
	}
	jobs, err := prepareJobs(resolvedPr, template, groups, promptRoot, cliArgs.batchSize)
	if err != nil {
		return err
	}

	// Execute jobs with graceful shutdown handling
	failed, failures, total, shutdownErr := executeJobsWithGracefulShutdown(ctx, jobs, &cliArgs)
	summarizeResults(failed, failures, total)

	// Check for shutdown errors first
	if shutdownErr != nil {
		fmt.Fprintf(os.Stderr, "\nShutdown interrupted: %v\n", shutdownErr)
		return shutdownErr
	}

	// Then check for job failures
	if len(failures) > 0 {
		return errors.New("one or more groups failed; see logs above")
	}
	_ = resolvedIssuesDir // keep local var used to mirror original logic flow
	return nil
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
		issuesDir = fmt.Sprintf("ai-docs/reviews-pr-%s/issues", pr)
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

func prepareJobs(pr, template string, groups map[string][]issueEntry, promptRoot string, batchSize int) ([]job, error) {
	// Sort code files for deterministic batching
	codeFiles := make([]string, 0, len(groups))
	for codeFile := range groups {
		codeFiles = append(codeFiles, codeFile)
	}
	sort.Strings(codeFiles)

	// Create batches
	batches := make([][]string, 0)
	if batchSize <= 0 {
		batchSize = 1
	}

	for i := 0; i < len(codeFiles); i += batchSize {
		end := i + batchSize
		if end > len(codeFiles) {
			end = len(codeFiles)
		}
		batches = append(batches, codeFiles[i:end])
	}

	// Create jobs for each batch
	jobs := make([]job, 0, len(batches))
	for batchIdx, batchFiles := range batches {
		batchGroups := make(map[string][]issueEntry)
		for _, codeFile := range batchFiles {
			batchGroups[codeFile] = groups[codeFile]
		}

		// Create a safe name for this batch
		safeName := fmt.Sprintf("batch_%03d", batchIdx+1)
		if len(batchFiles) == 1 {
			// Single file batch - use file-based name for backward compatibility
			safeName = safeFileName(func() string {
				if strings.HasPrefix(batchFiles[0], "__unknown__") {
					return unknownFileName
				}
				return batchFiles[0]
			}())
		}

		// Build the batch prompt
		promptStr := buildBatchedIssuesPrompt(buildBatchedIssuesParams{
			PR:           pr,
			BatchGroups:  batchGroups,
			BaseTemplate: template,
		})

		outPromptPath := filepath.Join(promptRoot, fmt.Sprintf("%s.prompt.md", safeName))
		if err := os.WriteFile(outPromptPath, []byte(promptStr), 0o600); err != nil {
			return nil, fmt.Errorf("write prompt: %w", err)
		}

		outLog := filepath.Join(promptRoot, fmt.Sprintf("%s.out.log", safeName))
		errLog := filepath.Join(promptRoot, fmt.Sprintf("%s.err.log", safeName))

		jobs = append(jobs, job{
			codeFiles:     batchFiles,
			groups:        batchGroups,
			safeName:      safeName,
			prompt:        []byte(promptStr),
			outPromptPath: outPromptPath,
			outLog:        outLog,
			errLog:        errLog,
		})
	}

	return jobs, nil
}

// executeJobsWithGracefulShutdown executes jobs with proper graceful shutdown handling
func executeJobsWithGracefulShutdown(ctx context.Context, jobs []job, args *cliArgs) (int32, []failInfo, int, error) {
	total := len(jobs)
	var completed int32
	var failed int32
	var failuresMu sync.Mutex
	failures := []failInfo{}
	sem := make(chan struct{}, maxInt(1, args.concurrent))
	var wg sync.WaitGroup

	cwd, err := os.Getwd()
	if err != nil {
		return 0, []failInfo{{err: fmt.Errorf("failed to get current working directory: %w", err)}}, total, nil
	}

	// Setup UI if enabled
	uiCh, uiProg := setupUI(ctx, jobs, !args.dryRun)
	defer func() {
		if uiProg != nil {
			close(uiCh)
			time.Sleep(80 * time.Millisecond)
			uiProg.Quit()
		}
	}()

	// Start job workers
	jobCtx, cancelJobs := context.WithCancel(ctx)
	defer cancelJobs()

	for idx := range jobs {
		jb := &jobs[idx]
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, j *job) {
			defer func() {
				<-sem
				wg.Done()
				atomic.AddInt32(&completed, 1)
			}()
			runOneJob(jobCtx, args, index, j, cwd, uiCh, &failed, &failuresMu, &failures)
		}(idx, jb)
	}

	// Channel to signal when all jobs are done
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Wait for either completion or shutdown signal
	select {
	case <-done:
		// All jobs completed normally
		return failed, failures, total, nil
	case <-ctx.Done():
		// Shutdown signal received, cancel all jobs
		fmt.Fprintf(os.Stderr, "\nReceived shutdown signal, canceling remaining jobs...\n")
		cancelJobs()

		// Wait for jobs to finish with a timeout for graceful shutdown
		shutdownTimeout := 30 * time.Second
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()

		select {
		case <-done:
			// Jobs finished within timeout
			fmt.Fprintf(os.Stderr, "All jobs completed gracefully within %v\n", shutdownTimeout)
			return failed, failures, total, nil
		case <-shutdownCtx.Done():
			// Timeout exceeded, force shutdown
			fmt.Fprintf(os.Stderr, "Shutdown timeout exceeded (%v), forcing exit\n", shutdownTimeout)
			return failed, failures, total, fmt.Errorf("shutdown timeout exceeded")
		}
	}
}

// executeJobs is kept for backward compatibility but delegates to the new implementation
//
//nolint:unused // kept for backward compatibility
func executeJobs(ctx context.Context, jobs []job, args *cliArgs) (int32, []failInfo, int) {
	failed, failures, total, err := executeJobsWithGracefulShutdown(ctx, jobs, args)
	if err != nil {
		// Log the error but don't fail the old interface
		fmt.Fprintf(os.Stderr, "Shutdown error in executeJobs: %v\n", err)
	}
	return failed, failures, total
}

func setupUI(ctx context.Context, jobs []job, enabled bool) (chan uiMsg, *tea.Program) {
	if !enabled {
		return nil, nil
	}
	total := len(jobs)
	uiCh := make(chan uiMsg, total*4)
	mdl := newUIModel(total)
	mdl.setEventSource(uiCh)
	prog := tea.NewProgram(mdl, tea.WithAltScreen())
	go func() {
		if err := prog.Start(); err != nil { //nolint:staticcheck // Start acceptable for goroutine UI
			fmt.Fprintf(os.Stderr, "UI program error: %v\n", err)
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
			Index:    idx,
			CodeFile: codeFileLabel,
			Issues:   totalIssues,
			SafeName: jb.safeName,
			OutLog:   jb.outLog,
			ErrLog:   jb.errLog,
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
) {
	useUI := uiCh != nil
	if ctx.Err() != nil {
		if useUI {
			uiCh <- jobFinishedMsg{Index: index, Success: false, ExitCode: -1}
		}
		return
	}

	notifyJobStart(useUI, uiCh, index, j, args.ide, args.model)

	if args.dryRun {
		if useUI {
			uiCh <- jobFinishedMsg{Index: index, Success: true, ExitCode: 0}
		}
		return
	}

	cmd, outF, errF := setupCommandExecution(ctx, args, j, cwd, useUI, uiCh, index)
	if cmd == nil {
		return
	}

	executeCommandAndHandleResult(ctx, cmd, outF, errF, j, index, useUI, uiCh, failed, failuresMu, failures)
}

func notifyJobStart(useUI bool, uiCh chan uiMsg, index int, j *job, ide string, model string) {
	if useUI {
		uiCh <- jobStartedMsg{Index: index}
	} else {
		var shellCmdStr string
		var ideName string
		switch ide {
		case ideCodex:
			if model != "" && model != "gpt-5" {
				shellCmdStr = fmt.Sprintf("codex --full-auto -m %s -c model_reasoning_effort=medium exec -", model)
			} else {
				shellCmdStr = "codex --full-auto -m gpt-5 -c model_reasoning_effort=medium exec -"
			}
			ideName = "Codex"
		case ideClaude:
			if model != "" {
				shellCmdStr = fmt.Sprintf("claude --headless -m %s", model)
			} else {
				shellCmdStr = "claude --headless"
			}
			ideName = "Claude"
		}
		totalIssues := 0
		for _, items := range j.groups {
			totalIssues += len(items)
		}
		codeFileLabel := strings.Join(j.codeFiles, ", ")
		if len(j.codeFiles) > 1 {
			codeFileLabel = fmt.Sprintf("%d files: %s", len(j.codeFiles), codeFileLabel)
		}
		fmt.Printf(
			"\n=== Running %s (headless) for batch: %s (%d issues)\n$ %s\n",
			ideName,
			codeFileLabel,
			totalIssues,
			shellCmdStr,
		)
	}
}

func createIDECommand(ctx context.Context, args *cliArgs) *exec.Cmd {
	model := args.model
	if model == "" {
		// Set default model based on IDE
		switch args.ide {
		case ideCodex:
			model = "gpt-5"
		case ideClaude:
			model = "" // Claude doesn't need explicit model parameter
		}
	}

	switch args.ide {
	case ideCodex:
		if model != "" {
			return exec.CommandContext(
				ctx,
				ideCodex,
				"--full-auto",
				"-m", model,
				"-c", "model_reasoning_effort=medium",
				"exec", "-",
			)
		}
		return exec.CommandContext(
			ctx,
			ideCodex,
			"--full-auto",
			"-c", "model_reasoning_effort=medium",
			"exec", "-",
		)
	case ideClaude:
		if model != "" {
			return exec.CommandContext(
				ctx,
				ideClaude,
				"--headless",
				"-m", model,
			)
		}
		return exec.CommandContext(
			ctx,
			ideClaude,
			"--headless",
		)
	default:
		return nil
	}
}

func setupCommandIO(
	cmd *exec.Cmd,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
) (*os.File, *os.File, error) {
	cmd.Dir = cwd
	cmd.Stdin = bytes.NewReader(j.prompt)

	outF, err := createLogFile(j.outLog, "out")
	if err != nil {
		return nil, nil, fmt.Errorf("create out log: %w", err)
	}

	errF, err := createLogFile(j.errLog, "err")
	if err != nil {
		outF.Close()
		return nil, nil, fmt.Errorf("create err log: %w", err)
	}

	const tailLines = 5
	outRing := newLineRing(tailLines)
	errRing := newLineRing(tailLines)
	var outTap, errTap io.Writer
	if useUI {
		// UI mode: write to file + UI tap (for live viewport updates)
		outTap = io.MultiWriter(outF, newUILogTap(index, false, outRing, errRing, uiCh))
		errTap = io.MultiWriter(errF, newUILogTap(index, true, outRing, errRing, uiCh))
	} else {
		// Headless mode: write to file + stdout/stderr (for live terminal output)
		outTap = io.MultiWriter(outF, os.Stdout)
		errTap = io.MultiWriter(errF, os.Stderr)
	}
	cmd.Stdout = outTap
	cmd.Stderr = errTap

	return outF, errF, nil
}

func setupCommandExecution(
	ctx context.Context,
	args *cliArgs,
	j *job,
	cwd string,
	useUI bool,
	uiCh chan uiMsg,
	index int,
) (*exec.Cmd, *os.File, *os.File) {
	cmd := createIDECommand(ctx, args)
	if cmd == nil {
		return nil, nil, nil
	}

	outF, errF, err := setupCommandIO(cmd, j, cwd, useUI, uiCh, index)
	if err != nil {
		recordFailureWithContext(nil, j, nil, err, -1)
		return nil, nil, nil
	}

	return cmd, outF, errF
}

func createLogFile(path, _ string) (*os.File, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func executeCommandAndHandleResult(
	ctx context.Context,
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
) {
	defer func() {
		if outF != nil {
			outF.Close()
		}
		if errF != nil {
			errF.Close()
		}
	}()

	// Create a channel to receive command completion
	cmdDone := make(chan error, 1)

	// Start command in background
	go func() {
		cmdDone <- cmd.Run()
	}()

	// Wait for either command completion or context cancellation
	select {
	case err := <-cmdDone:
		handleCommandCompletion(err, j, index, useUI, uiCh, failed, failuresMu, failures)
	case <-ctx.Done():
		handleCommandCancellation(ctx, cmd, cmdDone, j, index, useUI, uiCh, failed, failuresMu, failures)
	}
}

func handleCommandCompletion(
	err error,
	j *job,
	index int,
	useUI bool,
	uiCh chan uiMsg,
	failed *int32,
	failuresMu *sync.Mutex,
	failures *[]failInfo,
) {
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
		return
	}
	if useUI {
		uiCh <- jobFinishedMsg{Index: index, Success: true, ExitCode: 0}
	}
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

	// Try to terminate the process gracefully first
	if cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send SIGTERM to process: %v\n", err)
		}

		// Wait up to 5 seconds for graceful termination
		select {
		case <-cmdDone:
			// Process terminated gracefully
			fmt.Fprintf(os.Stderr, "Job %d terminated gracefully\n", index+1)
		case <-time.After(5 * time.Second):
			// Timeout - force kill
			fmt.Fprintf(os.Stderr, "Job %d did not terminate gracefully, force killing...\n", index+1)
			if err := cmd.Process.Kill(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
			}
		}
	}

	// Don't count shutdown cancellations as failures - they're expected
	// Just mark the job as canceled in the UI
	if useUI {
		uiCh <- jobFinishedMsg{Index: index, Success: false, ExitCode: -1}
	}
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

type uiJob struct {
	codeFile string
	issues   int
	safeName string
	outLog   string
	errLog   string
	state    jobState
	exitCode int
	lastOut  []string
	lastErr  []string
}

type tickMsg struct{}

type jobQueuedMsg struct {
	Index    int
	CodeFile string
	Issues   int
	SafeName string
	OutLog   string
	ErrLog   string
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

type uiModel struct {
	jobs      []uiJob
	total     int
	completed int
	failed    int
	frame     int
	events    <-chan uiMsg
	onQuit    func()
	viewport  viewport.Model
}

type uiMsg any

func newUIModel(total int) *uiModel {
	vp := viewport.New(80, 20) // Default size, will be updated on window resize
	return &uiModel{
		total:    total,
		viewport: vp,
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
		m.viewport.ScrollUp(1)
		return nil
	case "down", "j":
		m.viewport.ScrollDown(1)
		return nil
	case "pgup", "b", "u":
		m.viewport.HalfPageUp()
		return nil
	case "pgdown", "f", "d":
		m.viewport.HalfPageDown()
		return nil
	case "home", "g":
		m.viewport.GotoTop()
		return nil
	case "end", "G":
		m.viewport.GotoBottom()
		return nil
	default:
		return m.waitEvent()
	}
}

func (m *uiModel) handleWindowSize(v tea.WindowSizeMsg) {
	// Update viewport size and refresh content
	m.viewport.Width = v.Width
	m.viewport.Height = v.Height - 3 // Leave space for header
	m.refreshViewportContent()
}

func (m *uiModel) refreshViewportContent() {
	var b strings.Builder
	green := lipgloss.Color("42")
	red := lipgloss.Color("196")
	yellow := lipgloss.Color("220")
	gray := lipgloss.Color("245")
	cyan := lipgloss.Color("81")
	sDim := lipgloss.NewStyle().Foreground(gray)
	sFile := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	sIssues := lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	sSuccess := lipgloss.NewStyle().Foreground(green)
	sFail := lipgloss.NewStyle().Foreground(red)
	sRun := lipgloss.NewStyle().Foreground(yellow)
	sLog := lipgloss.NewStyle().Foreground(gray)

	for i := range m.jobs {
		j := &m.jobs[i]
		prefix := m.viewPrefix(j, &sDim, &sRun, &sSuccess, &sFail)
		b.WriteString(prefix)
		b.WriteString("  ")
		b.WriteString(sFile.Render(j.codeFile))
		b.WriteString(" ")
		b.WriteString(sIssues.Render(fmt.Sprintf("(%d issues)", j.issues)))
		b.WriteString("\n")
		m.appendLogs(&b, j, &sDim, &sLog)
	}

	m.viewport.SetContent(b.String())
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
		codeFile: v.CodeFile,
		issues:   v.Issues,
		safeName: v.SafeName,
		outLog:   v.OutLog,
		errLog:   v.ErrLog,
		state:    jobPending,
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobStarted(v jobStartedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.jobs[v.Index].state = jobRunning
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

func (m *uiModel) handleJobFinished(v jobFinishedMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		if v.Success {
			m.jobs[v.Index].state = jobSuccess
			m.completed++
		} else {
			m.jobs[v.Index].state = jobFailed
			m.jobs[v.Index].exitCode = v.ExitCode
			m.failed++
		}
	}
	m.refreshViewportContent()
	if m.completed+m.failed < m.total {
		return m.waitEvent()
	}
	return tea.Quit
}

func (m *uiModel) handleJobLogUpdate(v jobLogUpdateMsg) tea.Cmd {
	if v.Index < len(m.jobs) {
		m.jobs[v.Index].lastOut = v.Out
		m.jobs[v.Index].lastErr = v.Err
	}
	m.refreshViewportContent()
	return m.waitEvent()
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m *uiModel) View() string {
	sHeader := lipgloss.NewStyle().Bold(true)
	header := fmt.Sprintf("processing Codex jobs…: %d/%d completed, %d failed", m.completed, m.total, m.failed)
	helpText := "↑↓/jk scroll • pgup/pgdn/b/f half page • home/end/g/G top/bottom • q quit"

	return sHeader.Render(header) + "\n" +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(helpText) + "\n" +
		m.viewport.View()
}

func (m *uiModel) viewPrefix(j *uiJob, sDim, sRun, sSuccess, sFail *lipgloss.Style) string {
	switch j.state {
	case jobPending:
		return sDim.Render("…")
	case jobRunning:
		return sRun.Render(spinnerFrames[m.frame%len(spinnerFrames)])
	case jobSuccess:
		return sSuccess.Render("✓")
	case jobFailed:
		return sFail.Render("✗")
	default:
		return ""
	}
}

func (m *uiModel) appendLogs(b *strings.Builder, j *uiJob, sDim, sLog *lipgloss.Style) {
	if j.state == jobRunning {
		if len(j.lastOut) > 0 {
			b.WriteString("     ")
			b.WriteString(sDim.Render("stdout:"))
			b.WriteString("\n")
			for _, line := range j.lastOut {
				if line == "" {
					continue
				}
				b.WriteString("       ")
				b.WriteString(sLog.Render(line))
				b.WriteString("\n")
			}
		}
		if len(j.lastErr) > 0 {
			b.WriteString("     ")
			b.WriteString(sDim.Render("stderr:"))
			b.WriteString("\n")
			for _, line := range j.lastErr {
				if line == "" {
					continue
				}
				b.WriteString("       ")
				b.WriteString(sLog.Render(line))
				b.WriteString("\n")
			}
		}
		b.WriteString("     ")
		b.WriteString(sDim.Render(fmt.Sprintf("logs: %s (out), %s (err)", j.outLog, j.errLog)))
		b.WriteString("\n")
		return
	}
	if j.state == jobFailed || j.state == jobSuccess {
		b.WriteString("     ")
		b.WriteString(sDim.Render(fmt.Sprintf("logs: %s (out), %s (err)", j.outLog, j.errLog)))
		if j.state == jobFailed {
			b.WriteString(" ")
			styleFail := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			b.WriteString(styleFail.Render(fmt.Sprintf("exit=%d", j.exitCode)))
		}
		b.WriteString("\n")
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

func readIssueEntries(resolvedIssuesDir string) ([]issueEntry, error) {
	entries := []issueEntry{}
	files, err := os.ReadDir(resolvedIssuesDir)
	if err != nil {
		return nil, err
	}
	// deterministic order
	names := make([]string, 0, len(files))
	for _, f := range files {
		if !f.Type().IsRegular() {
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
		// included issues list
		var sb strings.Builder
		sb.WriteString(header)
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
	// Matches: **File:** `path/to/file.tsx:123` OR without line
	re := regexp.MustCompile(`\*\*File:\*\*\s*` + "`" + `([^` + "`" + `]+)` + "`")
	m := re.FindStringSubmatch(content)
	if len(m) < 2 {
		return ""
	}
	raw := strings.TrimSpace(m[1])
	// Strip trailing :line if present
	if idx := strings.LastIndex(raw, ":"); idx != -1 {
		// ensure there are only digits after colon
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
	// replace non [a-zA-Z0-9._-] with _
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

func readTemplateSafely() string {
	b, err := os.ReadFile(".cursor/commands/pr-fix.md")
	if err != nil {
		return ""
	}
	return string(b)
}

// Prompt builders

type buildBatchedIssuesParams struct {
	PR           string
	BatchGroups  map[string][]issueEntry
	BaseTemplate string
}

func buildBatchedIssuesPrompt(p buildBatchedIssuesParams) string {
	codeFiles := sortCodeFiles(p.BatchGroups)
	header := buildBatchHeader(p.PR, codeFiles, p.BatchGroups)
	critical := buildBatchCritical(p.PR, codeFiles)
	batchNotice := buildBatchNotice(codeFiles)
	issueGroups := buildIssueGroups(codeFiles, p.BatchGroups)
	task := buildBatchTask(p.PR, codeFiles)
	checklist := buildBatchChecklist(p.PR, codeFiles, p.BatchGroups)

	composed := strings.Join([]string{header, critical, batchNotice,
		issueGroups, task, checklist}, "\n\n")

	if t := strings.TrimSpace(p.BaseTemplate); t != "" {
		return t + "\n\n" + composed
	}
	return composed
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

func buildBatchCritical(pr string, codeFiles []string) string {
	codeFileList := strings.Join(codeFiles, "\n  - ")
	return fmt.Sprintf(`
<critical>
- You MUST fix ALL issues listed below across MULTIPLE files in this batch.
- Implement proper solutions; do not use workarounds.
- Follow project standards in .cursor/rules.
- Files in this batch:
  - %s
- After making changes, update ONLY the progress files generated by pr-review for this PR:
  - ai-docs/reviews-pr-%s/issues/_summary.md
  - Each included issue file under ai-docs/reviews-pr-%s/issues/
  - The grouped files for each file in ai-docs/reviews-pr-%s/issues/grouped/
- MUST DO: If these are GitHub review issues, after resolving them you need to call the gh command to resolve each
  thread as per the instructions in the issue files (look for a "Thread ID:" line and use the provided gh command).
</critical>`, codeFileList, pr, pr, pr)
}

func buildBatchNotice(codeFiles []string) string {
	return fmt.Sprintf(`
<important_batch_processing>
⚠️  BATCH PROCESSING MODE ⚠️

This batch contains issues from %d different files. You should:
- Address ALL issues across ALL files in this batch cohesively
- Consider interdependencies between files (e.g., shared types, utilities)
- Ensure changes are consistent across the codebase
- Run bun run lint && bun run typecheck && bun run test before concluding

Files in this batch: %s
</important_batch_processing>`, len(codeFiles), strings.Join(codeFiles, ", "))
}

func buildIssueGroups(codeFiles []string, batchGroups map[string][]issueEntry) string {
	var issueGroupsBuilder strings.Builder
	for _, codeFile := range codeFiles {
		items := batchGroups[codeFile]
		issueGroupsBuilder.WriteString(fmt.Sprintf(`
<file-group file="%s">
  <issues-count>%d</issues-count>
`, codeFile, len(items)))

		for idx, item := range items {
			relPath := normalizeForPrompt(item.absPath)
			issueGroupsBuilder.WriteString(fmt.Sprintf(`
  <issue index="%d">
    <from>%s</from>
    <content lang="markdown">
%s
    </content>
  </issue>
`, idx+1, relPath, item.content))
		}

		issueGroupsBuilder.WriteString("</file-group>\n")
	}
	return issueGroupsBuilder.String()
}

func buildBatchTask(pr string, codeFiles []string) string {
	groupedFiles := make([]string, len(codeFiles))
	for i, codeFile := range codeFiles {
		groupedFiles[i] = fmt.Sprintf("ai-docs/reviews-pr-%s/issues/grouped/%s.md", pr, safeFileName(codeFile))
	}

	return fmt.Sprintf(`
<task>
- Resolve ALL issues above across ALL %d files in a cohesive set of changes.
- Update ai-docs/reviews-pr-%s/issues/_summary.md to reflect resolution status for each included issue.
- In each included issue file under ai-docs/reviews-pr-%s/issues,
  update the status section/checkbox to RESOLVED ✓ when addressed.
- Update the grouped tracking files for each file in this batch:
  %s
- If a GitHub review thread ID is present in any issue,
  resolve it using gh as per the command snippet included in that issue.
- Run bun run lint && bun run typecheck && bun run test before concluding this batched task.
- If documentation updates are required, include them.
- For any included issue that is already solved (no code change required),
  you MUST still apply the progress updates above:
  - update _summary.md,
  - mark the specific issue file as RESOLVED ✓,
  - resolve its GitHub review thread via gh if a Thread ID is present.
</task>`, len(codeFiles), pr, pr, strings.Join(groupedFiles, "\n  "))
}

func buildBatchChecklist(pr string, codeFiles []string, batchGroups map[string][]issueEntry) string {
	var checklistPaths []string
	checklistPaths = append(checklistPaths, fmt.Sprintf("ai-docs/reviews-pr-%s/issues/_summary.md", pr))
	for _, codeFile := range codeFiles {
		checklistPaths = append(
			checklistPaths,
			fmt.Sprintf("ai-docs/reviews-pr-%s/issues/grouped/%s.md", pr, safeFileName(codeFile)),
		)
		for _, item := range batchGroups[codeFile] {
			checklistPaths = append(checklistPaths, normalizeForPrompt(item.absPath))
		}
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
		// drop oldest
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

// uiLogTap is an io.Writer that splits by newlines, appends to a ring buffer
// and emits UI updates with the newest snapshots.
type uiLogTap struct {
	idx   int
	isErr bool
	out   *lineRing
	err   *lineRing
	ch    chan<- uiMsg
	buf   []byte
}

func newUILogTap(idx int, isErr bool, outRing, errRing *lineRing, ch chan<- uiMsg) *uiLogTap {
	return &uiLogTap{idx: idx, isErr: isErr, out: outRing, err: errRing, ch: ch, buf: make([]byte, 0, 1024)}
}

func (t *uiLogTap) Write(p []byte) (int, error) {
	// Accumulate and split by newline. Treat CR as newline separators as well.
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
	// Emit an update with current snapshots (best-effort)
	select {
	case t.ch <- jobLogUpdateMsg{Index: t.idx, Out: t.out.snapshot(), Err: t.err.snapshot()}:
	default:
		// If channel is full, skip; UI will refresh on next tick
	}
	return len(p), nil
}
