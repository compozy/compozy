package daemon

import (
	"encoding/json"
	"time"

	taskpkg "github.com/compozy/compozy/internal/task"
)

type compactTaskReview struct {
	ReviewID          string                   `json:"review_id"`
	TaskID            string                   `json:"task_id"`
	RunID             string                   `json:"run_id"`
	ParentReviewID    string                   `json:"parent_review_id,omitempty"`
	Policy            taskpkg.ReviewPolicy     `json:"policy"`
	ReviewRound       int                      `json:"review_round"`
	Attempt           int                      `json:"attempt"`
	Status            taskpkg.RunReviewStatus  `json:"status"`
	Outcome           taskpkg.RunReviewOutcome `json:"outcome,omitempty"`
	Confidence        *float64                 `json:"confidence,omitzero"`
	Reason            string                   `json:"reason,omitempty"`
	DeliveryID        string                   `json:"delivery_id,omitempty"`
	MissingWork       json.RawMessage          `json:"missing_work,omitempty"`
	NextRoundGuidance string                   `json:"next_round_guidance,omitempty"`
	ReviewerSessionID string                   `json:"reviewer_session_id,omitempty"`
	ReviewerAgentName string                   `json:"reviewer_agent_name,omitempty"`
	RequestedAt       time.Time                `json:"requested_at"`
	ReviewedAt        time.Time                `json:"reviewed_at"`
	DeadlineAt        time.Time                `json:"deadline_at"`
}

func projectCompactTaskReview(p *taskpkg.RunReview) *compactTaskReview {
	if p == nil {
		return nil
	}
	result := &compactTaskReview{
		ReviewID:          p.ReviewID,
		TaskID:            p.TaskID,
		RunID:             p.RunID,
		ParentReviewID:    p.ParentReviewID,
		Policy:            p.Policy,
		ReviewRound:       p.ReviewRound,
		Attempt:           p.Attempt,
		Status:            p.Status,
		Outcome:           p.Outcome,
		Confidence:        p.Confidence,
		Reason:            p.Reason,
		DeliveryID:        p.DeliveryID,
		MissingWork:       p.MissingWork,
		NextRoundGuidance: p.NextRoundGuidance,
		ReviewerSessionID: p.ReviewerSessionID,
		ReviewerAgentName: p.ReviewerAgentName,
		RequestedAt:       p.RequestedAt,
		ReviewedAt:        p.ReviewedAt,
		DeadlineAt:        p.DeadlineAt,
	}
	return result
}

type compactTaskReviewRequest struct {
	Review  compactTaskReview `json:"review"`
	Created bool              `json:"created"`
}

func projectCompactTaskReviewRequest(p *taskReviewRequestResult) *compactTaskReviewRequest {
	if p == nil {
		return nil
	}
	result := &compactTaskReviewRequest{
		Created: p.Created,
	}
	result.Review = *projectCompactTaskReview(&p.Review)
	return result
}

func viewTaskReviewRequest(p taskReviewRequestResult) any { return projectCompactTaskReviewRequest(&p) }

type taskReviewRequestResult struct {
	Review  taskpkg.RunReview `json:"review"`
	Created bool              `json:"created"`
}

type compactTaskReviews struct {
	Reviews []compactTaskReview `json:"reviews"`
}

func projectCompactTaskReviews(p *taskReviewsResult) *compactTaskReviews {
	if p == nil {
		return nil
	}
	result := &compactTaskReviews{}
	if p.Reviews != nil {
		result.Reviews = make([]compactTaskReview, len(p.Reviews))
		for i := range p.Reviews {
			result.Reviews[i] = *projectCompactTaskReview(&p.Reviews[i])
		}
	}
	return result
}

func viewTaskReviews(p taskReviewsResult) any { return projectCompactTaskReviews(&p) }

type taskReviewsResult struct {
	Reviews []taskpkg.RunReview `json:"reviews"`
}

type compactTaskReviewSubmit struct {
	Review          compactTaskReview `json:"review"`
	ContinuationRun *compactTaskRun   `json:"continuation_run"`
	CircuitOpened   bool              `json:"circuit_opened"`
}

func projectCompactTaskReviewSubmit(p *taskReviewSubmitResult) *compactTaskReviewSubmit {
	if p == nil {
		return nil
	}
	result := &compactTaskReviewSubmit{
		CircuitOpened: p.CircuitOpened,
	}
	result.Review = *projectCompactTaskReview(&p.Review)
	result.ContinuationRun = projectCompactTaskRun(p.ContinuationRun)
	return result
}

func viewTaskReviewSubmit(p taskReviewSubmitResult) any { return projectCompactTaskReviewSubmit(&p) }

type taskReviewSubmitResult struct {
	Review          taskpkg.RunReview `json:"review"`
	ContinuationRun *taskpkg.Run      `json:"continuation_run"`
	CircuitOpened   bool              `json:"circuit_opened"`
}
