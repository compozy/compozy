package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	defaultRunReviewAttempt = 1
	defaultRunReviewRound   = 1

	maxRunReviewReasonBytes        = 4096
	maxRunReviewGuidanceBytes      = 8192
	maxRunReviewTextBytes          = MaxResultBytes
	maxRunReviewSelectorFieldBytes = 256
)

var defaultMissingWorkJSON = json.RawMessage("[]")

// ReviewPolicy identifies when a terminal task run needs a review gate.
type ReviewPolicy string

const (
	// ReviewPolicyNone disables review gating.
	ReviewPolicyNone ReviewPolicy = "none"
	// ReviewPolicyOnSuccess requests review only after a successful run.
	ReviewPolicyOnSuccess ReviewPolicy = "on_success"
	// ReviewPolicyOnFailure requests review only after a failed or canceled run.
	ReviewPolicyOnFailure ReviewPolicy = "on_failure"
	// ReviewPolicyAlways requests review after every terminal run.
	ReviewPolicyAlways ReviewPolicy = "always"
)

// RunReviewStatus identifies the lifecycle state of one run review request.
type RunReviewStatus string

const (
	// RunReviewStatusRequested reports a persisted review request awaiting routing.
	RunReviewStatusRequested RunReviewStatus = "requested"
	// RunReviewStatusRouted reports a review that has been routed but not yet bound.
	RunReviewStatusRouted RunReviewStatus = "routed"
	// RunReviewStatusInReview reports a review bound to a reviewer session.
	RunReviewStatusInReview RunReviewStatus = "in_review"
	// RunReviewStatusRecorded reports a persisted terminal reviewer verdict.
	RunReviewStatusRecorded RunReviewStatus = "recorded"
	// RunReviewStatusCircuitOpened reports review routing stopped by circuit policy.
	RunReviewStatusCircuitOpened RunReviewStatus = "circuit_opened"
	// RunReviewStatusCanceled reports an explicitly canceled review request.
	RunReviewStatusCanceled RunReviewStatus = "canceled"
)

// RunReviewOutcome identifies the authoritative reviewer verdict.
type RunReviewOutcome string

const (
	// RunReviewOutcomeApproved accepts the run result.
	RunReviewOutcomeApproved RunReviewOutcome = "approved"
	// RunReviewOutcomeRejected rejects the run and may request continuation work.
	RunReviewOutcomeRejected RunReviewOutcome = "rejected"
	// RunReviewOutcomeBlocked reports the review could not continue due to external blockers.
	RunReviewOutcomeBlocked RunReviewOutcome = "blocked"
	// RunReviewOutcomeError reports reviewer/tool execution failed.
	RunReviewOutcomeError RunReviewOutcome = "error"
	// RunReviewOutcomeTimeout reports the review exceeded its deadline.
	RunReviewOutcomeTimeout RunReviewOutcome = "timeout"
	// RunReviewOutcomeInvalidOutput reports the reviewer returned a malformed verdict.
	RunReviewOutcomeInvalidOutput RunReviewOutcome = "invalid_output"
)

