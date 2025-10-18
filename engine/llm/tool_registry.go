package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
	factorymetrics "github.com/compozy/compozy/engine/llm/factory/metrics"
	"github.com/compozy/compozy/engine/mcp"
	mcpmetrics "github.com/compozy/compozy/engine/mcp/metrics"
	"github.com/compozy/compozy/engine/schema"
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
	ParameterSchema() map[string]any
}

// ToolRegistryConfig configures the tool registry
type ToolRegistryConfig struct {
	ProxyClient *mcp.Client
	CacheTTL    time.Duration
	// EmptyCacheTTL controls how long an empty MCP tools state is cached
	// to avoid repeated proxy hits when no tools are available yet.
	// Defaults to 30s when zero.
	EmptyCacheTTL time.Duration
	// AllowedMCPNames restricts MCP tool advertisement/lookup to these MCP IDs.
	// When empty, all discovered MCP tools are eligible. Local tools are never filtered.
	AllowedMCPNames []string
	// DeniedMCPNames excludes MCP tool advertisement/lookup for these MCP IDs.
	// Deny list always takes precedence over the allowlist.
	DeniedMCPNames []string
}

// Implementation of ToolRegistry
type toolRegistry struct {
	config ToolRegistryConfig
	// Local tools - these take precedence over MCP tools
	localTools map[string]Tool
	localMu    sync.RWMutex
	// MCP tools cache
	mcpTools       []tools.Tool
	mcpToolIndex   map[string]tools.Tool
	mcpTotalCount  int
	mcpCacheTs     time.Time
	mcpCachedEmpty bool
	mcpMu          sync.RWMutex
	// Singleflight for cache refresh to prevent thundering herd
	sfGroup singleflight.Group
	// Fast membership check for allowed MCP names
	allowedMCPSet map[string]struct{}
	deniedMCPSet  map[string]struct{}
	now           func() time.Time
}

// mcpNamed is implemented by MCP-backed tools to expose their MCP server ID
type mcpNamed interface{ MCPName() string }

// NewToolRegistry creates a new tool registry bound to the provided context.
func NewToolRegistry(ctx context.Context, config ToolRegistryConfig) ToolRegistry {
	if ctx == nil {
		panic("context must not be nil")
	}
	start := time.Now()
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.EmptyCacheTTL == 0 {
		config.EmptyCacheTTL = 30 * time.Second
	}

	registry := &toolRegistry{
		config:        config,
		localTools:    make(map[string]Tool),
		allowedMCPSet: buildAllowedMCPSet(config.AllowedMCPNames),
		deniedMCPSet:  buildDeniedMCPSet(config.DeniedMCPNames),
		now:           time.Now,
	}
	factorymetrics.RecordCreate(ctx, factorymetrics.TypeTool, "registry", time.Since(start))
	return registry
}

func copySchemaMap(s *schema.Schema) map[string]any {
	if s == nil {
		return nil
	}
	cloned, err := core.DeepCopy(map[string]any(*s))
	if err != nil {
		// fall back to a shallow copy to avoid mutating original schema
		return core.CloneMap(*s)
	}
	return cloned
}

// buildAllowedMCPSet normalizes and constructs a fast lookup set for MCP IDs
func buildAllowedMCPSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		nn := strings.ToLower(strings.TrimSpace(n))
		if nn != "" {
			set[nn] = struct{}{}
		}
	}
	return set
}

// buildDeniedMCPSet normalizes and constructs a fast lookup set for MCP IDs
func buildDeniedMCPSet(names []string) map[string]struct{} {
	set := make(map[string]struct{}, len(names))
	for _, n := range names {
		nn := strings.ToLower(strings.TrimSpace(n))
		if nn != "" {
			set[nn] = struct{}{}
		}
	}
	return set
}

// mcpToolAllowed returns true when the given MCP tool is permitted by the allowlist
func (r *toolRegistry) mcpToolAllowed(t tools.Tool) bool {
	if len(r.allowedMCPSet) == 0 {
		return !r.mcpToolDenied(t)
	}
	if named, ok := any(t).(mcpNamed); ok {
		_, allowed := r.allowedMCPSet[r.canonicalize(named.MCPName())]
		if !allowed {
			return false
		}
		return !r.mcpToolDenied(t)
	}
	// Unknown tool type with allowlist active -> deny
	return false
}

// mcpToolDenied returns true when the given MCP tool is blocked by the deny list.
func (r *toolRegistry) mcpToolDenied(t tools.Tool) bool {
	if len(r.deniedMCPSet) == 0 {
		return false
	}
	if named, ok := any(t).(mcpNamed); ok {
		_, denied := r.deniedMCPSet[r.canonicalize(named.MCPName())]
		return denied
	}
	// If deny list exists but MCP name is unavailable, default to allowing.
	return false
}

