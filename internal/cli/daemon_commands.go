package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/compozy/compozy/internal/core/model"
	taskscore "github.com/compozy/compozy/internal/core/tasks"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/daemon"
	daemonlogger "github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/version"
	"github.com/spf13/cobra"
)

const (
	attachModeAuto   = "auto"
	attachModeUI     = "ui"
	attachModeStream = "stream"
	attachModeDetach = "detach"

	defaultDaemonStartupTimeout = 10 * time.Second
	defaultDaemonPollInterval   = 100 * time.Millisecond
)

var (
	resolveCLIDaemonHomePaths  = compozyconfig.ResolveHomePaths
	readCLIDaemonInfo          = daemon.ReadInfo
	sleepForCLIDaemonPoll      = time.Sleep
	nowForCLIDaemonPoll        = time.Now
	launchCLIDaemonProcess     = defaultLaunchCLIDaemonProcess
	resolveCLIDaemonExecutable = os.Executable
	newCLIDaemonBootstrap      = newDefaultCLIDaemonBootstrap
)

type daemonCommandClient interface {
	Target() apiclient.Target
	Health(context.Context) (apicore.DaemonHealth, error)
	DaemonStatus(context.Context) (apicore.DaemonStatus, error)
	StopDaemon(context.Context, bool) error
	RegisterWorkspace(context.Context, string, string) (apicore.WorkspaceRegisterResult, error)
	ListWorkspaces(context.Context) ([]apicore.Workspace, error)
	GetWorkspace(context.Context, string) (apicore.Workspace, error)
	DeleteWorkspace(context.Context, string) error
	ResolveWorkspace(context.Context, string) (apicore.Workspace, error)
	ListTaskWorkflows(context.Context, string) ([]apicore.WorkflowSummary, error)
	ArchiveTaskWorkflow(context.Context, string, string) (apicore.ArchiveResult, error)
	SyncWorkflow(context.Context, apicore.SyncRequest) (apicore.SyncResult, error)
	FetchReview(context.Context, string, string, apicore.ReviewFetchRequest) (apicore.ReviewFetchResult, error)
	GetLatestReview(context.Context, string, string) (apicore.ReviewSummary, error)
	GetReviewRound(context.Context, string, string, int) (apicore.ReviewRound, error)
	ListReviewIssues(context.Context, string, string, int) ([]apicore.ReviewIssue, error)
	StartTaskRun(context.Context, string, apicore.TaskRunRequest) (apicore.Run, error)
	StartTaskRunMultiple(context.Context, apicore.TaskRunMultipleRequest) (apicore.Run, error)
	GetTaskRunMultipleSnapshot(context.Context, string) (apicore.TaskRunMultipleSnapshot, error)
	StartReviewRun(context.Context, string, string, int, apicore.ReviewRunRequest) (apicore.Run, error)
	StartReviewWatch(context.Context, string, string, apicore.ReviewWatchRequest) (apicore.Run, error)
	StartExecRun(context.Context, apicore.ExecRequest) (apicore.Run, error)
	CancelRun(context.Context, string) error
	GetRunSnapshot(context.Context, string) (apicore.RunSnapshot, error)
	ListRunEvents(context.Context, string, apicore.StreamCursor, int) (apicore.RunEventPage, error)
	OpenRunStream(context.Context, string, apicore.StreamCursor) (apiclient.RunStream, error)
}

type cliDaemonBootstrap struct {
	resolveHomePaths func() (compozyconfig.HomePaths, error)
	readInfo         func(string) (daemon.Info, error)
	newClient        func(apiclient.Target) (daemonCommandClient, error)
	launch           func(compozyconfig.HomePaths) error
	sleep            func(time.Duration)
	now              func() time.Time
	cliVersion       func() string
	notify           func(string) error
	startupTimeout   time.Duration
	pollInterval     time.Duration
}

