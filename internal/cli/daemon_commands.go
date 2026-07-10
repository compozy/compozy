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
	"sort"
	"strings"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/gitenv"
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
	taskRunGuardRunListLimit    = 1000
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
	ListRuns(context.Context, apiclient.RunListOptions) ([]apicore.Run, error)
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
	PauseRunJob(context.Context, string, string) (apicore.RunJobControlResponse, error)
	SendRunJobMessage(
		context.Context,
		string,
		string,
		apicore.RunJobMessageRequest,
	) (apicore.RunJobControlResponse, error)
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
	DryRun                     *bool                             `json:"dry_run,omitempty"`
	RunID                      *string                           `json:"run_id,omitempty"`
	AutoCommit                 *bool                             `json:"auto_commit,omitempty"`
	IDE                        *string                           `json:"ide,omitempty"`
	Model                      *string                           `json:"model,omitempty"`
	AgentName                  *string                           `json:"agent_name,omitempty"`
	ExplicitRuntime            *model.ExplicitRuntimeFlags       `json:"explicit_runtime,omitempty"`
	OutputFormat               *string                           `json:"output_format,omitempty"`
	AddDirs                    *[]string                         `json:"add_dirs,omitempty"`
	TailLines                  *int                              `json:"tail_lines,omitempty"`
	ReasoningEffort            *string                           `json:"reasoning_effort,omitempty"`
	AccessMode                 *string                           `json:"access_mode,omitempty"`
	Timeout                    *string                           `json:"timeout,omitempty"`
	MaxRetries                 *int                              `json:"max_retries,omitempty"`
	RetryBackoffMultiplier     *float64                          `json:"retry_backoff_multiplier,omitempty"`
	Verbose                    *bool                             `json:"verbose,omitempty"`
	Persist                    *bool                             `json:"persist,omitempty"`
	IncludeCompleted           *bool                             `json:"include_completed,omitempty"`
	Recursive                  *bool                             `json:"recursive,omitempty"`
	TaskRuntimeRules           *[]model.TaskRuntimeRule          `json:"task_runtime_rules,omitempty"`
	EnableExecutableExtensions *bool                             `json:"enable_executable_extensions,omitempty"`
	Recovery                   *workspacecfg.AgentRecoveryConfig `json:"recovery,omitempty"`
	ParallelTasks              *workspacecfg.ParallelTasksConfig `json:"parallel_tasks,omitempty"`
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
	daemonContractVersion := firstNonEmptyDaemonContractVersion(status.ContractVersion, health.ContractVersion)
	cliContractVersion := contract.DaemonContractVersion
	if !daemonContractVersionsCompatible(daemonContractVersion, cliContractVersion) {
		return nil, &cliDaemonVersionMismatchError{
			info:                  info,
			client:                client,
			daemonVersion:         firstNonEmptyDaemonBuildVersion(status.Version, info.Version),
			cliVersion:            b.currentCLIVersion(),
			daemonContractVersion: daemonContractVersion,
			cliContractVersion:    cliContractVersion,
			activeRunCount:        status.ActiveRunCount,
			contractMismatch:      true,
		}
	}
	daemonVersion := firstNonEmptyDaemonBuildVersion(status.Version, info.Version)
	cliVersion := b.currentCLIVersion()
	if !daemonBuildVersionsCompatible(daemonVersion, cliVersion) {
		if status.ActiveRunCount > 0 {
			return client, nil
		}
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
	if mismatch.contractMismatch {
		if err := b.notify(fmt.Sprintf(
			"Restarting incompatible compozy daemon (daemon contract %s, CLI contract %s).",
			displayDaemonContractVersion(mismatch.daemonContractVersion),
			displayDaemonContractVersion(mismatch.cliContractVersion),
		)); err != nil {
			return fmt.Errorf("write daemon restart notice: %w", err)
		}
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
	info                  daemon.Info
	client                daemonCommandClient
	daemonVersion         string
	cliVersion            string
	daemonContractVersion string
	cliContractVersion    string
	activeRunCount        int
	contractMismatch      bool
}

func (e *cliDaemonVersionMismatchError) Error() string {
	if e == nil {
		return "daemon version mismatch"
	}
	if e.contractMismatch {
		if e.activeRunCount > 0 {
			return fmt.Sprintf(
				"running compozy daemon contract version %q does not match CLI contract version %q and has %d active runs; "+
					"retry after the active runs finish or, when it is safe to cancel them, run "+
					"`compozy daemon stop --force` and retry so the CLI starts the current daemon",
				displayDaemonContractVersion(e.daemonContractVersion),
				displayDaemonContractVersion(e.cliContractVersion),
				e.activeRunCount,
			)
		}
		return fmt.Sprintf(
			"running compozy daemon contract version %q does not match CLI contract version %q",
			displayDaemonContractVersion(e.daemonContractVersion),
			displayDaemonContractVersion(e.cliContractVersion),
		)
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

func firstNonEmptyDaemonContractVersion(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func daemonContractVersionsCompatible(daemonVersion string, cliVersion string) bool {
	daemonVersion = strings.TrimSpace(daemonVersion)
	cliVersion = strings.TrimSpace(cliVersion)
	if daemonVersion == "" {
		return true
	}
	return cliVersion != "" && daemonVersion == cliVersion
}

func displayDaemonContractVersion(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "legacy"
	}
	return trimmed
}

func daemonBuildVersionsCompatible(daemonVersion string, cliVersion string) bool {
	daemonVersion = strings.TrimSpace(daemonVersion)
	cliVersion = strings.TrimSpace(cliVersion)
	if daemonVersion == cliVersion {
		return true
	}
	if isDevBuildVersion(daemonVersion) || isDevBuildVersion(cliVersion) {
		return true
	}
	daemonIdentity, daemonOK := parseDaemonBuildIdentity(daemonVersion)
	cliIdentity, cliOK := parseDaemonBuildIdentity(cliVersion)
	return daemonOK && cliOK && daemonIdentity == cliIdentity
}

type daemonBuildIdentity struct {
	version string
	commit  string
}

func parseDaemonBuildIdentity(value string) (daemonBuildIdentity, bool) {
	versionPart, metadata, ok := strings.Cut(strings.TrimSpace(value), " (")
	if !ok {
		return daemonBuildIdentity{}, false
	}
	versionPart = strings.TrimSpace(versionPart)
	if versionPart == "" || isDevBuildVersion(versionPart) {
		return daemonBuildIdentity{}, false
	}
	metadata = strings.TrimSpace(metadata)
	if !strings.HasSuffix(metadata, ")") {
		return daemonBuildIdentity{}, false
	}
	metadata = strings.TrimSuffix(metadata, ")")

	var commit string
	var date string
	for _, field := range strings.Fields(metadata) {
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "commit":
			commit = strings.TrimSpace(val)
		case "date":
			date = strings.TrimSpace(val)
		}
	}
	if !isStableDaemonBuildCommit(commit) || strings.TrimSpace(date) == "" {
		return daemonBuildIdentity{}, false
	}
	return daemonBuildIdentity{
		version: versionPart,
		commit:  commit,
	}, true
}

func isStableDaemonBuildCommit(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized != "" && normalized != version.UnstampedCommit
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
through one daemon-owned parent run. The queue runs in enqueued order by default.
Pass --parallel (or set run_multiple_mode = "parallel") to run children in
parallel with git worktree isolation, and --parallel-limit to bound the
concurrent child fanout.

Pass --parallel-tasks on a single workflow run to execute pending task files in
dependency-aware waves using the resolved [tasks.run.parallel] config.`,
		Example: `  compozy tasks run my-feature
  compozy tasks run my-feature --parallel-tasks
  compozy tasks run --multiple alpha,beta --stream
  compozy tasks run --multiple alpha,beta --detach
  compozy tasks run --multiple alpha,beta --parallel
  compozy tasks run --multiple alpha,beta --parallel --parallel-limit 3
  compozy tasks run --multiple alpha,beta --ide codex --model gpt-5.6-sol
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

const (
	taskRunParallelTasksFlag                     = "parallel-tasks"
	taskRunParallelConflictResolverIDEFlag       = "parallel-conflict-resolver-ide"
	taskRunParallelConflictResolverModelFlag     = "parallel-conflict-resolver-model"
	taskRunParallelConflictResolverReasoningFlag = "parallel-conflict-resolver-reasoning"
)

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
	cmd.Flags().BoolVar(
		&state.parallel,
		"parallel",
		false,
		"Run --multiple task workflows in parallel with git worktree isolation "+
			"(overrides run_multiple_mode; valid only with --multiple)",
	)
	cmd.Flags().IntVar(
		&state.parallelLimit,
		"parallel-limit",
		workspacecfg.DefaultRunMultipleParallelLimit,
		"Maximum number of child runs started at once in --parallel mode "+
			"(overrides run_multiple_parallel_limit; must be greater than 0; valid only with --multiple)",
	)
	cmd.Flags().BoolVar(
		&state.parallelTasks,
		taskRunParallelTasksFlag,
		false,
		"Run this PRD task workflow in dependency-aware parallel task mode",
	)
	cmd.Flags().StringVar(&state.parallelConflictResolverIDE, taskRunParallelConflictResolverIDEFlag, "", "")
	cmd.Flags().StringVar(&state.parallelConflictResolverModel, taskRunParallelConflictResolverModelFlag, "", "")
	cmd.Flags().StringVar(
		&state.parallelConflictResolverReasoningEffort,
		taskRunParallelConflictResolverReasoningFlag,
		"",
		"",
	)
	hideTaskRunWizardFlag(cmd, taskRunParallelConflictResolverIDEFlag)
	hideTaskRunWizardFlag(cmd, taskRunParallelConflictResolverModelFlag)
	hideTaskRunWizardFlag(cmd, taskRunParallelConflictResolverReasoningFlag)
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
		`Per-task runtime override rule for task runs (repeatable). Use key=value pairs such as type=frontend,ide=codex,model=gpt-5.6-sol or id=task_01,reasoning-effort=xhigh`,
	)
}

func hideTaskRunWizardFlag(cmd *cobra.Command, flagName string) {
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		return
	}
	flag.Hidden = true
}

