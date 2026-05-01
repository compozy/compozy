package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/reviews"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const (
	defaultReviewWatchMaxRounds     = 6
	defaultReviewWatchPollInterval  = 30 * time.Second
	defaultReviewWatchTimeout       = 30 * time.Minute
	defaultReviewWatchQuietPeriod   = 20 * time.Second
	reviewWatchChildPollInterval    = 100 * time.Millisecond
	reviewWatchInvokingCommand      = "compozy reviews watch"
	reviewWatchPushStatusStartup    = "startup_unpushed_head"
	reviewWatchTerminalClean        = "review watch clean"
	reviewWatchTerminalMaxRounds    = "review watch reached max rounds"
	reviewWatchTerminalRoundHandled = "review watch round completed"
)

type reviewProviderRegistryFactory func(context.Context, string, string) (provider.RegistryReader, func(), error)

type reviewWatchKey struct {
	WorkspaceID string
	Provider    string
	PR          string
}

type reviewWatchOptions struct {
	Provider         string
	PR               string
	UntilClean       bool
	MaxRounds        int
	AutoPush         bool
	PushRemote       string
	PushBranch       string
	Nitpicks         bool
	PollInterval     time.Duration
	ReviewTimeout    time.Duration
	QuietPeriod      time.Duration
	RuntimeOverrides json.RawMessage
	Batching         json.RawMessage
}

type reviewWatchLoopOptions struct {
	UntilClean    bool
	MaxRounds     int
	AutoPush      bool
	PollInterval  time.Duration
	ReviewTimeout time.Duration
	QuietPeriod   time.Duration
}

type preparedReviewWatch struct {
	workspace      globaldb.Workspace
	workflowID     *string
	workflowSlug   string
	workflowRoot   string
	options        reviewWatchOptions
	lastRound      int
	lastChildRunID string
	lastHeadSHA    string
}

type reviewWatchCoordinatorRuntime struct {
	registry       provider.RegistryReader
	cleanup        func()
	reviewProvider provider.Provider
	options        reviewWatchOptions
	gitState       ReviewWatchGitState
}

func resolveReviewProviderRegistryFactory(
	factory reviewProviderRegistryFactory,
) reviewProviderRegistryFactory {
	if factory != nil {
		return factory
	}
	return buildWorkspaceReviewRegistry
}

func resolveReviewWatchGit(git ReviewWatchGit) ReviewWatchGit {
	if git != nil {
		return git
	}
	return newExecReviewWatchGit()
}

// StartReviewWatch starts one daemon-owned review-watch parent run.
func (m *RunManager) StartReviewWatch(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.ReviewWatchRequest,
) (apicore.Run, error) {
	prepared, runtimeCfg, err := m.prepareReviewWatchStart(detachContext(ctx), workspaceRef, workflowSlug, req)
	if err != nil {
		return apicore.Run{}, err
	}
	key := reviewWatchKey{
		WorkspaceID: prepared.workspace.ID,
		Provider:    prepared.options.Provider,
		PR:          prepared.options.PR,
	}
	if err := m.reserveReviewWatch(key); err != nil {
		return apicore.Run{}, err
	}
	started := false
	defer func() {
		if !started {
			m.releaseReviewWatch(key)
		}
	}()

	run, err := m.startRun(ctx, startRunSpec{
		workspace:        prepared.workspace,
		workflowID:       prepared.workflowID,
		workflowSlug:     prepared.workflowSlug,
		workflowRoot:     prepared.workflowRoot,
		mode:             runModeReviewWatch,
		presentationMode: defaultPresentationMode,
		runtimeCfg:       runtimeCfg,
		reviewWatch:      prepared,
		reviewWatchKey:   &key,
	})
	if err != nil {
		return apicore.Run{}, err
	}
	started = true
	return run, nil
}

