package transcript

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
)

// sessionPromptInput is the subset of `compozy__session_prompt` arguments the card reads.
type sessionPromptInput struct {
	SessionID   string `json:"session_id"`
	WorkspaceID string `json:"workspace_id"`
	MessageID   string `json:"message_id"`
	Mode        string `json:"mode"`
	InReplyTo   string `json:"in_reply_to"`
}

// sessionPromptArguments reads the call's arguments. Claude reports them as the tool input
// itself; Codex reports the MCP call envelope `{server, tool, arguments}`.
func sessionPromptArguments(raw []byte) sessionPromptInput {
	var envelope struct {
		sessionPromptInput `json:",inline"`

		Arguments jsontext.Value `json:"arguments"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &envelope) != nil {
		return sessionPromptInput{}
	}
	if envelope.SessionID != "" || len(envelope.Arguments) == 0 {
		return envelope.sessionPromptInput
	}
	arguments := []byte(envelope.Arguments)
	var encoded string
	if json.Unmarshal(arguments, &encoded) == nil {
		arguments = []byte(encoded)
	}
	var input sessionPromptInput
	if json.Unmarshal(arguments, &input) != nil {
		return sessionPromptInput{}
	}
	return input
}

// sessionMessageOutput is the subset of the `compozy__session_prompt` result the card reads.
type sessionMessageOutput struct {
	TargetWorkspaceID string `json:"target_workspace_id"`
	MessageID         string `json:"message_id"`
	Mode              string `json:"mode"`
	Delivery          string `json:"delivery"`
	SteerDelivery     string `json:"steer_delivery"`
	ReplyWatch        *struct {
		ID string `json:"id"`
	} `json:"reply_watch"`
}

// sessionResultSearchDepth bounds how many provider wrappers the result search unwraps.
const sessionResultSearchDepth = 6

// sessionPromptResultOutput finds the tool's result envelope (`{"prompt": {...}}`) in a tool
// result however the provider wrapped it: as the object itself, as a JSON string holding it
// (Claude), or inside an MCP CallToolResult's structuredContent or text content (Codex).
func sessionPromptResultOutput(result *ToolResult) (sessionMessageOutput, bool) {
	if result == nil {
		return sessionMessageOutput{}, false
	}
	for _, raw := range [][]byte{result.RawOutput, []byte(result.Content)} {
		var value any
		if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
			continue
		}
		if output, ok := findSessionPromptOutput(value, sessionResultSearchDepth); ok {
			return output, true
		}
	}
	return sessionMessageOutput{}, false
}

func findSessionPromptOutput(value any, depth int) (sessionMessageOutput, bool) {
	if depth == 0 {
		return sessionMessageOutput{}, false
	}
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		var decoded any
		if !strings.HasPrefix(text, "{") || json.Unmarshal([]byte(text), &decoded) != nil {
			return sessionMessageOutput{}, false
		}
		return findSessionPromptOutput(decoded, depth-1)
	case []any:
		for _, item := range typed {
			if output, ok := findSessionPromptOutput(item, depth-1); ok {
				return output, true
			}
		}
	case map[string]any:
		if prompt, ok := typed["prompt"].(map[string]any); ok {
			return sessionMessageOutputFromObject(prompt)
		}
		if _, ok := typed["message_id"].(string); ok {
			return sessionMessageOutputFromObject(typed)
		}
		for _, key := range []string{"structuredContent", "result", "content", "text"} {
			if output, ok := findSessionPromptOutput(typed[key], depth-1); ok {
				return output, true
			}
		}
	}
	return sessionMessageOutput{}, false
}

func sessionMessageOutputFromObject(object map[string]any) (sessionMessageOutput, bool) {
	data, err := json.Marshal(object)
	if err != nil {
		return sessionMessageOutput{}, false
	}
	var output sessionMessageOutput
	if json.Unmarshal(data, &output) != nil || output.MessageID == "" {
		return sessionMessageOutput{}, false
	}
	return output, true
}