func (s *commandState) runTaskWorkflow(cmd *cobra.Command, args []string) error {
	if commandFlagChanged(cmd, "multiple") {
		return s.runTaskWorkflowsMultiple(cmd, args)
	}
	if err := rejectMultipleOnlyParallelFlags(cmd); err != nil {
		return withExitCode(1, err)
	}

	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	if len(args) == 0 && strings.TrimSpace(s.name) == "" && strings.TrimSpace(s.multiple) == "" {
		if err := s.maybeCollectInteractiveParams(cmd); err != nil {
			return err
		}
	}
	if strings.TrimSpace(s.multiple) != "" {
		slugs, err := s.resolveTaskWorkflowSlugList(args)
		if err != nil {
			return withExitCode(1, err)
		}
		s.explicitRuntime = captureExplicitRuntimeFlags(cmd)
		return s.runTaskWorkflowsMultiplePrepared(ctx, cmd, slugs)
	}
	return s.runTaskWorkflowPrepared(ctx, cmd, args)
}

func (s *commandState) runTaskWorkflowPrepared(ctx context.Context, cmd *cobra.Command, args []string) error {
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
	execution := s.resolveSingleTaskExecution(cmd)
	if execution.Kind == apicore.ExecutionKindTaskParallel {
		if err := s.preflightParallelWorktreeMode(ctx); err != nil {
			return err
		}
	}

	presentationMode, err := s.resolveTaskPresentationMode(cmd)
	if err != nil {
		return withExitCode(1, err)
	}
	runtimeOverrides, err := s.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		return withExitCode(2, err)
	}
	if err := writeTaskExecutionResolution(cmd, execution); err != nil {
		return err
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}
	s.warnIfOtherWorkspaceTaskRunsActive(ctx, cmd, client)

	run, err := client.StartTaskRun(ctx, s.name, apicore.TaskRunRequest{
		Workspace:        s.workspaceRoot,
		PresentationMode: presentationMode,
		RuntimeOverrides: runtimeOverrides,
		Execution:        &execution,
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
	if commandFlagChanged(cmd, taskRunParallelTasksFlag) {
		return withExitCode(1, errors.New(
			"--parallel-tasks cannot be combined with --multiple; use --parallel for slug multi-run mode",
		))
	}
	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	s.explicitRuntime = captureExplicitRuntimeFlags(cmd)
	return s.runTaskWorkflowsMultiplePrepared(ctx, cmd, slugs)
}

func (s *commandState) runTaskWorkflowsMultiplePrepared(
	ctx context.Context,
	cmd *cobra.Command,
	slugs []string,
) error {
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
	execution := s.resolveTaskRunMultipleExecution(cmd, mode)
	parallelLimit, err := s.resolveTaskRunMultipleParallelLimit(cmd)
	if err != nil {
		return withExitCode(1, err)
	}
	// An explicit --parallel-limit has no effect in enqueued mode; reject it
	// instead of silently discarding the value when mode resolves to enqueued.
	if commandFlagChanged(cmd, "parallel-limit") && mode != workspacecfg.TaskRunMultipleModeParallel {
		return withExitCode(2, errors.New(
			`--parallel-limit requires parallel mode; pass --parallel or set run_multiple_mode = "parallel"`,
		))
	}
	if mode == workspacecfg.TaskRunMultipleModeParallel {
		if err := s.preflightParallelWorktreeMode(ctx); err != nil {
			return err
		}
	}
	runtimeOverrides, err := s.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		return withExitCode(2, err)
	}
	if err := writeTaskExecutionResolution(cmd, execution); err != nil {
		return err
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}
	s.warnIfOtherWorkspaceTaskRunsActive(ctx, cmd, client)

	request := apicore.TaskRunMultipleRequest{
		Workspace:        s.workspaceRoot,
		Slugs:            slugs,
		Mode:             mode,
		PresentationMode: presentationMode,
		RuntimeOverrides: runtimeOverrides,
		Execution:        &execution,
	}
	if mode == workspacecfg.TaskRunMultipleModeParallel {
		request.ParallelLimit = parallelLimit
	}
	run, err := client.StartTaskRunMultiple(ctx, request)
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

// rejectMultipleOnlyParallelFlags rejects --parallel and --parallel-limit when
// they are used without --multiple, before any daemon contact.
func rejectMultipleOnlyParallelFlags(cmd *cobra.Command) error {
	if commandFlagChanged(cmd, "parallel") {
		return errors.New("--parallel is only valid with --multiple")
	}
	if commandFlagChanged(cmd, "parallel-limit") {
		return errors.New("--parallel-limit is only valid with --multiple")
	}
	return nil
}

func (s *commandState) preflightParallelWorktreeMode(ctx context.Context) error {
	root := strings.TrimSpace(s.workspaceRoot)
	if root == "" {
		return withExitCode(2, errors.New("parallel worktree-backed task runs require a workspace root"))
	}
	paths, err := resolveCLIDaemonHomePaths()
	if err != nil {
		return withExitCode(2, fmt.Errorf("resolve daemon home paths for parallel worktree preflight: %w", err))
	}
	inside, err := cliPathEqualOrInside(paths.WorktreesDir, root)
	if err != nil {
		return withExitCode(2, fmt.Errorf("inspect Compozy-managed worktree root for %s: %w", root, err))
	}
	if inside {
		return withExitCode(
			2,
			fmt.Errorf(
				"workspace %s is inside Compozy-managed worktree root %s; "+
					"parallel task runs are not supported from a Compozy-managed worktree",
				root,
				paths.WorktreesDir,
			),
		)
	}
	branch, err := runTaskRunGitPreflight(ctx, root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return withExitCode(
			2,
			fmt.Errorf(
				"parallel worktree-backed task runs require a git workspace with a named branch at %s: %w",
				root,
				err,
			),
		)
	}
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" {
		return withExitCode(
			2,
			fmt.Errorf(
				"workspace %s is on a detached HEAD; checkout a branch before starting parallel worktree-backed task runs",
				root,
			),
		)
	}
	return nil
}