func (m *RunManager) prepareReviewWatchStart(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.ReviewWatchRequest,
) (*preparedReviewWatch, *model.RuntimeConfig, error) {
	workspaceRow, workflowID, projectCfg, err := m.resolveWorkflowContext(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return nil, nil, err
	}
	options, err := resolveReviewWatchOptions(projectCfg, req)
	if err != nil {
		return nil, nil, err
	}
	overrides, err := parseRuntimeOverrides(req.RuntimeOverrides)
	if err != nil {
		return nil, nil, err
	}
	if options.AutoPush && overrides.AutoCommit != nil && !*overrides.AutoCommit {
		return nil, nil, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_watch_request",
			"auto_push requires auto_commit to be true",
			map[string]any{"field": "runtime_overrides.auto_commit"},
			nil,
		)
	}
	childRuntimeOverrides, err := reviewWatchChildRuntimeOverrides(req.RuntimeOverrides, options.AutoPush)
	if err != nil {
		return nil, nil, err
	}
	options.RuntimeOverrides = childRuntimeOverrides
	if _, err := parseReviewBatching(req.Batching); err != nil {
		return nil, nil, err
	}
	options.Batching = cloneJSON(req.Batching)

	workflowRoot := model.TaskDirectoryForWorkspace(workspaceRow.RootDir, workflowSlug)
	if err := requireDirectory(workflowRoot); err != nil {
		return nil, nil, err
	}

	runtimeCfg := &model.RuntimeConfig{
		WorkspaceRoot:              workspaceRow.RootDir,
		Name:                       strings.TrimSpace(workflowSlug),
		Provider:                   options.Provider,
		PR:                         options.PR,
		TasksDir:                   workflowRoot,
		Mode:                       model.ExecutionModePRReview,
		DaemonOwned:                true,
		EnableExecutableExtensions: true,
	}
	applySoundConfig(runtimeCfg, projectCfg.Sound)
	if overrides.RunID != nil {
		runtimeCfg.RunID = strings.TrimSpace(*overrides.RunID)
	}
	runtimeCfg.ApplyDefaults()
	runtimeCfg.TUI = false
	runtimeCfg.EnableExecutableExtensions = true

	return &preparedReviewWatch{
		workspace:    workspaceRow,
		workflowID:   workflowID,
		workflowSlug: strings.TrimSpace(workflowSlug),
		workflowRoot: workflowRoot,
		options:      options,
	}, runtimeCfg, nil
}

func resolveReviewWatchOptions(
	projectCfg workspacecfg.ProjectConfig,
	req apicore.ReviewWatchRequest,
) (reviewWatchOptions, error) {
	providerName := resolveFetchProvider(projectCfg, req.Provider)
	if strings.TrimSpace(providerName) == "" {
		return reviewWatchOptions{}, reviewWatchValidationProblem(
			"provider_required",
			"reviews watch requires provider",
			"provider",
		)
	}
	pr := strings.TrimSpace(req.PRRef)
	if pr == "" {
		return reviewWatchOptions{}, reviewWatchValidationProblem(
			"pr_required",
			"reviews watch requires pr_ref",
			"pr_ref",
		)
	}

	watchCfg := projectCfg.WatchReviews
	loopOptions, err := resolveReviewWatchLoopOptions(watchCfg, req)
	if err != nil {
		return reviewWatchOptions{}, err
	}

	return reviewWatchOptions{
		Provider:      strings.TrimSpace(providerName),
		PR:            pr,
		UntilClean:    loopOptions.UntilClean,
		MaxRounds:     loopOptions.MaxRounds,
		AutoPush:      loopOptions.AutoPush,
		PushRemote:    firstNonEmptyString(req.PushRemote, optionalString(watchCfg.PushRemote)),
		PushBranch:    firstNonEmptyString(req.PushBranch, optionalString(watchCfg.PushBranch)),
		Nitpicks:      resolveFetchNitpicks(projectCfg),
		PollInterval:  loopOptions.PollInterval,
		ReviewTimeout: loopOptions.ReviewTimeout,
		QuietPeriod:   loopOptions.QuietPeriod,
	}, nil
}

func resolveReviewWatchLoopOptions(
	watchCfg workspacecfg.WatchReviewsConfig,
	req apicore.ReviewWatchRequest,
) (reviewWatchLoopOptions, error) {
	maxRounds := defaultReviewWatchMaxRounds
	if watchCfg.MaxRounds != nil && *watchCfg.MaxRounds > 0 {
		maxRounds = *watchCfg.MaxRounds
	}
	if req.MaxRounds > 0 {
		maxRounds = req.MaxRounds
	}

	untilClean := true
	if watchCfg.UntilClean != nil {
		untilClean = *watchCfg.UntilClean
	}
	if req.UntilClean {
		untilClean = true
	}
	if untilClean && maxRounds <= 0 {
		return reviewWatchLoopOptions{}, reviewWatchValidationProblem(
			"invalid_watch_request",
			"max_rounds must be greater than zero when until_clean is true",
			"max_rounds",
		)
	}

	autoPush := false
	if watchCfg.AutoPush != nil {
		autoPush = *watchCfg.AutoPush
	}
	if req.AutoPush {
		autoPush = true
	}

	pollInterval, err := resolveReviewWatchDuration(
		req.PollInterval,
		watchCfg.PollInterval,
		defaultReviewWatchPollInterval,
		"poll_interval",
	)
	if err != nil {
		return reviewWatchLoopOptions{}, err
	}
	reviewTimeout, err := resolveReviewWatchDuration(
		req.ReviewTimeout,
		watchCfg.ReviewTimeout,
		defaultReviewWatchTimeout,
		"review_timeout",
	)
	if err != nil {
		return reviewWatchLoopOptions{}, err
	}
	quietPeriod, err := resolveReviewWatchDuration(
		req.QuietPeriod,
		watchCfg.QuietPeriod,
		defaultReviewWatchQuietPeriod,
		"quiet_period",
	)
	if err != nil {
		return reviewWatchLoopOptions{}, err
	}

	return reviewWatchLoopOptions{
		UntilClean:    untilClean,
		MaxRounds:     maxRounds,
		AutoPush:      autoPush,
		PollInterval:  pollInterval,
		ReviewTimeout: reviewTimeout,
		QuietPeriod:   quietPeriod,
	}, nil
}

