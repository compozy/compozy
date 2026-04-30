package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/provider"
)

const (
	reviewWatchHookPreRound  = "review.watch_pre_round"
	reviewWatchHookPostRound = "review.watch_post_round"
	reviewWatchHookPrePush   = "review.watch_pre_push"
	reviewWatchHookFinished  = "review.watch_finished"
	reviewWatchStopPrefix    = "review watch stopped"
)

type reviewWatchPreRoundHookPayload struct {
	RunID            string          `json:"run_id"`
	Provider         string          `json:"provider"`
	PR               string          `json:"pr"`
	Workflow         string          `json:"workflow"`
	Round            int             `json:"round"`
	HeadSHA          string          `json:"head_sha"`
	ReviewID         string          `json:"review_id,omitempty"`
	ReviewState      string          `json:"review_state,omitempty"`
	Status           string          `json:"status,omitempty"`
	Nitpicks         bool            `json:"nitpicks"`
	RuntimeOverrides json.RawMessage `json:"runtime_overrides,omitempty"`
	Batching         json.RawMessage `json:"batching,omitempty"`
	Continue         bool            `json:"continue"`
	StopReason       string          `json:"stop_reason,omitempty"`
}

type reviewWatchPostRoundHookPayload struct {
	RunID      string `json:"run_id"`
	Provider   string `json:"provider"`
	PR         string `json:"pr"`
	Workflow   string `json:"workflow"`
	Round      int    `json:"round"`
	HeadSHA    string `json:"head_sha,omitempty"`
	ChildRunID string `json:"child_run_id,omitempty"`
	Status     string `json:"status,omitempty"`
	Remote     string `json:"remote,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Total      int    `json:"total,omitempty"`
	Resolved   int    `json:"resolved,omitempty"`
	Unresolved int    `json:"unresolved,omitempty"`
	Pushed     bool   `json:"pushed,omitempty"`
	StopReason string `json:"stop_reason,omitempty"`
	Error      string `json:"error,omitempty"`
}

type reviewWatchPrePushHookPayload struct {
	RunID      string `json:"run_id"`
	Provider   string `json:"provider"`
	PR         string `json:"pr"`
	Workflow   string `json:"workflow"`
	Round      int    `json:"round"`
	HeadSHA    string `json:"head_sha"`
	Remote     string `json:"remote"`
	Branch     string `json:"branch"`
	Push       bool   `json:"push"`
	StopReason string `json:"stop_reason,omitempty"`
}

type reviewWatchFinishedHookPayload struct {
	RunID          string `json:"run_id"`
	ChildRunID     string `json:"child_run_id,omitempty"`
	Provider       string `json:"provider"`
	PR             string `json:"pr"`
	Workflow       string `json:"workflow"`
	Round          int    `json:"round,omitempty"`
	HeadSHA        string `json:"head_sha,omitempty"`
	Status         string `json:"status"`
	TerminalReason string `json:"terminal_reason,omitempty"`
	Stopped        bool   `json:"stopped,omitempty"`
	Clean          bool   `json:"clean,omitempty"`
	MaxRounds      bool   `json:"max_rounds,omitempty"`
	Error          string `json:"error,omitempty"`
}

func (m *RunManager) applyReviewWatchPreRoundHook(
	active *activeRun,
	prepared *preparedReviewWatch,
	runtime *reviewWatchCoordinatorRuntime,
	status provider.WatchStatus,
	round int,
) (bool, string, error) {
	options := runtime.options
	payload := reviewWatchPreRoundHookPayload{
		RunID:            active.runID,
		Provider:         options.Provider,
		PR:               options.PR,
		Workflow:         prepared.workflowSlug,
		Round:            round,
		HeadSHA:          status.PRHeadSHA,
		ReviewID:         status.ReviewID,
		ReviewState:      status.ReviewState,
		Status:           string(status.State),
		Nitpicks:         options.Nitpicks,
		RuntimeOverrides: cloneJSON(options.RuntimeOverrides),
		Batching:         cloneJSON(options.Batching),
		Continue:         true,
	}
	updated, err := model.DispatchMutableHook(
		active.ctx,
		active.scope.RunManager(),
		reviewWatchHookPreRound,
		payload,
	)
	if err != nil {
		return false, "", err
	}
	if err := validateReviewWatchPreRoundHookPayload(payload, updated); err != nil {
		return false, "", err
	}

	childOverrides, err := reviewWatchChildRuntimeOverrides(updated.RuntimeOverrides, options.AutoPush)
	if err != nil {
		return false, "", err
	}
	if _, err := parseReviewBatching(updated.Batching); err != nil {
		return false, "", err
	}

	options.Nitpicks = updated.Nitpicks
	options.RuntimeOverrides = cloneJSON(childOverrides)
	options.Batching = cloneJSON(updated.Batching)
	runtime.options = options

	if updated.Continue {
		return false, "", nil
	}
	reason, err := requireReviewWatchStopReason(updated.StopReason, reviewWatchHookPreRound)
	if err != nil {
		return false, "", err
	}
	prepared.lastRound = round
	prepared.lastHeadSHA = status.PRHeadSHA
	return true, reviewWatchStoppedSummary(reason), nil
}

func validateReviewWatchPreRoundHookPayload(
	before reviewWatchPreRoundHookPayload,
	after reviewWatchPreRoundHookPayload,
) error {
	if before.RunID != after.RunID ||
		before.Provider != after.Provider ||
		before.PR != after.PR ||
		before.Workflow != after.Workflow ||
		before.Round != after.Round ||
		before.HeadSHA != after.HeadSHA ||
		before.ReviewID != after.ReviewID ||
		before.ReviewState != after.ReviewState ||
		before.Status != after.Status {
		return errors.New(
			"review.watch_pre_round patch may only change nitpicks, runtime_overrides, batching, continue, and stop_reason",
		)
	}
	if !after.Continue {
		if _, err := requireReviewWatchStopReason(after.StopReason, reviewWatchHookPreRound); err != nil {
			return err
		}
	}
	return nil
}

func (m *RunManager) applyReviewWatchPrePushHook(
	active *activeRun,
	options reviewWatchOptions,
	round int,
	state ReviewWatchGitState,
) (reviewWatchOptions, bool, string, error) {
	payload := reviewWatchPrePushHookPayload{
		RunID:    active.runID,
		Provider: options.Provider,
		PR:       options.PR,
		Workflow: active.workflowSlug,
		Round:    round,
		HeadSHA:  state.HeadSHA,
		Remote:   options.PushRemote,
		Branch:   options.PushBranch,
		Push:     true,
	}
	updated, err := model.DispatchMutableHook(
		active.ctx,
		active.scope.RunManager(),
		reviewWatchHookPrePush,
		payload,
	)
	if err != nil {
		return options, false, "", err
	}
	if err := validateReviewWatchPrePushHookPayload(payload, updated); err != nil {
		return options, false, "", err
	}
	options.PushRemote = strings.TrimSpace(updated.Remote)
	options.PushBranch = strings.TrimSpace(updated.Branch)
	if updated.Push {
		return options, false, "", nil
	}
	reason, err := requireReviewWatchStopReason(updated.StopReason, reviewWatchHookPrePush)
	if err != nil {
		return options, false, "", err
	}
	return options, true, reason, nil
}

func validateReviewWatchPrePushHookPayload(
	before reviewWatchPrePushHookPayload,
	after reviewWatchPrePushHookPayload,
) error {
	if before.RunID != after.RunID ||
		before.Provider != after.Provider ||
		before.PR != after.PR ||
		before.Workflow != after.Workflow ||
		before.Round != after.Round ||
		before.HeadSHA != after.HeadSHA {
		return errors.New(
			"review.watch_pre_push patch may only change remote, branch, push, and stop_reason",
		)
	}
	if after.Push && (strings.TrimSpace(after.Remote) == "" || strings.TrimSpace(after.Branch) == "") {
		return errors.New("review.watch_pre_push requires remote and branch when push is true")
	}
	if !after.Push {
		if _, err := requireReviewWatchStopReason(after.StopReason, reviewWatchHookPrePush); err != nil {
			return err
		}
	}
	return nil
}

func (m *RunManager) dispatchReviewWatchPostRoundHook(
	active *activeRun,
	runtime *reviewWatchCoordinatorRuntime,
	payload reviewWatchPostRoundHookPayload,
) error {
	if active == nil || active.scope == nil || runtime == nil {
		return nil
	}
	if payload.RunID == "" {
		payload.RunID = active.runID
	}
	if payload.Workflow == "" {
		payload.Workflow = active.workflowSlug
	}
	if payload.Provider == "" {
		payload.Provider = runtime.options.Provider
	}
	if payload.PR == "" {
		payload.PR = runtime.options.PR
	}
	model.DispatchObserverHook(active.ctx, active.scope.RunManager(), reviewWatchHookPostRound, payload)
	return waitReviewWatchObserverHooks(active.ctx, active.scope.RunManager())
}

func (m *RunManager) dispatchReviewWatchFinishedHook(
	active *activeRun,
	summary string,
	runErr error,
) error {
	if active == nil || active.scope == nil || active.reviewWatch == nil {
		return nil
	}
	payload := reviewWatchFinishedHookPayload{
		RunID:          active.runID,
		ChildRunID:     active.reviewWatch.lastChildRunID,
		Provider:       active.reviewWatch.options.Provider,
		PR:             active.reviewWatch.options.PR,
		Workflow:       active.workflowSlug,
		Round:          active.reviewWatch.lastRound,
		HeadSHA:        active.reviewWatch.lastHeadSHA,
		Status:         reviewWatchFinishedStatus(active, runErr),
		TerminalReason: strings.TrimSpace(summary),
		Stopped:        strings.HasPrefix(strings.TrimSpace(summary), reviewWatchStopPrefix),
		Clean:          strings.TrimSpace(summary) == reviewWatchTerminalClean,
		MaxRounds:      strings.TrimSpace(summary) == reviewWatchTerminalMaxRounds,
	}
	if runErr != nil {
		payload.Error = runErr.Error()
	}
	model.DispatchObserverHook(active.ctx, active.scope.RunManager(), reviewWatchHookFinished, payload)
	return waitReviewWatchObserverHooks(active.ctx, active.scope.RunManager())
}

func reviewWatchFinishedStatus(active *activeRun, runErr error) string {
	if runErr == nil {
		return runStatusCompleted
	}
	if active != nil && active.cancelWasRequested() {
		return runStatusCancelled
	}
	if errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) {
		return runStatusCancelled
	}
	return runStatusFailed
}

func requireReviewWatchStopReason(reason string, hook string) (string, error) {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "", fmt.Errorf("%s stop requires stop_reason", hook)
	}
	return trimmed, nil
}

func reviewWatchStoppedSummary(reason string) string {
	return reviewWatchStopPrefix + ": " + strings.TrimSpace(reason)
}

func waitReviewWatchObserverHooks(ctx context.Context, manager model.RuntimeManager) error {
	if manager == nil {
		return nil
	}
	waitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultRunCloseTimeout)
	defer cancel()
	if err := model.WaitForObserverHooks(waitCtx, manager); err != nil {
		return fmt.Errorf("wait for review watch observer hooks: %w", err)
	}
	return nil
}

func reviewWatchPostRoundStatus(err error, stopped bool) string {
	switch {
	case stopped:
		return "stopped"
	case err != nil:
		return runStatusFailed
	default:
		return runStatusCompleted
	}
}

func reviewWatchPostRoundPushed(options reviewWatchOptions, stopped bool, err error) bool {
	return options.AutoPush && !stopped && err == nil
}
