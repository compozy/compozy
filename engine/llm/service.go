package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

type Service struct {
	runtime     *runtime.Manager
	agent       *agent.Config
	action      *agent.ActionConfig
	mcps        []mcp.Config
	mcpTools    []tools.Tool
	connections map[string]*mcp.Connection

	// Dynamic tool management
	proxyClient  *mcp.Client
	toolsCache   []tools.Tool
	toolsCacheTs time.Time
	cacheTTL     time.Duration
	toolsMu      sync.RWMutex // Protects toolsCache and toolsCacheTs
}

// NewService creates a new service with MCP configuration
func NewService(
	runtime *runtime.Manager,
	agent *agent.Config,
	action *agent.ActionConfig,
	mcps []mcp.Config,
) *Service {
	service := &Service{
		runtime:  runtime,
		agent:    agent,
		action:   action,
		mcps:     mcps,
		cacheTTL: 5 * time.Minute, // Cache tools for 5 minutes
	}

	// Initialize proxy client if proxy is configured
	service.initProxyClient()

	return service
}

func (s *Service) CreateLLM() (llms.Model, error) {
	return s.agent.Config.CreateLLM(nil)
}

func (s *Service) GenerateContent(ctx context.Context) (*core.Output, error) {
	model, err := s.CreateLLM()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %w", err)
	}
	if err := s.initMCP(); err != nil {
		return nil, fmt.Errorf("failed to initialize MCP: %w", err)
	}
	// Validate input parameters if schema is defined
	if err := s.validateInput(ctx); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// Create message content using modern API
	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, s.agent.Instructions),
		llms.TextParts(llms.ChatMessageTypeSystem, s.enhancePromptForStructuredOutput()),
	}

	// Configure call options based on available tools and schemas
	callOptions := s.buildCallOptions()
	result, err := model.GenerateContent(ctx, messages, callOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute agent: %w", err)
	}
	// Process and validate the result
	output, err := s.processLLMResult(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("failed to process LLM result: %w", err)
	}
	return output, nil
}

// validateInput validates input parameters against the action's input schema
func (s *Service) validateInput(ctx context.Context) error {
	if s.action.InputSchema == nil {
		return nil
	}
	return s.action.ValidateInput(ctx, s.action.GetInput())
}

func (s *Service) initMCP() error {
	agentMCPs := make([]mcp.Config, 0, len(s.agent.MCPs))
	// Prefer agent MCPs over workflow MCPs
	if len(s.agent.MCPs) > 0 {
		agentMCPs = append(agentMCPs, s.agent.MCPs...)
	}
	// Add workflow MCPs if no agent MCPs are defined
	if len(agentMCPs) == 0 && len(s.mcps) > 0 {
		agentMCPs = append(agentMCPs, s.mcps...)
	}

	// Skip MCP initialization if no configurations exist
	if len(agentMCPs) == 0 {
		s.connections = make(map[string]*mcp.Connection)
		return nil
	}

	// Check if any MCP uses proxy mode
	useProxy := s.shouldUseProxy(agentMCPs)

	if useProxy {
		// Initialize tools via proxy discovery
		return s.initMCPViaProxy(agentMCPs)
	}

	// Use direct connections (existing behavior)
	bgCtx := context.Background()
	connections, err := mcp.InitConnections(bgCtx, agentMCPs)
	if err != nil {
		return fmt.Errorf("failed to create MCP connections: %w", err)
	}
	s.connections = connections

	// Log available tools for debugging
	for connID, connection := range connections {
		tools := connection.GetTools()
		logger.Debug("MCP connection tools available", "connection_id", connID, "tool_count", len(tools))
		for toolName := range tools {
			logger.Debug("  Tool available", "connection_id", connID, "tool_name", toolName)
		}
	}

	return nil
}

// getLLMCallTools returns properly configured tool definitions including MCP tools
func (s *Service) getLLMCallTools() []llms.Tool {
	var tools []llms.Tool

	// Add tools from direct connections (existing behavior)
	if len(s.connections) > 0 {
		for _, connection := range s.connections {
			for _, tool := range connection.GetTools() {
				tools = append(tools, connection.ConvertoToLLMTool(tool))
				s.mcpTools = append(s.mcpTools, tool)
			}
		}
	}

	// Add tools from proxy if available
	if s.proxyClient != nil {
		bgCtx := context.Background()
		proxyTools := s.getToolsFromProxy(bgCtx)
		for _, proxyTool := range proxyTools {
			// Convert proxy tool to LLM tool format
			// For proxy tools, we need to cast to our custom type to get schema
			var parameters map[string]any
			if pTool, ok := proxyTool.(*ProxyTool); ok {
				parameters = map[string]any{
					"type":       "object",
					"properties": pTool.inputSchema,
				}
			} else {
				parameters = map[string]any{
					"type": "object",
				}
			}

			llmTool := llms.Tool{
				Type: "function",
				Function: &llms.FunctionDefinition{
					Name:        proxyTool.Name(),
					Description: proxyTool.Description(),
					Parameters:  parameters,
				},
			}
			tools = append(tools, llmTool)
			s.mcpTools = append(s.mcpTools, proxyTool)
		}
	}

	// Add agent-specific tools
	for _, toolConfig := range s.agent.Tools {
		tools = append(tools, toolConfig.GetLLMDefinition())
	}

	return tools
}

