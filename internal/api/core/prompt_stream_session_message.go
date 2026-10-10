package core

import (
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

func (e *PromptStreamEncoder) emitSessionMessagePart(writer FlushWriter, event acp.AgentEvent) error {
	var payload transcript.UISessionMessagePayload
	if card := event.SessionMessageCard(); len(card) > 0 {
		if err := json.Unmarshal(card, &payload); err != nil {
			return err
		}
	} else {
		content, err := transcript.MarshalAgentEvent(event)
		if err != nil {
			return err
		}
		var ok bool
		payload, ok = transcript.SessionMessagePartForStoredEvent(
			store.SessionEvent{SessionID: e.sessionID, TurnID: event.TurnID, Type: event.Type, Content: content},
		)
		if !ok {
			return nil
		}
	}
	if e.sessionMessageCards == nil {
		e.sessionMessageCards = make(map[string]transcript.UISessionMessagePayload)
	}
	payload = payload.WithFallback(e.sessionMessageCards[payload.ToolCallID])
	e.sessionMessageCards[payload.ToolCallID] = payload
	return WriteSSE(writer, SSEMessage{Data: struct {
		Type string                             `json:"type"`
		ID   string                             `json:"id"`
		Data transcript.UISessionMessagePayload `json:"data"`
	}{Type: "data-compozy-session-message", ID: payload.ToolCallID, Data: payload}})
}
