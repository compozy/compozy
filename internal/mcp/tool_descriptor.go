package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/toolmeta"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpFriendlyVerbMetadataKey = "compozy/friendly_verb"
	mcpPreviewMetadataKey      = "compozy/preview"
)

func (e *CallExecutor) descriptorFromTool(
	source toolspkg.SourceRef,
	server compozyconfig.MCPServer,
	tool mcpsdk.Tool,
) (toolspkg.MCPToolDescriptor, error) {
	id, err := toolspkg.Canonicalize(server.Name, tool.Name)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, err
	}
	owner, err := mcpOwner(id)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, err
	}
	inputSchema, err := inputSchemaBytes(tool)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, err
	}
	outputSchema, err := outputSchemaBytes(tool)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, err
	}
	mcpSource := toolspkg.SourceRef{
		Kind:            toolspkg.SourceMCP,
		Owner:           owner,
		RawServerName:   strings.TrimSpace(server.Name),
		RawToolName:     strings.TrimSpace(tool.Name),
		ResourceID:      source.ResourceID,
		ResourceVersion: source.ResourceVersion,
		WorkspaceID:     source.WorkspaceID,
		Scope:           source.Scope,
	}
	readOnly := tool.Annotations != nil && tool.Annotations.ReadOnlyHint
	friendlyVerb, preview, err := mcpToolPresentationMetadata(tool)
	if err != nil {
		return toolspkg.MCPToolDescriptor{}, err
	}
	return toolspkg.MCPToolDescriptor{
		ID:           id,
		RawName:      strings.TrimSpace(tool.Name),
		Title:        mcpToolTitle(tool),
		FriendlyVerb: friendlyVerb,
		Preview:      preview,
		Description:  strings.TrimSpace(tool.Description),
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Source:       mcpSource,
		ReadOnly:     readOnly,
	}, nil
}

func mcpToolTitle(tool mcpsdk.Tool) string {
	if tool.Annotations == nil {
		return ""
	}
	return strings.TrimSpace(tool.Annotations.Title)
}

func mcpToolPresentationMetadata(tool mcpsdk.Tool) (string, string, error) {
	if tool.Meta == nil {
		return "", "", nil
	}
	friendlyVerb, err := mcpToolMetadataString(tool.Meta, mcpFriendlyVerbMetadataKey)
	if err != nil {
		return "", "", err
	}
	preview, err := mcpToolMetadataString(tool.Meta, mcpPreviewMetadataKey)
	if err != nil {
		return "", "", err
	}
	if err := toolmeta.ValidateDescriptorMetadata(friendlyVerb, preview); err != nil {
		return "", "", fmt.Errorf("mcp: invalid tool presentation metadata: %w", err)
	}
	return friendlyVerb, preview, nil
}

func mcpToolMetadataString(fields map[string]any, key string) (string, error) {
	raw, present := fields[key]
	if !present {
		return "", nil
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("mcp: tool presentation metadata %q must be a string", key)
	}
	return strings.TrimSpace(value), nil
}

func inputSchemaBytes(tool mcpsdk.Tool) (json.RawMessage, error) {
	if tool.InputSchema == nil {
		return nil, toolspkg.NewValidationError(
			"input_schema",
			toolspkg.ReasonSchemaInvalid,
			"mcp input schema is missing",
		)
	}
	data, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode input schema: %w", err)
	}
	return json.RawMessage(data), nil
}

func outputSchemaBytes(tool mcpsdk.Tool) (json.RawMessage, error) {
	if tool.OutputSchema == nil {
		return nil, nil
	}
	data, err := json.Marshal(tool.OutputSchema)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode output schema: %w", err)
	}
	return json.RawMessage(data), nil
}
