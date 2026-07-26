package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	redactpkg "github.com/compozy/agh/internal/redact"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func structuredResult(value any, preview string) (toolspkg.ToolResult, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: marshal native tool result: %w", err)
	}
	result := toolspkg.ToolResult{
		Structured: data,
		Preview:    strings.TrimSpace(preview),
	}
	if result.Preview != "" {
		result.Content = []toolspkg.ToolContent{{Type: nativeToolsTextKey, Text: result.Preview}}
	}
	return result, nil
}

func structuredNetworkResult(value any, preview string) (toolspkg.ToolResult, error) {
	result, err := structuredResult(value, preview)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	redactedStructured := json.RawMessage(redactpkg.String(string(result.Structured)))
	if !json.Valid(redactedStructured) {
		return toolspkg.ToolResult{}, errors.New("daemon: redacted network tool result is invalid JSON")
	}
	result.Structured = redactedStructured
	result.Preview = strings.TrimSpace(redactpkg.String(result.Preview))
	for idx := range result.Content {
		result.Content[idx].Text = redactpkg.String(result.Content[idx].Text)
	}
	return result, nil
}