// buildCallOptions constructs the appropriate call options based on tools and schemas
func (s *Service) buildCallOptions() []llms.CallOption {
	var options []llms.CallOption
	// Add tools if available
	tools := s.getLLMCallTools()
	if len(tools) > 0 {
		options = append(options, llms.WithTools(tools), llms.WithToolChoice("auto"))
		return options
	}
	// Enable structured output if supported and schema is defined (only when no tools)
	if s.shouldUseStructuredOutput() {
		options = append(options, llms.WithJSONMode())
	}
	return options
}

// shouldUseStructuredOutput determines if structured output should be enabled
func (s *Service) shouldUseStructuredOutput() bool {
	if !s.supportsStructuredOutput() {
		return false
	}
	if s.action.ShouldUseJSONOutput() {
		return true
	}
	tools := make([]tool.Config, 0, len(s.agent.Tools))
	tools = append(tools, s.agent.Tools...)
	for _, t := range tools {
		if t.HasSchema() {
			return true
		}
	}
	return false
}

// processLLMResult processes the LLM response and validates output against schema
func (s *Service) processLLMResult(ctx context.Context, result *llms.ContentResponse) (*core.Output, error) {
	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in LLM response")
	}
	choice := result.Choices[0]
	if len(choice.ToolCalls) > 0 {
		return s.executeToolCall(ctx, choice.ToolCalls[0])
	}
	output, err := s.parseContent(choice.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content: %w", err)
	}
	if err := s.validateOutput(ctx, &output); err != nil {
		return nil, fmt.Errorf("output validation failed: %w", err)
	}
	return &output, nil
}

// executeToolCall executes a tool call and validates the result
func (s *Service) executeToolCall(ctx context.Context, toolCall llms.ToolCall) (*core.Output, error) {
	if toolCall.FunctionCall == nil {
		return nil, fmt.Errorf("tool call missing function call")
	}
	toolName := toolCall.FunctionCall.Name
	arguments := toolCall.FunctionCall.Arguments
	// Try to find and execute an agent tool first
	agentTool := s.findTool(toolName)
	if agentTool != nil {
		input, err := s.parseArgs(arguments)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}
		tool := NewTool(agentTool, s.agent.Env, s.runtime)
		output, err := tool.Call(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}
		return output, nil
	}
	// Try to execute an MCP tool
	mcpTool := s.findMCPTool(toolName)
	if mcpTool != nil {
		result, err := s.executeMCPTool(ctx, mcpTool, arguments)
		if err != nil {
			return nil, fmt.Errorf("tool execution failed: %w", err)
		}
		if strings.Contains(strings.ToLower(result), "error") {
			return nil, fmt.Errorf("tool execution failed: %s", result)
		}
		// Try to parse as JSON first, fall back to text response
		var output core.Output
		if err := json.Unmarshal([]byte(result), &output); err != nil {
			logger.Error("failed to unmarshal tool result", "error", err, "result", result)
			// If JSON parsing fails, wrap the result as a text response
			output = core.Output{"response": result}
		}
		return &output, nil
	}
	return nil, fmt.Errorf("tool not found: %s", toolName)
}

func (s *Service) parseArgs(arguments string) (*core.Input, error) {
	var input core.Input
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return nil, fmt.Errorf("invalid tool arguments JSON: %w", err)
	}
	return &input, nil
}

// parseContent attempts to parse content as structured JSON or returns as text
func (s *Service) parseContent(content string) (core.Output, error) {
	// If action has output schema, expect structured content
	if s.action.ShouldUseJSONOutput() {
		var structuredOutput map[string]any
		if err := json.Unmarshal([]byte(content), &structuredOutput); err != nil {
			// Try to clean up common JSON syntax issues and retry
			cleanedContent := s.cleanupJSON(content)
			if err := json.Unmarshal([]byte(cleanedContent), &structuredOutput); err != nil {
				return nil, fmt.Errorf(
					"expected structured output but received invalid JSON: %w. Content: %s",
					err,
					content,
				)
			}
		}
		return core.Output(structuredOutput), nil
	}
	// Try to parse as JSON, but fall back to text if it fails
	var jsonOutput map[string]any
	if err := json.Unmarshal([]byte(content), &jsonOutput); err == nil {
		return core.Output(jsonOutput), nil
	}
	// Return as text response
	return core.Output{"response": content}, nil
}

