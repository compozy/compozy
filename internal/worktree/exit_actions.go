package worktree

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

type ExitPhase string

const (
	ExitPhaseCommit       ExitPhase = "commit"
	ExitPhasePush         ExitPhase = "push"
	ExitPhasePR           ExitPhase = "pr"
	exitStepCompleted               = "completed"
	exitStepFailed                  = "failed"
	exitStepSkipped                 = "skipped"
	exitOperationRunning            = "running"
	exitOperationCanceled           = "canceled"
	exitOpenBrowserLabel            = "Open in browser"
	exitViewPRLabel                 = "View PR"
	exitRemoveAction                = "remove"
)

type ExitActionRequest struct {
	Action  ExitAction `json:"action"`
	Message string     `json:"message,omitempty"`
	Title   string     `json:"title,omitempty"`
	Body    string     `json:"body,omitempty"`
	Draft   bool       `json:"draft,omitempty"`
	Base    string     `json:"base,omitempty"`
}

type ExitStepResult struct {
	Phase    ExitPhase `json:"phase"`
	State    string    `json:"state"`
	Reason   string    `json:"reason,omitempty"`
	Output   string    `json:"output,omitempty"`
	SHA      string    `json:"sha,omitempty"`
	Upstream string    `json:"upstream,omitempty"`
	PRStatus string    `json:"pr_status,omitempty"`
	PRNumber int       `json:"pr_number,omitempty"`
	URL      string    `json:"url,omitempty"`
}

type ExitCTA struct {
	Action ExitAction `json:"action"`
	Label  string     `json:"label"`
	URL    string     `json:"url,omitempty"`
}

type ExitActionResult struct {
	OperationID string           `json:"op_id"`
	Action      ExitAction       `json:"action"`
	Steps       []ExitStepResult `json:"steps"`
	CTA         *ExitCTA         `json:"cta,omitempty"`
}

type exitExecutionError struct {
	phase  ExitPhase
	result ExitStepResult
	cause  error
}

func (e *exitExecutionError) Error() string { return e.cause.Error() }
func (e *exitExecutionError) Unwrap() error { return e.cause }

func (s *Service) RunExitAction(
	ctx context.Context,
	workspaceID string,
	id string,
	request ExitActionRequest,
) (string, error) {
	if !request.Action.executable() {
		return "", ErrExitActionInvalid
	}
	plan, err := s.ExitPlan(ctx, workspaceID, id)
	if err != nil {
		return "", err
	}
	if err := validateRequestedExitAction(plan, request.Action); err != nil {
		return "", err
	}
	item, err := s.store.Get(ctx, workspaceID, plan.WorktreeID)
	if err != nil || item == nil {
		if err == nil {
			err = ErrNotFound
		}
		return "", err
	}
	opID, err := s.newID("op")
	if err != nil {
		return "", fmt.Errorf("worktree: generate exit operation id: %w", err)
	}
	operation := ExitOperation{
		ID: opID, WorkspaceID: workspaceID, WorktreeID: item.ID,
		Action: string(request.Action), State: exitOperationRunning, StartedAt: s.now().UTC(),
	}
	if err := s.store.InsertExitOperation(ctx, operation); err != nil {
		return "", err
	}
	executionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	control := &exitOperationControl{
		workspaceID: workspaceID, worktreeID: item.ID, cancel: cancel, done: make(chan struct{}),
	}
	s.exitMu.Lock()
	s.exits[opID] = control
	s.exitMu.Unlock()
	phases := request.Action.phases()
	s.emitExit(executionCtx, EventExitActionStarted, operation, ExitEventPayload{
		OperationID: opID, Action: request.Action, Phases: phases, State: exitOperationRunning,
	})
	go s.executeExitOperation(executionCtx, control, operation, *item, plan, request)
	return opID, nil
}