func resolveReviewWatchDuration(
	requested string,
	configured *string,
	defaultValue time.Duration,
	field string,
) (time.Duration, error) {
	value := firstNonEmptyString(requested, optionalString(configured))
	if value == "" {
		return defaultValue, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, reviewWatchValidationProblem(
			"invalid_watch_request",
			fmt.Sprintf("%s must be a positive duration", field),
			field,
		)
	}
	return duration, nil
}

func reviewWatchValidationProblem(code string, message string, field string) error {
	return apicore.NewProblem(
		http.StatusUnprocessableEntity,
		code,
		message,
		map[string]any{"field": strings.TrimSpace(field)},
		nil,
	)
}

func (m *RunManager) executeReviewWatchRun(active *activeRun, row globaldb.Run) {
	scope := active.scope
	var fallback terminalState

	if err := context.Cause(active.ctx); err != nil {
		fallback = cancelledTerminalState(err)
		m.finishRun(active, row, fallback)
		return
	}
	if err := startScopeRuntime(active.ctx, scope); err != nil {
		fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested())
		m.finishRun(active, row, fallback)
		return
	}

	row.Status = runStatusRunning
	updated, err := m.globalDB.UpdateRun(detachContext(active.ctx), row)
	if err != nil {
		fallback = failedTerminalState(scope.RunArtifacts(), err)
		m.finishRun(active, row, fallback)
		return
	}
	row = updated
	m.publishRunWorkspaceEvent(active.ctx, row, active.workflowSlug, apicore.WorkspaceEventKindRunStatusChanged)

	summary, err := m.runReviewWatchCoordinator(active)
	if hookErr := m.dispatchReviewWatchFinishedHook(active, summary, err); hookErr != nil {
		err = errors.Join(err, hookErr)
	}
	fallback = fallbackTerminalState(scope.RunArtifacts(), err, active.cancelWasRequested())
	if err == nil {
		fallback = completedTerminalState(scope.RunArtifacts(), summary)
	}
	m.finishRun(active, row, fallback)
}

func (m *RunManager) runReviewWatchCoordinator(active *activeRun) (string, error) {
	if active == nil || active.reviewWatch == nil {
		return "", errors.New("review watch run is not configured")
	}
	prepared := active.reviewWatch
	runtime, err := m.prepareReviewWatchCoordinator(active, prepared)
	if err != nil {
		return "", err
	}
	if runtime.cleanup != nil {
		defer runtime.cleanup()
	}

	done, summary, err := m.reconcileReviewWatchStartupPush(active, prepared, runtime)
	if done || err != nil {
		return summary, err
	}

	for rounds := 0; ; rounds++ {
		done, summary, err := m.runReviewWatchRound(active, prepared, runtime, rounds)
		if done || err != nil {
			return summary, err
		}
	}
}

func (m *RunManager) prepareReviewWatchCoordinator(
	active *activeRun,
	prepared *preparedReviewWatch,
) (*reviewWatchCoordinatorRuntime, error) {
	options := prepared.options
	registry, cleanup, err := m.reviewProviderRegistry(
		active.ctx,
		prepared.workspace.RootDir,
		reviewWatchInvokingCommand,
	)
	if err != nil {
		return nil, fmt.Errorf("build review provider registry: %w", err)
	}
	reviewProvider, err := registry.Get(options.Provider)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	gitState, err := m.reviewWatchGit.State(active.ctx, prepared.workspace.RootDir)
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	options = resolveReviewWatchPushTarget(options, gitState)
	if options.AutoPush && (options.PushRemote == "" || options.PushBranch == "") {
		if cleanup != nil {
			cleanup()
		}
		return nil, reviewWatchValidationProblem(
			"invalid_watch_request",
			"auto_push requires push remote and branch or a configured upstream",
			"push_remote",
		)
	}
	if err := m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchStarted, kinds.ReviewWatchPayload{
		Provider:        options.Provider,
		PR:              options.PR,
		Workflow:        prepared.workflowSlug,
		HeadSHA:         gitState.HeadSHA,
		Remote:          options.PushRemote,
		Branch:          options.PushBranch,
		Dirty:           gitState.Dirty,
		UnpushedCommits: gitState.UnpushedCommits,
	}); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	return &reviewWatchCoordinatorRuntime{
		registry:       registry,
		cleanup:        cleanup,
		reviewProvider: reviewProvider,
		options:        options,
		gitState:       gitState,
	}, nil
}

