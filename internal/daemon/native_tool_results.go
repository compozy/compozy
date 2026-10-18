package daemon

import (
	"encoding/json"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
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

func untrustedTerminalResult(value any, preview string) (toolspkg.ToolResult, error) {
	result, err := structuredResult(value, preview)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	result.Trust = toolspkg.ResultTrustUntrustedModel
	return result, nil
}