type daemonRuntimeOverrides struct {
	DryRun                     *bool                       `json:"dry_run,omitempty"`
	RunID                      *string                     `json:"run_id,omitempty"`
	AutoCommit                 *bool                       `json:"auto_commit,omitempty"`
	IDE                        *string                     `json:"ide,omitempty"`
	Model                      *string                     `json:"model,omitempty"`
	AgentName                  *string                     `json:"agent_name,omitempty"`
	ExplicitRuntime            *model.ExplicitRuntimeFlags `json:"explicit_runtime,omitempty"`
	OutputFormat               *string                     `json:"output_format,omitempty"`
	AddDirs                    *[]string                   `json:"add_dirs,omitempty"`
	TailLines                  *int                        `json:"tail_lines,omitempty"`
	ReasoningEffort            *string                     `json:"reasoning_effort,omitempty"`
	AccessMode                 *string                     `json:"access_mode,omitempty"`
	Timeout                    *string                     `json:"timeout,omitempty"`
	MaxRetries                 *int                        `json:"max_retries,omitempty"`
	RetryBackoffMultiplier     *float64                    `json:"retry_backoff_multiplier,omitempty"`
	Verbose                    *bool                       `json:"verbose,omitempty"`
	Persist                    *bool                       `json:"persist,omitempty"`
	IncludeCompleted           *bool                       `json:"include_completed,omitempty"`
	Recursive                  *bool                       `json:"recursive,omitempty"`
	TaskRuntimeRules           *[]model.TaskRuntimeRule    `json:"task_runtime_rules,omitempty"`
	EnableExecutableExtensions *bool                       `json:"enable_executable_extensions,omitempty"`
}

func newDefaultCLIDaemonBootstrap() cliDaemonBootstrap {
	return cliDaemonBootstrap{
		resolveHomePaths: resolveCLIDaemonHomePaths,
		readInfo:         readCLIDaemonInfo,
		newClient: func(target apiclient.Target) (daemonCommandClient, error) {
			return apiclient.New(target)
		},
		launch:     launchCLIDaemonProcess,
		sleep:      sleepForCLIDaemonPoll,
		now:        nowForCLIDaemonPoll,
		cliVersion: version.String,
		notify: func(message string) error {
			_, err := fmt.Fprintln(os.Stderr, message)
			return err
		},
		startupTimeout: defaultDaemonStartupTimeout,
		pollInterval:   defaultDaemonPollInterval,
	}
}

func (b cliDaemonBootstrap) ensure(ctx context.Context) (daemonCommandClient, error) {
	paths, err := b.resolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("resolve daemon home paths: %w", err)
	}

	client, err := b.probe(ctx, paths.InfoPath)
	if err == nil {
		return client, nil
	}
	var versionMismatch *cliDaemonVersionMismatchError
	if errors.As(err, &versionMismatch) {
		return b.handleVersionMismatch(ctx, paths, versionMismatch)
	}
	lastErr := err

	if err := b.launch(paths); err != nil {
		return nil, fmt.Errorf("start daemon process: %w", err)
	}

	return b.waitForDaemonReadiness(ctx, paths.InfoPath, lastErr)
}

func (b cliDaemonBootstrap) waitForDaemonReadiness(
	ctx context.Context,
	infoPath string,
	lastErr error,
) (daemonCommandClient, error) {
	deadline := b.now().Add(b.startupTimeout)
	for b.now().Before(deadline) || b.now().Equal(deadline) {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait for daemon readiness: %w", err)
		}

		client, err := b.probe(ctx, infoPath)
		if err == nil {
			return client, nil
		}
		var versionMismatch *cliDaemonVersionMismatchError
		if errors.As(err, &versionMismatch) {
			return nil, versionMismatch
		}
		lastErr = err
		b.sleep(b.pollInterval)
	}

	if lastErr == nil {
		lastErr = errors.New("daemon did not become ready")
	}
	return nil, fmt.Errorf("wait for daemon readiness: %w", lastErr)
}

