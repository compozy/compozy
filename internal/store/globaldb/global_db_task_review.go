package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	globalDBTaskReviewReviewIDKey = "review_id"
	globalDBTaskReviewSourceKey   = "source"
)

var _ taskpkg.RunReviewStore = (*TaskRunRepo)(nil)

// RequestRunReview persists one review request or returns the existing idempotent request.
func (g *TaskRunRepo) RequestRunReview(
	ctx context.Context,
	review *taskpkg.RunReview,
) (stored taskpkg.RunReview, created bool, err error) {
	if err := g.checkReady(ctx, "request task run review"); err != nil {
		return taskpkg.RunReview{}, false, err
	}
	normalized, err := review.Normalize(g.now().UTC())
	if err != nil {
		return taskpkg.RunReview{}, false, err
	}

	if err := g.tasks.withTaskImmediateTransaction(ctx, "request task run review", func(exec taskSQLExecutor) error {
		run, err := g.tasks.getTaskRunWithExecutor(ctx, exec, normalized.RunID)
		if err != nil {
			return err
		}
		if strings.TrimSpace(run.TaskID) != normalized.TaskID {
			return fmt.Errorf(
				"%w: run %q belongs to task %q, not task %q",
				taskpkg.ErrValidation,
				run.ID,
				run.TaskID,
				normalized.TaskID,
			)
		}
		if !taskpkg.IsTerminalRunStatus(run.Status) {
			return fmt.Errorf(
				"%w: run %q is %q and cannot be reviewed until terminal",
				taskpkg.ErrInvalidStatusTransition,
				run.ID,
				run.Status.Normalize(),
			)
		}
		if !normalized.Policy.MatchesRunStatus(run.Status) {
			return fmt.Errorf(
				"%w: review policy %q does not apply to run status %q",
				taskpkg.ErrInvalidStatusTransition,
				normalized.Policy,
				run.Status.Normalize(),
			)
		}
		created, err = insertRunReviewRequest(ctx, exec, normalized)
		if err != nil {
			return err
		}
		stored, err = getRunReviewByRunRoundAttempt(
			ctx,
			exec,
			normalized.RunID,
			normalized.ReviewRound,
			normalized.Attempt,
		)
		if err != nil {
			return err
		}
		return linkTaskRunReviewRequest(ctx, exec, stored)
	}); err != nil {
		return taskpkg.RunReview{}, false, err
	}
	return stored, created, nil
}

// GetRunReview returns one persisted run review by id.
func (g *TaskRunRepo) GetRunReview(ctx context.Context, reviewID string) (taskpkg.RunReview, error) {
	if err := g.checkReady(ctx, "get task run review"); err != nil {
		return taskpkg.RunReview{}, err
	}
	trimmedID, err := requireTaskValue(reviewID, "task run review id")
	if err != nil {
		return taskpkg.RunReview{}, err
	}
	return getRunReviewByID(ctx, g.db, trimmedID)
}