// validateOutput validates the output against the action's output schema
func (s *Service) validateOutput(ctx context.Context, output *core.Output) error {
	if s.action.OutputSchema == nil {
		return nil
	}
	return s.action.ValidateOutput(ctx, output)
}

// findTool locates the tool configuration by name
func (s *Service) findTool(toolName string) *tool.Config {
	tools := make([]tool.Config, 0, len(s.agent.Tools))
	tools = append(tools, s.agent.Tools...)
	for i := range tools {
		if tools[i].ID == toolName {
			return &tools[i]
		}
	}
	return nil
}

func (s *Service) findMCPTool(toolName string) tools.Tool {
	for _, tool := range s.mcpTools {
		if tool.Name() == toolName {
			return tool
		}
	}
	return nil
}

// Close cleans up MCP connections and other resources
func (s *Service) Close() error {
	if s.connections != nil {
		mcp.CloseConnections(s.connections)
		s.connections = nil
	}
	return nil
}

// supportsStructuredOutput checks if the current provider supports structured outputs
func (s *Service) supportsStructuredOutput() bool {
	provider := s.agent.Config.Provider
	switch provider {
	case core.ProviderOpenAI, core.ProviderGroq, core.ProviderOllama:
		return true
	default:
		return false
	}
}

// cleanupJSON attempts to fix common JSON syntax issues
func (s *Service) cleanupJSON(content string) string {
	content = strings.TrimSpace(content)
	// Remove any markdown code blocks
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	// Fix trailing commas in arrays and objects using regex
	// This is a simple approach - for more complex cases, a proper JSON parser would be needed
	content = strings.ReplaceAll(content, ",]", "]")
	content = strings.ReplaceAll(content, ",}", "}")
	content = strings.ReplaceAll(content, ", ]", "]")
	content = strings.ReplaceAll(content, ", }", "}")
	return content
}

// shouldUseProxy determines if any MCP configuration uses proxy mode
func (s *Service) shouldUseProxy(mcps []mcp.Config) bool {
	for _, mcpConfig := range mcps {
		if mcpConfig.UseProxy {
			return true
		}
	}
	return false
}

// initMCPViaProxy initializes MCP tools via proxy discovery instead of direct connections
func (s *Service) initMCPViaProxy(mcps []mcp.Config) error {
	logger.Info("Initializing MCP tools via proxy mode", "mcp_count", len(mcps))

	// Initialize empty connections map since we're not using direct connections
	s.connections = make(map[string]*mcp.Connection)

	// For proxy mode, we discover tools by temporarily connecting through the proxy
	var allTools []tools.Tool

	for _, mcpConfig := range mcps {
		if !mcpConfig.UseProxy || mcpConfig.ProxyURL == "" {
			logger.Warn("Skipping MCP in proxy mode: not configured for proxy", "mcp_id", mcpConfig.ID)
			continue
		}

		tools, err := s.discoverToolsViaProxy(&mcpConfig)
		if err != nil {
			logger.Error("Failed to discover tools for MCP via proxy", "mcp_id", mcpConfig.ID, "error", err)
			continue
		}

		allTools = append(allTools, tools...)
		logger.Info("Discovered tools via proxy", "mcp_id", mcpConfig.ID, "tool_count", len(tools))
	}

	// Store tools for later use
	s.mcpTools = allTools

	logger.Info("MCP proxy initialization completed", "total_tools", len(allTools))
	return nil
}

// discoverToolsViaProxy discovers available tools for an MCP via proxy connection
func (s *Service) discoverToolsViaProxy(mcpConfig *mcp.Config) ([]tools.Tool, error) {
	// Build the proxy endpoint URL for this MCP
	proxyEndpoint := fmt.Sprintf("%s/%s/%s", mcpConfig.ProxyURL, mcpConfig.ID, mcpConfig.Transport)

	// Create a temporary configuration for proxy connection
	tempConfig := mcpConfig.Clone()
	tempConfig.URL = proxyEndpoint
	tempConfig.UseProxy = false // Don't use proxy for the actual connection since we're connecting to proxy

	// Create temporary connection to discover tools
	connection, err := mcp.NewConnection(tempConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy connection: %w", err)
	}
	defer connection.Close()

	// Get tools from the connection
	toolsMap := connection.GetTools()
	if toolsMap == nil {
		return nil, fmt.Errorf("no tools discovered")
	}

	// Convert map to slice
	var tools []tools.Tool
	for _, tool := range toolsMap {
		tools = append(tools, tool)
	}

	return tools, nil
}