func (b cliDaemonBootstrap) probe(ctx context.Context, infoPath string) (daemonCommandClient, error) {
	info, err := b.readInfo(strings.TrimSpace(infoPath))
	if err != nil {
		return nil, fmt.Errorf("read daemon info: %w", err)
	}

	client, err := b.newClient(apiclient.Target{
		SocketPath: strings.TrimSpace(info.SocketPath),
		HTTPPort:   info.HTTPPort,
	})
	if err != nil {
		return nil, fmt.Errorf("build daemon client: %w", err)
	}

	health, err := client.Health(ctx)
	if err != nil {
		return nil, fmt.Errorf("probe daemon health via %s: %w", client.Target().String(), err)
	}
	if !health.Ready {
		return nil, fmt.Errorf("probe daemon health via %s: %w", client.Target().String(), cliDaemonHealthError(health))
	}
	status, err := client.DaemonStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("probe daemon status via %s: %w", client.Target().String(), err)
	}
	daemonVersion := firstNonEmptyDaemonBuildVersion(status.Version, info.Version)
	cliVersion := b.currentCLIVersion()
	if !daemonBuildVersionsCompatible(daemonVersion, cliVersion) {
		return nil, &cliDaemonVersionMismatchError{
			info:           info,
			client:         client,
			daemonVersion:  daemonVersion,
			cliVersion:     cliVersion,
			activeRunCount: status.ActiveRunCount,
		}
	}
	return client, nil
}

func (b cliDaemonBootstrap) handleVersionMismatch(
	ctx context.Context,
	paths compozyconfig.HomePaths,
	mismatch *cliDaemonVersionMismatchError,
) (daemonCommandClient, error) {
	if mismatch == nil {
		return nil, errors.New("daemon version mismatch details are required")
	}
	if mismatch.activeRunCount > 0 {
		return nil, mismatch
	}
	if err := b.notifyVersionMismatchRestart(mismatch); err != nil {
		return nil, err
	}
	if err := mismatch.client.StopDaemon(ctx, false); err != nil {
		return nil, fmt.Errorf("stop stale daemon: %w", err)
	}
	if err := b.waitForDaemonInfoRelease(ctx, paths.InfoPath, mismatch.info); err != nil {
		return nil, err
	}
	if err := b.launch(paths); err != nil {
		return nil, fmt.Errorf("start daemon process: %w", err)
	}
	return b.waitForDaemonReadiness(ctx, paths.InfoPath, nil)
}

func (b cliDaemonBootstrap) notifyVersionMismatchRestart(mismatch *cliDaemonVersionMismatchError) error {
	if b.notify == nil {
		return nil
	}
	if err := b.notify(fmt.Sprintf(
		"Restarting stale compozy daemon (daemon %s, CLI %s).",
		displayDaemonBuildVersion(mismatch.daemonVersion),
		displayDaemonBuildVersion(mismatch.cliVersion),
	)); err != nil {
		return fmt.Errorf("write daemon restart notice: %w", err)
	}
	return nil
}

func (b cliDaemonBootstrap) waitForDaemonInfoRelease(
	ctx context.Context,
	infoPath string,
	previous daemon.Info,
) error {
	deadline := b.now().Add(b.startupTimeout)
	var lastErr error
	for b.now().Before(deadline) || b.now().Equal(deadline) {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("wait for stale daemon shutdown: %w", err)
		}
		info, err := b.readInfo(strings.TrimSpace(infoPath))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			lastErr = fmt.Errorf("read daemon info while waiting for stale daemon release: %w", err)
			b.sleep(b.pollInterval)
			continue
		}
		if !sameDaemonInfoOwner(info, previous) {
			return nil
		}
		lastErr = fmt.Errorf("stale daemon still owns daemon info (pid=%d)", previous.PID)
		b.sleep(b.pollInterval)
	}
	if lastErr == nil {
		lastErr = errors.New("stale daemon info was not released")
	}
	return fmt.Errorf("wait for stale daemon shutdown: %w", lastErr)
}

func (b cliDaemonBootstrap) currentCLIVersion() string {
	if b.cliVersion == nil {
		return version.String()
	}
	return b.cliVersion()
}

