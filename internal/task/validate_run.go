package task

import (
	"fmt"
	"strings"
)

// Validate reports whether the task-run record contains the canonical persisted shape.
func (r Run) Validate() error {
	if err := validateRunIdentity(r); err != nil {
		return err
	}
	if err := validateRunExecutorBinding(r); err != nil {
		return err
	}
	if err := validateRunActorOriginSession(r); err != nil {
		return err
	}
	if err := ValidateCapabilityIDs(r.RequiredCapabilities, "task_run.required_capabilities"); err != nil {
		return err
	}
	if err := ValidateCapabilityIDs(r.PreferredCapabilities, "task_run.preferred_capabilities"); err != nil {
		return err
	}
	if err := validateTaskRunReviewGateFields(r); err != nil {
		return err
	}
	if r.TokensUsed < 0 {
		return fmt.Errorf(
			"%w: task_run.tokens_used must be zero or positive: %d",
			ErrValidation,
			r.TokensUsed,
		)
	}
	return validateRunPayloads(r)
}

func validateRunIdentity(r Run) error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%w: task_run.id is required", ErrValidation)
	}
	wakeID, targetSessionID, ownerKey := r.NetworkWakeCorrelation()
	kind := normalizeRunKindOrDefault(r.RunKind)
	if kind == RunKindNetworkWake {
		if strings.TrimSpace(r.TaskID) != "" {
			return fmt.Errorf("%w: task_run.task_id must be empty for network_wake runs", ErrValidation)
		}
		if strings.TrimSpace(r.WorkspaceID) == "" {
			return fmt.Errorf("%w: task_run.workspace_id is required for network_wake runs", ErrValidation)
		}
		if strings.TrimSpace(wakeID) == "" ||
			strings.TrimSpace(targetSessionID) == "" ||
			strings.TrimSpace(ownerKey) == "" {
			return fmt.Errorf(
				"%w: network_wake runs require network_wake_id, network_target_session_id, and network_owner_key",
				ErrValidation,
			)
		}
	} else {
		if strings.TrimSpace(r.TaskID) == "" {
			return fmt.Errorf("%w: task_run.task_id is required", ErrValidation)
		}
		if strings.TrimSpace(wakeID) != "" ||
			strings.TrimSpace(targetSessionID) != "" ||
			strings.TrimSpace(ownerKey) != "" {
			return fmt.Errorf("%w: task-anchored runs cannot carry network wake correlation", ErrValidation)
		}
	}
	if r.WorkspaceID != strings.TrimSpace(r.WorkspaceID) {
		return fmt.Errorf("%w: task_run.workspace_id must be canonical", ErrValidation)
	}
	if r.Attempt <= 0 {
		return fmt.Errorf("%w: task_run.attempt must be positive: %d", ErrValidation, r.Attempt)
	}
	if r.RecoveryCount < 0 {
		return fmt.Errorf(
			"%w: task_run.recovery_count must be zero or positive: %d",
			ErrValidation,
			r.RecoveryCount,
		)
	}
	return validateRunLineageFields(r)
}

func validateRunExecutorBinding(r Run) error {
	if err := normalizeRunKindOrDefault(r.RunKind).Validate("task_run.run_kind"); err != nil {
		return err
	}
	if normalizeRunKindOrDefault(r.RunKind) == RunKindCoordinator &&
		strings.TrimSpace(r.LoopRunID) == "" {
		return fmt.Errorf(
			"%w: task_run.loop_run_id is required for coordinator runs",
			ErrValidation,
		)
	}
	if err := r.Status.Validate("task_run.status"); err != nil {
		return err
	}
	return nil
}

func validateRunActorOriginSession(r Run) error {
	if r.ClaimedBy != nil {
		if err := r.ClaimedBy.Validate("task_run.claimed_by"); err != nil {
			return err
		}
	}
	if err := r.Origin.Validate("task_run.origin"); err != nil {
		return err
	}
	status := r.Status.Normalize()
	sessionID := strings.TrimSpace(r.SessionID)
	if status == TaskRunStatusQueued && sessionID != "" {
		return fmt.Errorf(
			"%w: task_run.session_id must be empty while status is %q",
			ErrValidation,
			status,
		)
	}
	if status == TaskRunStatusNeedsAttention && sessionID != "" &&
		(r.ClaimedBy == nil || r.ClaimedAt.IsZero()) {
		return fmt.Errorf(
			"%w: task_run claimed_by and claimed_at are required for session-bound needs_attention",
			ErrValidation,
		)
	}
	if err := validateRunLeaseMetadata(r); err != nil {
		return err
	}
	return nil
}

