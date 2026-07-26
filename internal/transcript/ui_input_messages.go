package transcript

import (
	"encoding/json"
	"strings"

	"github.com/compozy/agh/internal/acp"
)

func inputUIMessage(decoded *decodedStoredEvent, role string) *UIMessage {
	text := strings.TrimSpace(decoded.parsed.Text)
	if text == "" {
		return nil
	}
	return &UIMessage{
		ID:       inputMessageID(decoded, role),
		Role:     role,
		Metadata: inputUIMessageMetadata(decoded.agent),
		Parts: []UIMessagePart{{
			Type:  uiPartText,
			Text:  text,
			State: uiPartStateDone,
		}},
	}
}

func inputUIMessageMetadata(event acp.AgentEvent) json.RawMessage {
	turnID := strings.TrimSpace(event.TurnID)
	clientMessageID := event.ClientMessageIDValue()
	goal := acp.CloneGoalPromptMeta(event.Goal)
	if turnID == "" && clientMessageID == "" && goal == nil {
		return nil
	}
	encoded, err := json.Marshal(struct {
		TurnID          string              `json:"turn_id,omitempty"`
		ClientMessageID string              `json:"client_message_id,omitempty"`
		Goal            *acp.GoalPromptMeta `json:"goal,omitempty"`
	}{TurnID: turnID, ClientMessageID: clientMessageID, Goal: goal})
	if err != nil {
		return nil
	}
	return json.RawMessage(encoded)
}

// Markers use the assistant data-agh-event wire contract.
func runtimeMarkerUIMessage(decoded *decodedStoredEvent) UIMessage {
	return UIMessage{
		ID: fallbackMessageID(
			strings.TrimSpace(decoded.stored.ID),
			strings.TrimSpace(decoded.parsed.ID),
			"runtime-marker",
		),
		Role: UIRoleAssistant,
		Parts: []UIMessagePart{{
			Type: uiPartDataEvent,
			Data: decoded.dataPayload(),
		}},
	}
}