type cliDaemonVersionMismatchError struct {
	info           daemon.Info
	client         daemonCommandClient
	daemonVersion  string
	cliVersion     string
	activeRunCount int
}

func (e *cliDaemonVersionMismatchError) Error() string {
	if e == nil {
		return "daemon version mismatch"
	}
	if e.activeRunCount > 0 {
		return fmt.Sprintf(
			"running compozy daemon version %q does not match CLI version %q and has %d active runs; "+
				"retry after the active runs finish or, when it is safe to cancel them, run "+
				"`compozy daemon stop --force` and retry so the CLI starts the current daemon",
			displayDaemonBuildVersion(e.daemonVersion),
			displayDaemonBuildVersion(e.cliVersion),
			e.activeRunCount,
		)
	}
	return fmt.Sprintf(
		"running compozy daemon version %q does not match CLI version %q",
		displayDaemonBuildVersion(e.daemonVersion),
		displayDaemonBuildVersion(e.cliVersion),
	)
}

func firstNonEmptyDaemonBuildVersion(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func daemonBuildVersionsCompatible(daemonVersion string, cliVersion string) bool {
	daemonVersion = strings.TrimSpace(daemonVersion)
	cliVersion = strings.TrimSpace(cliVersion)
	if daemonVersion == cliVersion {
		return true
	}
	return isDevBuildVersion(daemonVersion) || isDevBuildVersion(cliVersion)
}

func isDevBuildVersion(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "" ||
		normalized == "dev" ||
		strings.HasPrefix(normalized, "dev ") ||
		strings.HasPrefix(normalized, "dev(")
}

func displayDaemonBuildVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown build"
	}
	return trimmed
}

func sameDaemonInfoOwner(a daemon.Info, b daemon.Info) bool {
	return a.PID > 0 && a.PID == b.PID && a.StartedAt.Equal(b.StartedAt)
}

func defaultLaunchCLIDaemonProcess(paths compozyconfig.HomePaths) error {
	executable, err := resolveLaunchCLIDaemonExecutable()
	if err != nil {
		return err
	}
	return launchCLIDaemonProcessWithExecutable(paths, executable)
}

func resolveLaunchCLIDaemonExecutable() (string, error) {
	executable, err := resolveCLIDaemonExecutable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}

	base := strings.ToLower(filepath.Base(strings.TrimSpace(executable)))
	if strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".test.exe") {
		return "", errors.New(
			"daemon auto-start cannot relaunch a Go test binary; " +
				"install a daemon bootstrap stub or use a real compozy executable",
		)
	}

	return executable, nil
}

func launchCLIDaemonProcessWithExecutable(paths compozyconfig.HomePaths, executable string) error {
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		return err
	}
	if _, err := cliDaemonHTTPPortFromEnv(); err != nil {
		return fmt.Errorf("resolve daemon http port: %w", err)
	}

	if err := daemonlogger.ValidateDaemonFilePath(paths.LogFile); err != nil {
		return fmt.Errorf("open daemon log file: %w", err)
	}

	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer func() {
		_ = nullFile.Close()
	}()

	command := exec.CommandContext(
		context.Background(),
		executable,
		"daemon",
		"start",
		"--"+daemonStartInternalChildFlag,
	)
	command.Stdin = nullFile
	command.Stdout = nullFile
	command.Stderr = nullFile
	command.SysProcAttr = daemonLaunchSysProcAttr()

	if err := command.Start(); err != nil {
		return fmt.Errorf("launch daemon start command: %w", err)
	}
	return command.Process.Release()
}

func newTasksCommand(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "tasks",
		Short:        "Inspect, validate, and run task workflows",
		SilenceUsage: true,
	}

	cmd.AddCommand(
		newTasksValidateCommand(),
		newTasksRunCommandWithDefaults(dispatcher, defaults),
	)
	return cmd
}