func validateRunPayloads(r Run) error {
	if err := ValidateMetadataSize(r.Metadata, "task_run.metadata"); err != nil {
		return err
	}
	if hasRawClaimTokenField(r.Metadata) {
		return fmt.Errorf(
			"%w: task_run.metadata must not contain raw lease credentials",
			ErrValidation,
		)
	}
	if err := ValidateResultSize(r.Result, "task_run.result"); err != nil {
		return err
	}
	if hasRawClaimTokenField(r.Result) {
		return fmt.Errorf(
			"%w: task_run.result must not contain raw lease credentials",
			ErrValidation,
		)
	}
	return nil
}

func normalizeRunKindOrDefault(kind RunKind) RunKind {
	normalized := kind.Normalize()
	if normalized == RunKindUnknown {
		return RunKindWorker
	}
	return normalized
}

func validateRunLineageFields(r Run) error {
	if strings.TrimSpace(r.PreviousRunID) == r.ID {
		return fmt.Errorf("%w: task_run.previous_run_id must not equal task_run.id", ErrValidation)
	}
	if strings.TrimSpace(r.FailureKind) == "" ||
		strings.TrimSpace(r.FailureKind) == FailureKindOperatorForced {
		return nil
	}
	return fmt.Errorf(
		"%w: task_run.failure_kind must be %q when set: %q",
		ErrValidation,
		FailureKindOperatorForced,
		r.FailureKind,
	)
}

func validateTaskRunReviewGateFields(r Run) error {
	if r.Review == nil {
		return nil
	}
	lineage := *r.Review
	if lineage.RequestRound < 0 {
		return fmt.Errorf(
			"%w: task_run.review.request_round must be zero or positive: %d",
			ErrValidation,
			lineage.RequestRound,
		)
	}
	if lineage.ReviewRound < 0 {
		return fmt.Errorf(
			"%w: task_run.review.review_round must be zero or positive: %d",
			ErrValidation,
			lineage.ReviewRound,
		)
	}
	if lineage.PolicySnapshot.Normalize() != "" {
		if err := lineage.PolicySnapshot.Validate("task_run.review.policy_snapshot"); err != nil {
			return err
		}
	}
	if strings.TrimSpace(lineage.ReviewID) != "" {
		if strings.TrimSpace(lineage.ParentRunID) == "" {
			return fmt.Errorf(
				"%w: task_run.review.parent_run_id is required when review_id is set",
				ErrValidation,
			)
		}
		if lineage.ReviewRound <= 0 {
			return fmt.Errorf(
				"%w: task_run.review.review_round must be positive when review_id is set",
				ErrValidation,
			)
		}
	}
	if err := validateBoundedReviewText(
		lineage.ContinuationReason,
		maxRunReviewReasonBytes,
		"task_run.review.continuation_reason",
	); err != nil {
		return err
	}
	if err := validateRunReviewMissingWork(normalizeReviewMissingWork(lineage.MissingWork)); err != nil {
		return err
	}
	return validateBoundedReviewText(
		lineage.NextRoundGuidance,
		maxRunReviewGuidanceBytes,
		"task_run.review.next_round_guidance",
	)
}

func validateRunLeaseMetadata(r Run) error {
	claimTokenHash := strings.TrimSpace(r.ClaimTokenHash)
	if claimTokenHash != "" && !isCanonicalClaimTokenHash(claimTokenHash) {
		return fmt.Errorf(
			"%w: task_run.claim_token_hash must be a lowercase sha256 hex hash",
			ErrValidation,
		)
	}
	if !r.LeaseUntil.IsZero() && claimTokenHash == "" {
		return fmt.Errorf(
			"%w: task_run.claim_token_hash is required when lease_until is set",
			ErrValidation,
		)
	}
	if !r.HeartbeatAt.IsZero() && claimTokenHash == "" {
		return fmt.Errorf(
			"%w: task_run.claim_token_hash is required when heartbeat_at is set",
			ErrValidation,
		)
	}
	if !r.ClaimedAt.IsZero() && !r.LeaseUntil.IsZero() && !r.LeaseUntil.After(r.ClaimedAt) {
		return fmt.Errorf("%w: task_run.lease_until must be after claimed_at", ErrValidation)
	}
	if !r.HeartbeatAt.IsZero() && !r.LeaseUntil.IsZero() && r.HeartbeatAt.After(r.LeaseUntil) {
		return fmt.Errorf("%w: task_run.heartbeat_at must not be after lease_until", ErrValidation)
	}
	if !r.ClaimedAt.IsZero() && !r.HeartbeatAt.IsZero() && r.HeartbeatAt.Before(r.ClaimedAt) {
		return fmt.Errorf("%w: task_run.heartbeat_at must not be before claimed_at", ErrValidation)
	}
	return nil
}

func isCanonicalClaimTokenHash(value string) bool {
	hash := strings.TrimSpace(value)
	hash = strings.TrimPrefix(hash, "sha256:")
	if len(hash) != 64 {
		return false
	}
	for _, r := range hash {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}
