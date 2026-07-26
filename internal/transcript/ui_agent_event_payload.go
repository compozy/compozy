package transcript

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

// UIAgentEventPayload mirrors the prompt-stream data payload shape.
type UIAgentEventPayload struct {
	Type             string                `json:"type"`
	SessionID        string                `json:"session_id,omitempty"`
	TurnID           string                `json:"turn_id,omitempty"`
	RequestID        string                `json:"request_id,omitempty"`
	Timestamp        string                `json:"timestamp,omitempty"`
	Text             string                `json:"text,omitempty"`
	Title            string                `json:"title,omitempty"`
	ToolCallID       string                `json:"tool_call_id,omitempty"`
	StopReason       string                `json:"stop_reason,omitempty"`
	PromptStopReason acp.PromptStopReason  `json:"prompt_stop_reason,omitempty"`
	Action           string                `json:"action,omitempty"`
	Resource         string                `json:"resource,omitempty"`
	Decision         string                `json:"decision,omitempty"`
	Error            string                `json:"error,omitempty"`
	Failure          *store.SessionFailure `json:"failure,omitempty"`
	Goal             *acp.GoalPromptMeta   `json:"goal,omitempty"`
	Usage            *UITokenUsagePayload  `json:"usage,omitempty"`
	Runtime          *acp.RuntimeActivity  `json:"runtime,omitempty"`
	Raw              json.RawMessage       `json:"raw,omitempty"`
}

// UIAgentEventPayloadFromEvent converts an ACP event into the prompt-stream data payload.
func UIAgentEventPayloadFromEvent(event acp.AgentEvent) UIAgentEventPayload {
	event = RedactAgentEvent(event)
	payload := UIAgentEventPayload{
		Type: event.Type, SessionID: event.SessionID, TurnID: event.TurnID, RequestID: event.RequestID,
		Text: event.Text, Title: event.Title, ToolCallID: event.ToolCallID, StopReason: event.StopReason,
		PromptStopReason: event.PromptStopReason, Action: event.Action, Resource: event.Resource,
		Decision: event.Decision, Error: event.Error, Failure: store.CloneSessionFailure(event.Failure),
		Goal:  acp.CloneGoalPromptMeta(event.Goal),
		Usage: uiTokenUsagePayloadFromUsage(event.Usage), Runtime: cloneRuntimeActivity(event.Runtime),
		Raw: payloadJSONBytes(event.Raw),
	}
	if !event.Timestamp.IsZero() {
		payload.Timestamp = event.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	return payload
}