func (m *RunManager) reconcileReviewWatchStartupPush(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
) (bool, string, error) {
	if runtime == nil || !runtime.options.AutoPush || runtime.gitState.UnpushedCommits <= 0 {
		return false, "", nil
	}
	state := runtime.gitState
	prepared.lastHeadSHA = state.HeadSHA
	options, stopped, stopReason, err := m.pushReviewWatchRound(active, runtime.options, 0, state)
	runtime.options = options
	if err != nil {
		return false, "", err
	}
	if stopped {
		return true, reviewWatchStoppedSummary(stopReason), nil
	}
	refreshed, err := m.reviewWatchGit.State(active.ctx, prepared.workspace.RootDir)
	if err != nil {
		return false, "", fmt.Errorf("refresh git state after startup push: %w", err)
	}
	runtime.gitState = refreshed
	prepared.lastHeadSHA = refreshed.HeadSHA
	return false, "", nil
}

func (m *RunManager) runReviewWatchRound(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	rounds int,
) (bool, string, error) {
	done, summary, err := m.finishReviewWatchIfMaxRounds(active, prepared, runtime, rounds)
	if done || err != nil {
		return done, summary, err
	}
	status, err := m.waitForCurrentReview(
		active,
		runtime.reviewProvider,
		runtime.options,
		runtime.gitState.HeadSHA,
	)
	if err != nil {
		return false, "", err
	}
	done, summary, err = m.applyReviewWatchPreRoundHook(active, prepared, runtime, status, rounds+1)
	if done || err != nil {
		return done, summary, err
	}
	pending, err := m.fetchReviewWatchPending(active, prepared, runtime)
	if err != nil {
		return false, "", err
	}
	done, summary, err = m.finishReviewWatchIfClean(active, prepared, runtime, status, pending)
	if done || err != nil {
		return done, summary, err
	}

	result, err := m.persistReviewWatchPending(active, prepared, runtime, status, pending)
	if err != nil {
		return false, "", err
	}
	beforeFix, childRun, err := m.startReviewWatchChild(active, prepared, runtime, result)
	if err != nil {
		return false, "", err
	}
	if err := m.waitAndValidateReviewWatchChild(active, prepared, runtime, result.Round, childRun.RunID); err != nil {
		return false, "", m.dispatchReviewWatchFailedRoundHook(active, runtime, result.Round, childRun.RunID, err)
	}
	done, summary, err = m.verifyAndMaybePushReviewWatchRound(active, prepared, runtime, result, beforeFix)
	if err != nil {
		return false, "", m.dispatchReviewWatchFailedRoundHook(active, runtime, result.Round, childRun.RunID, err)
	}
	if done {
		return true, summary, nil
	}
	if !runtime.options.UntilClean {
		return true, reviewWatchTerminalRoundHandled, nil
	}
	return false, "", nil
}

func (m *RunManager) dispatchReviewWatchFailedRoundHook(
	active *activeRun,
	runtime *reviewWatchCoordinatorRuntime,
	round int,
	childRunID string,
	roundErr error,
) error {
	if roundErr == nil {
		return nil
	}
	if hookErr := m.dispatchReviewWatchPostRoundHook(active, runtime, reviewWatchPostRoundHookPayload{
		Round:      round,
		HeadSHA:    runtime.gitState.HeadSHA,
		ChildRunID: childRunID,
		Status:     runStatusFailed,
		Error:      roundErr.Error(),
	}); hookErr != nil {
		return errors.Join(roundErr, hookErr)
	}
	return roundErr
}

func (m *RunManager) finishReviewWatchIfMaxRounds(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	rounds int,
) (bool, string, error) {
	options := runtime.options
	if !options.UntilClean || rounds < options.MaxRounds {
		return false, "", nil
	}
	prepared.lastRound = rounds
	prepared.lastHeadSHA = runtime.gitState.HeadSHA
	err := m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchMaxRounds, kinds.ReviewWatchPayload{
		Provider: options.Provider,
		PR:       options.PR,
		Workflow: prepared.workflowSlug,
		Round:    rounds,
		HeadSHA:  runtime.gitState.HeadSHA,
		Status:   "max_rounds",
	})
	if err != nil {
		return true, "", err
	}
	return true, reviewWatchTerminalMaxRounds, nil
}

func (m *RunManager) fetchReviewWatchPending(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
) (*corepkg.FetchedReviewItems, error) {
	options := runtime.options
	pending, err := corepkg.FetchReviewItemsWithRegistryDirect(active.ctx, corepkg.Config{
		WorkspaceRoot: prepared.workspace.RootDir,
		Name:          prepared.workflowSlug,
		Provider:      options.Provider,
		PR:            options.PR,
		Nitpicks:      options.Nitpicks,
	}, runtime.registry)
	if err != nil {
		return nil, err
	}
	return pending, nil
}

func (m *RunManager) finishReviewWatchIfClean(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	status provider.WatchStatus,
	pending *corepkg.FetchedReviewItems,
) (bool, string, error) {
	if pending != nil && len(pending.Items) > 0 {
		return false, "", nil
	}
	options := runtime.options
	prepared.lastHeadSHA = status.PRHeadSHA
	err := m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchClean, kinds.ReviewWatchPayload{
		Provider:    options.Provider,
		PR:          options.PR,
		Workflow:    prepared.workflowSlug,
		HeadSHA:     status.PRHeadSHA,
		ReviewID:    status.ReviewID,
		ReviewState: status.ReviewState,
		Status:      string(status.State),
	})
	if err != nil {
		return true, "", err
	}
	return true, reviewWatchTerminalClean, nil
}

