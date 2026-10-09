package session

import (
	"cmp"
	"encoding/json"
	"fmt"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// NativeSubagentEventFromStored converts a committed tool event for native ingest.
func NativeSubagentEventFromStored(workspaceID string, stored store.SessionEvent) (NativeSubagentEvent, bool, error) {
	event, err := transcript.UnmarshalAgentEvent(stored.Content)
	if err != nil {
		return NativeSubagentEvent{}, false, err
	}
	if (event.Type != acp.EventTypeToolCall && event.Type != acp.EventTypeToolResult) ||
		(event.ProviderToolName() != "Agent" && event.ProviderToolName() != "Task") || event.ToolCallID == "" {
		return NativeSubagentEvent{}, false, nil
	}
	var payload struct {
		ToolResult *transcript.ToolResult `json:"tool_result"`
	}
	if err := json.Unmarshal([]byte(stored.Content), &payload); err != nil {
		return NativeSubagentEvent{}, false, fmt.Errorf("session: decode native tool result: %w", err)
	}
	var input struct {
		Model string `json:"model"`
	}
	model := ""
	if json.Unmarshal(event.ToolInput(), &input) == nil {
		model = input.Model
	}
	status := event.ToolStatus()
	if status == "" {
		status = string(acpsdk.ToolCallStatusInProgress)
		if event.Type == acp.EventTypeToolResult {
			status = string(acpsdk.ToolCallStatusCompleted)
		}
	}
	if event.ToolError() {
		status = string(acpsdk.ToolCallStatusFailed)
	}
	result, detail := "", event.ToolErrorDetail()
	if payload.ToolResult != nil {
		result = cmp.Or(payload.ToolResult.Content, payload.ToolResult.Stdout, payload.ToolResult.Stderr)
		detail = cmp.Or(detail, payload.ToolResult.Error)
	}
	at := stored.Timestamp
	if at.IsZero() {
		at = event.Timestamp
	}
	return NativeSubagentEvent{
		WorkspaceID: workspaceID, ParentTurnID: cmp.Or(stored.TurnID, event.TurnID),
		ProviderToolCallID: event.ToolCallID, ToolName: event.ProviderToolName(),
		Title: transcript.NativeSubagentTitle(event), Model: model, Status: status, Result: result, Error: detail, At: at,
	}, true, nil
}
