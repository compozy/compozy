package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/tmc/langchaingo/tools"
	"golang.org/x/sync/singleflight"
)

// ToolRegistry manages tool discovery, registration, and caching
type ToolRegistry interface {
	// Register registers a local tool
	Register(ctx context.Context, tool Tool) error
	// Find finds a tool by name, checking local tools first, then MCP tools
	Find(ctx context.Context, name string) (Tool, bool)
	// ListAll returns all available tools (local + MCP)
	ListAll(ctx context.Context) ([]Tool, error)
	// InvalidateCache clears the MCP tools cache
	InvalidateCache(ctx context.Context)
	// Close cleans up resources
	Close() error
}

// Tool represents a unified tool interface
type Tool interface {
	Name() string
	Description() string
	Call(ctx context.Context, input string) (string, error)
}

// ToolRegistryConfig configures the tool registry
type ToolRegistryConfig struct {
	ProxyClient *mcp.Client
	CacheTTL    time.Duration
}

// Implementation of ToolRegistry
type toolRegistry struct {
	config ToolRegistryConfig
	// Local tools - these take precedence over MCP tools
	localTools map[string]Tool
	localMu    sync.RWMutex
	// MCP tools cache
	mcpTools   []tools.Tool
	mcpCacheTs time.Time
	mcpMu      sync.RWMutex
	// Singleflight for cache refresh to prevent thundering herd
	sfGroup singleflight.Group
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry(config ToolRegistryConfig) ToolRegistry {
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}

	return &toolRegistry{
		config:     config,
		localTools: make(map[string]Tool),
	}
}

// Register registers a local tool with precedence over MCP tools
func (r *toolRegistry) Register(ctx context.Context, tool Tool) error {
	log := logger.FromContext(ctx)
	canonical := r.canonicalize(tool.Name())

	r.localMu.Lock()
	defer r.localMu.Unlock()

	r.localTools[canonical] = tool
	log.Debug("registered local tool", "name", canonical)

	return nil
}

// Find finds a tool by name, checking local tools first
func (r *toolRegistry) Find(ctx context.Context, name string) (Tool, bool) {
	log := logger.FromContext(ctx)
	canonical := r.canonicalize(name)

	// Check local tools first (they have precedence)
	r.localMu.RLock()
	if localTool, exists := r.localTools[canonical]; exists {
		r.localMu.RUnlock()
		return localTool, true
	}
	r.localMu.RUnlock()

	// Check MCP tools
	mcpTools, err := r.getMCPTools(ctx)
	if err != nil {
		log.Warn("failed to get MCP tools", "error", err)
		return nil, false
	}

	for _, mcpTool := range mcpTools {
		if r.canonicalize(mcpTool.Name()) == canonical {
			return &mcpToolAdapter{mcpTool}, true
		}
	}

	return nil, false
}

// ListAll returns all available tools
func (r *toolRegistry) ListAll(ctx context.Context) ([]Tool, error) {
	var allTools []Tool

	// Add local tools
	r.localMu.RLock()
	for _, tool := range r.localTools {
		allTools = append(allTools, tool)
	}
	r.localMu.RUnlock()

	// Add MCP tools (only if not overridden by local tools)
	mcpTools, err := r.getMCPTools(ctx)
	if err != nil {
		return allTools, core.NewError(err, "MCP_TOOLS_ERROR", map[string]any{
			"operation": "list_all_tools",
		})
	}

	for _, mcpTool := range mcpTools {
		canonical := r.canonicalize(mcpTool.Name())

		// Skip if overridden by local tool
		r.localMu.RLock()
		_, isOverridden := r.localTools[canonical]
		r.localMu.RUnlock()

		if !isOverridden {
			allTools = append(allTools, &mcpToolAdapter{mcpTool})
		}
	}

	return allTools, nil
}

// InvalidateCache clears the MCP tools cache
func (r *toolRegistry) InvalidateCache(ctx context.Context) {
	log := logger.FromContext(ctx)
	r.mcpMu.Lock()
	defer r.mcpMu.Unlock()

	r.mcpTools = nil
	r.mcpCacheTs = time.Time{}
	log.Debug("invalidated MCP tools cache")
}

// Close cleans up resources
func (r *toolRegistry) Close() error {
	// Currently no cleanup needed for local implementation
	return nil
}

// getMCPTools gets MCP tools with caching and singleflight
func (r *toolRegistry) getMCPTools(ctx context.Context) ([]tools.Tool, error) {
	r.mcpMu.RLock()
	if r.isCacheValid() {
		cached := append([]tools.Tool(nil), r.mcpTools...)
		r.mcpMu.RUnlock()
		return cached, nil
	}
	r.mcpMu.RUnlock()

	// Use singleflight to prevent multiple concurrent refreshes
	v, err, _ := r.sfGroup.Do("refresh-mcp-tools", func() (any, error) {
		return r.refreshMCPTools(ctx)
	})

	if err != nil {
		return nil, err
	}

	cachedTools, ok := v.([]tools.Tool)
	if !ok {
		return nil, fmt.Errorf("cached value is not []tools.Tool")
	}
	return cachedTools, nil
}