func runTaskRunGitPreflight(ctx context.Context, workspaceRoot string, args ...string) (string, error) {
	return gitenv.Run(ctx, workspaceRoot, args...)
}

func cliPathEqualOrInside(parent string, child string) (bool, error) {
	parentPath, err := cliCleanContainmentPath(parent)
	if err != nil {
		return false, err
	}
	childPath, err := cliCleanContainmentPath(child)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(parentPath, childPath)
	if err != nil {
		return false, fmt.Errorf("relativize path %s to %s: %w", childPath, parentPath, err)
	}
	return rel == "." || (rel != "" && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

func cliCleanContainmentPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", errors.New("path is required")
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", trimmed, err)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		absolute = resolved
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("resolve symlinks for %s: %w", absolute, err)
	}
	return filepath.Clean(absolute), nil
}

// resolveTaskRunMultipleMode resolves the multi-run scheduling mode with
// precedence: --parallel, then configured run_multiple_mode, then enqueued.
func (s *commandState) resolveTaskRunMultipleMode(cmd *cobra.Command) (string, error) {
	if commandFlagChanged(cmd, "parallel") {
		if s.parallel {
			return workspacecfg.TaskRunMultipleModeParallel, nil
		}
		return workspacecfg.TaskRunMultipleModeEnqueued, nil
	}
	mode := s.projectConfig.Tasks.Run.EffectiveRunMultipleMode()
	switch mode {
	case workspacecfg.TaskRunMultipleModeEnqueued, workspacecfg.TaskRunMultipleModeParallel:
		return mode, nil
	default:
		return "", fmt.Errorf("tasks.run.run_multiple_mode must be %q or %q (got %q)",
			workspacecfg.TaskRunMultipleModeEnqueued,
			workspacecfg.TaskRunMultipleModeParallel,
			mode,
		)
	}
}