func (m *RunManager) persistReviewWatchPending(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	status provider.WatchStatus,
	pending *corepkg.FetchedReviewItems,
) (*corepkg.FetchResult, error) {
	result, err := corepkg.WriteFetchedReviewRoundDirect(pending)
	if err != nil {
		return nil, err
	}
	roundRow, err := m.syncFetchedReviewRound(active.ctx, prepared.workspace, prepared.workflowSlug, result)
	if err != nil {
		return nil, err
	}
	options := runtime.options
	prepared.lastRound = result.Round
	prepared.lastHeadSHA = status.PRHeadSHA
	err = m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchRoundFetched, kinds.ReviewWatchPayload{
		Provider:   options.Provider,
		PR:         options.PR,
		Workflow:   prepared.workflowSlug,
		Round:      result.Round,
		HeadSHA:    status.PRHeadSHA,
		Total:      roundRow.ResolvedCount + roundRow.UnresolvedCount,
		Resolved:   roundRow.ResolvedCount,
		Unresolved: roundRow.UnresolvedCount,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *RunManager) startReviewWatchChild(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	result *corepkg.FetchResult,
) (ReviewWatchGitState, apicore.Run, error) {
	beforeFix, err := m.reviewWatchGit.State(active.ctx, prepared.workspace.RootDir)
	if err != nil {
		return ReviewWatchGitState{}, apicore.Run{}, err
	}
	childRun, err := m.startReviewRun(
		active.ctx,
		prepared.workspace.RootDir,
		prepared.workflowSlug,
		result.Round,
		apicore.ReviewRunRequest{
			Workspace:        prepared.workspace.RootDir,
			PresentationMode: defaultPresentationMode,
			RuntimeOverrides: runtime.options.RuntimeOverrides,
			Batching:         runtime.options.Batching,
		},
		active.runID,
	)
	if err != nil {
		return ReviewWatchGitState{}, apicore.Run{}, err
	}
	prepared.lastChildRunID = childRun.RunID
	err = m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchFixStarted, kinds.ReviewWatchPayload{
		Provider:   runtime.options.Provider,
		PR:         runtime.options.PR,
		Workflow:   prepared.workflowSlug,
		Round:      result.Round,
		HeadSHA:    beforeFix.HeadSHA,
		ChildRunID: childRun.RunID,
	})
	if err != nil {
		return ReviewWatchGitState{}, apicore.Run{}, err
	}
	return beforeFix, childRun, nil
}

func (m *RunManager) waitAndValidateReviewWatchChild(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	round int,
	childRunID string,
) error {
	childRow, err := m.waitForReviewWatchChild(active.ctx, childRunID)
	if err != nil {
		emitErr := m.emitReviewWatchFixCompleted(
			active,
			prepared,
			runtime,
			round,
			childRunID,
			runStatusCancelled,
			err.Error(),
		)
		if emitErr != nil {
			return errors.Join(err, fmt.Errorf("emit review watch child completion: %w", emitErr))
		}
		return err
	}
	if err := m.emitReviewWatchFixCompleted(
		active,
		prepared,
		runtime,
		round,
		childRunID,
		childRow.Status,
		childRow.ErrorText,
	); err != nil {
		return err
	}
	if childRow.Status == runStatusCompleted {
		return nil
	}
	return fmt.Errorf(
		"review watch child run %s ended with status %s: %s",
		childRunID,
		childRow.Status,
		childRow.ErrorText,
	)
}

func (m *RunManager) emitReviewWatchFixCompleted(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	round int,
	childRunID string,
	status string,
	errorText string,
) error {
	return m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchFixCompleted, kinds.ReviewWatchPayload{
		Provider:   runtime.options.Provider,
		PR:         runtime.options.PR,
		Workflow:   prepared.workflowSlug,
		Round:      round,
		ChildRunID: childRunID,
		Status:     status,
		Error:      errorText,
	})
}

