package aghsdk

import "encoding/json"

const (
	resultErrorKey = "error"
)

// EmptyResult returns an empty successful tool result.
func EmptyResult() ToolResult {
	return ToolResult{Truncated: false, Bytes: 0, DurationMS: 0}
}

// TextResult returns a text content result.
func TextResult(text string) ToolResult {
	return ToolResult{
		Content:    []ToolContent{{Type: "text", Text: text}},
		Preview:    text,
		Truncated:  false,
		Bytes:      int64(len(text)),
		DurationMS: 0,
	}
}

// StructuredResult marshals a structured payload into the result envelope.
func StructuredResult(value any) (ToolResult, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ToolResult{}, NewInvalidParamsError("structured result must be JSON serializable", map[string]any{
			resultErrorKey: err.Error(),
		})
	}
	return ToolResult{
		Structured: encoded,
		Truncated:  false,
		Bytes:      int64(len(encoded)),
		DurationMS: 0,
	}, nil
}