func (s *commandState) resolveSingleTaskExecution(cmd *cobra.Command) apicore.TaskExecutionDescriptor {
	if commandFlagChanged(cmd, taskRunParallelTasksFlag) && s.parallelTasks {
		return apicore.TaskExecutionDescriptor{
			Kind:          apicore.ExecutionKindTaskParallel,
			Label:         "Parallel tasks (task worktrees + integration branch)",
			UsesWorktrees: true,
			Source:        "--parallel-tasks=true",
		}
	}
	source := "per-run default"
	if commandFlagChanged(cmd, taskRunParallelTasksFlag) {
		source = "--parallel-tasks=false"
	}
	return apicore.TaskExecutionDescriptor{
		Kind:          apicore.ExecutionKindTaskStandard,
		Label:         "Standard task run",
		UsesWorktrees: false,
		Source:        source,
	}
}

func (s *commandState) resolveTaskRunMultipleExecution(
	cmd *cobra.Command,
	mode string,
) apicore.TaskExecutionDescriptor {
	source := "built-in default"
	if commandFlagChanged(cmd, "parallel") {
		if mode == workspacecfg.TaskRunMultipleModeParallel {
			source = "--parallel=true"
		} else {
			source = "--parallel=false"
		}
	} else if s.projectConfig.Tasks.Run.RunMultipleMode != nil {
		source = "workspace tasks.run.run_multiple_mode"
	}
	if mode == workspacecfg.TaskRunMultipleModeParallel {
		return apicore.TaskExecutionDescriptor{
			Kind:          apicore.ExecutionKindTaskMultiParallel,
			Label:         "Parallel workflows (git worktrees)",
			UsesWorktrees: true,
			Source:        source,
		}
	}
	return apicore.TaskExecutionDescriptor{
		Kind:          apicore.ExecutionKindTaskMultiEnqueued,
		Label:         "Serial queue (no worktrees)",
		UsesWorktrees: false,
		Source:        source,
	}
}

