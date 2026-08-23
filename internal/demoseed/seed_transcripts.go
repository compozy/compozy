package demoseed

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// transcriptEventGap keeps authored beats visibly ordered inside a session window.
const transcriptEventGap = 20 * time.Second

// buildTranscript expands one authored session script into persisted events plus its usage row.
func buildTranscript(story sessionStory, workspaceID string) ([]store.SessionEvent, store.TokenUsage, error) {
	turnID := "turn_" + story.ID
	agentEvents, err := transcriptAgentEvents(story, workspaceID, turnID)
	if err != nil {
		return nil, store.TokenUsage{}, err
	}
	usage := transcriptUsage(story, turnID)
	agentUsage := acp.TokenUsage{
		TurnID: turnID, InputTokens: &story.Input, OutputTokens: &story.Output,
		TotalTokens: usage.TotalTokens, CostAmount: &story.CostUSD,
		CostCurrency: usage.CostCurrency, Timestamp: story.EndedAt,
	}
	agentEvents = append(agentEvents, acp.AgentEvent{
		Type: acp.EventTypeDone, SessionID: story.ID, TurnID: turnID,
		StopReason: story.stopReason(), Usage: &agentUsage, Timestamp: story.EndedAt,
	})
	persisted, err := persistedTranscriptEvents(story, agentEvents)
	if err != nil {
		return nil, store.TokenUsage{}, err
	}
	return persisted, usage, nil
}

func (s sessionStory) stopReason() string {
	if strings.TrimSpace(s.StopReason) != "" {
		return s.StopReason
	}
	return string(store.StopCompleted)
}

func transcriptUsage(story sessionStory, turnID string) store.TokenUsage {
	total := story.Input + story.Output
	currency := "USD"
	return store.TokenUsage{
		TurnID: turnID, InputTokens: &story.Input, OutputTokens: &story.Output,
		TotalTokens: &total, CostAmount: &story.CostUSD, CostCurrency: &currency,
		Timestamp: story.EndedAt,
	}
}

func transcriptAgentEvents(
	story sessionStory,
	workspaceID string,
	turnID string,
) ([]acp.AgentEvent, error) {
	events := make([]acp.AgentEvent, 0, len(story.Steps)*2+1)
	at := story.StartedAt
	for index, step := range story.Steps {
		expanded, err := expandTranscriptStep(story, workspaceID, turnID, index, step, &at)
		if err != nil {
			return nil, err
		}
		events = append(events, expanded...)
	}
	return events, nil
}

func expandTranscriptStep(
	story sessionStory,
	workspaceID string,
	turnID string,
	index int,
	step transcriptStep,
	at *time.Time,
) ([]acp.AgentEvent, error) {
	switch step.Kind {
	case stepUser:
		return []acp.AgentEvent{simpleTranscriptEvent(
			story.ID, turnID, acp.EventTypeUserMessage, step.Text, nextTranscriptTime(at),
		)}, nil
	case stepThinking:
		return []acp.AgentEvent{simpleTranscriptEvent(
			story.ID, turnID, acp.EventTypeThought, step.Text, nextTranscriptTime(at),
		)}, nil
	case stepAgent:
		return []acp.AgentEvent{simpleTranscriptEvent(
			story.ID, turnID, acp.EventTypeAgentMessage, step.Text, nextTranscriptTime(at),
		)}, nil
	case stepTool:
		return expandToolStep(story, workspaceID, turnID, index, step, at)
	default:
		return nil, fmt.Errorf("demo seed: unknown transcript step kind %q in session %q", step.Kind, story.ID)
	}
}

func expandToolStep(
	story sessionStory,
	workspaceID string,
	turnID string,
	index int,
	step transcriptStep,
	at *time.Time,
) ([]acp.AgentEvent, error) {
	toolCallID := fmt.Sprintf("call_%s_%02d", story.ID, index+1)
	input := json.RawMessage(strings.ReplaceAll(step.ToolInput, "$WORKSPACE_ID", workspaceID))
	if !json.Valid(input) {
		return nil, fmt.Errorf("demo seed: invalid tool input for session %q step %d", story.ID, index+1)
	}
	result := json.RawMessage(step.ToolResult)
	if !json.Valid(result) {
		return nil, fmt.Errorf("demo seed: invalid tool result for session %q step %d", story.ID, index+1)
	}
	call := acp.AgentEvent{
		Type: acp.EventTypeToolCall, SessionID: story.ID, TurnID: turnID,
		Title: step.ToolName, ToolCallID: toolCallID, Timestamp: nextTranscriptTime(at),
	}
	call = call.WithTool(step.ToolName, input, false).WithToolKind(step.ToolKind)
	toolResult := acp.AgentEvent{
		Type: acp.EventTypeToolResult, SessionID: story.ID, TurnID: turnID,
		Title: step.ToolName, ToolCallID: toolCallID, Raw: result,
		Timestamp: nextTranscriptTime(at),
	}
	toolResult = toolResult.WithTool(step.ToolName, input, false).WithToolKind(step.ToolKind)
	return []acp.AgentEvent{call, toolResult}, nil
}

func simpleTranscriptEvent(
	sessionID string,
	turnID string,
	eventType string,
	text string,
	at time.Time,
) acp.AgentEvent {
	return acp.AgentEvent{
		Type: eventType, SessionID: sessionID, TurnID: turnID, Text: text, Timestamp: at,
	}
}

func nextTranscriptTime(at *time.Time) time.Time {
	current := *at
	*at = current.Add(transcriptEventGap)
	return current
}

func persistedTranscriptEvents(
	story sessionStory,
	agentEvents []acp.AgentEvent,
) ([]store.SessionEvent, error) {
	persisted := make([]store.SessionEvent, 0, len(agentEvents))
	for index, event := range agentEvents {
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			return nil, fmt.Errorf("demo seed: encode event %d for session %q: %w", index, story.ID, err)
		}
		persisted = append(persisted, store.SessionEvent{
			ID: fmt.Sprintf("evt_%s_%02d", story.ID, index+1), SessionID: story.ID,
			TurnID: "turn_" + story.ID, Type: event.Type, AgentName: story.AgentName,
			Content: content, Timestamp: event.Timestamp,
		})
	}
	return persisted, nil
}