// RunReview is the task-domain record for one post-terminal review gate.
type RunReview struct {
	ReviewID          string           `json:"review_id"`
	TaskID            string           `json:"task_id"`
	RunID             string           `json:"run_id"`
	ParentReviewID    string           `json:"parent_review_id,omitempty"`
	Policy            ReviewPolicy     `json:"policy"`
	ReviewRound       int              `json:"review_round"`
	Attempt           int              `json:"attempt"`
	Status            RunReviewStatus  `json:"status"`
	Outcome           RunReviewOutcome `json:"outcome,omitempty"`
	Confidence        *float64         `json:"confidence,omitempty"`
	Reason            string           `json:"reason,omitempty"`
	DeliveryID        string           `json:"delivery_id,omitempty"`
	MissingWork       json.RawMessage  `json:"missing_work,omitempty"`
	NextRoundGuidance string           `json:"next_round_guidance,omitempty"`
	ReviewText        string           `json:"review_text,omitempty"`
	ReviewerSessionID string           `json:"reviewer_session_id,omitempty"`
	ReviewerAgentName string           `json:"reviewer_agent_name,omitempty"`
	ReviewerPeerID    string           `json:"reviewer_peer_id,omitempty"`
	ReviewerChannelID string           `json:"reviewer_channel_id,omitempty"`
	ReviewedBy        *ActorIdentity   `json:"reviewed_by,omitempty"`
	RequestedAt       time.Time        `json:"requested_at"`
	RoutedAt          time.Time        `json:"routed_at"`
	StartedAt         time.Time        `json:"started_at"`
	ReviewedAt        time.Time        `json:"reviewed_at"`
	DeadlineAt        time.Time        `json:"deadline_at"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// RunReviewRequest captures one authoritative request to review a terminal run.
type RunReviewRequest struct {
	TaskID         string       `json:"task_id"`
	RunID          string       `json:"run_id"`
	ReviewRound    int          `json:"review_round,omitempty"`
	Attempt        int          `json:"attempt,omitempty"`
	Policy         ReviewPolicy `json:"policy,omitempty"`
	ParentReviewID string       `json:"parent_review_id,omitempty"`
	Reason         string       `json:"reason,omitempty"`
	DeadlineAt     time.Time    `json:"deadline_at"`
}

// RunReviewRequestedNotification is emitted after one review request is
// durably created and its audit event has been recorded.
type RunReviewRequestedNotification struct {
	Review RunReview
	Task   Task
	Run    Run
	Actor  ActorContext
}

// RunReviewRequestedObserver receives typed review-request wakeups without
// tailing task events or storage tables.
type RunReviewRequestedObserver interface {
	OnRunReviewRequested(ctx context.Context, notification *RunReviewRequestedNotification)
}

// BindRunReviewSessionRequest captures a reviewer-session binding.
type BindRunReviewSessionRequest struct {
	ReviewID          string `json:"review_id"`
	SessionID         string `json:"session_id"`
	ReviewerAgentName string `json:"reviewer_agent_name,omitempty"`
	ReviewerPeerID    string `json:"reviewer_peer_id,omitempty"`
	ReviewerChannelID string `json:"reviewer_channel_id,omitempty"`
}

// RunReviewBinding is the lookup shape consumed by reviewer-session tooling.
type RunReviewBinding struct {
	Review            RunReview `json:"review"`
	SessionID         string    `json:"session_id"`
	ReviewerAgentName string    `json:"reviewer_agent_name,omitempty"`
	ReviewerPeerID    string    `json:"reviewer_peer_id,omitempty"`
	ReviewerChannelID string    `json:"reviewer_channel_id,omitempty"`
}

// RunReviewQuery captures supported review read filters.
type RunReviewQuery struct {
	TaskID            string          `json:"task_id,omitempty"`
	RunID             string          `json:"run_id,omitempty"`
	Status            RunReviewStatus `json:"status,omitempty"`
	ReviewerSessionID string          `json:"reviewer_session_id,omitempty"`
	Limit             int             `json:"limit,omitempty"`
}

// RunReviewVerdict captures the terminal reviewer payload persisted by the task domain.
type RunReviewVerdict struct {
	Outcome           RunReviewOutcome `json:"outcome"`
	Confidence        *float64         `json:"confidence"`
	Reason            string           `json:"reason"`
	DeliveryID        string           `json:"delivery_id"`
	MissingWork       json.RawMessage  `json:"missing_work,omitempty"`
	NextRoundGuidance string           `json:"next_round_guidance,omitempty"`
	ReviewText        string           `json:"review_text,omitempty"`
}

// RecordRunReviewRequest captures an authoritative persisted review verdict write.
type RecordRunReviewRequest struct {
	ReviewID string           `json:"review_id"`
	RunID    string           `json:"run_id"`
	Verdict  RunReviewVerdict `json:"verdict"`
}

// RunReviewResult reports the stored verdict and optional rejected-review continuation run.
type RunReviewResult struct {
	Review          RunReview `json:"review"`
	ContinuationRun *Run      `json:"continuation_run,omitempty"`
	CircuitOpened   bool      `json:"circuit_opened,omitempty"`
}

// Normalize returns the normalized review policy value.
func (p ReviewPolicy) Normalize() ReviewPolicy {
	return ReviewPolicy(strings.ToLower(strings.TrimSpace(string(p))))
}

// Validate reports whether the review policy is one of the supported values.
func (p ReviewPolicy) Validate(path string) error {
	switch p.Normalize() {
	case ReviewPolicyNone, ReviewPolicyOnSuccess, ReviewPolicyOnFailure, ReviewPolicyAlways:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be one of %q, %q, %q, or %q: %q",
			ErrValidation,
			path,
			ReviewPolicyNone,
			ReviewPolicyOnSuccess,
			ReviewPolicyOnFailure,
			ReviewPolicyAlways,
			p,
		)
	}
}

// MatchesRunStatus reports whether this policy applies to a terminal run status.
func (p ReviewPolicy) MatchesRunStatus(status RunStatus) bool {
	switch p.Normalize() {
	case ReviewPolicyOnSuccess:
		return status.Normalize() == TaskRunStatusCompleted
	case ReviewPolicyOnFailure:
		switch status.Normalize() {
		case TaskRunStatusFailed, TaskRunStatusCanceled:
			return true
		default:
			return false
		}
	case ReviewPolicyAlways:
		return IsTerminalRunStatus(status)
	default:
		return false
	}
}

// Normalize returns the normalized run-review status.
func (s RunReviewStatus) Normalize() RunReviewStatus {
	return RunReviewStatus(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the run-review status is supported.
func (s RunReviewStatus) Validate(path string) error {
	switch s.Normalize() {
	case RunReviewStatusRequested,
		RunReviewStatusRouted,
		RunReviewStatusInReview,
		RunReviewStatusRecorded,
		RunReviewStatusCircuitOpened,
		RunReviewStatusCanceled:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, s)
	}
}

// Normalize returns the normalized run-review outcome.
func (o RunReviewOutcome) Normalize() RunReviewOutcome {
	return RunReviewOutcome(strings.ToLower(strings.TrimSpace(string(o))))
}

// Validate reports whether the run-review outcome is supported.
func (o RunReviewOutcome) Validate(path string) error {
	switch o.Normalize() {
	case RunReviewOutcomeApproved,
		RunReviewOutcomeRejected,
		RunReviewOutcomeBlocked,
		RunReviewOutcomeError,
		RunReviewOutcomeTimeout,
		RunReviewOutcomeInvalidOutput:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, o)
	}
}