func (m *RunManager) verifyAndMaybePushReviewWatchRound(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	result *corepkg.FetchResult,
	beforeFix ReviewWatchGitState,
) (bool, string, error) {
	afterFix, err := m.verifyReviewWatchRound(
		active.ctx,
		prepared.workspace.RootDir,
		result,
		runtime.options,
		beforeFix,
	)
	if err != nil {
		return false, "", err
	}
	runtime.gitState = afterFix
	prepared.lastHeadSHA = afterFix.HeadSHA
	if !runtime.options.AutoPush {
		if err := m.dispatchReviewWatchPostRoundHook(active, runtime, reviewWatchPostRoundHookPayload{
			Round:      result.Round,
			HeadSHA:    afterFix.HeadSHA,
			ChildRunID: prepared.lastChildRunID,
			Status:     runStatusCompleted,
			Total:      result.Total,
			Resolved:   result.Total,
		}); err != nil {
			return false, "", err
		}
		return false, "", nil
	}
	options, stopped, stopReason, err := m.pushReviewWatchRound(active, runtime.options, result.Round, afterFix)
	runtime.options = options
	if err != nil {
		return false, "", err
	}
	if err := m.dispatchReviewWatchPostRoundHook(active, runtime, reviewWatchPostRoundHookPayload{
		Round:      result.Round,
		HeadSHA:    afterFix.HeadSHA,
		ChildRunID: prepared.lastChildRunID,
		Status:     reviewWatchPostRoundStatus(nil, stopped),
		Remote:     options.PushRemote,
		Branch:     options.PushBranch,
		Total:      result.Total,
		Resolved:   result.Total,
		Pushed:     reviewWatchPostRoundPushed(options, stopped, nil),
		StopReason: stopReason,
	}); err != nil {
		return false, "", err
	}
	if stopped {
		return true, reviewWatchStoppedSummary(stopReason), nil
	}
	return false, "", nil
}

func (m *RunManager) waitForCurrentReview(
	active *activeRun,
	reviewProvider provider.Provider,
	options reviewWatchOptions,
	expectedHeadSHA string,
) (provider.WatchStatus, error) {
	ctx, cancel := context.WithTimeout(active.ctx, options.ReviewTimeout)
	defer cancel()

	for {
		status, err := provider.FetchWatchStatus(ctx, reviewProvider, provider.WatchStatusRequest{PR: options.PR})
		if err != nil {
			return provider.WatchStatus{}, err
		}
		if status.State == provider.WatchStatusCurrentReviewed &&
			reviewWatchHeadMatches(status.PRHeadSHA, expectedHeadSHA) {
			if err := m.waitForReviewWatchSettled(ctx, options, status); err != nil {
				return provider.WatchStatus{}, reviewWatchContextError(err, "provider review wait timed out")
			}
			confirmed, err := provider.FetchWatchStatus(
				ctx,
				reviewProvider,
				provider.WatchStatusRequest{PR: options.PR},
			)
			if err != nil {
				return provider.WatchStatus{}, err
			}
			if confirmed.State == provider.WatchStatusCurrentReviewed &&
				reviewWatchHeadMatches(confirmed.PRHeadSHA, expectedHeadSHA) {
				return confirmed, nil
			}
			status = confirmed
		} else if status.State == provider.WatchStatusCurrentReviewed {
			status.State = provider.WatchStatusStale
		}
		if err := m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchWaiting, kinds.ReviewWatchPayload{
			Provider:    options.Provider,
			PR:          options.PR,
			Workflow:    active.workflowSlug,
			HeadSHA:     status.PRHeadSHA,
			ReviewID:    status.ReviewID,
			ReviewState: status.ReviewState,
			Status:      string(status.State),
		}); err != nil {
			return provider.WatchStatus{}, err
		}
		if err := waitReviewWatchDuration(ctx, options.PollInterval); err != nil {
			return provider.WatchStatus{}, reviewWatchContextError(err, "provider review wait timed out")
		}
	}
}

func reviewWatchHeadMatches(providerHeadSHA string, expectedHeadSHA string) bool {
	expected := strings.TrimSpace(expectedHeadSHA)
	if expected == "" {
		return true
	}
	return strings.TrimSpace(providerHeadSHA) == expected
}

func (m *RunManager) waitForReviewWatchSettled(
	ctx context.Context,
	options reviewWatchOptions,
	status provider.WatchStatus,
) error {
	if options.QuietPeriod <= 0 {
		return nil
	}
	signalAt := reviewWatchProviderSignalAt(status)
	if signalAt.IsZero() {
		return waitReviewWatchDuration(ctx, options.QuietPeriod)
	}
	waitUntil := signalAt.UTC().Add(options.QuietPeriod)
	remaining := waitUntil.Sub(m.now().UTC())
	if remaining <= 0 {
		return nil
	}
	return waitReviewWatchDuration(ctx, remaining)
}

func reviewWatchProviderSignalAt(status provider.WatchStatus) time.Time {
	signalAt := status.SubmittedAt
	if status.ProviderStatusUpdatedAt.After(signalAt) {
		signalAt = status.ProviderStatusUpdatedAt
	}
	return signalAt
}

