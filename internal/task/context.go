package task

import "time"

// ContextRequest identifies the active lease context one agent session may read.
type ContextRequest struct {
	SessionID string    `json:"session_id"`
	RunID     string    `json:"run_id,omitempty"`
	Now       time.Time `json:"now"`
}

// OperatorTaskContextRequest identifies the task context an operator read path may request.
type OperatorTaskContextRequest struct {
	TaskID string    `json:"task_id"`
	Now    time.Time `json:"now"`
}

// RuntimeLimits reports the runtime limits relevant to one task context bundle.
type RuntimeLimits struct {
	MaxRuntimeSeconds int64 `json:"max_runtime_seconds"`
	SummaryMaxBytes   int   `json:"summary_max_bytes"`
	ContextMaxBytes   int   `json:"context_body_max_bytes"`
}

// ReviewContinuation is the rejected-review guidance that produced a continuation run.
type ReviewContinuation struct {
	ReviewID          string   `json:"review_id"`
	ReviewedRunID     string   `json:"reviewed_run_id"`
	ReviewRound       int      `json:"review_round"`
	Outcome           string   `json:"outcome"`
	Reason            string   `json:"reason"`
	MissingWork       []string `json:"missing_work"`
	NextRoundGuidance string   `json:"next_round_guidance"`
}

// RunReviewSummary is the redacted review history shape included in task context.
type RunReviewSummary struct {
	ReviewID      string `json:"review_id"`
	RunID         string `json:"run_id"`
	ReviewRound   int    `json:"review_round"`
	Attempt       int    `json:"attempt"`
	Status        string `json:"status"`
	Outcome       string `json:"outcome,omitempty"`
	Reason        string `json:"reason,omitempty"`
	ReviewedAt    string `json:"reviewed_at,omitempty"`
	ReviewerLabel string `json:"reviewer_label,omitempty"`
}

// ContextBundle is the bounded task/run context injected into task sessions.
type ContextBundle struct {
	Task               Reference           `json:"task"`
	LatestEventSeq     int64               `json:"latest_event_seq"`
	CurrentRun         *RunSummary         `json:"current_run,omitempty"`
	PriorAttempts      []RunSummary        `json:"prior_attempts"`
	RecentEvents       []TimelineItem      `json:"recent_events"`
	HandoffSummary     string              `json:"handoff_summary,omitempty"`
	Limits             RuntimeLimits       `json:"limits"`
	ExecutionProfile   *ExecutionProfile   `json:"execution_profile,omitempty"`
	ReviewContinuation *ReviewContinuation `json:"review_continuation,omitempty"`
	ReviewHistory      []RunReviewSummary  `json:"review_history"`
}

// NormalizeContextBundle returns a bundle with stable empty array fields.
func NormalizeContextBundle(bundle ContextBundle) ContextBundle {
	if bundle.PriorAttempts == nil {
		bundle.PriorAttempts = []RunSummary{}
	}
	if bundle.RecentEvents == nil {
		bundle.RecentEvents = []TimelineItem{}
	}
	if bundle.ReviewHistory == nil {
		bundle.ReviewHistory = []RunReviewSummary{}
	}
	if bundle.ReviewContinuation != nil && bundle.ReviewContinuation.MissingWork == nil {
		continuation := *bundle.ReviewContinuation
		continuation.MissingWork = []string{}
		bundle.ReviewContinuation = &continuation
	}
	return bundle
}
