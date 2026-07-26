package session

import (
	"slices"
	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/store"
)

func ensureRepairTurn(turns map[string]*repairTurnState, turnID string) *repairTurnState {
	turn := turns[turnID]
	if turn != nil {
		return turn
	}
	turn = &repairTurnState{
		turnID:      turnID,
		toolCalls:   make(map[string]repairToolCall),
		toolResults: make(map[string]struct{}),
	}
	turns[turnID] = turn
	return turn
}

func applyRepairEvent(
	turn *repairTurnState,
	event *repairEvent,
	eventType string,
	analysis *repairAnalysis,
) {
	switch eventType {
	case acp.EventTypeUserMessage,
		acp.EventTypeSyntheticReentry,
		acp.EventTypeAgentMessage,
		acp.EventTypeThought,
		acp.EventTypePlan,
		acp.EventTypeRuntimeProgress,
		acp.EventTypeRuntimeWarning,
		acp.EventTypePermission,
		acp.EventTypeSystem:
		turn.hasPromptData = true
	case acp.EventTypeToolCall:
		turn.hasPromptData = true
		toolCallID := strings.TrimSpace(event.agent.ToolCallID)
		if toolCallID == "" {
			analysis.issues = append(analysis.issues, RepairIssue{
				Code:     RepairIssueDanglingToolCallMissingID,
				Severity: RepairSeverityWarning,
				TurnID:   turn.turnID,
				EventID:  event.stored.ID,
				Detail:   "tool_call event cannot be individually closed because tool_call_id is empty",
			})
			return
		}
		turn.toolCalls[toolCallID] = repairToolCall{
			toolName: strings.TrimSpace(event.agent.Title),
		}
	case acp.EventTypeToolResult:
		turn.hasPromptData = true
		toolCallID := strings.TrimSpace(event.agent.ToolCallID)
		if toolCallID != "" {
			turn.toolResults[toolCallID] = struct{}{}
		}
	case acp.EventTypeDone, acp.EventTypeError:
		turn.hasPromptData = true
		turn.terminal = true
	}
}

func repairDefaultStopReason(reason store.StopReason) bool {
	switch reason {
	case store.StopAgentCrashed, store.StopError:
		return true
	default:
		return false
	}
}

func planRepairActions(turn repairTurnState, includeTerminal bool) []RepairAction {
	actions := make([]RepairAction, 0, len(turn.toolCalls)+1)
	toolCallIDs := make([]string, 0, len(turn.toolCalls))
	for toolCallID := range turn.toolCalls {
		if _, ok := turn.toolResults[toolCallID]; ok {
			continue
		}
		toolCallIDs = append(toolCallIDs, toolCallID)
	}
	slices.Sort(toolCallIDs)

	for _, toolCallID := range toolCallIDs {
		toolCall := turn.toolCalls[toolCallID]
		actions = append(actions, RepairAction{
			Code:       RepairActionAppendInterruptedToolResult,
			TurnID:     turn.turnID,
			ToolCallID: toolCallID,
			ToolName:   toolCall.toolName,
		})
	}
	if includeTerminal && turn.hasPromptData {
		actions = append(actions, RepairAction{
			Code:   RepairActionAppendTerminalError,
			TurnID: turn.turnID,
		})
	}
	return actions
}
