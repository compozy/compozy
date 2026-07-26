package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

func validateRunReviewBindingTransition(review taskpkg.RunReview, sessionID string) error {
	currentSessionID := strings.TrimSpace(review.ReviewerSessionID)
	nextSessionID := strings.TrimSpace(sessionID)
	if currentSessionID != "" &&
		currentSessionID != nextSessionID &&
		review.Status.Normalize() != taskpkg.RunReviewStatusInReview {
		return fmt.Errorf(
			"%w: task run review %q is already bound to session %q",
			taskpkg.ErrInvalidStatusTransition,
			review.ReviewID,
			review.ReviewerSessionID,
		)
	}
	switch review.Status.Normalize() {
	case taskpkg.RunReviewStatusRequested,
		taskpkg.RunReviewStatusRouted,
		taskpkg.RunReviewStatusInReview:
		return nil
	default:
		return fmt.Errorf(
			"%w: task run review %q with status %q cannot bind a reviewer session",
			taskpkg.ErrInvalidStatusTransition,
			review.ReviewID,
			review.Status.Normalize(),
		)
	}
}

func updateRunReviewBinding(
	ctx context.Context,
	exec taskSQLExecutor,
	req taskpkg.BindRunReviewSessionRequest,
	boundAt time.Time,
) error {
	affected, err := sqlcgen.New(exec).UpdateRunReviewBinding(ctx, sqlcgen.UpdateRunReviewBindingParams{
		Status:            string(taskpkg.RunReviewStatusInReview),
		ReviewerSessionID: nullableTaskString(req.SessionID),
		ReviewerAgentName: req.ReviewerAgentName,
		ReviewerPeerID:    req.ReviewerPeerID,
		ReviewerChannelID: req.ReviewerChannelID,
		StartedAt:         nullableTaskTime(boundAt), UpdatedAt: store.FormatTimestamp(boundAt),
		ReviewID: req.ReviewID,
	})
	if err != nil {
		return mapRunReviewBindError(req.ReviewID, req.SessionID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: task run review %q: %w", req.ReviewID, taskpkg.ErrRunReviewNotFound)
	}
	return nil
}

func mapRunReviewBindError(reviewID string, sessionID string, err error) error {
	if err == nil {
		return nil
	}
	if isTaskRunReviewSQLiteUniqueConstraint(err) {
		return fmt.Errorf(
			"%w: reviewer session %q already has an active task run review",
			taskpkg.ErrInvalidStatusTransition,
			sessionID,
		)
	}
	return fmt.Errorf("store: bind task run review %q to session %q: %w", reviewID, sessionID, err)
}

func isTaskRunReviewSQLiteUniqueConstraint(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}

type runReviewScanFields struct {
	parentReviewID    sql.NullString
	policy            string
	status            string
	outcome           sql.NullString
	confidence        sql.NullFloat64
	deliveryID        sql.NullString
	missingWork       string
	reviewerSessionID sql.NullString
	reviewedByKind    sql.NullString
	reviewedByRef     sql.NullString
	requestedAtRaw    string
	routedAtRaw       sql.NullString
	startedAtRaw      sql.NullString
	reviewedAtRaw     sql.NullString
	deadlineAtRaw     sql.NullString
	createdAtRaw      string
	updatedAtRaw      string
}

func (fields runReviewScanFields) record(review taskpkg.RunReview) (taskpkg.RunReview, error) {
	review.ParentReviewID = taskNullStringValue(fields.parentReviewID)
	review.Policy = taskpkg.ReviewPolicy(strings.TrimSpace(fields.policy))
	review.Status = taskpkg.RunReviewStatus(strings.TrimSpace(fields.status))
	review.Outcome = taskpkg.RunReviewOutcome(taskNullStringValue(fields.outcome))
	if fields.confidence.Valid {
		confidence := fields.confidence.Float64
		review.Confidence = &confidence
	}
	review.DeliveryID = taskNullStringValue(fields.deliveryID)
	review.MissingWork = []byte(strings.TrimSpace(fields.missingWork))
	review.ReviewerSessionID = taskNullStringValue(fields.reviewerSessionID)
	if strings.TrimSpace(fields.reviewedByKind.String) != "" || strings.TrimSpace(fields.reviewedByRef.String) != "" {
		review.ReviewedBy = &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKind(strings.TrimSpace(fields.reviewedByKind.String)),
			Ref:  strings.TrimSpace(fields.reviewedByRef.String),
		}
	}
	if err := assignRunReviewTimestamps(&review, fields); err != nil {
		return taskpkg.RunReview{}, err
	}
	normalized, err := (&review).Normalize(review.UpdatedAt)
	if err != nil {
		return taskpkg.RunReview{}, err
	}
	return normalized, nil
}

func assignRunReviewTimestamps(review *taskpkg.RunReview, fields runReviewScanFields) error {
	requestedAt, err := store.ParseTimestamp(fields.requestedAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse task run review requested_at: %w", err)
	}
	createdAt, err := store.ParseTimestamp(fields.createdAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse task run review created_at: %w", err)
	}
	updatedAt, err := store.ParseTimestamp(fields.updatedAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse task run review updated_at: %w", err)
	}
	review.RequestedAt = requestedAt
	review.CreatedAt = createdAt
	review.UpdatedAt = updatedAt
	if err := assignNullableTaskTimestamp(&review.RoutedAt, fields.routedAtRaw); err != nil {
		return fmt.Errorf("store: parse task run review routed_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&review.StartedAt, fields.startedAtRaw); err != nil {
		return fmt.Errorf("store: parse task run review started_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&review.ReviewedAt, fields.reviewedAtRaw); err != nil {
		return fmt.Errorf("store: parse task run review reviewed_at: %w", err)
	}
	if err := assignNullableTaskTimestamp(&review.DeadlineAt, fields.deadlineAtRaw); err != nil {
		return fmt.Errorf("store: parse task run review deadline_at: %w", err)
	}
	return nil
}

func runReviewActorKindValue(actor *taskpkg.ActorIdentity) string {
	if actor == nil {
		return ""
	}
	return string(actor.Kind)
}

func runReviewActorRefValue(actor *taskpkg.ActorIdentity) string {
	if actor == nil {
		return ""
	}
	return actor.Ref
}

func normalizeRunReviewQuery(query taskpkg.RunReviewQuery) taskpkg.RunReviewQuery {
	normalized := query
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.Status = normalized.Status.Normalize()
	normalized.ReviewerSessionID = strings.TrimSpace(normalized.ReviewerSessionID)
	return normalized
}

func cloneRunReviewForStore(review taskpkg.RunReview) taskpkg.RunReview {
	cloned := review
	cloned.MissingWork = cloneTaskRawJSON(review.MissingWork)
	if review.ReviewedBy != nil {
		reviewedBy := *review.ReviewedBy
		cloned.ReviewedBy = &reviewedBy
	}
	if review.Confidence != nil {
		confidence := *review.Confidence
		cloned.Confidence = &confidence
	}
	return cloned
}

func cloneTaskRawJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}
