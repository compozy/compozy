package globaldb

import (
	"database/sql"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func runReviewFromGenerated(row sqlcgen.TaskRunReview) (taskpkg.RunReview, error) {
	review := taskpkg.RunReview{
		ReviewID: row.ReviewID, TaskID: row.TaskID, RunID: row.RunID,
		ReviewRound: int(row.ReviewRound), Attempt: int(row.Attempt), Reason: row.Reason,
		NextRoundGuidance: row.NextRoundGuidance, ReviewText: row.ReviewText,
		ReviewerAgentName: row.ReviewerAgentName, ReviewerPeerID: row.ReviewerPeerID,
		ReviewerChannelID: row.ReviewerChannelID,
	}
	fields := runReviewScanFields{
		parentReviewID: row.ParentReviewID, policy: row.Policy, status: row.Status,
		outcome: row.Outcome, confidence: row.Confidence, deliveryID: row.DeliveryID,
		missingWork: row.MissingWorkJson, reviewerSessionID: row.ReviewerSessionID,
		reviewedByKind: nullableTaskString(row.ReviewedByKind),
		reviewedByRef:  nullableTaskString(row.ReviewedByRef),
		requestedAtRaw: row.RequestedAt, routedAtRaw: row.RoutedAt, startedAtRaw: row.StartedAt,
		reviewedAtRaw: row.ReviewedAt, deadlineAtRaw: row.DeadlineAt,
		createdAtRaw: row.CreatedAt, updatedAtRaw: row.UpdatedAt,
	}
	return fields.record(review)
}

func insertRunReviewRequestParams(review taskpkg.RunReview) sqlcgen.InsertRunReviewRequestParams {
	return sqlcgen.InsertRunReviewRequestParams{
		ReviewID: review.ReviewID, TaskID: review.TaskID, RunID: review.RunID,
		ParentReviewID: nullableTaskString(review.ParentReviewID), Policy: string(review.Policy),
		ReviewRound: int64(review.ReviewRound), Attempt: int64(review.Attempt), Status: string(review.Status),
		Outcome: nullableRunReviewOutcome(review.Outcome), Confidence: nullableRunReviewConfidence(review.Confidence),
		Reason: review.Reason, DeliveryID: nullableTaskString(review.DeliveryID),
		MissingWorkJson: string(review.MissingWork), NextRoundGuidance: review.NextRoundGuidance,
		ReviewText: review.ReviewText, ReviewerSessionID: nullableTaskString(review.ReviewerSessionID),
		ReviewerAgentName: review.ReviewerAgentName, ReviewerPeerID: review.ReviewerPeerID,
		ReviewerChannelID: review.ReviewerChannelID,
		ReviewedByKind:    runReviewActorKindValue(review.ReviewedBy),
		ReviewedByRef:     runReviewActorRefValue(review.ReviewedBy),
		RequestedAt:       store.FormatTimestamp(review.RequestedAt), RoutedAt: nullableTaskTime(review.RoutedAt),
		StartedAt: nullableTaskTime(review.StartedAt), ReviewedAt: nullableTaskTime(review.ReviewedAt),
		DeadlineAt: nullableTaskTime(review.DeadlineAt), CreatedAt: store.FormatTimestamp(review.CreatedAt),
		UpdatedAt: store.FormatTimestamp(review.UpdatedAt),
	}
}

func updateRunReviewVerdictParams(review taskpkg.RunReview) sqlcgen.UpdateRunReviewVerdictParams {
	return sqlcgen.UpdateRunReviewVerdictParams{
		Status: string(taskpkg.RunReviewStatusRecorded), Outcome: nullableRunReviewOutcome(review.Outcome),
		Confidence: nullableRunReviewConfidence(review.Confidence), Reason: review.Reason,
		DeliveryID: nullableTaskString(review.DeliveryID), MissingWorkJson: string(review.MissingWork),
		NextRoundGuidance: review.NextRoundGuidance, ReviewText: review.ReviewText,
		ReviewedByKind: runReviewActorKindValue(review.ReviewedBy),
		ReviewedByRef:  runReviewActorRefValue(review.ReviewedBy),
		ReviewedAt:     nullableTaskTime(review.ReviewedAt), UpdatedAt: store.FormatTimestamp(review.UpdatedAt),
		ReviewID: review.ReviewID,
	}
}

func nullableRunReviewOutcome(value taskpkg.RunReviewOutcome) sql.NullString {
	return nullableTaskString(string(value.Normalize()))
}

func nullableRunReviewConfidence(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}