// resolveTaskRunMultipleParallelLimit resolves the parallel fanout limit with
// precedence: --parallel-limit, then configured run_multiple_parallel_limit,
// then the default. It rejects zero and negative limits before daemon contact.
func (s *commandState) resolveTaskRunMultipleParallelLimit(cmd *cobra.Command) (int, error) {
	limit := s.projectConfig.Tasks.Run.EffectiveRunMultipleParallelLimit()
	if commandFlagChanged(cmd, "parallel-limit") {
		limit = s.parallelLimit
	}
	if limit <= 0 {
		return 0, fmt.Errorf("--parallel-limit must be greater than 0 (got %d)", limit)
	}
	return limit, nil
}

type taskRunBusyWorkspace struct {
	workspaceID string
	rootDir     string
	name        string
	runIDs      []string
}

var taskRunGuardActiveStatuses = []string{"starting", "running", "pending", "retrying"}

func (s *commandState) warnIfOtherWorkspaceTaskRunsActive(
	ctx context.Context,
	cmd *cobra.Command,
	client daemonCommandClient,
) {
	if s.dryRun {
		return
	}
	status, err := client.DaemonStatus(ctx)
	if err != nil {
		// This guard is advisory; inability to inspect daemon state must not block a task run.
		return
	}
	if status.ActiveRunCount <= 0 {
		return
	}

	activeRuns, err := listTaskRunGuardActiveRuns(ctx, client)
	if err != nil {
		return
	}
	if len(activeRuns) == 0 {
		return
	}
	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		return
	}
	busy := otherWorkspaceActiveRuns(s.workspaceRoot, activeRuns, workspaces)
	if len(busy) == 0 {
		return
	}
	writeTaskRunConcurrencyWarning(cmd, busy)
}

