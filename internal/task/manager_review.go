package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// RequestRunReview persists or returns the idempotent review request for a terminal task run.
func (m *Service) RequestRunReview(
	ctx context.Context,
	req RunReviewRequest,
	actor ActorContext,
) (RunReview, bool, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return RunReview{}, false, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate("run_review_request"); err != nil {
		return RunReview{}, false, err
	}

	taskRecord, err := m.loadAuthorizedTask(ctx, m.store, normalized.TaskID, actor)
	if err != nil {
		return RunReview{}, false, err
	}
	run, err := m.store.GetTaskRun(ctx, normalized.RunID)
	if err != nil {
		return RunReview{}, false, err
	}
	if strings.TrimSpace(run.TaskID) != strings.TrimSpace(taskRecord.ID) {
		return RunReview{}, false, fmt.Errorf(
			"%w: run %q belongs to task %q, not task %q",
			ErrValidation,
			run.ID,
			run.TaskID,
			taskRecord.ID,
		)
	}
	if err := m.authorizeRunResource(ctx, actor, run, taskRecord); err != nil {
		return RunReview{}, false, err
	}
	if !IsTerminalRunStatus(run.Status) {
		return RunReview{}, false, fmt.Errorf(
			"%w: run %q is %q and cannot be reviewed until terminal",
			ErrInvalidStatusTransition,
			run.ID,
			run.Status.Normalize(),
		)
	}
	if !normalized.Policy.MatchesRunStatus(run.Status) {
		return RunReview{}, false, fmt.Errorf(
			"%w: review policy %q does not apply to terminal run status %q",
			ErrInvalidStatusTransition,
			normalized.Policy,
			run.Status.Normalize(),
		)
	}

	reviewID, err := m.newID("review")
	if err != nil {
		return RunReview{}, false, fmt.Errorf("task: generate run review id: %w", err)
	}
	review := runReviewFromRequest(reviewID, normalized, m.now().UTC())
	requestedEvent := runReviewRequestedEvent(review)
	preparedEvents, err := m.prepareReviewTaskEvents(actor, []reviewTaskEvent{requestedEvent})
	if err != nil {
		return RunReview{}, false, err
	}
	stored, created, err := m.store.RequestRunReview(ctx, &review)
	if err != nil {
		return RunReview{}, false, err
	}
	if created {
		if err := m.recordReviewTaskEvents(ctx, preparedEvents); err != nil {
			return RunReview{}, false, err
		}
		m.notifyRunReviewRequestedBestEffort(ctx, &RunReviewRequestedNotification{
			Review: cloneRunReview(&stored),
			Task:   taskRecord,
			Run:    run,
			Actor:  actor,
		})
	}
	return stored, created, nil
}

func (m *Service) notifyRunReviewRequestedBestEffort(
	ctx context.Context,
	notification *RunReviewRequestedNotification,
) {
	if m == nil || m.reviewObserver == nil || notification == nil {
		return
	}

	postCommitCtx, cancel := taskPostCommitContext(ctx)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error(
				"task: run review observer panicked during post-commit notification",
				"panic", recovered,
				reviewEvidenceIDKey, notification.Review.ReviewID,
				"task_id", notification.Review.TaskID,
				"run_id", notification.Review.RunID,
			)
		}
	}()

	m.reviewObserver.OnRunReviewRequested(postCommitCtx, notification)
}

// GetRunReview returns one persisted task-run review.
func (m *Service) GetRunReview(ctx context.Context, reviewID string, actor ActorContext) (RunReview, error) {
	if err := requireReadAuthority(actor); err != nil {
		return RunReview{}, err
	}
	trimmedID := strings.TrimSpace(reviewID)
	if trimmedID == "" {
		return RunReview{}, fmt.Errorf("%w: review_id is required", ErrValidation)
	}
	if err := ValidateReferenceSize(trimmedID, reviewEvidenceIDKey); err != nil {
		return RunReview{}, err
	}
	review, err := m.store.GetRunReview(ctx, trimmedID)
	if err != nil {
		return RunReview{}, err
	}
	if err := m.authorizeRunReviewResource(ctx, actor, review); err != nil {
		return RunReview{}, err
	}
	return review, nil
}

