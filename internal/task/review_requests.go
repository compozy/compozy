package task

import (
	"fmt"
	"strings"
)

// Normalize returns a canonical review request.
func (r RunReviewRequest) Normalize() RunReviewRequest {
	normalized := r
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
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	if !normalized.DeadlineAt.IsZero() {
		normalized.DeadlineAt = normalized.DeadlineAt.UTC()
	}
	return normalized
}

// Validate reports whether the review request has enough data for a persisted request row.
func (r RunReviewRequest) Validate(path string) error {
	if strings.TrimSpace(r.TaskID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "task_id"))
	}
	if strings.TrimSpace(r.RunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "run_id"))
	}
	if err := r.Policy.Validate(nestedPath(path, "policy")); err != nil {
		return err
	}
	if r.Policy.Normalize() == ReviewPolicyNone {
		return fmt.Errorf("%w: %s cannot be %q", ErrValidation, nestedPath(path, "policy"), ReviewPolicyNone)
	}
	if r.ReviewRound <= 0 {
		return fmt.Errorf(
			"%w: %s must be positive: %d",
			ErrValidation,
			nestedPath(path, "review_round"),
			r.ReviewRound,
		)
	}
	if r.Attempt <= 0 {
		return fmt.Errorf("%w: %s must be positive: %d", ErrValidation, nestedPath(path, "attempt"), r.Attempt)
	}
	return validateBoundedReviewText(r.Reason, maxRunReviewReasonBytes, nestedPath(path, "reason"))
}

// Normalize returns a canonical reviewer-session binding request.
func (r BindRunReviewSessionRequest) Normalize() BindRunReviewSessionRequest {
	normalized := r
	normalized.ReviewID = strings.TrimSpace(normalized.ReviewID)
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.ReviewerAgentName = strings.TrimSpace(normalized.ReviewerAgentName)
	normalized.ReviewerPeerID = strings.TrimSpace(normalized.ReviewerPeerID)
	normalized.ReviewerChannelID = strings.TrimSpace(normalized.ReviewerChannelID)
	return normalized
}

// Validate reports whether the binding request can bind a reviewer session.
func (r BindRunReviewSessionRequest) Validate(path string) error {
	if strings.TrimSpace(r.ReviewID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "review_id"))
	}
	if strings.TrimSpace(r.SessionID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "session_id"))
	}
	return validateRunReviewSelectorFields(
		map[string]string{
			"reviewer_agent_name": r.ReviewerAgentName,
			"reviewer_peer_id":    r.ReviewerPeerID,
			"reviewer_channel_id": r.ReviewerChannelID,
		},
		path,
	)
}

// Validate reports whether review query filters are internally consistent.
func (q RunReviewQuery) Validate(path string) error {
	if q.Status.Normalize() != "" {
		if err := q.Status.Validate(nestedPath(path, "status")); err != nil {
			return err
		}
	}
	if q.Limit < 0 {
		return fmt.Errorf("%w: %s must be zero or positive: %d", ErrValidation, nestedPath(path, "limit"), q.Limit)
	}
	return nil
}

// Normalize returns a canonical verdict payload.
func (v RunReviewVerdict) Normalize() RunReviewVerdict {
	normalized := v
	normalized.Outcome = normalized.Outcome.Normalize()
	normalized.Reason = strings.TrimSpace(normalized.Reason)
	normalized.DeliveryID = strings.TrimSpace(normalized.DeliveryID)
	normalized.MissingWork = normalizeReviewMissingWork(normalized.MissingWork)
	normalized.NextRoundGuidance = strings.TrimSpace(normalized.NextRoundGuidance)
	normalized.ReviewText = strings.TrimSpace(normalized.ReviewText)
	return normalized
}

// Validate reports whether the verdict can be persisted as an authoritative reviewer decision.
func (v RunReviewVerdict) Validate(path string) error {
	if err := v.Outcome.Validate(nestedPath(path, "outcome")); err != nil {
		return err
	}
	if v.Confidence == nil {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "confidence"))
	}
	if *v.Confidence < 0 || *v.Confidence > 1 {
		return fmt.Errorf("%w: %s must be between 0 and 1", ErrValidation, nestedPath(path, "confidence"))
	}
	if strings.TrimSpace(v.Reason) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "reason"))
	}
	if strings.TrimSpace(v.DeliveryID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "delivery_id"))
	}
	missingItems, err := decodeVerdictMissingWork(v.MissingWork, nestedPath(path, "missing_work"))
	if err != nil {
		return err
	}
	if v.Outcome.Normalize() == RunReviewOutcomeApproved && len(missingItems) > 0 {
		return fmt.Errorf(
			"%w: %s must be empty when outcome is %q",
			ErrValidation,
			nestedPath(path, "missing_work"),
			v.Outcome,
		)
	}
	if v.Outcome.Normalize() == RunReviewOutcomeRejected &&
		len(missingItems) == 0 &&
		strings.TrimSpace(v.NextRoundGuidance) == "" {
		return fmt.Errorf(
			"%w: rejected reviews require missing_work or next_round_guidance",
			ErrValidation,
		)
	}
	if err := validateBoundedReviewText(v.Reason, maxRunReviewReasonBytes, nestedPath(path, "reason")); err != nil {
		return err
	}
	if err := validateBoundedReviewText(
		v.NextRoundGuidance,
		maxRunReviewGuidanceBytes,
		nestedPath(path, "next_round_guidance"),
	); err != nil {
		return err
	}
	if err := validateBoundedReviewText(
		v.ReviewText,
		maxRunReviewTextBytes,
		nestedPath(path, "review_text"),
	); err != nil {
		return err
	}
	return validateRunReviewSelectorFields(map[string]string{"delivery_id": v.DeliveryID}, path)
}

// Normalize returns a canonical verdict-recording request.
func (r RecordRunReviewRequest) Normalize() RecordRunReviewRequest {
	normalized := r
	normalized.ReviewID = strings.TrimSpace(normalized.ReviewID)
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.Verdict = normalized.Verdict.Normalize()
	return normalized
}

// Validate reports whether the verdict-recording request can identify a review and run.
func (r RecordRunReviewRequest) Validate(path string) error {
	if strings.TrimSpace(r.ReviewID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "review_id"))
	}
	if strings.TrimSpace(r.RunID) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "run_id"))
	}
	return r.Verdict.Validate(nestedPath(path, "verdict"))
}