func listTaskRunGuardActiveRuns(ctx context.Context, client daemonCommandClient) ([]apicore.Run, error) {
	listed, err := client.ListRuns(ctx, apiclient.RunListOptions{
		Statuses: taskRunGuardActiveStatuses,
		Limit:    taskRunGuardRunListLimit,
	})
	if err != nil {
		return nil, err
	}
	runs := make([]apicore.Run, 0, len(listed))
	seen := make(map[string]struct{})
	for i := range listed {
		run := listed[i]
		runID := strings.TrimSpace(run.RunID)
		if runID == "" {
			continue
		}
		if _, ok := seen[runID]; ok {
			continue
		}
		seen[runID] = struct{}{}
		runs = append(runs, run)
	}
	return runs, nil
}

func otherWorkspaceActiveRuns(
	currentRoot string,
	activeRuns []apicore.Run,
	workspaces []apicore.Workspace,
) []taskRunBusyWorkspace {
	workspaceByID, currentWorkspaceID, currentKey := taskRunGuardWorkspaceIndex(currentRoot, workspaces)
	groupsByWorkspace := make(map[string]*taskRunBusyWorkspace)
	for i := range activeRuns {
		run := activeRuns[i]
		runID := strings.TrimSpace(run.RunID)
		workspaceID := strings.TrimSpace(run.WorkspaceID)
		if runID == "" || workspaceID == "" {
			continue
		}
		workspace := workspaceByID[workspaceID]
		if taskRunGuardIsCurrentWorkspace(workspaceID, currentWorkspaceID, currentKey, workspace) {
			continue
		}
		// The registry can miss a workspace referenced by a durable run; keep reporting it by id.
		group := taskRunGuardBusyWorkspaceGroup(groupsByWorkspace, workspaceID, workspace)
		group.runIDs = append(group.runIDs, runID)
	}
	return taskRunGuardSortedBusyWorkspaces(groupsByWorkspace)
}

func taskRunGuardWorkspaceIndex(
	currentRoot string,
	workspaces []apicore.Workspace,
) (map[string]*apicore.Workspace, string, string) {
	workspaceByID := make(map[string]*apicore.Workspace, len(workspaces))
	currentWorkspaceID := ""
	currentKey := taskRunGuardWorkspaceRootKey(currentRoot)
	for i := range workspaces {
		workspace := &workspaces[i]
		workspaceID := strings.TrimSpace(workspace.ID)
		if workspaceID != "" {
			workspaceByID[workspaceID] = workspace
		}
		if currentKey != "" && taskRunGuardWorkspaceRootKey(workspace.RootDir) == currentKey {
			currentWorkspaceID = workspaceID
		}
	}
	return workspaceByID, currentWorkspaceID, currentKey
}

func taskRunGuardIsCurrentWorkspace(
	workspaceID string,
	currentWorkspaceID string,
	currentKey string,
	workspace *apicore.Workspace,
) bool {
	if currentWorkspaceID != "" {
		return workspaceID == currentWorkspaceID
	}
	return currentKey != "" && workspace != nil && taskRunGuardWorkspaceRootKey(workspace.RootDir) == currentKey
}