// refreshMCPTools refreshes the MCP tools cache
func (r *toolRegistry) refreshMCPTools(ctx context.Context) ([]tools.Tool, error) {
	log := logger.FromContext(ctx)
	if r.config.ProxyClient == nil {
		return []tools.Tool{}, nil
	}

	toolDefs, err := r.config.ProxyClient.ListTools(ctx)
	if err != nil {
		return nil, core.NewError(err, "MCP_PROXY_ERROR", map[string]any{
			"operation": "list_tools",
		})
	}

	// Convert ToolDefinitions to tools.Tool
	var mcpTools []tools.Tool
	for _, toolDef := range toolDefs {
		// Create a simple proxy tool adapter
		mcpTool := &mcpProxyTool{
			name:        toolDef.Name,
			description: toolDef.Description,
			client:      r.config.ProxyClient,
		}
		mcpTools = append(mcpTools, mcpTool)
	}

	r.mcpMu.Lock()
	r.mcpTools = mcpTools
	r.mcpCacheTs = time.Now()
	r.mcpMu.Unlock()
	log.Debug("refreshed MCP tools cache", "count", len(mcpTools))
	return mcpTools, nil
}

// isCacheValid checks if the MCP cache is still valid
func (r *toolRegistry) isCacheValid() bool {
	return !r.mcpCacheTs.IsZero() && time.Since(r.mcpCacheTs) < r.config.CacheTTL
}

// canonicalize normalizes tool names to prevent conflicts
func (r *toolRegistry) canonicalize(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// mcpToolAdapter adapts langchain tools.Tool to our Tool interface
type mcpToolAdapter struct {
	tool tools.Tool
}

func (a *mcpToolAdapter) Name() string {
	return a.tool.Name()
}

func (a *mcpToolAdapter) Description() string {
	return a.tool.Description()
}

func (a *mcpToolAdapter) Call(ctx context.Context, input string) (string, error) {
	return a.tool.Call(ctx, input)
}

// mcpProxyTool implements tools.Tool for MCP proxy tools
type mcpProxyTool struct {
	name        string
	description string
	client      *mcp.Client
}

func (m *mcpProxyTool) Name() string {
	return m.name
}

func (m *mcpProxyTool) Description() string {
	return m.description
}

func (m *mcpProxyTool) Call(_ context.Context, _ string) (string, error) {
	// TODO: Implement actual proxy call using m.client.ExecuteTool
	// For now, fail fast rather than returning misleading success
	return "", core.NewError(
		fmt.Errorf("MCP proxy tool execution not yet implemented"),
		"MCP_TOOL_UNIMPLEMENTED",
		map[string]any{
			"tool_name": m.name,
		},
	)
}

func (m *mcpProxyTool) ArgsType() any {
	return nil
}

// localToolAdapter adapts engine/tool.Config to our Tool interface
type localToolAdapter struct {
	config  *tool.Config
	runtime ToolRuntime
}

// ToolRuntime interface for executing local tools
type ToolRuntime interface {
	ExecuteTool(ctx context.Context, toolConfig *tool.Config, input map[string]any) (*core.Output, error)
}

func NewLocalToolAdapter(config *tool.Config, runtime ToolRuntime) Tool {
	return &localToolAdapter{
		config:  config,
		runtime: runtime,
	}
}

func (a *localToolAdapter) Name() string {
	return a.config.ID
}

func (a *localToolAdapter) Description() string {
	return a.config.Description
}

func (a *localToolAdapter) Call(ctx context.Context, input string) (string, error) {
	// Parse input as JSON
	var inputMap map[string]any
	if err := json.Unmarshal([]byte(input), &inputMap); err != nil {
		return "", core.NewError(err, "INVALID_TOOL_INPUT", map[string]any{
			"tool":  a.config.ID,
			"input": input,
		})
	}

	// Execute tool
	output, err := a.runtime.ExecuteTool(ctx, a.config, inputMap)
	if err != nil {
		return "", core.NewError(err, "TOOL_EXECUTION_ERROR", map[string]any{
			"tool": a.config.ID,
		})
	}
	if output == nil {
		return "", core.NewError(fmt.Errorf("nil output"), "TOOL_EMPTY_OUTPUT", map[string]any{
			"tool": a.config.ID,
		})
	}

	// Return as JSON string
	result, err := json.Marshal(*output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}
	return string(result), nil
}
