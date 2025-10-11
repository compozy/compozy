package llmadapter

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// CloneMessages produces deep copies of the provided messages, including nested
// tool calls, results, and multi-modal content parts.
func CloneMessages(messages []Message) ([]Message, error) {
	if len(messages) == 0 {
		return nil, nil
	}
	out := make([]Message, len(messages))
	for i := range messages {
		clone, err := cloneMessage(&messages[i])
		if err != nil {
			return nil, err
		}
		out[i] = clone
	}
	return out, nil
}

// CloneContentParts deep copies a slice of content parts, preserving binary payloads.
func CloneContentParts(parts []ContentPart) ([]ContentPart, error) {
	return cloneContentParts(parts)
}

func cloneMessage(m *Message) (Message, error) {
	if m == nil {
		return Message{}, nil
	}
	clone := *m
	if len(m.Parts) > 0 {
		parts, err := cloneContentParts(m.Parts)
		if err != nil {
			return Message{}, err
		}
		clone.Parts = parts
	}
	if len(m.ToolCalls) > 0 {
		clone.ToolCalls = cloneToolCalls(m.ToolCalls)
	}
	if len(m.ToolResults) > 0 {
		clone.ToolResults = cloneToolResults(m.ToolResults)
	}
	return clone, nil
}

func cloneToolCalls(calls []ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]ToolCall, len(calls))
	for i := range calls {
		out[i] = cloneToolCall(&calls[i])
	}
	return out
}

func cloneToolCall(call *ToolCall) ToolCall {
	if call == nil {
		return ToolCall{}
	}
	clone := *call
	if len(call.Arguments) > 0 {
		clone.Arguments = append(json.RawMessage(nil), call.Arguments...)
	}
	return clone
}

func cloneToolResults(results []ToolResult) []ToolResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]ToolResult, len(results))
	for i := range results {
		out[i] = cloneToolResult(&results[i])
	}
	return out
}

func cloneToolResult(result *ToolResult) ToolResult {
	if result == nil {
		return ToolResult{}
	}
	clone := *result
	if len(result.JSONContent) > 0 {
		clone.JSONContent = append(json.RawMessage(nil), result.JSONContent...)
	}
	return clone
}

func cloneContentParts(parts []ContentPart) ([]ContentPart, error) {
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]ContentPart, len(parts))
	for i, part := range parts {
		switch p := part.(type) {
		case TextPart:
			out[i] = TextPart{Text: p.Text}
		case ImageURLPart:
			out[i] = ImageURLPart{
				URL:    p.URL,
				Detail: p.Detail,
			}
		case BinaryPart:
			data := make([]byte, len(p.Data))
			copy(data, p.Data)
			out[i] = BinaryPart{
				MIMEType: p.MIMEType,
				Data:     data,
			}
		default:
			copied, err := core.DeepCopy[any](part)
			if err != nil {
				return nil, fmt.Errorf("clone content part %T: %w", part, err)
			}
			cloned, ok := copied.(ContentPart)
			if !ok {
				return nil, fmt.Errorf("clone content part %T: unsupported type", part)
			}
			out[i] = cloned
		}
	}
	return out, nil
}
