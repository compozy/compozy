package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/redact"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxMCPToolListPages = 1_000
const maxDiscoveryDiagnosticExamples = 3

// ToolDiscovery is a successful listing, including any nonfatal discovery diagnostic.
type ToolDiscovery struct {
	Descriptors     []toolspkg.MCPToolDescriptor
	ToolCount       int
	ProtocolVersion string
	Diagnostic      string
}

func (d ToolDiscovery) clone() ToolDiscovery {
	d.Descriptors = cloneMCPToolDescriptors(d.Descriptors)
	return d
}

func (e *CallExecutor) descriptorsFromTools(
	source toolspkg.SourceRef,
	resolved *ResolvedServer,
	protocolVersion string,
	tools []*mcpsdk.Tool,
	snapshot *redact.SecretSnapshot,
) (ToolDiscovery, error) {
	server := resolved.Server
	descriptors := make([]toolspkg.MCPToolDescriptor, 0, len(tools))
	counts := make(map[toolspkg.ToolID]int, len(tools))
	var skipped int
	var examples []string
	for index, tool := range tools {
		if tool == nil {
			return ToolDiscovery{}, toolspkg.NewValidationError(
				"tools", toolspkg.ReasonSchemaInvalid, "mcp server returned an empty tool descriptor",
			)
		}
		var descriptor toolspkg.MCPToolDescriptor
		err := validateMCPTransportToolName(server, protocolVersion, tool.Name)
		if err == nil {
			descriptor, err = e.descriptorFromTool(source, server, *tool, snapshot)
		}
		if err != nil {
			validation, ok := errors.AsType[*toolspkg.ValidationError](err)
			if !ok || (validation.Field != "mcp.tool" && validation.Reason != toolspkg.ReasonIDTooLong) {
				return ToolDiscovery{}, fmt.Errorf("mcp: normalize tool %s: %w", mcpDiagnosticToolName(tool.Name), err)
			}
			skipped++
			if len(examples) < maxDiscoveryDiagnosticExamples {
				examples = append(examples, fmt.Sprintf(
					"Entry %d (%s): %s", index+1, mcpDiagnosticToolName(tool.Name), validation.Detail,
				))
			}
			continue
		}
		if descriptor.ReadOnlyOverride &&
			resolved.overrideOrigin() == toolspkg.MCPReadOnlyOverrideOperatorConfig {
			descriptor.ReadOnlyOverrideOrigin = resolved.overrideOrigin()
		}
		descriptors = append(descriptors, descriptor)
		counts[descriptor.ID]++
	}
	toolCount := 0
	// The registry needs ambiguous entries to quarantine their IDs across all visible sources.
	for _, descriptor := range descriptors {
		if counts[descriptor.ID] == 1 {
			toolCount++
			continue
		}
		skipped++
		if len(examples) < maxDiscoveryDiagnosticExamples {
			examples = append(examples, fmt.Sprintf(
				"Tool %s shares canonical ID %q with another entry",
				mcpDiagnosticToolName(descriptor.RawName), descriptor.ID,
			))
		}
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
	result := ToolDiscovery{Descriptors: descriptors, ToolCount: toolCount, ProtocolVersion: protocolVersion}
	if skipped > 0 {
		result.Diagnostic = redactMCPDiagnosticString(fmt.Sprintf(
			"Skipped %d MCP tool entries. %s.", skipped, strings.Join(examples, ". "),
		))
	}
	return result, nil
}

func validateMCPTransportToolName(server compozyconfig.MCPServer, protocolVersion, name string) error {
	if protocolVersion != protocolVersionModern || server.EffectiveTransport() != compozyconfig.MCPServerTransportHTTP {
		return nil
	}
	if name == strings.Trim(name, " \t") && !strings.ContainsFunc(name, invalidMCPNameHeaderRune) {
		return nil
	}
	return toolspkg.NewValidationError(
		"mcp.tool", toolspkg.ReasonIDInvalidFormat, "tool name cannot be sent unchanged in the Mcp-Name header",
	)
}

func invalidMCPNameHeaderRune(r rune) bool {
	return (r < ' ' && r != '\t') || r == 0x7f
}

func mcpDiagnosticToolName(name string) string {
	return fmt.Sprintf("%.128q", redactMCPDiagnosticString(name))
}

type toolListTTLKey struct{}

// executorToolListCache delegates list freshness to the executor so explicit checks always send an RPC.
func executorToolListCache(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		result, err := next(ctx, method, req)
		if err != nil || method != mcpToolsListMethod {
			return result, err
		}
		list, ok := result.(*mcpsdk.ListToolsResult)
		if !ok || list == nil || list.TTLMs <= 0 {
			return result, nil
		}
		if ttl, ok := ctx.Value(toolListTTLKey{}).(*int); ok && (*ttl == 0 || list.TTLMs < *ttl) {
			*ttl = list.TTLMs
		}
		uncached := *list
		uncached.TTLMs = 0
		return &uncached, nil
	}
}

func listMCPTools(ctx context.Context, client *mcpsdk.ClientSession) ([]*mcpsdk.Tool, int, error) {
	tools := make([]*mcpsdk.Tool, 0)
	cursor := ""
	seen := map[string]struct{}{}
	ttlMs := 0
	ctx = context.WithValue(ctx, toolListTTLKey{}, &ttlMs)
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
