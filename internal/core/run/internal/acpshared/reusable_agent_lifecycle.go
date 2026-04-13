package acpshared

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runtimeevents"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

const runAgentToolName = "run_agent"

type nestedReusableAgentCall struct {
	Name string
}

type runAgentToolInput struct {
	Name string `json:"name"`
}

type runAgentToolResult struct {
	Name            string                           `json:"name"`
	Source          string                           `json:"source"`
	RunID           string                           `json:"run_id,omitempty"`
	Success         bool                             `json:"success"`
	Error           string                           `json:"error,omitempty"`
	Blocked         bool                             `json:"blocked,omitempty"`
	BlockedReason   kinds.ReusableAgentBlockedReason `json:"blocked_reason,omitempty"`
	ParentAgentName string                           `json:"parent_agent_name,omitempty"`
	Depth           int                              `json:"depth,omitempty"`
	MaxDepth        int                              `json:"max_depth,omitempty"`
}

func emitReusableAgentSetupLifecycle(
	ctx context.Context,
	runJournal runtimeEventSubmitter,
	runID string,
	jb *job,
) error {
	if jb == nil || jb.ReusableAgent == nil || !hasRuntimeEventSubmitter(runJournal) {
		return nil
	}

	payloads := buildReusableAgentSetupLifecycle(jb)
	for i := range payloads {
		if err := submitReusableAgentLifecycle(ctx, runJournal, runID, payloads[i]); err != nil {
			return err
		}
	}
	return nil
}

func buildReusableAgentSetupLifecycle(jb *job) []kinds.ReusableAgentLifecyclePayload {
	if jb == nil || jb.ReusableAgent == nil {
		return nil
	}

	agentName := strings.TrimSpace(jb.ReusableAgent.Name)
	agentSource := strings.TrimSpace(jb.ReusableAgent.Source)
	return []kinds.ReusableAgentLifecyclePayload{
		{
			Stage:       kinds.ReusableAgentLifecycleStageResolved,
			AgentName:   agentName,
			AgentSource: agentSource,
		},
		{
			Stage:             kinds.ReusableAgentLifecycleStagePromptAssembled,
			AgentName:         agentName,
			AgentSource:       agentSource,
			AvailableAgents:   jb.ReusableAgent.AvailableAgentCount,
			SystemPromptBytes: len(jb.SystemPrompt),
		},
		{
			Stage:       kinds.ReusableAgentLifecycleStageMCPMerged,
			AgentName:   agentName,
			AgentSource: agentSource,
			MCPServers:  reusableAgentMCPServerNames(jb.MCPServers),
			Resumed:     strings.TrimSpace(jb.ResumeSession) != "",
		},
	}
}

