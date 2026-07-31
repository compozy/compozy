package mcp

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// hostedToolResult projects a Compozy tool result into MCP content.
func hostedToolResult(result tools.ToolResult) (*sdkmcp.CallToolResult, error) {
	isError, err := hostedToolResultIsError(result)
	if err != nil {
		return nil, err
	}
	if len(result.Structured) > 0 {
		var structured any
		if err := json.Unmarshal(result.Structured, &structured); err == nil {
			converted := &sdkmcp.CallToolResult{
				StructuredContent: structured,
				Content: []sdkmcp.Content{
					&sdkmcp.TextContent{Text: hostedResultFallback(result)},
				},
			}
			return finishHostedToolResult(converted, result, isError)
		}
	}
	if len(result.Content) == 0 {
		converted := &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: hostedResultFallback(result)},
			},
		}
		return finishHostedToolResult(converted, result, isError)
	}
	content := make([]sdkmcp.Content, 0, len(result.Content))
	for _, block := range result.Content {
		converted, err := hostedToolContent(block)
		if err != nil {
			return nil, err
		}
		if converted != nil {
			content = append(content, converted)
		}
	}
	if len(content) == 0 {
		converted := &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: hostedResultFallback(result)},
			},
		}
		return finishHostedToolResult(converted, result, isError)
	}
	return finishHostedToolResult(&sdkmcp.CallToolResult{Content: content}, result, isError)
}

func finishHostedToolResult(
	converted *sdkmcp.CallToolResult,
	result tools.ToolResult,
	isError bool,
) (*sdkmcp.CallToolResult, error) {
	if converted == nil {
		return nil, errors.New("mcp: hosted tool result is required")
	}
	converted.IsError = isError
	if len(result.Artifacts) == 0 {
		return converted, nil
	}
	payload := struct {
		Artifacts []tools.ArtifactRef `json:"artifacts"`
		ReadTool  tools.ToolID        `json:"read_tool"`
	}{
		Artifacts: append([]tools.ArtifactRef(nil), result.Artifacts...),
		ReadTool:  tools.ToolIDToolArtifactRead,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode hosted tool artifact references: %w", err)
	}
	converted.Content = append(converted.Content, &sdkmcp.TextContent{Text: string(encoded)})
	converted.Meta = sdkmcp.Meta{
		"compozy/artifacts": payload.Artifacts,
		"compozy/readTool":  payload.ReadTool,
	}
	return converted, nil
}

type hostedPartialResultError interface {
	error
	PartialToolResult() *tools.ToolResult
}

func hostedToolPartialErrorResult(err error) (*sdkmcp.CallToolResult, bool, error) {
	if err == nil {
		return nil, false, nil
	}
	var partial *tools.ToolResult
	if toolErr, ok := errors.AsType[*tools.ToolError](err); ok {
		partial = toolErr.PartialResult
	} else if carrier, ok := errors.AsType[hostedPartialResultError](err); ok {
		partial = carrier.PartialToolResult()
	}
	if partial == nil {
		return nil, false, nil
	}
	converted, convertErr := hostedToolResult(*partial)
	if convertErr != nil {
		return nil, true, convertErr
	}
	converted.IsError = true
	converted.Content = append(converted.Content, &sdkmcp.TextContent{Text: hostedToolErrorMessage(err)})
	return converted, true, nil
}

func hostedToolResultIsError(result tools.ToolResult) (bool, error) {
	raw, ok := result.Metadata[toolResultIsErrorKey]
	if !ok || len(raw) == 0 {
		return false, nil
	}
	var isError bool
	if err := json.Unmarshal(raw, &isError); err != nil {
		return false, fmt.Errorf("mcp: decode hosted MCP error flag: %w", err)
	}
	return isError, nil
}

func hostedToolContent(block tools.ToolContent) (sdkmcp.Content, error) {
	switch strings.TrimSpace(block.Type) {
	case hostedProxyTextKey:
		return &sdkmcp.TextContent{Text: block.Text}, nil
	case hostedProxyImageKey:
		data, err := hostedToolContentData(block, hostedProxyImageKey)
		if err != nil {
			return nil, err
		}
		return &sdkmcp.ImageContent{Data: data, MIMEType: block.MIMEType}, nil
	case hostedProxyAudioKey:
		data, err := hostedToolContentData(block, hostedProxyAudioKey)
		if err != nil {
			return nil, err
		}
		return &sdkmcp.AudioContent{Data: data, MIMEType: block.MIMEType}, nil
	default:
		if len(block.Data) > 0 {
			return &sdkmcp.TextContent{Text: string(block.Data)}, nil
		}
		if strings.TrimSpace(block.Text) != "" {
			return &sdkmcp.TextContent{Text: block.Text}, nil
		}
	}
	return nil, nil
}

func hostedToolContentData(block tools.ToolContent, contentType string) ([]byte, error) {
	var data string
	if err := json.Unmarshal(block.Data, &data); err != nil {
		return nil, fmt.Errorf("mcp: decode hosted MCP %s content: %w", contentType, err)
	}
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("mcp: decode hosted MCP %s base64 data: %w", contentType, err)
	}
	return decoded, nil
}

func hostedResultFallback(result tools.ToolResult) string {
	if preview := strings.TrimSpace(result.Preview); preview != "" {
		return preview
	}
	if len(result.Structured) > 0 {
		return string(result.Structured)
	}
	if len(result.Content) > 0 {
		parts := make([]string, 0, len(result.Content))
		for _, block := range result.Content {
			if text := strings.TrimSpace(block.Text); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return "{}"
}
