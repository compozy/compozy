package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

type runReviewRequestedEventPayload struct {
	ReviewID    string       `json:"review_id"`
	RunID       string       `json:"run_id"`
	Policy      ReviewPolicy `json:"policy"`
	ReviewRound int          `json:"review_round"`
	Attempt     int          `json:"attempt"`
	Created     bool         `json:"created"`
}

type runReviewBoundEventPayload struct {
	ReviewID          string `json:"review_id"`
	RunID             string `json:"run_id"`
	SessionID         string `json:"session_id"`
	ReviewerAgentName string `json:"reviewer_agent_name,omitempty"`
	ReviewerPeerID    string `json:"reviewer_peer_id,omitempty"`
	ReviewerChannelID string `json:"reviewer_channel_id,omitempty"`
}

type runReviewRecordedEventPayload struct {
	ReviewID     string           `json:"review_id"`
	RunID        string           `json:"run_id"`
	Outcome      RunReviewOutcome `json:"outcome"`
	Confidence   float64          `json:"confidence"`
	DeliveryID   string           `json:"delivery_id"`
	ReviewRound  int              `json:"review_round"`
	Continuation string           `json:"continuation_run_id,omitempty"`
}

type runReviewRetryEnqueuedEventPayload struct {
	ReviewID          string `json:"review_id"`
	RunID             string `json:"run_id"`
	ContinuationRunID string `json:"continuation_run_id"`
	ReviewRound       int    `json:"review_round"`
}

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

	review := runReviewFromRequest(m.newID("review"), normalized, m.now().UTC())
	stored, created, err := m.store.RequestRunReview(ctx, &review)
	if err != nil {
		return RunReview{}, false, err
	}
	if created {
		if err := m.recordTaskEvent(
			ctx,
			stored.TaskID,
			stored.RunID,
			taskEventRunReviewRequested,
			actor,
			runReviewRequestedEventPayload{
				ReviewID:    stored.ReviewID,
				RunID:       stored.RunID,
				Policy:      stored.Policy,
				ReviewRound: stored.ReviewRound,
				Attempt:     stored.Attempt,
				Created:     created,
			},
		); err != nil {
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

	postCommitCtx := context.Background()
	if ctx != nil {
		postCommitCtx = context.WithoutCancel(ctx)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Error(
				"task: run review observer panicked during post-commit notification",
				"panic", recovered,
				"review_id", notification.Review.ReviewID,
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

	result, err := m.store.RecordRunReview(
		ctx,
		normalized,
		actor,
		m.now().UTC(),
		m.newID("run"),
	)
	if err != nil {
		return RunReviewResult{}, err
	}
	if result.ContinuationRun != nil {
		if _, err := m.reconcileTaskCascade(ctx, result.Review.TaskID, actor); err != nil {
			return RunReviewResult{}, err
		}
	}
	if err := m.recordRunReviewVerdictEvents(ctx, result, actor); err != nil {
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

	stored, err := m.store.BindRunReviewSession(ctx, normalized, m.now().UTC())
	if err != nil {
		return RunReviewBinding{}, err
	}
	if err := m.recordTaskEvent(
		ctx,
		stored.TaskID,
		stored.RunID,
		taskEventRunReviewBound,
		actor,
		runReviewBoundEventPayload{
			ReviewID:          stored.ReviewID,
			RunID:             stored.RunID,
			SessionID:         stored.ReviewerSessionID,
			ReviewerAgentName: stored.ReviewerAgentName,
			ReviewerPeerID:    stored.ReviewerPeerID,
			ReviewerChannelID: stored.ReviewerChannelID,
		},
	); err != nil {
		return RunReviewBinding{}, err
	}
	return runReviewBindingFromReview(stored), nil
}

func (m *Service) recordRunReviewVerdictEvents(
	ctx context.Context,
	result RunReviewResult,
	actor ActorContext,
) error {
	confidence := 0.0
	if result.Review.Confidence != nil {
		confidence = *result.Review.Confidence
	}
	payload := runReviewRecordedEventPayload{
		ReviewID:    result.Review.ReviewID,
		RunID:       result.Review.RunID,
		Outcome:     result.Review.Outcome,
		Confidence:  confidence,
		DeliveryID:  result.Review.DeliveryID,
		ReviewRound: result.Review.ReviewRound,
	}
	if result.ContinuationRun != nil {
		payload.Continuation = result.ContinuationRun.ID
	}
	if err := m.recordTaskEvent(
		ctx,
		result.Review.TaskID,
		result.Review.RunID,
		taskEventRunReviewRecorded,
		actor,
		payload,
	); err != nil {
		return err
	}
	if err := m.recordRunReviewOutcomeEvent(ctx, result, actor, payload); err != nil {
		return err
	}
	if result.ContinuationRun == nil {
		return nil
	}
	return m.recordTaskEvent(
		ctx,
		result.ContinuationRun.TaskID,
		result.ContinuationRun.ID,
		taskEventRunReviewRetry,
		actor,
		runReviewRetryEnqueuedEventPayload{
			ReviewID:          result.Review.ReviewID,
			RunID:             result.Review.RunID,
			ContinuationRunID: result.ContinuationRun.ID,
			ReviewRound:       runReviewRound(result.ContinuationRun),
		},
	)
}

func runReviewRound(run *Run) int {
	if run == nil || run.Review == nil {
		return 0
	}
	return run.Review.ReviewRound
}

func (m *Service) recordRunReviewOutcomeEvent(
	ctx context.Context,
	result RunReviewResult,
	actor ActorContext,
	payload runReviewRecordedEventPayload,
) error {
	eventType := runReviewOutcomeEventType(result.Review.Outcome)
	if eventType == "" {
		return nil
	}
	return m.recordTaskEvent(ctx, result.Review.TaskID, result.Review.RunID, eventType, actor, payload)
}

func runReviewOutcomeEventType(outcome RunReviewOutcome) string {
	switch outcome.Normalize() {
	case RunReviewOutcomeApproved:
		return taskEventRunReviewApproved
	case RunReviewOutcomeRejected:
		return taskEventRunReviewRejected
	case RunReviewOutcomeBlocked:
		return taskEventRunReviewBlocked
	case RunReviewOutcomeError:
		return taskEventRunReviewError
	case RunReviewOutcomeTimeout:
		return taskEventRunReviewTimeout
	case RunReviewOutcomeInvalidOutput:
		return taskEventRunReviewInvalid
	default:
		return ""
	}
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
