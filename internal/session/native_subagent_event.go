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

const nativeSubagentAgentTool = "Agent"
const nativeSubagentTaskTool = "Task"

// NativeSubagentEventFromStored converts a committed tool event for native ingest.
func NativeSubagentEventFromStored(workspaceID string, stored store.SessionEvent) (NativeSubagentEvent, bool, error) {
	event, err := transcript.UnmarshalAgentEvent(stored.Content)
	if err != nil {
		return NativeSubagentEvent{}, false, err
	}
	if event.Type == acp.EventTypeNativeTask {
		native, ok := nativeTaskEventFromStored(workspaceID, stored, event)
		return native, ok, nil
	}
	toolName := event.ProviderToolName()
	isSubagent := toolName == nativeSubagentAgentTool || toolName == nativeSubagentTaskTool
	if (event.Type != acp.EventTypeToolCall && event.Type != acp.EventTypeToolResult) ||
		!isSubagent || event.ToolCallID == "" {
		return NativeSubagentEvent{}, false, nil
	}
	var payload struct {
		ToolResult *transcript.ToolResult `json:"tool_result"`
	}
	if err := json.Unmarshal([]byte(stored.Content), &payload); err != nil {
		return NativeSubagentEvent{}, false, fmt.Errorf("session: decode native tool result: %w", err)
	}
	var input struct {
		Model           string `json:"model"`
		RunInBackground bool   `json:"run_in_background"`
	}
	if json.Unmarshal(event.ToolInput(), &input) != nil {
		input.Model, input.RunInBackground = "", false
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
		WorkspaceID:        workspaceID,
		ParentTurnID:       cmp.Or(stored.TurnID, event.TurnID),
		ProviderToolCallID: event.ToolCallID,
		ToolName:           event.ProviderToolName(),
		Title: transcript.NativeSubagentTitle(
			event,
		),
		Model:      input.Model,
		Status:     status,
		Background: input.RunInBackground || event.ProviderToolAsync(),
		Result:     result,
		Error:      detail,
		At:         at,
	}, true, nil
}

// nativeTaskEventFromStored converts a committed provider task observation. It
// names the launching call; ingest verifies that call is a native Agent/Task row.
func nativeTaskEventFromStored(
	workspaceID string,
	stored store.SessionEvent,
	event acp.AgentEvent,
) (NativeSubagentEvent, bool) {
	task := event.NativeTask()
	if task == nil || event.ToolCallID == "" {
		return NativeSubagentEvent{}, false
	}
	progress := task.LastToolName
	if task.Description != "" {
		progress = task.Description
	}
	if task.Summary != nil && *task.Summary != "" {
		progress = *task.Summary
	}
	at := stored.Timestamp
	if at.IsZero() {
		at = event.Timestamp
	}
	return NativeSubagentEvent{
		WorkspaceID:        workspaceID,
		ParentTurnID:       cmp.Or(stored.TurnID, event.TurnID),
		ProviderToolCallID: event.ToolCallID,
		Task: &NativeSubagentTask{
			Kind:             task.Kind,
			Status:           task.Status,
			Summary:          task.Summary,
			SummaryTruncated: task.SummaryTruncated,
			Error:            task.Error,
			Progress:         progress,
			Background:       task.Background,
		},
		At: at,
	}, true
}