func submitReusableAgentLifecycle(
	ctx context.Context,
	runJournal runtimeEventSubmitter,
	runID string,
	payload kinds.ReusableAgentLifecyclePayload,
) error {
	if !hasRuntimeEventSubmitter(runJournal) {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	event, err := runtimeevents.NewRuntimeEvent(runID, events.EventKindReusableAgentLifecycle, payload)
	if err != nil {
		return err
	}
	if err := runJournal.Submit(ctx, event); err != nil {
		return fmt.Errorf("submit reusable agent lifecycle event: %w", err)
	}
	return nil
}

func reusableAgentMCPServerNames(servers []model.MCPServer) []string {
	names := make([]string, 0, len(servers))
	for _, server := range servers {
		if server.Stdio == nil {
			continue
		}
		name := strings.TrimSpace(server.Stdio.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func (h *SessionUpdateHandler) emitReusableAgentLifecycleFromUpdate(update model.SessionUpdate) error {
	if h == nil || !hasRuntimeEventSubmitter(h.journal) {
		return nil
	}

	for _, block := range update.Blocks {
		if toolUse, err := block.AsToolUse(); err == nil && isRunAgentToolUse(toolUse) {
			if err := h.handleNestedReusableAgentToolUse(update, toolUse); err != nil {
				return err
			}
			continue
		}

		toolResult, err := block.AsToolResult()
		if err != nil {
			continue
		}
		if err := h.handleNestedReusableAgentToolResult(update, toolResult); err != nil {
			return err
		}
	}
	return nil
}

func (h *SessionUpdateHandler) handleNestedReusableAgentToolUse(
	update model.SessionUpdate,
	block model.ToolUseBlock,
) error {
	toolCallID := firstNonEmpty(strings.TrimSpace(update.ToolCallID), strings.TrimSpace(block.ID))
	if toolCallID == "" {
		return nil
	}

	h.mu.Lock()
	if _, exists := h.nestedToolCalls[toolCallID]; exists {
		h.mu.Unlock()
		return nil
	}
	h.mu.Unlock()

	call := nestedReusableAgentCall{}
	if input, ok := decodeRunAgentToolInput(block); ok {
		call.Name = input.Name
	}

	if err := submitReusableAgentLifecycle(h.ctx, h.journal, h.runID, kinds.ReusableAgentLifecyclePayload{
		Stage:           kinds.ReusableAgentLifecycleStageNestedStarted,
		AgentName:       call.Name,
		ParentAgentName: h.currentReusableAgentName(),
		ToolCallID:      toolCallID,
	}); err != nil {
		return err
	}

	h.mu.Lock()
	if _, exists := h.nestedToolCalls[toolCallID]; !exists {
		h.nestedToolCalls[toolCallID] = call
	}
	result, hasPendingResult := h.pendingNestedResults[toolCallID]
	h.mu.Unlock()

	if !hasPendingResult {
		return nil
	}

	if err := h.emitNestedReusableAgentResultLifecycle(toolCallID, call, result); err != nil {
		h.mu.Lock()
		h.pendingNestedResults[toolCallID] = result
		h.mu.Unlock()
		return err
	}
	return nil
}

func (h *SessionUpdateHandler) handleNestedReusableAgentToolResult(
	update model.SessionUpdate,
	block model.ToolResultBlock,
) error {
	toolCallID := firstNonEmpty(strings.TrimSpace(block.ToolUseID), strings.TrimSpace(update.ToolCallID))
	if toolCallID == "" {
		return nil
	}

	h.mu.Lock()
	call, tracked := h.nestedToolCalls[toolCallID]
	h.mu.Unlock()

	result, ok := decodeRunAgentToolResult(block.Content)
	if !ok {
		return nil
	}
	if !tracked {
		h.mu.Lock()
		h.pendingNestedResults[toolCallID] = result
		h.mu.Unlock()
		return nil
	}

	return h.emitNestedReusableAgentResultLifecycle(toolCallID, call, result)
}

func (h *SessionUpdateHandler) emitNestedReusableAgentResultLifecycle(
	toolCallID string,
	call nestedReusableAgentCall,
	result runAgentToolResult,
) error {
	if h == nil {
		return nil
	}

	payload := kinds.ReusableAgentLifecyclePayload{
		Stage:           kinds.ReusableAgentLifecycleStageNestedCompleted,
		AgentName:       firstNonEmpty(strings.TrimSpace(result.Name), call.Name),
		AgentSource:     strings.TrimSpace(result.Source),
		ParentAgentName: firstNonEmpty(strings.TrimSpace(result.ParentAgentName), h.currentReusableAgentName()),
		ToolCallID:      toolCallID,
		NestedDepth:     result.Depth,
		MaxNestedDepth:  result.MaxDepth,
		OutputRunID:     strings.TrimSpace(result.RunID),
		Success:         result.Success,
		Error:           strings.TrimSpace(result.Error),
	}
	if result.Blocked || result.BlockedReason != "" {
		payload.Stage = kinds.ReusableAgentLifecycleStageNestedBlocked
		payload.Blocked = true
		payload.BlockedReason = result.BlockedReason
	}

	if err := submitReusableAgentLifecycle(h.ctx, h.journal, h.runID, payload); err != nil {
		return err
	}

	h.mu.Lock()
	delete(h.nestedToolCalls, toolCallID)
	delete(h.pendingNestedResults, toolCallID)
	h.mu.Unlock()
	return nil
}

func (h *SessionUpdateHandler) currentReusableAgentName() string {
	if h == nil || h.reusableAgent == nil {
		return ""
	}
	return strings.TrimSpace(h.reusableAgent.Name)
}

func isRunAgentToolUse(block model.ToolUseBlock) bool {
	return strings.EqualFold(strings.TrimSpace(block.ToolName), runAgentToolName) ||
		strings.EqualFold(strings.TrimSpace(block.Name), runAgentToolName)
}

func decodeRunAgentToolInput(block model.ToolUseBlock) (runAgentToolInput, bool) {
	for _, raw := range []json.RawMessage{block.Input, block.RawInput} {
		if len(raw) == 0 {
			continue
		}

		var payload runAgentToolInput
		if err := json.Unmarshal(raw, &payload); err != nil {
			continue
		}
		if strings.TrimSpace(payload.Name) == "" {
			continue
		}
		return payload, true
	}
	return runAgentToolInput{}, false
}

func decodeRunAgentToolResult(content string) (runAgentToolResult, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return runAgentToolResult{}, false
	}

	var payload runAgentToolResult
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return runAgentToolResult{}, false
	}
	if strings.TrimSpace(payload.Name) == "" &&
		strings.TrimSpace(payload.Source) == "" &&
		strings.TrimSpace(payload.RunID) == "" &&
		strings.TrimSpace(payload.Error) == "" &&
		payload.BlockedReason == "" &&
		!payload.Success &&
		!payload.Blocked {
		return runAgentToolResult{}, false
	}
	return payload, true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
