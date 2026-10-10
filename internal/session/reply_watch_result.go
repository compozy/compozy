package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

type TurnResult struct {
	Outcome string
	Text    string
	Error   string
}

func (m *Manager) TurnResult(ctx context.Context, sessionID, turnID string) (TurnResult, error) {
	if strings.TrimSpace(turnID) == "" {
		return TurnResult{}, errors.New("session: reply turn id is required")
	}
	events, err := m.replyTurnEvents(ctx, sessionID, turnID)
	if err != nil {
		return TurnResult{}, err
	}
	result, err := replyTurnOutcome(events)
	if err != nil {
		return result, err
	}

	if result.Outcome == "" {
		active, activeErr := m.ActivePromptRun(ctx, sessionID)
		if activeErr != nil && !errors.Is(activeErr, ErrPromptNotActive) {
			return result, activeErr
		}
		if (activeErr != nil || active.TurnID != turnID) && len(events) > 0 {
			info, err := m.Status(ctx, sessionID)
			if err != nil {
				return result, err
			}
			if info.State == StateStopped {
				result.Outcome = store.ReplyOutcomeCanceled
				if info.Failure != nil && info.Failure.Kind != store.FailureCanceled {
					result.Outcome, result.Error = store.ReplyOutcomeFailed, info.Failure.Summary
				}
			}
		}
	}
	messages, err := transcript.Assemble(events)
	if err != nil {
		return result, err
	}
	for _, message := range slices.Backward(messages) {
		if message.Role == transcript.RoleAssistant && strings.TrimSpace(message.Content) != "" {
			result.Text = message.Content
			break
		}
	}
	return result, nil
}

func replyTurnOutcome(events []store.SessionEvent) (TurnResult, error) {
	var result TurnResult
	for _, event := range events {
		decoded, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			return result, err
		}
		switch event.Type {
		case acp.EventTypeError:
			result.Outcome = store.ReplyOutcomeFailed
			result.Error = firstTrimmedNonEmpty(decoded.Error, decoded.Text, "turn failed")
			if decoded.Failure != nil && decoded.Failure.Kind == store.FailureCanceled {
				result.Outcome, result.Error = store.ReplyOutcomeCanceled, ""
			}
		case EventTypeSessionStopped:
			if result.Outcome == "" {
				result.Outcome = store.ReplyOutcomeCanceled
				if decoded.Error != "" && (decoded.Failure == nil || decoded.Failure.Kind != store.FailureCanceled) {
					result.Outcome, result.Error = store.ReplyOutcomeFailed, decoded.Error
				}
			}
		case acp.EventTypeDone:
			if decoded.PromptStopReason == acp.PromptStopReasonCancelled ||
				decoded.StopReason == string(acp.PromptStopReasonCancelled) {
				result.Outcome = store.ReplyOutcomeCanceled
			} else if result.Outcome == "" {
				result.Outcome = store.ReplyOutcomeCompleted
			}
		}
	}
	return result, nil
}

func (m *Manager) replyTurnEvents(ctx context.Context, sessionID, turnID string) ([]store.SessionEvent, error) {
	query := store.EventQuery{TurnID: turnID, Limit: 200, Forward: true}
	var events []store.SessionEvent
	for {
		page, err := m.Events(ctx, sessionID, query)
		if err != nil {
			return nil, err
		}
		for _, event := range page {
			decoded, err := transcript.UnmarshalAgentEvent(event.Content)
			if err != nil {
				return nil, err
			}
			if decoded.ParentToolCallID() == "" {
				events = append(events, event)
			}
		}
		if len(page) < query.Limit {
			return events, nil
		}
		query.AfterSequence = page[len(page)-1].Sequence
	}
}

func (m *Manager) replyMessageTurn(
	ctx context.Context, w store.ReplyWatch, admission store.SessionPromptAdmission,
	inputs []store.SessionInputQueueEntry, recovering bool,
) (string, error) {
	ids := []string{admission.EventID}
	for i := range inputs {
		input := &inputs[i]
		if input.EventID != "" && !slices.Contains(ids, input.EventID) {
			ids = append(ids, input.EventID)
		}
	}
	for _, id := range ids {
		if id == "" {
			continue
		}
		events, err := m.Events(
			ctx,
			w.TargetSessionID,
			store.EventQuery{ID: id, Type: acp.EventTypeUserMessage, Limit: 1},
		)
		if err != nil {
			return "", err
		}
		if len(events) > 0 {
			return events[0].TurnID, nil
		}
	}
	if !recovering {
		return "", nil
	}
	// Old incomplete bindings may be recovered from one bounded page of input events.
	events, err := m.Events(ctx, w.TargetSessionID, store.EventQuery{Type: acp.EventTypeUserMessage, Limit: 200})
	if err != nil {
		return "", err
	}
	for _, event := range events {
		decoded, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			return "", err
		}
		if decoded.MessageIDValue() == w.MessageID && decoded.ParentToolCallID() == "" {
			return event.TurnID, nil
		}
	}
	return "", nil
}

func replyDispatchTime(admission store.SessionPromptAdmission, inputs []store.SessionInputQueueEntry) time.Time {
	since := admission.CreatedAt
	if admission.DispatchCommittedAt != nil {
		since = *admission.DispatchCommittedAt
	}
	for i := range inputs {
		input := &inputs[i]
		if input.DispatchStartedAt != nil && input.DispatchStartedAt.After(since) {
			since = *input.DispatchStartedAt
		}
	}
	return since
}

func (m *Manager) replyTargetSettledSince(ctx context.Context, id string, since time.Time) (bool, error) {
	info, err := m.Status(ctx, id)
	if err != nil {
		return false, err
	}
	if info.State == StateStopped && !info.UpdatedAt.Before(since) {
		return true, nil
	}
	events, err := m.Events(ctx, id, store.EventQuery{Type: acp.EventTypeDone, Since: since, Limit: 1})
	return len(events) > 0, err
}