func newTasksRunCommandWithDefaults(_ *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindTasksRun, core.ModePRDTasks, defaults)
	cmd := &cobra.Command{
		Use:          "run [slug]",
		Short:        "Start a daemon-backed task workflow run",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Start a task workflow through the shared home-scoped daemon.

The CLI resolves the workspace root and attach mode locally, ensures the daemon
is running, and then sends the workflow request over the daemon transport.

Use --multiple with one comma-separated slug list to start several task workflows
through one daemon-owned parent run. V1 runs the queue in enqueued order;
configured parallel mode prints a V2 worktree-isolation fallback message before
sending enqueued execution to the daemon.`,
		Example: `  compozy tasks run my-feature
  compozy tasks run --multiple alpha,beta --stream
  compozy tasks run --multiple alpha,beta --detach
  compozy tasks run --multiple alpha,beta --ide codex --model gpt-5.5
  compozy tasks run my-feature --stream
  compozy tasks run my-feature --detach
  compozy tasks run --name my-feature --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return state.runTaskWorkflow(cmd, args)
		},
	}

	addTaskRunFlags(cmd, state, taskRunFlagOptions{includeName: true})
	return cmd
}

type taskRunFlagOptions struct {
	includeName bool
}

func addTaskRunFlags(cmd *cobra.Command, state *commandState, opts taskRunFlagOptions) {
	addCommonFlags(cmd, state, commonFlagOptions{})
	if opts.includeName {
		cmd.Flags().StringVar(&state.name, "name", "", "Task workflow slug (defaults to the positional slug)")
	}
	cmd.Flags().StringVar(
		&state.multiple,
		"multiple",
		"",
		"Comma-separated task workflow slugs to run through one daemon-owned parent queue",
	)
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "Include completed tasks")
	cmd.Flags().BoolVarP(
		&state.recursive,
		"recursive",
		"r",
		false,
		"Recursively discover task_NNN.md files in subdirectories. "+
			"Skips dot-, underscore-prefixed, reviews-*, adrs, and memory directories. "+
			"Note: DB sync and extension Host API still operate on the slug root only.",
	)
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
	cmd.Flags().StringVar(
		&state.attachMode,
		"attach",
		attachModeAuto,
		"Attach mode: auto, ui, stream, or detach",
	)
	cmd.Flags().Bool("ui", false, "Force interactive UI attach mode")
	cmd.Flags().Bool("stream", false, "Force textual stream attach mode")
	cmd.Flags().Bool("detach", false, "Start the run without attaching a client")
	cmd.Flags().Var(
		newTaskRuntimeFlagValue(&state.executionTaskRuntimeRules),
		"task-runtime",
		`Per-task runtime override rule for task runs (repeatable). Use key=value pairs such as type=frontend,ide=codex,model=gpt-5.5 or id=task_01,reasoning-effort=xhigh`,
	)
}

func (s *commandState) runTaskWorkflow(cmd *cobra.Command, args []string) error {
	if commandFlagChanged(cmd, "multiple") {
		return s.runTaskWorkflowsMultiple(cmd, args)
	}

	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	if len(args) == 0 && strings.TrimSpace(s.name) == "" {
		if err := s.maybeCollectInteractiveParams(cmd); err != nil {
			return err
		}
	}
	if err := s.resolveTaskWorkflowName(args); err != nil {
		return withExitCode(1, err)
	}

	resolvedTasksDir, err := resolveTaskWorkflowDir(s.workspaceRoot, s.name, "")
	if err != nil {
		return withExitCode(2, err)
	}
	s.tasksDir = resolvedTasksDir
	s.explicitRuntime = captureExplicitRuntimeFlags(cmd)

	cfg, err := s.buildConfig()
	if err != nil {
		return withExitCode(2, err)
	}
	if err := s.preflightTaskMetadata(ctx, cmd, cfg); err != nil {
		return err
	}

	presentationMode, err := s.resolveTaskPresentationMode(cmd)
	if err != nil {
		return withExitCode(1, err)
	}
	runtimeOverrides, err := s.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		return withExitCode(2, err)
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	run, err := client.StartTaskRun(ctx, s.name, apicore.TaskRunRequest{
		Workspace:        s.workspaceRoot,
		PresentationMode: presentationMode,
		RuntimeOverrides: runtimeOverrides,
	})
	if err != nil {
		return mapDaemonCommandError(err)
	}
	return handleStartedTaskRun(ctx, cmd, client, run)
}

