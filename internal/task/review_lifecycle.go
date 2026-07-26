package task

import (
	"fmt"
	"strings"
	"time"
)

// Normalize returns a canonical review record.
func (r *RunReview) Normalize(now time.Time) (RunReview, error) {
	if r == nil {
		return RunReview{}, fmt.Errorf("%w: task_run_review is required", ErrValidation)
	}
	normalized := *r
	normalized.ReviewID = strings.TrimSpace(normalized.ReviewID)
	normalized.TaskID = strings.TrimSpace(normalized.TaskID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.ParentReviewID = strings.TrimSpace(normalized.ParentReviewID)
	normalized.Policy = normalized.Policy.Normalize()
	if normalized.Policy == "" {
		normalized.Policy = ReviewPolicyAlways
	}
	if normalized.ReviewRound == 0 {
		normalized.ReviewRound = defaultRunReviewRound
	}
	if normalized.Attempt == 0 {
		normalized.Attempt = defaultRunReviewAttempt
	}
	normalized.Status = normalized.Status.Normalize()
	if normalized.Status == "" {
		normalized.Status = RunReviewStatusRequested
	}
	normalized.Outcome = normalized.Outcome.Normalize()
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.DeliveryID = strings.TrimSpace(normalized.DeliveryID)
	normalized.MissingWork = normalizeReviewMissingWork(normalized.MissingWork)
	normalized.NextRoundGuidance = strings.TrimSpace(normalized.NextRoundGuidance)
	normalized.ReviewText = strings.TrimSpace(normalized.ReviewText)
	normalized.ReviewerSessionID = strings.TrimSpace(normalized.ReviewerSessionID)
	normalized.ReviewerAgentName = strings.TrimSpace(normalized.ReviewerAgentName)
	normalized.ReviewerPeerID = strings.TrimSpace(normalized.ReviewerPeerID)
	normalized.ReviewerChannelID = strings.TrimSpace(normalized.ReviewerChannelID)
	if normalized.ReviewedBy != nil {
		reviewedBy := *normalized.ReviewedBy
		reviewedBy.Kind = reviewedBy.Kind.Normalize()
		reviewedBy.Ref = strings.TrimSpace(reviewedBy.Ref)
		normalized.ReviewedBy = &reviewedBy
	}
	normalized = normalizeRunReviewTimes(normalized, now)

	if err := (&normalized).Validate(); err != nil {
		return RunReview{}, err
	}
	return normalized, nil
}

// Validate reports whether the run-review record can be persisted.
func (r *RunReview) Validate() error {
	if r == nil {
		return fmt.Errorf("%w: task_run_review is required", ErrValidation)
	}
	if err := validateRunReviewIdentity(r); err != nil {
		return err
	}
	if err := validateRunReviewLifecycle(r); err != nil {
		return err
	}
	if err := validateRunReviewContent(r); err != nil {
		return err
	}
	return validateRunReviewActorsAndTimes(r)
}

func validateRunReviewIdentity(review *RunReview) error {
	if strings.TrimSpace(review.ReviewID) == "" {
		return fmt.Errorf("%w: task_run_review.review_id is required", ErrValidation)
	}
	if strings.TrimSpace(review.TaskID) == "" {
		return fmt.Errorf("%w: task_run_review.task_id is required", ErrValidation)
	}
	if strings.TrimSpace(review.RunID) == "" {
		return fmt.Errorf("%w: task_run_review.run_id is required", ErrValidation)
	}
	return nil
}

func validateRunReviewLifecycle(review *RunReview) error {
	if err := review.Policy.Validate("task_run_review.policy"); err != nil {
		return err
	}
	if review.Policy.Normalize() == ReviewPolicyNone && review.Status.Normalize() != RunReviewStatusCanceled {
		return fmt.Errorf("%w: task_run_review.policy cannot be %q for active reviews", ErrValidation, ReviewPolicyNone)
	}
	if review.ReviewRound <= 0 {
		return fmt.Errorf("%w: task_run_review.review_round must be positive: %d", ErrValidation, review.ReviewRound)
	}
	if review.Attempt <= 0 {
		return fmt.Errorf("%w: task_run_review.attempt must be positive: %d", ErrValidation, review.Attempt)
	}
	if err := review.Status.Validate("task_run_review.status"); err != nil {
		return err
	}
	if review.Outcome.Normalize() != "" {
		if err := review.Outcome.Validate("task_run_review.outcome"); err != nil {
			return err
		}
	}
	if review.Status.Normalize() == RunReviewStatusRecorded && review.Outcome.Normalize() == "" {
		return fmt.Errorf("%w: task_run_review.outcome is required when status is recorded", ErrValidation)
	}
	if review.Confidence != nil && (*review.Confidence < 0 || *review.Confidence > 1) {
		return fmt.Errorf("%w: task_run_review.confidence must be between 0 and 1", ErrValidation)
	}
	return nil
}

func validateRunReviewContent(review *RunReview) error {
	if err := validateBoundedReviewText(review.Reason, maxRunReviewReasonBytes, "task_run_review.reason"); err != nil {
		return err
	}
	if err := validateRunReviewMissingWork(review.MissingWork); err != nil {
		return err
	}
	if err := validateBoundedReviewText(
		review.NextRoundGuidance,
		maxRunReviewGuidanceBytes,
		"task_run_review.next_round_guidance",
	); err != nil {
		return err
	}
	if err := validateBoundedReviewText(
		review.ReviewText,
		maxRunReviewTextBytes,
		"task_run_review.review_text",
	); err != nil {
		return err
	}
	return validateRunReviewSelectors(review)
}

func validateRunReviewActorsAndTimes(review *RunReview) error {
	if review.ReviewedBy != nil {
		if err := review.ReviewedBy.Validate("task_run_review.reviewed_by"); err != nil {
			return err
		}
	}
	if review.RequestedAt.IsZero() {
		return fmt.Errorf("%w: task_run_review.requested_at is required", ErrValidation)
	}
	if review.CreatedAt.IsZero() {
		return fmt.Errorf("%w: task_run_review.created_at is required", ErrValidation)
	}
	if review.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: task_run_review.updated_at is required", ErrValidation)
	}
	return nil
}
