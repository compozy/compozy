package task

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func runReviewFromRequest(reviewID string, req RunReviewRequest, now time.Time) RunReview {
	return RunReview{
		ReviewID:       strings.TrimSpace(reviewID),
		TaskID:         req.TaskID,
		RunID:          req.RunID,
		ParentReviewID: req.ParentReviewID,
		Policy:         req.Policy,
		ReviewRound:    req.ReviewRound,
		Attempt:        req.Attempt,
		Status:         RunReviewStatusRequested,
		Reason:         req.Reason,
		MissingWork:    defaultMissingWorkJSON,
		RequestedAt:    now,
		DeadlineAt:     req.DeadlineAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func runReviewBindingFromReview(review RunReview) RunReviewBinding {
	return RunReviewBinding{
		Review:            cloneRunReview(&review),
		SessionID:         review.ReviewerSessionID,
		ReviewerAgentName: review.ReviewerAgentName,
		ReviewerPeerID:    review.ReviewerPeerID,
		ReviewerChannelID: review.ReviewerChannelID,
	}
}

func cloneRunReview(review *RunReview) RunReview {
	if review == nil {
		return RunReview{}
	}
	cloned := *review
	cloned.MissingWork = cloneRawJSON(review.MissingWork)
	cloned.ReviewedBy = cloneActorIdentity(review.ReviewedBy)
	if review.Confidence != nil {
		confidence := *review.Confidence
		cloned.Confidence = &confidence
	}
	return cloned
}

func normalizeRunReviewTimes(review RunReview, now time.Time) RunReview {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if review.RequestedAt.IsZero() {
		review.RequestedAt = now
	} else {
		review.RequestedAt = review.RequestedAt.UTC()
	}
	if !review.RoutedAt.IsZero() {
		review.RoutedAt = review.RoutedAt.UTC()
	}
	if !review.StartedAt.IsZero() {
		review.StartedAt = review.StartedAt.UTC()
	}
	if !review.ReviewedAt.IsZero() {
		review.ReviewedAt = review.ReviewedAt.UTC()
	}
	if !review.DeadlineAt.IsZero() {
		review.DeadlineAt = review.DeadlineAt.UTC()
	}
	if review.CreatedAt.IsZero() {
		review.CreatedAt = now
	} else {
		review.CreatedAt = review.CreatedAt.UTC()
	}
	if review.UpdatedAt.IsZero() {
		review.UpdatedAt = now
	} else {
		review.UpdatedAt = review.UpdatedAt.UTC()
	}
	return review
}

func normalizeReviewMissingWork(raw json.RawMessage) json.RawMessage {
	normalized := normalizeRawJSON(raw)
	if len(normalized) == 0 {
		return cloneRawJSON(defaultMissingWorkJSON)
	}
	return normalized
}

func validateRunReviewMissingWork(raw json.RawMessage) error {
	if err := ValidatePayloadSize(raw, "task_run_review.missing_work"); err != nil {
		return err
	}
	var decoded []json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("%w: task_run_review.missing_work must be a JSON array", ErrValidation)
	}
	return nil
}

func decodeVerdictMissingWork(raw json.RawMessage, path string) ([]string, error) {
	if err := ValidatePayloadSize(raw, path); err != nil {
		return nil, err
	}
	var decoded []string
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("%w: %s must be a JSON array of strings", ErrValidation, path)
	}
	items := make([]string, 0, len(decoded))
	for idx, item := range decoded {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return nil, fmt.Errorf("%w: %s[%d] must not be empty", ErrValidation, path, idx)
		}
		items = append(items, trimmed)
	}
	return items, nil
}
