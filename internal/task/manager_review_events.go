package task

import (
	"context"
	"fmt"
)

type runReviewRequestedEventPayload struct {
	ReviewID    string       `json:"review_id"`
	RunID       string       `json:"run_id"`
	Policy      ReviewPolicy `json:"policy"`
	ReviewRound int          `json:"review_round"`
	Attempt     int          `json:"attempt"`
	Created     bool         `json:"created"`
}

type runReviewBoundEventPayload struct {
	ReviewID          string `json:"review_id"`
	RunID             string `json:"run_id"`
	SessionID         string `json:"session_id"`
	ReviewerAgentName string `json:"reviewer_agent_name,omitempty"`
	ReviewerPeerID    string `json:"reviewer_peer_id,omitempty"`
	ReviewerChannelID string `json:"reviewer_channel_id,omitempty"`
}

type runReviewRecordedEventPayload struct {
	ReviewID     string           `json:"review_id"`
	RunID        string           `json:"run_id"`
	Outcome      RunReviewOutcome `json:"outcome"`
	Confidence   float64          `json:"confidence"`
	DeliveryID   string           `json:"delivery_id"`
	ReviewRound  int              `json:"review_round"`
	Continuation string           `json:"continuation_run_id,omitempty"`
}

type runReviewRetryEnqueuedEventPayload struct {
	ReviewID          string `json:"review_id"`
	RunID             string `json:"run_id"`
	ContinuationRunID string `json:"continuation_run_id"`
	ReviewRound       int    `json:"review_round"`
}

type reviewTaskEvent struct {
	taskID    string
	runID     string
	eventType string
	payload   any
}

func runReviewRequestedEvent(review RunReview) reviewTaskEvent {
	return reviewTaskEvent{
		taskID:    review.TaskID,
		runID:     review.RunID,
		eventType: taskEventRunReviewRequested,
		payload: runReviewRequestedEventPayload{
			ReviewID:    review.ReviewID,
			RunID:       review.RunID,
			Policy:      review.Policy,
			ReviewRound: review.ReviewRound,
			Attempt:     review.Attempt,
			Created:     true,
		},
	}
}

func runReviewBoundEvent(review RunReview, request BindRunReviewSessionRequest) reviewTaskEvent {
	return reviewTaskEvent{
		taskID:    review.TaskID,
		runID:     review.RunID,
		eventType: taskEventRunReviewBound,
		payload: runReviewBoundEventPayload{
			ReviewID:          review.ReviewID,
			RunID:             review.RunID,
			SessionID:         request.SessionID,
			ReviewerAgentName: request.ReviewerAgentName,
			ReviewerPeerID:    request.ReviewerPeerID,
			ReviewerChannelID: request.ReviewerChannelID,
		},
	}
}

func anticipatedRunReviewResult(
	review RunReview,
	verdict RunReviewVerdict,
	continuationRunID string,
) RunReviewResult {
	recorded := cloneRunReview(&review)
	recorded.Outcome = verdict.Outcome
	recorded.Confidence = verdict.Confidence
	recorded.DeliveryID = verdict.DeliveryID
	result := RunReviewResult{Review: recorded}
	if verdict.Outcome.Normalize() != RunReviewOutcomeRejected {
		return result
	}
	result.ContinuationRun = &Run{
		ID:     continuationRunID,
		TaskID: recorded.TaskID,
		Review: &RunReviewLineage{ReviewRound: recorded.ReviewRound + 1},
	}
	return result
}

