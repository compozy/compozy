package task

import "fmt"

func validateBoundedReviewText(value string, maxBytes int, path string) error {
	if len(value) > maxBytes {
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrValidation, path, maxBytes)
	}
	return nil
}

func validateRunReviewSelectors(review *RunReview) error {
	return validateRunReviewSelectorFields(
		map[string]string{
			"reviewer_session_id": review.ReviewerSessionID,
			"reviewer_agent_name": review.ReviewerAgentName,
			"reviewer_peer_id":    review.ReviewerPeerID,
			"reviewer_channel_id": review.ReviewerChannelID,
			"parent_review_id":    review.ParentReviewID,
			"delivery_id":         review.DeliveryID,
		},
		"task_run_review",
	)
}

func validateRunReviewSelectorFields(values map[string]string, path string) error {
	for field, value := range values {
		if len(value) > maxRunReviewSelectorFieldBytes {
			return fmt.Errorf(
				"%w: %s exceeds %d bytes",
				ErrValidation,
				nestedPath(path, field),
				maxRunReviewSelectorFieldBytes,
			)
		}
	}
	return nil
}

func IsTerminalRunStatus(status RunStatus) bool {
	switch status.Normalize() {
	case TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCanceled:
		return true
	default:
		return false
	}
}
