package acp

import (
	"encoding/json"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/store"
)

func translateSessionUpdate(
	notification acpsdk.SessionNotification,
	rawUpdate json.RawMessage,
	turnID string,
) (AgentEvent, error) {
	event := AgentEvent{
		SessionID: string(notification.SessionId),
		TurnID:    turnID,
		Timestamp: timeNowUTC(),
		Raw:       CloneRawMessage(rawUpdate),
	}

	switch {
	case notification.Update.UserMessageChunk != nil:
		event.Type = EventTypeUserMessage
		event.Text = extractContentText(notification.Update.UserMessageChunk.Content)
	case notification.Update.AgentMessageChunk != nil:
		event.Type = EventTypeAgentMessage
		event.Text = extractContentText(notification.Update.AgentMessageChunk.Content)
	case notification.Update.AgentThoughtChunk != nil:
		event.Type = EventTypeThought
		event.Text = extractContentText(notification.Update.AgentThoughtChunk.Content)
	case notification.Update.ToolCall != nil:
		toolCall := notification.Update.ToolCall
		event.Type = EventTypeToolCall
		event.Title = toolCall.Title
		event.ToolCallID = string(toolCall.ToolCallId)
		if err := attachToolUpdatePayload(
			&event,
			toolCall.Title,
			&toolCall.Kind,
			toolCall.RawInput,
			&toolCall.Status,
		); err != nil {
			return AgentEvent{}, err
		}
	case notification.Update.ToolCallUpdate != nil:
		update := notification.Update.ToolCallUpdate
		translateToolCallUpdate(&event, update)
		title := ""
		if update.Title != nil {
			title = *update.Title
		}
		if err := attachToolUpdatePayload(&event, title, update.Kind, update.RawInput, update.Status); err != nil {
			return AgentEvent{}, err
		}
	case notification.Update.Plan != nil:
		event.Type = EventTypePlan
	case notification.Update.AvailableCommandsUpdate != nil:
		event.Type = EventTypeAvailableCommands
		event.Title = SystemEventTitleAvailableCommandsUpdate
		event.AvailableCommands = NewAvailableCommandSet(availableCommandNames(
			notification.Update.AvailableCommandsUpdate.AvailableCommands,
		))
	case notification.Update.CurrentModeUpdate != nil:
		event.Type = EventTypeSystem
		event.Title = "current_mode_update"
	case notification.Update.ConfigOptionUpdate != nil:
		event.Type = EventTypeSystem
		event.Title = sessionUpdateConfigOption
	default:
		event.Type = EventTypeSystem
	}

	return event, nil
}

func attachToolUpdatePayload(
	event *AgentEvent,
	title string,
	kind *acpsdk.ToolKind,
	rawInput any,
	status *acpsdk.ToolCallStatus,
) error {
	toolKind := ""
	if kind != nil {
		toolKind = strings.TrimSpace(string(*kind))
	}
	var input json.RawMessage
	if rawInput != nil {
		encoded, err := json.Marshal(rawInput)
		if err != nil {
			return fmt.Errorf("acp: encode tool raw input: %w", err)
		}
		input = encoded
	}
	failed := status != nil && *status == acpsdk.ToolCallStatusFailed
	*event = event.WithTool(strings.TrimSpace(title), input, failed).WithToolKind(toolKind)
	return nil
}

func translateToolCallUpdate(event *AgentEvent, update *acpsdk.SessionToolCallUpdate) {
	event.ToolCallID = string(update.ToolCallId)
	if update.Title != nil {
		event.Title = *update.Title
	}
	if update.Status != nil &&
		(*update.Status == acpsdk.ToolCallStatusCompleted || *update.Status == acpsdk.ToolCallStatusFailed) {
		event.Type = EventTypeToolResult
		return
	}
	event.Type = EventTypeToolCall
}

func availableCommandNames(commands []acpsdk.AvailableCommand) []store.SessionAdvertisedCommand {
	result := make([]store.SessionAdvertisedCommand, 0, len(commands))
	for _, command := range commands {
		name := strings.TrimSpace(command.Name)
		if name == "" {
			continue
		}
		next := store.SessionAdvertisedCommand{
			Name:        name,
			Description: strings.TrimSpace(command.Description),
		}
		if command.Input != nil && command.Input.Unstructured != nil {
			next.Input = &store.SessionAdvertisedCommandInput{
				Hint: strings.TrimSpace(command.Input.Unstructured.Hint),
			}
		}
		result = append(result, next)
	}
	return result
}