func (s *commandState) runTaskWorkflowsMultiple(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	slugs, err := s.resolveTaskWorkflowSlugList(args)
	if err != nil {
		return withExitCode(1, err)
	}
	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}

	s.explicitRuntime = captureExplicitRuntimeFlags(cmd)
	if err := s.preflightTaskWorkflowSlugs(ctx, cmd, slugs); err != nil {
		return err
	}

	presentationMode, err := s.resolveTaskPresentationMode(cmd)
	if err != nil {
		return withExitCode(1, err)
	}
	mode, err := s.resolveTaskRunMultipleMode(cmd)
	if err != nil {
		return withExitCode(2, err)
	}
	runtimeOverrides, err := s.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		return withExitCode(2, err)
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	run, err := client.StartTaskRunMultiple(ctx, apicore.TaskRunMultipleRequest{
		Workspace:        s.workspaceRoot,
		Slugs:            slugs,
		Mode:             mode,
		PresentationMode: presentationMode,
		RuntimeOverrides: runtimeOverrides,
	})
	if err != nil {
		return mapDaemonCommandError(err)
	}
	return handleStartedTaskRunMultiple(ctx, cmd, client, run)
}

func (s *commandState) resolveTaskWorkflowSlugList(args []string) ([]string, error) {
	if len(args) > 0 || strings.TrimSpace(s.name) != "" {
		return nil, errors.New("--multiple cannot be combined with a positional slug or --name")
	}
	if strings.TrimSpace(s.multiple) == "" {
		return nil, errors.New("workflow slug list is required; pass comma-separated slugs to --multiple")
	}
	slugs, err := taskscore.ParseCommaSeparatedSlugs(s.multiple)
	if err != nil {
		return nil, fmt.Errorf("workflow slug list: %w", err)
	}
	return slugs, nil
}

func (s *commandState) preflightTaskWorkflowSlugs(ctx context.Context, cmd *cobra.Command, slugs []string) error {
	cfg, err := s.buildConfig()
	if err != nil {
		return withExitCode(2, err)
	}

	originalName := s.name
	originalTasksDir := s.tasksDir
	defer func() {
		s.name = originalName
		s.tasksDir = originalTasksDir
	}()

	for _, slug := range slugs {
		resolvedTasksDir, err := resolveTaskWorkflowDir(s.workspaceRoot, slug, "")
		if err != nil {
			return withExitCode(2, err)
		}
		s.name = slug
		s.tasksDir = resolvedTasksDir
		cfg.Name = slug
		cfg.TasksDir = resolvedTasksDir
		if err := s.preflightTaskMetadata(ctx, cmd, cfg); err != nil {
			return err
		}
	}
	return nil
}

func (s *commandState) resolveTaskRunMultipleMode(cmd *cobra.Command) (string, error) {
	mode := s.projectConfig.Tasks.Run.EffectiveRunMultipleMode()
	switch mode {
	case workspacecfg.TaskRunMultipleModeEnqueued:
		return mode, nil
	case workspacecfg.TaskRunMultipleModeParallel:
		if _, err := fmt.Fprintln(
			cmd.ErrOrStderr(),
			"parallel multi-task execution is planned for V2 with git worktree isolation; running this queue in enqueued mode.",
		); err != nil {
			return "", fmt.Errorf("write multi-run fallback message: %w", err)
		}
		return workspacecfg.TaskRunMultipleModeEnqueued, nil
	default:
		return "", fmt.Errorf("tasks.run.run_multiple_mode must be %q or %q (got %q)",
			workspacecfg.TaskRunMultipleModeEnqueued,
			workspacecfg.TaskRunMultipleModeParallel,
			mode,
		)
	}
}

