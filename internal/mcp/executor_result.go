package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func validateMCPToolResultCapabilities(id toolspkg.ToolID, result *mcpsdk.CallToolResult) error {
	if result == nil || !result.NeedsInput() {
		return nil
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		id,
		"mcp server requires an unsupported client capability",
		&UnsupportedCapabilityError{Capability: "input_requests"},
		toolspkg.ReasonMCPUnreachable,
	)
}

func toolResultFromMCP(result *mcpsdk.CallToolResult) (toolspkg.ToolResult, error) {
	if result == nil {
		return toolspkg.ToolResult{}, nil
	}
	content := make([]toolspkg.ToolContent, 0, len(result.Content))
	var redactions []toolspkg.Redaction
	for i := range result.Content {
		converted, wasRedacted, err := toolContentFromMCPWithRedaction(result.Content[i])
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		content = append(content, converted)
		if wasRedacted {
			redactions = append(redactions, toolspkg.Redaction{
				Path:   fmt.Sprintf("$.content[%d].data", i),
				Reason: toolspkg.ReasonSecretMetadata,
				Bytes:  int64(len(converted.Data)),
			})
		}
	}
	var structured json.RawMessage
	if result.StructuredContent != nil {
		data, wasRedacted, err := encodeMCPResultJSON(result.StructuredContent, "structured content")
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		structured = data
		if wasRedacted {
			redactions = append(redactions, toolspkg.Redaction{
				Path:   "$.structured",
				Reason: toolspkg.ReasonSecretMetadata,
				Bytes:  int64(len(data)),
			})
		}
	}
	metadata := map[string]json.RawMessage{}
	if result.IsError {
		metadata[toolResultIsErrorKey] = json.RawMessage(`true`)
	}
	if len(metadata) == 0 {
		metadata = nil
	}
	return toolspkg.ToolResult{
		Content:    content,
		Structured: structured,
		Preview:    mcpPreview(content, result.IsError),
		Metadata:   metadata,
		Redactions: redactions,
	}, nil
}

func toolContentFromMCP(content mcpsdk.Content) (toolspkg.ToolContent, error) {
	converted, _, err := toolContentFromMCPWithRedaction(content)
	return converted, err
}

func toolContentFromMCPWithRedaction(content mcpsdk.Content) (toolspkg.ToolContent, bool, error) {
	switch typed := content.(type) {
	case *mcpsdk.TextContent:
		if typed == nil {
			return toolspkg.ToolContent{}, false, errors.New("mcp: text content is nil")
		}
		return toolspkg.ToolContent{Type: hostedProxyTextKey, Text: typed.Text}, false, nil
	case *mcpsdk.ImageContent:
		if typed == nil {
			return toolspkg.ToolContent{}, false, errors.New("mcp: image content is nil")
		}
		data, wasRedacted, err := encodeMCPBinaryData(typed.Data, "image content")
		if err != nil {
			return toolspkg.ToolContent{}, false, err
		}
		return toolspkg.ToolContent{
			Type:     hostedProxyImageKey,
			Data:     data,
			MIMEType: typed.MIMEType,
		}, wasRedacted, nil
	case *mcpsdk.AudioContent:
		if typed == nil {
			return toolspkg.ToolContent{}, false, errors.New("mcp: audio content is nil")
		}
		data, wasRedacted, err := encodeMCPBinaryData(typed.Data, "audio content")
		if err != nil {
			return toolspkg.ToolContent{}, false, err
		}
		return toolspkg.ToolContent{
			Type:     hostedProxyAudioKey,
			Data:     data,
			MIMEType: typed.MIMEType,
		}, wasRedacted, nil
	default:
		redactedContent, binaryRedaction, err := redactMCPContentBytes(content)
		if err != nil {
			return toolspkg.ToolContent{}, false, err
		}
		data, encodedRedaction, err := encodeMCPResultJSON(redactedContent, "content block")
		if err != nil {
			return toolspkg.ToolContent{}, false, err
		}
		return toolspkg.ToolContent{
			Type: executorMCPKey,
			Data: data,
		}, binaryRedaction || encodedRedaction, nil
	}
}

func mcpPreview(content []toolspkg.ToolContent, isError bool) string {
	prefix := ""
	if isError {
		prefix = "error: "
	}
	for _, item := range content {
		if strings.TrimSpace(item.Text) != "" {
			return prefix + strings.TrimSpace(item.Text)
		}
	}
	if len(content) == 0 {
		return prefix + "empty MCP result"
	}
	return fmt.Sprintf("%s%d MCP content blocks", prefix, len(content))
}