func taskRunGuardBusyWorkspaceGroup(
	groupsByWorkspace map[string]*taskRunBusyWorkspace,
	workspaceID string,
	workspace *apicore.Workspace,
) *taskRunBusyWorkspace {
	group, ok := groupsByWorkspace[workspaceID]
	if ok {
		return group
	}
	group = &taskRunBusyWorkspace{workspaceID: workspaceID}
	if workspace != nil {
		group.rootDir = strings.TrimSpace(workspace.RootDir)
		group.name = strings.TrimSpace(workspace.Name)
	}
	groupsByWorkspace[workspaceID] = group
	return group
}

func taskRunGuardSortedBusyWorkspaces(
	groupsByWorkspace map[string]*taskRunBusyWorkspace,
) []taskRunBusyWorkspace {
	busy := make([]taskRunBusyWorkspace, 0, len(groupsByWorkspace))
	for _, group := range groupsByWorkspace {
		sort.Strings(group.runIDs)
		busy = append(busy, *group)
	}
	sort.Slice(busy, func(i, j int) bool {
		left := taskRunBusyWorkspaceLabel(busy[i])
		right := taskRunBusyWorkspaceLabel(busy[j])
		if left == right {
			return busy[i].workspaceID < busy[j].workspaceID
		}
		return left < right
	})
	return busy
}

func taskRunGuardWorkspaceRootKey(root string) string {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return ""
	}
	absolute, err := filepath.Abs(trimmed)
	if err != nil {
		absolute = trimmed
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		absolute = resolved
	}
	return filepath.Clean(absolute)
}

func writeTaskRunConcurrencyWarning(cmd *cobra.Command, busy []taskRunBusyWorkspace) {
	writer := cmd.ErrOrStderr()
	if _, err := fmt.Fprintln(
		writer,
		"Warning: daemon already has active run(s) in another workspace; starting this task run concurrently.",
	); err != nil {
		// The warning is advisory; a closed stderr must not prevent starting the run.
		return
	}
	if _, err := fmt.Fprintln(writer, "Busy workspace(s):"); err != nil {
		return
	}
	for _, workspace := range busy {
		if _, err := fmt.Fprintf(
			writer,
			"  - %s: %s\n",
			taskRunBusyWorkspaceLabel(workspace),
			strings.Join(workspace.runIDs, ", "),
		); err != nil {
			return
		}
	}
}

func taskRunBusyWorkspaceLabel(workspace taskRunBusyWorkspace) string {
	parts := make([]string, 0, 3)
	if workspace.name != "" {
		parts = append(parts, workspace.name)
	}
	if workspace.rootDir != "" {
		parts = append(parts, workspace.rootDir)
	}
	if workspace.workspaceID != "" {
		parts = append(parts, "id="+workspace.workspaceID)
	}
	if len(parts) == 0 {
		return "unknown workspace"
	}
	return strings.Join(parts, " | ")
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
				return streamTaskRunMultipleToTerminal(ctx, cmd, client, run.RunID)
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
	return streamTaskRunMultipleToTerminal(ctx, cmd, client, run.RunID)
}