// BindRunReviewSession binds an active review request to a reviewer session.
func (g *TaskRunRepo) BindRunReviewSession(
	ctx context.Context,
	req taskpkg.BindRunReviewSessionRequest,
	boundAt time.Time,
) (stored taskpkg.RunReview, err error) {
	if err := g.checkReady(ctx, "bind task run review session"); err != nil {
		return taskpkg.RunReview{}, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate("run_review_binding"); err != nil {
		return taskpkg.RunReview{}, err
	}
	if boundAt.IsZero() {
		boundAt = g.now().UTC()
	} else {
		boundAt = boundAt.UTC()
	}

	if err := g.tasks.withTaskImmediateTransaction(
		ctx,
		"bind task run review session",
		func(exec taskSQLExecutor) error {
			current, err := getRunReviewByID(ctx, exec, normalized.ReviewID)
			if err != nil {
				return err
			}
			if err := validateRunReviewBindingTransition(current, normalized.SessionID); err != nil {
				return err
			}
			if err := updateRunReviewBinding(ctx, exec, normalized, boundAt); err != nil {
				return err
			}
			stored, err = getRunReviewByID(ctx, exec, normalized.ReviewID)
			return err
		},
	); err != nil {
		return taskpkg.RunReview{}, err
	}
	return stored, nil
}

// LookupRunReviewBySession returns the active run review bound to one reviewer session.
func (g *TaskRunRepo) LookupRunReviewBySession(ctx context.Context, sessionID string) (taskpkg.RunReview, error) {
	if err := g.checkReady(ctx, "lookup task run review by session"); err != nil {
		return taskpkg.RunReview{}, err
	}
	trimmedID, err := requireTaskValue(sessionID, "task run review session id")
	if err != nil {
		return taskpkg.RunReview{}, err
	}

	row, err := g.queries.LookupRunReviewBySession(ctx, sqlcgen.LookupRunReviewBySessionParams{
		ReviewerSessionID: nullableTaskString(trimmedID),
		RoutedStatus:      string(taskpkg.RunReviewStatusRouted),
		InReviewStatus:    string(taskpkg.RunReviewStatusInReview),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
		}
		return taskpkg.RunReview{}, err
	}
	return runReviewFromGenerated(row)
}

// ListRunReviews returns persisted run reviews that match the supplied filters.
func (g *TaskRunRepo) ListRunReviews(
	ctx context.Context,
	query taskpkg.RunReviewQuery,
) ([]taskpkg.RunReview, error) {
	if err := g.checkReady(ctx, "list task run reviews"); err != nil {
		return nil, err
	}
	normalized := normalizeRunReviewQuery(query)
	if err := normalized.Validate("run_review_query"); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListRunReviews(ctx, sqlcgen.ListRunReviewsParams{
		TaskID: normalized.TaskID, RunID: normalized.RunID, Status: string(normalized.Status),
		ReviewerSessionID: normalized.ReviewerSessionID, ResultLimit: taskQueryLimit(normalized.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("store: query task run reviews: %w", err)
	}
	reviews := make([]taskpkg.RunReview, 0, len(rows))
	for _, row := range rows {
		review, mapErr := runReviewFromGenerated(row)
		if mapErr != nil {
			return nil, mapErr
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}

// RecordRunReview persists one authoritative review verdict and optional continuation run.
func (g *TaskRunRepo) RecordRunReview(
	ctx context.Context,
	req taskpkg.RecordRunReviewRequest,
	actor taskpkg.ActorContext,
	recordedAt time.Time,
	continuationRunID string,
) (result taskpkg.RunReviewResult, err error) {
	if err := g.checkReady(ctx, "record task run review"); err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	normalized := req.Normalize()
	if err := normalized.Validate("record_run_review"); err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	if err := actor.Validate(); err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	if recordedAt.IsZero() {
		recordedAt = g.now().UTC()
	} else {
		recordedAt = recordedAt.UTC()
	}
	if normalized.Verdict.Outcome == taskpkg.RunReviewOutcomeRejected {
		if _, err := requireTaskValue(continuationRunID, "continuation task run id"); err != nil {
			return taskpkg.RunReviewResult{}, err
		}
	}

	if err := g.tasks.withTaskImmediateTransaction(ctx, "record task run review", func(exec taskSQLExecutor) error {
		stored, txErr := g.recordRunReviewWithExecutor(ctx, exec, normalized, actor, recordedAt, continuationRunID)
		if txErr != nil {
			return txErr
		}
		result = stored
		return nil
	}); err != nil {
		return taskpkg.RunReviewResult{}, err
	}
	return result, nil
}

func insertRunReviewRequest(
	ctx context.Context,
	exec taskSQLExecutor,
	review taskpkg.RunReview,
) (bool, error) {
	affected, err := sqlcgen.New(exec).InsertRunReviewRequest(ctx, insertRunReviewRequestParams(review))
	if err != nil {
		return false, fmt.Errorf("store: create task run review %q: %w", review.ReviewID, err)
	}
	return affected > 0, nil
}

func getRunReviewByID(ctx context.Context, exec taskSQLExecutor, reviewID string) (taskpkg.RunReview, error) {
	row, err := sqlcgen.New(exec).GetRunReviewByID(ctx, reviewID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
		}
		return taskpkg.RunReview{}, err
	}
	return runReviewFromGenerated(row)
}

func getRunReviewByRunRoundAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
	round int,
	attempt int,
) (taskpkg.RunReview, error) {
	row, err := sqlcgen.New(exec).GetRunReviewByRunRoundAttempt(
		ctx,
		sqlcgen.GetRunReviewByRunRoundAttemptParams{
			RunID: runID, ReviewRound: int64(round), Attempt: int64(attempt),
		},
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
		}
		return taskpkg.RunReview{}, err
	}
	return runReviewFromGenerated(row)
}

func linkTaskRunReviewRequest(ctx context.Context, exec taskSQLExecutor, review taskpkg.RunReview) error {
	affected, err := sqlcgen.New(exec).LinkTaskRunReviewRequest(
		ctx,
		sqlcgen.LinkTaskRunReviewRequestParams{
			ReviewRequestRound: int64(review.ReviewRound), ReviewPolicySnapshot: string(review.Policy),
			ReviewRequestID: nullableTaskString(review.ReviewID), ID: review.RunID,
		},
	)
	if err != nil {
		return fmt.Errorf("store: link task run %q to review %q: %w", review.RunID, review.ReviewID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task run %q: %w", review.RunID, taskpkg.ErrTaskRunNotFound)
	}
	return nil
}
