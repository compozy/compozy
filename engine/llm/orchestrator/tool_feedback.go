package orchestrator

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
)

func buildFailureGuidanceMessages(results []llmadapter.ToolResult) []llmadapter.Message {
	messages := make([]llmadapter.Message, 0, len(results))
	for i := range results {
		observation := failureObservation(results[i])
		if observation == "" {
			continue
		}
		messages = append(messages, llmadapter.Message{
			Role:    roleAssistant,
			Content: observation,
		})
	}
	return messages
}

func failureObservation(result llmadapter.ToolResult) string {
	payload, ok := parseToolResultPayload(result)
	if !ok || !isResultFailure(payload) {
		return ""
	}
	errorMessage := extractFailureMessage(payload)
	hint := extractRemediationHint(payload, result.Name, errorMessage)
	builder := strings.Builder{}
	builder.WriteString("Observation: tool ")
	builder.WriteString(result.Name)
	builder.WriteString(" failed.")
	if errorMessage != "" {
		builder.WriteString(" Error: ")
		builder.WriteString(core.RedactString(errorMessage))
	}
	if hint != "" {
		hint = core.RedactString(hint)
		builder.WriteString(" Next step: ")
		builder.WriteString(hint)
	} else {
		builder.WriteString(" Review the failure details and adjust your next action before retrying.")
	}
	return builder.String()
}

func parseToolResultPayload(result llmadapter.ToolResult) (map[string]any, bool) {
	if len(result.JSONContent) > 0 {
		var payload map[string]any
		if err := json.Unmarshal(result.JSONContent, &payload); err == nil {
			return payload, true
		}
	}
	if strings.TrimSpace(result.Content) == "" {
		return nil, false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(result.Content), &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func isResultFailure(payload map[string]any) bool {
	successRaw, ok := payload["success"]
	if ok {
		if success, ok := successRaw.(bool); ok {
			return !success
		}
	}
	if _, ok := payload["error"]; ok {
		return true
	}
	if okValue, exists := payload["ok"]; exists {
		if okBool, ok := okValue.(bool); ok {
			return !okBool
		}
	}
	if errorsRaw, ok := payload["errors"]; ok {
		if entries, ok := errorsRaw.([]any); ok && len(entries) > 0 {
			return true
		}
	}
	return false
}

func extractFailureMessage(payload map[string]any) string {
	if msg, ok := stringValue(payload["error"]); ok {
		return msg
	}
	if errMap, ok := mapValue(payload["error"]); ok {
		if msg, ok := stringValue(errMap["message"]); ok {
			return msg
		}
		if details, ok := stringValue(errMap["details"]); ok {
			return details
		}
	}
	errorsRaw, hasErrors := payload["errors"]
	if !hasErrors {
		return ""
	}
	if entries, ok := errorsRaw.([]any); ok {
		for _, item := range entries {
			if entry, ok := mapValue(item); ok {
				if msg, ok := stringValue(entry["error"]); ok {
					return msg
				}
			}
		}
	}
	return ""
}

func extractRemediationHint(
	payload map[string]any,
	toolName string,
	errorMessage string,
) string {
	if hint, ok := stringValue(payload["remediation_hint"]); ok && hint != "" {
		return hint
	}
	if errMap, ok := mapValue(payload["error"]); ok {
		if hint, ok := stringValue(errMap["remediation_hint"]); ok && hint != "" {
			return hint
		}
	}
	if errorsRaw, ok := payload["errors"]; ok {
		if entries, ok := errorsRaw.([]any); ok {
			for _, entry := range entries {
				if entryMap, ok := mapValue(entry); ok {
					if hint, ok := stringValue(entryMap["hint"]); ok && hint != "" {
						return hint
					}
				}
			}
		}
	}
	canonical := buildCanonicalHint(toolName, errorMessage)
	if canonical != "" {
		return canonical
	}
	return ""
}

func buildCanonicalHint(toolName, errorMessage string) string {
	lowerMsg := strings.ToLower(errorMessage)
	switch {
	case toolName == agentCallToolName && strings.Contains(lowerMsg, "agent_id"):
		return "Include \"agent_id\" using a value returned from cp__list_agents before calling cp__call_agent."
	case toolName == agentCallToolName && strings.Contains(lowerMsg, "either action_id or prompt"):
		return "Provide \"action_id\" for the agent (when listed by cp__describe_agent) " +
			"or add a \"prompt\" describing the task."
	default:
		return ""
	}
}

func stringValue(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	default:
		return "", false
	}
}

func mapValue(value any) (map[string]any, bool) {
	if m, ok := value.(map[string]any); ok {
		return m, true
	}
	return nil, false
}
