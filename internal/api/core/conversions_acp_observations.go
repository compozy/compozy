package core

import (
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
)

func availableCommandPayloads(commands []store.SessionAdvertisedCommand) []contract.ACPAvailableCommandPayload {
	payloads := make([]contract.ACPAvailableCommandPayload, 0, len(commands))
	for _, command := range commands {
		payload := contract.ACPAvailableCommandPayload{
			Name: strings.TrimSpace(command.Name), Description: strings.TrimSpace(command.Description),
		}
		if command.Input != nil {
			payload.Input = &contract.ACPAvailableCommandInputPayload{Hint: strings.TrimSpace(command.Input.Hint)}
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

// AgentEventPayloadFromEvent converts an agent event into the shared raw-stream payload.
func AgentEventPayloadFromEvent(event acp.AgentEvent) contract.AgentEventPayload {
	return contract.AgentEventPayload{
		Type: event.Type, SessionID: event.SessionID, TurnID: event.TurnID,
		ClientMessageID: event.ClientMessageIDValue(), RequestID: event.RequestID,
		Timestamp: event.Timestamp, Text: event.Text, Title: event.Title, ToolCallID: event.ToolCallID,
		StopReason: event.StopReason, PromptStopReason: contract.ACPPromptStopReason(event.PromptStopReason),
		AvailableCommands: availableCommandPayloads(event.AvailableCommands.Values()), Action: event.Action,
		Resource: event.Resource, Decision: event.Decision, Error: event.Error,
		Failure: SessionFailurePayloadFromStore(event.Failure), Usage: TokenUsagePayloadFromUsage(event.Usage),
		Goal: goalPromptMetaPayload(event.Goal), Runtime: runtimeActivityPayloadFromEvent(event.Runtime),
		Raw: payloadJSONBytes(event.Raw),
	}
}

func goalPromptMetaPayload(meta *acp.GoalPromptMeta) *contract.GoalPromptMeta {
	normalized := acp.CloneGoalPromptMeta(meta)
	if normalized == nil {
		return nil
	}
	return &contract.GoalPromptMeta{
		Kind:          contract.GoalPromptKind(normalized.Kind),
		RunID:         normalized.RunID,
		NodeID:        normalized.NodeID,
		Generation:    normalized.Generation,
		ItemIndex:     normalized.ItemIndex,
		Turn:          normalized.Turn,
		PromptAttempt: normalized.PromptAttempt,
		PromptID:      normalized.PromptID,
	}
}