// streamTaskRunMultipleToTerminal streams the parent queue until it settles, then
// always writes the final per-child worktree handoff. The aggregate watch error
// (failed/canceled/crashed -> exit 1) takes precedence so the command still exits
// non-zero, while the handoff is printed best-effort even on failure.
func streamTaskRunMultipleToTerminal(
	ctx context.Context,
	cmd *cobra.Command,
	client daemonCommandClient,
	runID string,
) error {
	watchErr := watchCLIRunUntilTerminalSuccess(ctx, cmd.OutOrStdout(), client, runID)
	// A canceled context (e.g. Ctrl+C) makes the watch return nil; skip the
	// handoff fetch, which would otherwise fail with a context-canceled error and
	// turn a clean interrupt into a reported failure.
	if ctx.Err() != nil {
		return nil
	}
	handoffErr := writeTaskRunMultipleHandoff(ctx, cmd.OutOrStdout(), client, runID)
	if watchErr != nil {
		return mapDaemonCommandError(watchErr)
	}
	return handoffErr
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

func writeTaskExecutionResolution(cmd *cobra.Command, execution apicore.TaskExecutionDescriptor) error {
	if cmd == nil {
		return errors.New("task execution output command is required")
	}
	if _, err := fmt.Fprintf(
		cmd.ErrOrStderr(),
		"execution: %s (kind=%s, worktrees=%t, source=%s)\n",
		execution.Label,
		execution.Kind,
		execution.UsesWorktrees,
		execution.Source,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write task execution resolution: %w", err))
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
		{name: "background", value: attachModeDetach},
	} {
		if !commandFlagChanged(cmd, item.name) {
			continue
		}
		mode = item.value
		explicitModes++
	}
	if explicitModes > 1 {
		message := "choose only one of --attach, --ui, --stream, or --detach"
		if cmd != nil && cmd.Flags() != nil && cmd.Flags().Lookup("background") != nil {
			message = "choose only one of --attach, --ui, --stream, --detach, or --background"
		}
		return "", errors.New(message)
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
	recovery, err := s.recoveryFlagOverrides(cmd)
	if err != nil {
		return nil, err
	}
	if recovery != nil {
		overrides.Recovery = recovery
		hasOverrides = true
	}
	parallelTasks, err := s.parallelTasksFlagOverrides(cmd)
	if err != nil {
		return nil, err
	}
	if parallelTasks != nil {
		overrides.ParallelTasks = parallelTasks
		hasOverrides = true
	}

	if !hasOverrides {
		return nil, nil
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		return nil, fmt.Errorf("encode runtime overrides: %w", err)
	}
	return payload, nil
}

func (s *commandState) recoveryFlagOverrides(cmd *cobra.Command) (*workspacecfg.AgentRecoveryConfig, error) {
	cfg := workspacecfg.AgentRecoveryConfig{}
	changed := false
	if commandFlagChanged(cmd, "recovery") {
		cfg.Enabled = boolPointer(s.recoveryEnabled)
		changed = true
	}
	if commandFlagChanged(cmd, "no-recovery") && s.recoveryDisabled {
		cfg.Enabled = boolPointer(false)
		changed = true
	}
	if commandFlagChanged(cmd, "recovery-ide") {
		cfg.IDE = stringPointer(s.recoveryIDE)
		changed = true
	}
	if commandFlagChanged(cmd, "recovery-model") {
		cfg.Model = stringPointer(s.recoveryModel)
		changed = true
	}
	if commandFlagChanged(cmd, "recovery-reasoning") {
		cfg.ReasoningEffort = stringPointer(s.recoveryReasoningEffort)
		changed = true
	}
	if commandFlagChanged(cmd, "recovery-max-attempts") {
		cfg.MaxAttempts = intPointer(s.recoveryMaxAttempts)
		changed = true
	}
	if !changed {
		return nil, nil
	}
	if err := workspacecfg.ValidateAgentRecoveryConfig("CLI recovery flags", cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *commandState) parallelTasksFlagOverrides(cmd *cobra.Command) (*workspacecfg.ParallelTasksConfig, error) {
	cfg := workspacecfg.ParallelTasksConfig{}
	changed := false
	if commandFlagChanged(cmd, taskRunParallelTasksFlag) {
		cfg.Enabled = boolPointer(s.parallelTasks)
		changed = true
	}
	resolver := workspacecfg.ConflictResolverConfig{}
	resolverChanged := false
	if commandFlagChanged(cmd, taskRunParallelConflictResolverIDEFlag) {
		resolver.IDE = stringPointer(s.parallelConflictResolverIDE)
		resolverChanged = true
	}
	if commandFlagChanged(cmd, taskRunParallelConflictResolverModelFlag) {
		resolver.Model = stringPointer(s.parallelConflictResolverModel)
		resolverChanged = true
	}
	if commandFlagChanged(cmd, taskRunParallelConflictResolverReasoningFlag) {
		resolver.ReasoningEffort = stringPointer(s.parallelConflictResolverReasoningEffort)
		resolverChanged = true
	}
	if resolverChanged {
		cfg.ConflictResolver = &resolver
		changed = true
		if err := workspacecfg.ValidateConflictResolverConfig(
			"CLI parallel conflict resolver flags",
			resolver,
		); err != nil {
			return nil, err
		}
	}
	if !changed {
		return nil, nil
	}
	return &cfg, nil
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