// Register registers a local tool with precedence over MCP tools
func (r *toolRegistry) Register(ctx context.Context, tool Tool) error {
	log := logger.FromContext(ctx)
	canonical := r.canonicalize(tool.Name())

	r.localMu.Lock()
	defer r.localMu.Unlock()

	r.localTools[canonical] = tool
	log.Debug("Registered local tool", "name", canonical)

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
	_, stale, err := r.getMCPTools(ctx)
	if err != nil {
		log.Warn("Failed to get MCP tools", "error", err)
		return nil, false
	}

	lookupStart := time.Now()
	if tool, ok := r.findMCPTool(canonical); ok {
		mcpmetrics.RecordRegistryLookup(ctx, time.Since(lookupStart), true)
		if stale {
			r.triggerBackgroundRefresh(ctx)
		}
		return tool, true
	}
	mcpmetrics.RecordRegistryLookup(ctx, time.Since(lookupStart), false)

	if stale {
		_, refreshErr := r.fetchFreshMCPTools(ctx)
		if refreshErr != nil {
			log.Warn("Failed to refresh MCP tools after stale cache miss", "error", refreshErr)
			return nil, false
		}
		refreshStart := time.Now()
		if tool, ok := r.findMCPTool(canonical); ok {
			mcpmetrics.RecordRegistryLookup(ctx, time.Since(refreshStart), true)
			return tool, true
		}
		mcpmetrics.RecordRegistryLookup(ctx, time.Since(refreshStart), false)
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
	mcpTools, stale, err := r.getMCPTools(ctx)
	if err != nil {
		return allTools, core.NewError(err, "MCP_TOOLS_ERROR", map[string]any{
			"operation": "list_all_tools",
		})
	}
	if stale {
		r.triggerBackgroundRefresh(ctx)
	}

	r.mcpMu.RLock()
	totalCount := r.mcpTotalCount
	r.mcpMu.RUnlock()

	allowedCount := 0
	filteredCount := totalCount - len(mcpTools)
	if filteredCount < 0 {
		filteredCount = 0
	}
	for _, mcpTool := range mcpTools {
		canonical := r.canonicalize(mcpTool.Name())

		// Skip if overridden by local tool
		r.localMu.RLock()
		_, isOverridden := r.localTools[canonical]
		r.localMu.RUnlock()

		if isOverridden {
			continue
		}

		allowedCount++

		allTools = append(allTools, &mcpToolAdapter{mcpTool})
	}
	logger.FromContext(ctx).Debug("MCP allowlist filtering",
		"total", totalCount,
		"cached_allowed", len(mcpTools),
		"allowed", allowedCount,
		"filtered", filteredCount,
		"allowlist_size", len(r.allowedMCPSet),
		"allowlist_ids", r.allowlistIDs(),
		"denylist_size", len(r.deniedMCPSet),
		"denylist_ids", r.denylistIDs(),
	)

	return allTools, nil
}

// InvalidateCache clears the MCP tools cache
func (r *toolRegistry) InvalidateCache(ctx context.Context) {
	log := logger.FromContext(ctx)
	r.mcpMu.Lock()
	defer r.mcpMu.Unlock()

	r.mcpTools = nil
	r.mcpToolIndex = nil
	r.mcpCacheTs = time.Time{}
	r.mcpCachedEmpty = false
	log.Debug("Invalidated MCP tools cache")
}

// Close cleans up resources
func (r *toolRegistry) Close() error {
	// Currently no cleanup needed for local implementation
	return nil
}

// getMCPTools returns the cached MCP tools and whether the cache is stale.
func (r *toolRegistry) getMCPTools(ctx context.Context) ([]tools.Tool, bool, error) {
	now := r.now()

	r.mcpMu.RLock()
	hasCache := !r.mcpCacheTs.IsZero()
	snapshot := append([]tools.Tool(nil), r.mcpTools...)
	cachedEmpty := r.mcpCachedEmpty
	var ttl time.Duration
	if cachedEmpty && r.config.EmptyCacheTTL > 0 {
		ttl = r.config.EmptyCacheTTL
	} else {
		ttl = r.config.CacheTTL
	}
	stale := hasCache && now.Sub(r.mcpCacheTs) >= ttl
	r.mcpMu.RUnlock()

	if hasCache {
		return snapshot, stale, nil
	}

	fresh, err := r.fetchFreshMCPTools(ctx)
	return fresh, false, err
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

	// Convert ToolDefinitions to tools.Tool and build canonical lookup index.
	var (
		filteredTools []tools.Tool
		index         = make(map[string]tools.Tool)
		total         int
	)
	for _, toolDef := range toolDefs {
		proxyTool := NewProxyTool(toolDef, r.config.ProxyClient)
		total++

		canonical := r.canonicalize(proxyTool.Name())
		if canonical == "" {
			continue
		}
		if _, exists := index[canonical]; exists {
			continue
		}
		if !r.mcpToolAllowed(proxyTool) {
			continue
		}
		index[canonical] = proxyTool
		filteredTools = append(filteredTools, proxyTool)
	}

	// Cache results, including empty, to avoid repeated proxy hits.
	r.mcpMu.Lock()
	r.mcpTools = filteredTools
	r.mcpToolIndex = index
	r.mcpTotalCount = total
	r.mcpCacheTs = r.now()
	r.mcpCachedEmpty = len(filteredTools) == 0
	r.mcpMu.Unlock()
	if r.mcpCachedEmpty {
		log.Debug("Refreshed MCP tools cache (empty)")
	} else {
		log.Debug("Refreshed MCP tools cache", "count", len(filteredTools), "filtered_total", total-len(filteredTools))
	}
	return filteredTools, nil
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

func (a *mcpToolAdapter) ParameterSchema() map[string]any {
	type inputSchemaProvider interface {
		InputSchema() *schema.Schema
	}
	if sp, ok := any(a.tool).(inputSchemaProvider); ok {
		if s := sp.InputSchema(); s != nil {
			return copySchemaMap(s)
		}
	}
	type argsTyper interface {
		ArgsType() any
	}
	if at, ok := any(a.tool).(argsTyper); ok {
		if v, isMap := at.ArgsType().(map[string]any); isMap {
			if len(v) == 0 {
				return nil
			}
			copied, err := core.DeepCopy(v)
			if err != nil {
				return core.CloneMap(v)
			}
			return copied
		}
	}
	return nil
}

// ArgsType forwards the argument schema when the underlying MCP tool exposes it.
// This allows the orchestrator to advertise proper JSON Schema to the LLM so it
// can provide required arguments instead of calling tools with empty payloads.
func (a *mcpToolAdapter) ArgsType() any {
	type argsTyper interface{ ArgsType() any }
	if at, ok := any(a.tool).(argsTyper); ok {
		return at.ArgsType()
	}
	return nil
}

// MCPName forwards the MCP server identifier when the underlying tool exposes it.
// This preserves allowlist filtering behavior in registries that restrict tools
// by MCP ID.
func (a *mcpToolAdapter) MCPName() string {
	if mn, ok := any(a.tool).(mcpNamed); ok {
		return mn.MCPName()
	}
	return ""
}

// allowlistIDs returns configured allowlist MCP IDs for debug logging
func (r *toolRegistry) allowlistIDs() []string {
	if len(r.allowedMCPSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(r.allowedMCPSet))
	for id := range r.allowedMCPSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// denylistIDs returns configured denylist MCP IDs for debug logging
func (r *toolRegistry) denylistIDs() []string {
	if len(r.deniedMCPSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(r.deniedMCPSet))
	for id := range r.deniedMCPSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// mcpProxyTool implements tools.Tool for MCP proxy tools
// legacy mcpProxyTool removed; ProxyTool is canonical

// Legacy mcpProxyTool removed; ProxyTool is the canonical MCP tool implementation

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

func (a *localToolAdapter) ParameterSchema() map[string]any {
	if a.config == nil || a.config.InputSchema == nil {
		return nil
	}
	source := map[string]any(*a.config.InputSchema)
	copied, err := core.DeepCopy(source)
	if err != nil {
		return core.CloneMap(source)
	}
	return copied
}

func (a *localToolAdapter) Call(ctx context.Context, input string) (string, error) {
	// Parse input as JSON
	var inputMap map[string]any
	if err := json.Unmarshal([]byte(input), &inputMap); err != nil {
		return "", core.NewError(err, "INVALID_TOOL_INPUT", map[string]any{
			"tool": a.config.ID,
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

// fetchFreshMCPTools retrieves the latest MCP tool list using singleflight.
func (r *toolRegistry) fetchFreshMCPTools(ctx context.Context) ([]tools.Tool, error) {
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

// triggerBackgroundRefresh schedules a cache refresh without blocking the caller.
func (r *toolRegistry) triggerBackgroundRefresh(ctx context.Context) {
	if r.config.ProxyClient == nil {
		return
	}
	bgCtx := context.WithoutCancel(ctx)
	log := logger.FromContext(bgCtx)
	ch := r.sfGroup.DoChan("refresh-mcp-tools", func() (any, error) {
		return r.refreshMCPTools(bgCtx)
	})
	go func() {
		res := <-ch
		if res.Err != nil {
			log.Warn("Asynchronous MCP tools refresh failed", "error", res.Err)
			return
		}
		if tools, ok := res.Val.([]tools.Tool); ok {
			log.Debug("Asynchronous MCP tools refresh completed", "count", len(tools))
		}
	}()
}

// findMCPTool performs an indexed lookup for the canonical tool name.
func (r *toolRegistry) findMCPTool(canonical string) (Tool, bool) {
	r.mcpMu.RLock()
	mcpTool, ok := r.mcpToolIndex[canonical]
	r.mcpMu.RUnlock()
	if !ok {
		return nil, false
	}
	return &mcpToolAdapter{mcpTool}, true
}