func (s *Service) CancelExitAction(ctx context.Context, workspaceID, id, opID string) error {
	item, err := s.store.Get(ctx, workspaceID, id)
	if errors.Is(err, ErrNotFound) || item == nil {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("worktree: read exit cancellation target: %w", err)
	}
	id = item.ID
	opID = strings.TrimSpace(opID)
	s.exitMu.Lock()
	control := s.exits[opID]
	s.exitMu.Unlock()
	if control != nil && control.workspaceID == workspaceID && control.worktreeID == id {
		control.cancel()
		select {
		case <-control.done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	operation := ExitOperation{ID: opID, WorkspaceID: workspaceID, WorktreeID: id, State: exitOperationCanceled}
	_, err = s.finishExitOperation(ctx, operation, exitOperationCanceled, EventExitActionCanceled, ExitEventPayload{
		OperationID: opID, State: exitOperationCanceled, Message: "Exit action canceled.",
	})
	return err
}

func (s *Service) executeExitOperation(
	ctx context.Context,
	control *exitOperationControl,
	operation ExitOperation,
	item Worktree,
	plan *ExitPlan,
	request ExitActionRequest,
) {
	defer func() {
		control.cancel()
		s.exitMu.Lock()
		delete(s.exits, operation.ID)
		s.exitMu.Unlock()
		close(control.done)
	}()
	result := ExitActionResult{OperationID: operation.ID, Action: request.Action}
	workspace, err := s.resolveWorkspace(ctx, item.WorkspaceID)
	if err == nil {
		var commonDir string
		commonDir, err = s.commonDir(ctx, workspace.Root)
		if err == nil {
			var release func()
			release, err = s.locks.Acquire(ctx, commonDir)
			if err == nil {
				defer release()
				err = s.revalidateExitTarget(ctx, item)
				if err == nil {
					result.Steps, err = s.runExitSequence(ctx, operation, item, plan, request)
				}
			}
		}
	}
	if err != nil {
		s.finishExitFailure(ctx, operation, result, err)
		return
	}
	result.CTA = exitResultCTA(request.Action, plan, result.Steps)
	s.refreshAfterExit(context.WithoutCancel(ctx), item)

	if ctx.Err() != nil {
		s.finishExitFailure(ctx, operation, result, ctx.Err())
		return
	}
	finished, finishErr := s.finishExitOperation(
		context.WithoutCancel(ctx),
		operation,
		"completed",
		EventExitActionCompleted,
		ExitEventPayload{
			OperationID: operation.ID, Action: request.Action, State: exitStepCompleted, Result: &result,
		},
	)
	if finishErr != nil {
		s.logger.ErrorContext(ctx, "worktree exit terminalization failed", "op_id", operation.ID, "error", finishErr)
		return
	}
	if !finished {
		s.logger.WarnContext(ctx, "worktree exit operation was already terminal", "op_id", operation.ID)
		return
	}
}

func (s *Service) revalidateExitTarget(ctx context.Context, item Worktree) error {
	current, err := s.store.Get(ctx, item.WorkspaceID, item.ID)
	if err != nil || current == nil {
		if err == nil {
			err = ErrNotFound
		}
		return err
	}
	if current.State != StateReady {
		return ErrNotReady
	}
	return s.requireNoActiveSession(ctx, *current)
}

func (s *Service) runExitSequence(
	ctx context.Context,
	operation ExitOperation,
	item Worktree,
	plan *ExitPlan,
	request ExitActionRequest,
) ([]ExitStepResult, error) {
	steps := make([]ExitStepResult, 0, len(request.Action.phases()))
	for _, phase := range request.Action.phases() {
		s.emitExit(ctx, EventExitActionStep, operation, ExitEventPayload{
			OperationID: operation.ID, Action: request.Action, Phase: phase, State: exitOperationRunning,
		})
		var step ExitStepResult
		var err error
		switch phase {
		case ExitPhaseCommit:
			step, err = s.runExitCommit(ctx, operation, request.Action, item, plan.CommitScope, request.Message)
		case ExitPhasePush:
			forcePush := request.Action == ExitActionCommitPush
			step, err = s.runExitPush(ctx, item, forcePush)
		case ExitPhasePR:
			step, err = s.runExitPR(ctx, item, plan, request)
		}
		steps = append(steps, step)
		state := step.State
		if err != nil {
			state = exitStepFailed
		}
		s.emitExit(ctx, EventExitActionStep, operation, ExitEventPayload{
			OperationID: operation.ID, Action: request.Action, Phase: phase, State: state,
			Result: &ExitActionResult{OperationID: operation.ID, Action: request.Action, Steps: []ExitStepResult{step}},
		})
		if err != nil {
			return steps, &exitExecutionError{phase: phase, result: step, cause: err}
		}
	}
	return steps, nil
}

func (s *Service) runExitPush(
	ctx context.Context,
	item Worktree,
	force bool,
) (ExitStepResult, error) {
	step := ExitStepResult{Phase: ExitPhasePush, State: exitStepCompleted}
	status, err := s.readCurrentGitStatus(ctx, item.Path)
	if err != nil {
		step.State, step.Output = exitStepFailed, err.Error()
		return step, err
	}
	if !force && status.HasUpstream && valueOrZero(status.Ahead) == 0 {
		step.State, step.Reason = exitStepSkipped, "already up to date"
		return step, nil
	}
	args := []string{string(ExitPhasePush)}
	if !status.HasUpstream {
		args = append(args, "-u", "origin", item.Branch)
	}
	stdout, stderr, err := s.runner.Run(ctx, item.Path, args...)
	if err != nil {
		step.State, step.Output = exitStepFailed, exitCommandOutput(stdout, stderr, err)
		return step, err
	}
	step.Upstream = status.Upstream
	if step.Upstream == "" {
		step.Upstream = "origin/" + item.Branch
	}
	sha, shaStderr, shaErr := s.runner.Run(ctx, item.Path, "rev-parse", "HEAD")
	if shaErr != nil {
		step.State = exitStepFailed
		step.Output = exitCommandOutput(sha, shaStderr, shaErr)
		return step, fmt.Errorf("worktree: read pushed HEAD: %w", shaErr)
	}
	step.SHA = strings.TrimSpace(string(sha))
	return step, nil
}

func (s *Service) runExitPR(
	ctx context.Context,
	item Worktree,
	plan *ExitPlan,
	request ExitActionRequest,
) (ExitStepResult, error) {
	step := ExitStepResult{Phase: ExitPhasePR, State: exitStepCompleted}
	if s.forge == nil || plan.Forge == nil {
		if plan.BrowserURL == "" {
			return step, ErrForgeUnavailable
		}
		step.PRStatus, step.URL = "browser", plan.BrowserURL
		return step, nil
	}
	base := strings.TrimSpace(request.Base)
	if base == "" {
		base = plan.Base
	}
	body := request.Body
	if strings.TrimSpace(body) == "" {
		if base == plan.Base && plan.PRPrefill != nil {
			body = plan.PRPrefill.Body
		} else {
			body = s.detectPRTemplate(ctx, item.Path, base, plan.Forge.TemplatePaths)
		}
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		if base == plan.Base && plan.PRPrefill != nil && strings.TrimSpace(plan.PRPrefill.Title) != "" {
			title = strings.TrimSpace(plan.PRPrefill.Title)
		} else {
			title = item.Branch
		}
	}
	created, err := s.forge.CreatePR(ctx, ForgePRRequest{
		WorkspaceID: item.WorkspaceID, WorktreeID: item.ID, RemoteURLs: plan.RemoteURLs,
		Head: item.Branch, Base: base, Title: title, Body: body, Draft: request.Draft,
	})
	if err != nil {
		step.State, step.Output = exitStepFailed, diagnostics.RedactAndBound(err.Error(), 2048)
		return step, fmt.Errorf("%w: %w", ErrForge, err)
	}
	if created == nil {
		return step, fmt.Errorf("%w: provider returned no pull request result", ErrForge)
	}
	createdURL, err := sanitizeForgeWebURL(created.URL)
	if err != nil {
		return step, fmt.Errorf("%w: provider returned an invalid pull request URL", ErrForge)
	}
	step.PRStatus, step.PRNumber, step.URL = created.Status, created.Number, createdURL
	state, merged := "open", false
	fetchedAt := s.now().UTC()
	forgeStatus := ForgeStatus{
		WorktreeID: item.ID, Provider: plan.Forge.Provider, PRNumber: &created.Number,
		PRState: &state, PRURL: createdURL, Merged: &merged, FetchedAt: &fetchedAt,
	}
	if err := s.store.SaveForgeStatus(ctx, item.WorkspaceID, item.ID, forgeStatus); err != nil {
		return step, fmt.Errorf("worktree: persist created pull request: %w", err)
	}
	return step, nil
}

func (s *Service) readCurrentGitStatus(ctx context.Context, path string) (GitStatus, error) {
	stdout, stderr, err := s.runner.Run(ctx, path, "status", "--porcelain=v2", "--branch", "-z")
	if err != nil {
		return GitStatus{}, refusal(ErrStatusUnreadable, exitCommandOutput(nil, stderr, err))
	}
	status, err := ParseStatusV2(stdout)
	if err != nil {
		return GitStatus{}, refusal(ErrStatusUnreadable, err.Error())
	}
	return status, nil
}

func (s *Service) finishExitFailure(
	ctx context.Context,
	operation ExitOperation,
	result ExitActionResult,
	cause error,
) {
	state, event := exitStepFailed, EventExitActionFailed
	if errors.Is(ctx.Err(), context.Canceled) {
		state, event = "canceled", EventExitActionCanceled
	}
	message := diagnostics.RedactAndBound(cause.Error(), 2048)
	phase := ExitPhase("")
	var executionErr *exitExecutionError
	if errors.As(cause, &executionErr) {
		phase = executionErr.phase
	}
	_, err := s.finishExitOperation(context.WithoutCancel(ctx), operation, state, event, ExitEventPayload{
		OperationID: operation.ID, Action: ExitAction(operation.Action), Phase: phase,
		State: state, Result: &result, Message: message, Cause: ForgeFailureCause(cause),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "worktree exit terminalization failed", "op_id", operation.ID, "error", err)
	}
}

func validateRequestedExitAction(plan *ExitPlan, action ExitAction) error {
	for _, row := range plan.Actions {
		if row.Action != action {
			continue
		}
		if row.Enabled || (action == ExitActionCommit && row.BlockedReason == reasonNoChanges) ||
			(action == ExitActionPush && row.BlockedReason == reasonNoPendingPush) {
			return nil
		}
		return refusal(ErrExitActionInvalid, row.BlockedReason)
	}
	if action == ExitActionOpenPR {
		return ErrForgeUnavailable
	}
	return ErrExitActionInvalid
}

func (a ExitAction) executable() bool {
	return a == ExitActionCommit || a == ExitActionCommitPush || a == ExitActionPush || a == ExitActionOpenPR
}

func (a ExitAction) phases() []ExitPhase {
	switch a {
	case ExitActionCommit:
		return []ExitPhase{ExitPhaseCommit}
	case ExitActionCommitPush:
		return []ExitPhase{ExitPhaseCommit, ExitPhasePush}
	case ExitActionPush:
		return []ExitPhase{ExitPhasePush}
	case ExitActionOpenPR:
		return []ExitPhase{ExitPhasePR}
	default:
		return nil
	}
}

func exitResultCTA(action ExitAction, plan *ExitPlan, steps []ExitStepResult) *ExitCTA {
	if action == ExitActionOpenPR && len(steps) > 0 {
		last := steps[len(steps)-1]
		if last.PRStatus == "browser" {
			return &ExitCTA{Action: ExitActionOpenPR, Label: exitOpenBrowserLabel, URL: last.URL}
		}
		return &ExitCTA{Action: ExitActionViewPR, Label: exitViewPRLabel, URL: last.URL}
	}
	if action == ExitActionCommit && len(plan.RemoteURLs) > 0 {
		return &ExitCTA{Action: ExitActionPush, Label: "Push"}
	}
	if plan.ForgeStatus != nil && plan.ForgeStatus.PRURL != "" {
		return &ExitCTA{Action: ExitActionViewPR, Label: exitViewPRLabel, URL: plan.ForgeStatus.PRURL}
	}
	if plan.Forge != nil {
		return &ExitCTA{Action: ExitActionOpenPR, Label: plan.Forge.OpenActionLabel}
	}
	if plan.BrowserURL != "" {
		return &ExitCTA{Action: ExitActionOpenPR, Label: exitOpenBrowserLabel, URL: plan.BrowserURL}
	}
	return &ExitCTA{Action: exitRemoveAction, Label: "Remove worktree"}
}

func exitCommandOutput(stdout, stderr []byte, cause error) string {
	parts := make([]string, 0, 3)
	if value := strings.TrimSpace(string(stdout)); value != "" {
		parts = append(parts, value)
	}
	if value := strings.TrimSpace(string(stderr)); value != "" {
		parts = append(parts, value)
	}
	if len(parts) == 0 && cause != nil {
		parts = append(parts, cause.Error())
	}
	return diagnostics.RedactAndBound(strings.Join(parts, "\n"), 4096)
}
