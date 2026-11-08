package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxMCPToolListPages = 1_000

func (e *CallExecutor) descriptorsFromTools(
	source toolspkg.SourceRef,
	resolved ResolvedServer,
	tools []*mcpsdk.Tool,
) ([]toolspkg.MCPToolDescriptor, error) {
	server := resolved.Server
	descriptors := make([]toolspkg.MCPToolDescriptor, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return nil, toolspkg.NewValidationError(
				"tools", toolspkg.ReasonSchemaInvalid, "mcp server returned an empty tool descriptor",
			)
		}
		descriptor, err := e.descriptorFromTool(source, server, *tool)
		if err != nil {
			return nil, fmt.Errorf("mcp: normalize tool %q: %w", tool.Name, err)
		}
		if descriptor.ReadOnlyOverride &&
			resolved.overrideOrigin() == toolspkg.MCPReadOnlyOverrideOperatorConfig {
			descriptor.ReadOnlyOverrideOrigin = resolved.overrideOrigin()
		}
		descriptors = append(descriptors, descriptor)
	}
	for _, name := range server.ReadOnlyTools {
		found := false
		for _, tool := range tools {
			if tool != nil && tool.Name == name {
				found = true
				break
			}
		}
		if !found {
			slog.Warn("mcp_read_only_override_unknown_tool", "server", server.Name, "tool", name)
		}
	}
	return descriptors, nil
}

func listMCPTools(ctx context.Context, client *mcpsdk.ClientSession) ([]*mcpsdk.Tool, int, error) {
	tools := make([]*mcpsdk.Tool, 0)
	cursor := ""
	seen := map[string]struct{}{}
	ttlMs := 0
	for range maxMCPToolListPages {
		result, err := client.ListTools(ctx, &mcpsdk.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, 0, err
		}
		if result == nil {
			return nil, 0, errors.New("mcp: tools/list returned an empty result")
		}
		tools = append(tools, result.Tools...)
		if ttl := result.GetTTLMs(); ttl > 0 && (ttlMs == 0 || ttl < ttlMs) {
			ttlMs = ttl
		}
		next := strings.TrimSpace(result.NextCursor)
		if next == "" {
			return tools, ttlMs, nil
		}
		if _, duplicate := seen[next]; duplicate {
			return nil, 0, errors.New("mcp: tools/list returned a repeated cursor")
		}
		seen[next] = struct{}{}
		cursor = next
	}
	return nil, 0, errors.New("mcp: tools/list exceeded maximum page count")
}