// RecordRunReview persists an authoritative reviewer verdict and optional continuation run.
func (m *Service) RecordRunReview(
	ctx context.Context,
	req RecordRunReviewRequest,
	actor ActorContext,
) (RunReviewResult, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return RunReviewResult{}, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate("record_run_review"); err != nil {
		return RunReviewResult{}, err
	}
	review, err := m.store.GetRunReview(ctx, normalized.ReviewID)
	if err != nil {
		return RunReviewResult{}, err
	}
	if err := m.authorizeRunReviewResource(ctx, actor, review); err != nil {
		return RunReviewResult{}, err
	}
	alreadyRecorded := review.Status.Normalize() == RunReviewStatusRecorded
	// The store decides replay inside its transaction and discards this id when it replays.
	continuationRunID := ""
	if normalized.Verdict.Outcome.Normalize() == RunReviewOutcomeRejected {
		continuationRunID, err = m.newID("run")
		if err != nil {
			return RunReviewResult{}, fmt.Errorf("task: generate review continuation run id: %w", err)
		}
	}
	anticipated := anticipatedRunReviewResult(review, normalized.Verdict, continuationRunID)
	anticipatedEvents := runReviewVerdictEvents(anticipated)
	var preparedEvents []Event
	if !alreadyRecorded {
		preparedEvents, err = m.prepareReviewTaskEvents(actor, anticipatedEvents)
		if err != nil {
			return RunReviewResult{}, err
		}
	}

	result, err := m.store.RecordRunReview(
		ctx,
		normalized,
		actor,
		m.now().UTC(),
		continuationRunID,
	)
	if err != nil {
		return RunReviewResult{}, err
	}
	if alreadyRecorded || !sameRunReviewEventProjection(result, anticipated) {
		return result, nil
	}
	if result.ContinuationRun != nil {
		if _, err := m.reconcileTaskCascade(ctx, result.Review.TaskID, actor); err != nil {
			return RunReviewResult{}, err
		}
	}
	if err := m.recordReviewTaskEvents(ctx, preparedEvents); err != nil {
		return RunReviewResult{}, err
	}
	return result, nil
}

// BindRunReviewSession binds one persisted review request to a reviewer session.
func (m *Service) BindRunReviewSession(
	ctx context.Context,
	req BindRunReviewSessionRequest,
	actor ActorContext,
) (RunReviewBinding, error) {
	if err := requireWriteAuthority(actor); err != nil {
		return RunReviewBinding{}, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate("run_review_binding"); err != nil {
		return RunReviewBinding{}, err
	}
	review, err := m.store.GetRunReview(ctx, normalized.ReviewID)
	if err != nil {
		return RunReviewBinding{}, err
	}
	if err := m.authorizeRunReviewResource(ctx, actor, review); err != nil {
		return RunReviewBinding{}, err
	}
	event := runReviewBoundEvent(review, normalized)
	preparedEvents, err := m.prepareReviewTaskEvents(actor, []reviewTaskEvent{event})
	if err != nil {
		return RunReviewBinding{}, err
	}

	stored, err := m.store.BindRunReviewSession(ctx, normalized, m.now().UTC())
	if err != nil {
		return RunReviewBinding{}, err
	}
	if err := m.recordReviewTaskEvents(ctx, preparedEvents); err != nil {
		return RunReviewBinding{}, err
	}
	return runReviewBindingFromReview(stored), nil
}

// LookupRunReviewForSession returns the active review bound to one reviewer session.
func (m *Service) LookupRunReviewForSession(
	ctx context.Context,
	sessionID string,
	actor ActorContext,
) (RunReviewBinding, error) {
	if err := requireReadAuthority(actor); err != nil {
		return RunReviewBinding{}, err
	}
	trimmedSessionID := strings.TrimSpace(sessionID)
	if trimmedSessionID == "" {
		return RunReviewBinding{}, fmt.Errorf("%w: reviewer session id is required", ErrValidation)
	}
	if err := ValidateReferenceSize(trimmedSessionID, "reviewer_session_id"); err != nil {
		return RunReviewBinding{}, err
	}
	review, err := m.store.LookupRunReviewBySession(ctx, trimmedSessionID)
	if err != nil {
		return RunReviewBinding{}, err
	}
	if err := m.authorizeRunReviewResource(ctx, actor, review); err != nil {
		return RunReviewBinding{}, err
	}
	return runReviewBindingFromReview(review), nil
}

// ListRunReviews returns persisted review requests that match the supplied filters.
func (m *Service) ListRunReviews(
	ctx context.Context,
	query RunReviewQuery,
	actor ActorContext,
) ([]RunReview, error) {
	if err := requireReadAuthority(actor); err != nil {
		return nil, err
	}
	normalized := query
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.Status = normalized.Status.Normalize()
	normalized.ReviewerSessionID = strings.TrimSpace(normalized.ReviewerSessionID)
	if err := normalized.Validate("run_review_query"); err != nil {
		return nil, err
	}
	if normalized.TaskID != "" {
		if _, err := m.loadAuthorizedTask(ctx, m.store, normalized.TaskID, actor); err != nil {
			return nil, err
		}
	}
	if normalized.RunID != "" {
		run, taskRecord, err := m.loadRunWithTask(ctx, normalized.RunID)
		if err != nil {
			return nil, err
		}
		if err := m.authorizeRunResource(ctx, actor, run, taskRecord); err != nil {
			return nil, err
		}
	}
	reviews, err := m.store.ListRunReviews(ctx, normalized)
	if err != nil || isTaskOperator(actor) {
		return reviews, err
	}
	visible := make([]RunReview, 0, len(reviews))
	for idx := range reviews {
		if err := m.authorizeRunReviewResource(ctx, actor, reviews[idx]); err == nil {
			visible = append(visible, reviews[idx])
		} else if !errors.Is(err, ErrPermissionDenied) && !errors.Is(err, ErrTaskNotFound) {
			return nil, err
		}
	}
	return visible, nil
}