func handleStartedTaskRun(
	ctx context.Context,
	cmd *cobra.Command,
	client daemonCommandClient,
	run apicore.Run,
) error {
	if run.PresentationMode == attachModeUI {
		if err := attachStartedCLIRunUI(ctx, client, run.RunID); err != nil {
			if errors.Is(err, errRunSettledBeforeUIAttach) {
				if err := watchCLIRun(ctx, cmd.OutOrStdout(), client, run.RunID); err != nil {
					return mapDaemonCommandError(err)
				}
				return nil
			}
			return mapDaemonCommandError(err)
		}
		return nil
	}
	if err := writeStartedTaskRun(cmd, run); err != nil {
		return err
	}
	if run.PresentationMode != attachModeStream {
		return nil
	}
	if err := watchCLIRun(ctx, cmd.OutOrStdout(), client, run.RunID); err != nil {
		return mapDaemonCommandError(err)
	}
	return nil
}

func handleStartedTaskRunMultiple(
	ctx context.Context,
	cmd *cobra.Command,
	client daemonCommandClient,
	run apicore.Run,
) error {
	if run.PresentationMode == attachModeUI {
		if err := attachStartedCLIRunUI(ctx, client, run.RunID); err != nil {
			if errors.Is(err, errRunSettledBeforeUIAttach) {
				if watchErr := watchCLIRunUntilTerminalSuccess(
					ctx,
					cmd.OutOrStdout(),
					client,
					run.RunID,
				); watchErr != nil {
					return mapDaemonCommandError(watchErr)
				}
				return nil
			}
			return mapDaemonCommandError(err)
		}
		return nil
	}
	if err := writeStartedTaskRunMultiple(cmd, run); err != nil {
		return err
	}
	if run.PresentationMode != attachModeStream {
		return nil
	}
	if err := watchCLIRunUntilTerminalSuccess(ctx, cmd.OutOrStdout(), client, run.RunID); err != nil {
		return mapDaemonCommandError(err)
	}
	return nil
}

func writeStartedTaskRun(cmd *cobra.Command, run apicore.Run) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"task run started: %s (mode=%s)\n",
		run.RunID,
		run.PresentationMode,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write task run summary: %w", err))
	}
	return nil
}

func writeStartedTaskRunMultiple(cmd *cobra.Command, run apicore.Run) error {
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"task multi-run started: %s (mode=%s)\n",
		run.RunID,
		run.PresentationMode,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write task multi-run summary: %w", err))
	}
	return nil
}

func (s *commandState) resolveTaskWorkflowName(args []string) error {
	positional := ""
	if len(args) > 0 {
		positional = strings.TrimSpace(args[0])
	}
	flagValue := strings.TrimSpace(s.name)

	switch {
	case positional != "" && flagValue != "" && positional != flagValue:
		return fmt.Errorf("workflow slug mismatch: positional %q does not match --name %q", positional, flagValue)
	case positional != "":
		s.name = positional
	case flagValue != "":
		s.name = flagValue
	default:
		return errors.New("workflow slug is required; pass it as a positional argument or with --name")
	}

	return nil
}

func (s *commandState) resolveTaskPresentationMode(cmd *cobra.Command) (string, error) {
	mode := strings.TrimSpace(s.attachMode)
	if mode == "" {
		mode = attachModeAuto
	}

	explicitModes := 0
	if commandFlagChanged(cmd, "attach") {
		explicitModes++
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{name: "ui", value: attachModeUI},
		{name: "stream", value: attachModeStream},
		{name: "detach", value: attachModeDetach},
	} {
		if !commandFlagChanged(cmd, item.name) {
			continue
		}
		mode = item.value
		explicitModes++
	}
	if explicitModes > 1 {
		return "", errors.New("choose only one of --attach, --ui, --stream, or --detach")
	}

	isInteractive := s.isInteractive
	if isInteractive == nil {
		isInteractive = isInteractiveTerminal
	}

	switch mode {
	case attachModeAuto:
		if isInteractive() {
			return attachModeUI, nil
		}
		return attachModeStream, nil
	case attachModeUI:
		if !isInteractive() {
			return "", fmt.Errorf(
				"%s requires an interactive terminal for ui mode; rerun with --stream or --detach",
				cmd.CommandPath(),
			)
		}
		return attachModeUI, nil
	case attachModeStream, attachModeDetach:
		return mode, nil
	default:
		return "", fmt.Errorf("attach mode must be one of auto, ui, stream, or detach (got %q)", mode)
	}
}