func runReviewVerdictEvents(result RunReviewResult) []reviewTaskEvent {
	confidence := 0.0
	if result.Review.Confidence != nil {
		confidence = *result.Review.Confidence
	}
	payload := runReviewRecordedEventPayload{
		ReviewID:    result.Review.ReviewID,
		RunID:       result.Review.RunID,
		Outcome:     result.Review.Outcome,
		Confidence:  confidence,
		DeliveryID:  result.Review.DeliveryID,
		ReviewRound: result.Review.ReviewRound,
	}
	if result.ContinuationRun != nil {
		payload.Continuation = result.ContinuationRun.ID
	}
	events := []reviewTaskEvent{
		{
			taskID:    result.Review.TaskID,
			runID:     result.Review.RunID,
			eventType: taskEventRunReviewRecorded,
			payload:   payload,
		},
	}
	if outcomeEventType := runReviewOutcomeEventType(result.Review.Outcome); outcomeEventType != "" {
		events = append(events, reviewTaskEvent{
			taskID:    result.Review.TaskID,
			runID:     result.Review.RunID,
			eventType: outcomeEventType,
			payload:   payload,
		})
	}
	if result.ContinuationRun != nil {
		events = append(events, reviewTaskEvent{
			taskID:    result.ContinuationRun.TaskID,
			runID:     result.ContinuationRun.ID,
			eventType: taskEventRunReviewRetry,
			payload: runReviewRetryEnqueuedEventPayload{
				ReviewID:          result.Review.ReviewID,
				RunID:             result.Review.RunID,
				ContinuationRunID: result.ContinuationRun.ID,
				ReviewRound:       runReviewRound(result.ContinuationRun),
			},
		})
	}
	return events
}

func sameRunReviewEventProjection(actual RunReviewResult, expected RunReviewResult) bool {
	if actual.Review.ReviewID != expected.Review.ReviewID ||
		actual.Review.TaskID != expected.Review.TaskID ||
		actual.Review.RunID != expected.Review.RunID ||
		actual.Review.Outcome != expected.Review.Outcome ||
		actual.Review.DeliveryID != expected.Review.DeliveryID ||
		actual.Review.ReviewRound != expected.Review.ReviewRound ||
		!sameRunReviewConfidence(actual.Review.Confidence, expected.Review.Confidence) {
		return false
	}
	if actual.ContinuationRun == nil || expected.ContinuationRun == nil {
		return actual.ContinuationRun == nil && expected.ContinuationRun == nil
	}
	return actual.ContinuationRun.ID == expected.ContinuationRun.ID &&
		actual.ContinuationRun.TaskID == expected.ContinuationRun.TaskID &&
		runReviewRound(actual.ContinuationRun) == runReviewRound(expected.ContinuationRun)
}

func sameRunReviewConfidence(actual *float64, expected *float64) bool {
	if actual == nil || expected == nil {
		return actual == nil && expected == nil
	}
	return *actual == *expected
}

func (m *Service) prepareReviewTaskEvents(actor ActorContext, events []reviewTaskEvent) ([]Event, error) {
	prepared := make([]Event, 0, len(events))
	for _, event := range events {
		preparedEvent, err := m.newTaskEvent(event.taskID, event.runID, event.eventType, actor, event.payload)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedEvent)
	}
	return prepared, nil
}

func (m *Service) recordReviewTaskEvents(ctx context.Context, events []Event) error {
	for _, event := range events {
		if err := m.store.CreateTaskEvent(ctx, event); err != nil {
			return fmt.Errorf("task: record run review event %q: %w", event.EventType, err)
		}
		m.publishTaskEventsAfterCommand(ctx, []Event{event})
	}
	return nil
}

func runReviewRound(run *Run) int {
	if run == nil || run.Review == nil {
		return 0
	}
	return run.Review.ReviewRound
}

func runReviewOutcomeEventType(outcome RunReviewOutcome) string {
	switch outcome.Normalize() {
	case RunReviewOutcomeApproved:
		return taskEventRunReviewApproved
	case RunReviewOutcomeRejected:
		return taskEventRunReviewRejected
	case RunReviewOutcomeBlocked:
		return taskEventRunReviewBlocked
	case RunReviewOutcomeError:
		return taskEventRunReviewError
	case RunReviewOutcomeTimeout:
		return taskEventRunReviewTimeout
	case RunReviewOutcomeInvalidOutput:
		return taskEventRunReviewInvalid
	default:
		return ""
	}
}