// executeMCPTool executes an MCP tool, handling both direct and proxy modes
func (s *Service) executeMCPTool(ctx context.Context, tool tools.Tool, arguments string) (string, error) {
	start := time.Now()
	defer mcp.TimeOperation("mcp_tool_execution", start)
	mcp.IncrementToolExecution()

	result, err := tool.Call(ctx, arguments)
	if err != nil {
		mcp.IncrementToolExecutionFailure()
		return "", err
	}

	return result, nil
}

// enhancePromptForStructuredOutput adds JSON instruction if structured output is needed but not mentioned
func (s *Service) enhancePromptForStructuredOutput() string {
	prompt := s.action.Prompt
	// If structured output is needed but prompt doesn't mention JSON, enhance it
	if s.shouldUseStructuredOutput() {
		if s.action.OutputSchema != nil {
			return prompt + "\n\nIMPORTANT: You MUST respond with a valid JSON object only. " +
				"Do not include any HTML, markdown, or other formatting. " +
				"Return only valid JSON matching the following schema: " + s.action.OutputSchema.String()
		}
		return prompt + "\n\nIMPORTANT: You MUST respond in valid JSON format only. " +
			"No HTML, markdown, or other formatting." +
			"Just use a tool call if needed."
	}
	return prompt
}

// initProxyClient initializes the proxy client if proxy configuration is available
func (s *Service) initProxyClient() {
	// Check if any MCP uses proxy mode
	for _, mcpConfig := range s.mcps {
		if mcpConfig.UseProxy && mcpConfig.ProxyURL != "" {
			timeout := 30 * time.Second
			s.proxyClient = mcp.NewProxyClient(mcpConfig.ProxyURL, "", timeout)
			logger.Info("Initialized proxy client for dynamic tool discovery", "proxy_url", mcpConfig.ProxyURL)
			return
		}
	}

	// Also check environment variables as fallback
	if proxyURL := os.Getenv("MCP_PROXY_URL"); proxyURL != "" {
		adminToken := os.Getenv("MCP_PROXY_ADMIN_TOKEN")
		timeout := 30 * time.Second
		s.proxyClient = mcp.NewProxyClient(proxyURL, adminToken, timeout)
		logger.Info("Initialized proxy client from environment", "proxy_url", proxyURL)
	}
}

// refreshToolsFromProxy refreshes the tools cache by fetching from the proxy
func (s *Service) refreshToolsFromProxy(ctx context.Context) error {
	if s.proxyClient == nil {
		return fmt.Errorf("proxy client not available")
	}

	toolDefs, err := s.proxyClient.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools from proxy: %w", err)
	}

	// Convert tool definitions to langchain tools
	var newTools []tools.Tool
	for _, toolDef := range toolDefs {
		// Create a proxy tool that will execute via the proxy
		proxyTool := NewProxyTool(toolDef, s.proxyClient)
		newTools = append(newTools, proxyTool)
	}

	// Update cache with write lock
	s.toolsMu.Lock()
	s.toolsCache = newTools
	s.toolsCacheTs = time.Now()
	s.toolsMu.Unlock()

	logger.Info("Refreshed tools cache from proxy", "tool_count", len(newTools))
	return nil
}

// getToolsFromProxy returns tools from cache or refreshes if needed
func (s *Service) getToolsFromProxy(ctx context.Context) []tools.Tool {
	// First check with read lock if cache is valid
	s.toolsMu.RLock()
	needsRefresh := s.proxyClient != nil && (s.toolsCache == nil || time.Since(s.toolsCacheTs) > s.cacheTTL)
	cachedTools := s.toolsCache
	s.toolsMu.RUnlock()

	// If cache needs refresh, do it outside the read lock
	if needsRefresh {
		if err := s.refreshToolsFromProxy(ctx); err != nil {
			logger.Warn("Failed to refresh tools from proxy, using cached tools", "error", err)
			// Continue with cached tools if refresh fails
		}
		// Get the updated cache
		s.toolsMu.RLock()
		cachedTools = s.toolsCache
		s.toolsMu.RUnlock()
	}

	return cachedTools
}

// InvalidateToolsCache forces a refresh of the tools cache on next access
func (s *Service) InvalidateToolsCache() {
	s.toolsMu.Lock()
	s.toolsCacheTs = time.Time{} // Set to zero time to force refresh
	s.toolsMu.Unlock()
	logger.Debug("Tools cache invalidated")
}
