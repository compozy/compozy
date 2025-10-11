package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
)

const (
	agentCallToolName = "cp__call_agent"
)

// containsSuccessfulAgentCall checks if any tool results contain a successful cp__call_agent execution.
func containsSuccessfulAgentCall(results []llmadapter.ToolResult) bool {
	for _, result := range results {
		if result.Name == agentCallToolName && isToolResultSuccess(result) {
			return true
		}
	}
	return false
}

// buildAgentCallCompletionHint creates a completion hint message for successful agent calls.
func buildAgentCallCompletionHint(results []llmadapter.ToolResult) string {
	var agentResults []string
	for _, result := range results {
		if result.Name == agentCallToolName && isToolResultSuccess(result) {
			agentID := extractAgentIDFromResult(result)
			if agentID != "" {
				agentResults = append(agentResults, fmt.Sprintf("'%s'", agentID))
			}
		}
	}
	if len(agentResults) == 0 {
		return ""
	}
	agentList := strings.Join(agentResults, ", ")
	if len(agentResults) == 1 {
		return fmt.Sprintf(
			"The agent %s has successfully completed its task and returned a response. "+
				"Based on this response, provide your final answer to complete the workflow.",
			agentList,
		)
	}
	return fmt.Sprintf(
		"The agents %s have successfully completed their tasks and returned responses. "+
			"Based on these responses, provide your final answer to complete the workflow.",
		agentList,
	)
}

// extractAgentIDFromResult attempts to extract the agent_id from the tool result content.
func extractAgentIDFromResult(result llmadapter.ToolResult) string {
	if len(result.JSONContent) > 0 {
		return extractAgentIDFromJSON(result.JSONContent)
	}
	content := strings.TrimSpace(result.Content)
	if strings.HasPrefix(content, "{") {
		return extractAgentIDFromJSON([]byte(content))
	}
	return ""
}

// extractAgentIDFromJSON parses JSON and extracts the agent_id field.
func extractAgentIDFromJSON(data []byte) string {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return ""
	}
	if agentID, ok := obj["agent_id"].(string); ok && agentID != "" {
		return agentID
	}
	return ""
}