func (s *commandState) buildTaskRunRuntimeOverrides(cmd *cobra.Command) (json.RawMessage, error) {
	overrides := daemonRuntimeOverrides{}
	hasOverrides := false
	set := func(changed bool, apply func()) {
		if !changed {
			return
		}
		apply()
		hasOverrides = true
	}

	set(commandFlagChanged(cmd, "dry-run"), func() { overrides.DryRun = boolPointer(s.dryRun) })
	set(commandFlagChanged(cmd, "auto-commit"), func() { overrides.AutoCommit = boolPointer(s.autoCommit) })
	set(commandFlagChanged(cmd, "ide"), func() { overrides.IDE = stringPointer(s.ide) })
	set(commandFlagChanged(cmd, "model"), func() { overrides.Model = stringPointer(s.model) })
	set(commandFlagChanged(cmd, "add-dir"), func() {
		addDirs := core.NormalizeAddDirs(s.addDirs)
		overrides.AddDirs = &addDirs
	})
	set(commandFlagChanged(cmd, "tail-lines"), func() { overrides.TailLines = intPointer(s.tailLines) })
	set(commandFlagChanged(cmd, "reasoning-effort"), func() {
		overrides.ReasoningEffort = stringPointer(s.reasoningEffort)
	})
	set(commandFlagChanged(cmd, "access-mode"), func() { overrides.AccessMode = stringPointer(s.accessMode) })
	set(commandFlagChanged(cmd, "timeout"), func() { overrides.Timeout = stringPointer(s.timeout) })
	set(commandFlagChanged(cmd, "max-retries"), func() { overrides.MaxRetries = intPointer(s.maxRetries) })
	set(commandFlagChanged(cmd, "retry-backoff-multiplier"), func() {
		overrides.RetryBackoffMultiplier = float64Pointer(s.retryBackoffMultiplier)
	})
	set(commandFlagChanged(cmd, "include-completed"), func() {
		overrides.IncludeCompleted = boolPointer(s.includeCompleted)
	})
	set(commandFlagChanged(cmd, "recursive"), func() {
		overrides.Recursive = boolPointer(s.recursive)
	})
	set(commandFlagChanged(cmd, "task-runtime") || s.replaceConfiguredTaskRunRules, func() {
		rules := model.CloneTaskRuntimeRules(s.taskRuntimeRules())
		if rules == nil {
			rules = []model.TaskRuntimeRule{}
		}
		overrides.TaskRuntimeRules = &rules
	})

	if !hasOverrides {
		return nil, nil
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		return nil, fmt.Errorf("encode runtime overrides: %w", err)
	}
	return payload, nil
}

func mapDaemonCommandError(err error) error {
	if err == nil {
		return nil
	}

	var exitErr *commandExitError
	if errors.As(err, &exitErr) {
		return err
	}

	var remoteErr *apiclient.RemoteError
	if errors.As(err, &remoteErr) {
		switch remoteErr.StatusCode {
		case http.StatusConflict, http.StatusUnprocessableEntity:
			return withExitCode(1, remoteErr)
		default:
			return withExitCode(2, remoteErr)
		}
	}

	return withExitCode(2, err)
}

func cliDaemonHealthError(health apicore.DaemonHealth) error {
	message := "daemon is not ready"
	if len(health.Details) > 0 {
		detail := strings.TrimSpace(health.Details[0].Message)
		if detail != "" {
			message = detail
		}
	}
	return errors.New(message)
}

func boolPointer(value bool) *bool {
	return &value
}

func intPointer(value int) *int {
	return &value
}

func float64Pointer(value float64) *float64 {
	return &value
}
