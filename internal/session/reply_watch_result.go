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

func (m *Manager) replyMessageTurn(ctx context.Context, sessionID, messageID string) (string, error) {
	events, err := m.replyTurnEvents(ctx, sessionID, "")
	if err != nil {
		return "", err
	}
	for _, event := range events {
		decoded, err := transcript.UnmarshalAgentEvent(event.Content)
		if err != nil {
			return "", err
		}
		if decoded.MessageIDValue() == messageID && event.Type == acp.EventTypeUserMessage {
			return event.TurnID, nil
		}
	}
	return "", nil
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