func (m *RunManager) waitForReviewWatchChild(ctx context.Context, runID string) (globaldb.Run, error) {
	ticker := time.NewTicker(reviewWatchChildPollInterval)
	defer ticker.Stop()

	for {
		row, err := m.globalDB.GetRun(detachContext(ctx), strings.TrimSpace(runID))
		if err == nil && isTerminalRunStatus(row.Status) {
			return row, nil
		}
		select {
		case <-ctx.Done():
			if err := m.Cancel(detachContext(context.Background()), runID); err != nil {
				return globaldb.Run{}, errors.Join(ctx.Err(), fmt.Errorf("cancel child run %s: %w", runID, err))
			}
			return globaldb.Run{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *RunManager) verifyReviewWatchRound(
	ctx context.Context,
	workspaceRoot string,
	result *corepkg.FetchResult,
	options reviewWatchOptions,
	beforeFix ReviewWatchGitState,
) (ReviewWatchGitState, error) {
	if result == nil {
		return ReviewWatchGitState{}, errors.New("review watch fetch result is required")
	}
	meta, err := reviews.SnapshotRoundMeta(result.ReviewsDir)
	if err != nil {
		return ReviewWatchGitState{}, fmt.Errorf("verify review round %d: %w", result.Round, err)
	}
	if meta.Unresolved != 0 {
		return ReviewWatchGitState{}, fmt.Errorf(
			"review round %d still has %d unresolved issue(s)",
			result.Round,
			meta.Unresolved,
		)
	}
	afterFix, err := m.reviewWatchGit.State(ctx, workspaceRoot)
	if err != nil {
		return ReviewWatchGitState{}, err
	}
	if options.AutoPush && strings.TrimSpace(afterFix.HeadSHA) == strings.TrimSpace(beforeFix.HeadSHA) {
		return ReviewWatchGitState{}, fmt.Errorf("review round %d completed without advancing HEAD", result.Round)
	}
	return afterFix, nil
}

func (m *RunManager) pushReviewWatchRound(
	active *activeRun,
	options reviewWatchOptions,
	round int,
	state ReviewWatchGitState,
) (reviewWatchOptions, bool, string, error) {
	options, stopped, stopReason, err := m.applyReviewWatchPrePushHook(active, options, round, state)
	if err != nil || stopped {
		return options, stopped, stopReason, err
	}
	if err := m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchPushStarted, kinds.ReviewWatchPayload{
		Provider:        options.Provider,
		PR:              options.PR,
		Workflow:        active.workflowSlug,
		Round:           round,
		HeadSHA:         state.HeadSHA,
		Status:          reviewWatchPushEventStatus(round),
		Remote:          options.PushRemote,
		Branch:          options.PushBranch,
		UnpushedCommits: state.UnpushedCommits,
	}); err != nil {
		return options, false, "", err
	}
	if err := m.reviewWatchGit.Push(
		active.ctx,
		active.reviewWatch.workspace.RootDir,
		options.PushRemote,
		options.PushBranch,
	); err != nil {
		emitErr := m.emitReviewWatchEvent(active, eventspkg.EventKindReviewWatchPushFailed, kinds.ReviewWatchPayload{
			Provider:        options.Provider,
			PR:              options.PR,
			Workflow:        active.workflowSlug,
			Round:           round,
			HeadSHA:         state.HeadSHA,
			Status:          reviewWatchPushEventStatus(round),
			Remote:          options.PushRemote,
			Branch:          options.PushBranch,
			UnpushedCommits: state.UnpushedCommits,
			Error:           err.Error(),
		})
		if emitErr != nil {
			return options, false, "", errors.Join(err, fmt.Errorf("emit review watch push failure: %w", emitErr))
		}
		return options, false, "", err
	}
	return options, false, "", m.emitReviewWatchEvent(
		active,
		eventspkg.EventKindReviewWatchPushCompleted,
		kinds.ReviewWatchPayload{
			Provider:        options.Provider,
			PR:              options.PR,
			Workflow:        active.workflowSlug,
			Round:           round,
			HeadSHA:         state.HeadSHA,
			Status:          reviewWatchPushEventStatus(round),
			Remote:          options.PushRemote,
			Branch:          options.PushBranch,
			UnpushedCommits: state.UnpushedCommits,
		},
	)
}

func reviewWatchPushEventStatus(round int) string {
	if round == 0 {
		return reviewWatchPushStatusStartup
	}
	return ""
}

func (m *RunManager) syncFetchedReviewRound(
	ctx context.Context,
	workspaceRow globaldb.Workspace,
	workflowSlug string,
	result *corepkg.FetchResult,
) (globaldb.ReviewRound, error) {
	slug := strings.TrimSpace(workflowSlug)
	syncResult, err := corepkg.SyncWithDB(ctx, m.globalDB, workspaceRow, corepkg.SyncConfig{
		WorkspaceRoot: workspaceRow.RootDir,
		Name:          slug,
	})
	if err != nil {
		return globaldb.ReviewRound{}, reviewFetchPostWriteProblem(
			"review_fetch_sync_failed",
			"review issues were fetched, but catalog sync failed",
			slug,
			result.Round,
			result.ReviewsDir,
			err,
		)
	}
	syncedWorkflow, err := m.globalDB.GetActiveWorkflowBySlug(ctx, workspaceRow.ID, slug)
	if err != nil {
		return globaldb.ReviewRound{}, reviewFetchPostWriteProblem(
			"review_fetch_round_lookup_failed",
			"review issues were fetched, but the synced workflow could not be reloaded",
			slug,
			result.Round,
			result.ReviewsDir,
			err,
		)
	}
	workflowID := syncedWorkflow.ID
	m.publishWorkflowSyncWorkspaceEvent(ctx, workspaceRow.ID, &workflowID, slug, syncResult.SyncedPaths)
	roundRow, err := m.globalDB.GetReviewRound(ctx, syncedWorkflow.ID, result.Round)
	if err != nil {
		return globaldb.ReviewRound{}, reviewFetchPostWriteProblem(
			"review_fetch_round_lookup_failed",
			"review issues were fetched, but the synced review round could not be loaded",
			slug,
			result.Round,
			result.ReviewsDir,
			err,
		)
	}
	return roundRow, nil
}

func (m *RunManager) emitReviewWatchEvent(
	active *activeRun,
	kind eventspkg.EventKind,
	payload kinds.ReviewWatchPayload,
) error {
	if active == nil || active.scope == nil || active.scope.RunJournal() == nil {
		return nil
	}
	payload.RunID = active.runID
	if payload.Workflow == "" {
		payload.Workflow = active.workflowSlug
	}
	if payload.Provider == "" && active.reviewWatch != nil {
		payload.Provider = active.reviewWatch.options.Provider
	}
	if payload.PR == "" && active.reviewWatch != nil {
		payload.PR = active.reviewWatch.options.PR
	}
	if err := submitSyntheticEvent(active.ctx, active.scope.RunJournal(), active.runID, kind, payload); err != nil {
		return err
	}
	m.publishWorkspaceEvent(active.ctx, apicore.WorkspaceEvent{
		WorkspaceID:  active.workspaceID,
		WorkflowID:   cloneStringPtr(active.workflowID),
		WorkflowSlug: active.workflowSlug,
		RunID:        active.runID,
		Mode:         active.mode,
		Status:       runStatusRunning,
		Kind:         apicore.WorkspaceEventKindRunStatusChanged,
	})
	return nil
}

func (m *RunManager) reserveReviewWatch(key reviewWatchKey) error {
	key = normalizeReviewWatchKey(key)
	if key.WorkspaceID == "" || key.Provider == "" || key.PR == "" {
		return errors.New("review watch key is incomplete")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if runID := strings.TrimSpace(m.activeReviewWatches[key]); runID != "" || keyReserved(m.activeReviewWatches, key) {
		return apicore.NewProblem(
			http.StatusConflict,
			"review_watch_already_active",
			"review watch is already active for this workspace, provider, and PR",
			map[string]any{
				"workspace_id": key.WorkspaceID,
				"provider":     key.Provider,
				"pr":           key.PR,
				"run_id":       runID,
			},
			nil,
		)
	}
	m.activeReviewWatches[key] = ""
	return nil
}

func (m *RunManager) releaseReviewWatch(key reviewWatchKey) {
	key = normalizeReviewWatchKey(key)
	m.mu.Lock()
	delete(m.activeReviewWatches, key)
	m.mu.Unlock()
}

func keyReserved(values map[reviewWatchKey]string, key reviewWatchKey) bool {
	_, ok := values[key]
	return ok
}

func normalizeReviewWatchKey(key reviewWatchKey) reviewWatchKey {
	return reviewWatchKey{
		WorkspaceID: strings.TrimSpace(key.WorkspaceID),
		Provider:    strings.ToLower(strings.TrimSpace(key.Provider)),
		PR:          strings.TrimSpace(key.PR),
	}
}

func cloneReviewWatchKey(key *reviewWatchKey) *reviewWatchKey {
	if key == nil {
		return nil
	}
	cloned := normalizeReviewWatchKey(*key)
	return &cloned
}

func resolveReviewWatchPushTarget(options reviewWatchOptions, state ReviewWatchGitState) reviewWatchOptions {
	if strings.TrimSpace(options.PushRemote) == "" {
		options.PushRemote = strings.TrimSpace(state.UpstreamRemote)
	}
	if strings.TrimSpace(options.PushBranch) == "" {
		options.PushBranch = strings.TrimSpace(state.UpstreamBranch)
	}
	return options
}

func waitReviewWatchDuration(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func reviewWatchContextError(err error, timeoutMessage string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", timeoutMessage, err)
	}
	return err
}

func reviewWatchChildRuntimeOverrides(raw json.RawMessage, autoPush bool) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		if !autoPush {
			return nil, nil
		}
		return json.RawMessage(`{"auto_commit":true}`), nil
	}
	values := make(map[string]json.RawMessage)
	if err := json.Unmarshal(trimmed, &values); err != nil {
		return nil, apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"invalid_runtime_overrides",
			fmt.Sprintf("runtime_overrides: %v", err),
			nil,
			err,
		)
	}
	delete(values, "run_id")
	if autoPush {
		values["auto_commit"] = json.RawMessage(`true`)
	}
	if len(values) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode child runtime overrides: %w", err)
	}
	return encoded, nil
}

func cloneJSON(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
