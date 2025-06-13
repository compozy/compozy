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
	"github.com/kaptinlin/jsonrepair"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/tools"
)

type Service struct {
	runtime  *runtime.Manager
	agent    *agent.Config
	action   *agent.ActionConfig
	mcps     []mcp.Config
	mcpTools []tools.Tool

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
) (*Service, error) {
	service := &Service{
		runtime:  runtime,
		agent:    agent,
		action:   action,
		mcps:     mcps,
		cacheTTL: 5 * time.Minute, // Cache tools for 5 minutes
	}
	// Only initialize proxy client if MCP configurations exist
	hasMCPConfigs := len(agent.MCPs) > 0 || len(mcps) > 0
	if hasMCPConfigs {
		client, err := initProxyClient()
		if err != nil {
			// Log warning but don't fail service creation
			logger.Warn("Failed to initialize MCP proxy client", "error", err)
		} else {
			service.proxyClient = client
		}
	}
	// Initialize MCP once during service creation
	ctx := context.Background()
	service.initMCP(ctx)
	return service, nil
}

func (s *Service) CreateLLM() (llms.Model, error) {
	return s.agent.Config.CreateLLM(nil)
}

func (s *Service) GenerateContent(ctx context.Context) (*core.Output, error) {
	model, err := s.CreateLLM()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM: %w", err)
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
	callOptions := s.buildCallOptions(ctx)
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

func (s *Service) initMCP(ctx context.Context) {
	mcps := make([]mcp.Config, 0, len(s.agent.MCPs))
	// Prefer agent MCPs over workflow MCPs
	if len(s.agent.MCPs) > 0 {
		mcps = append(mcps, s.agent.MCPs...)
	}
	// Add workflow MCPs if no agent MCPs are defined
	if len(mcps) == 0 && len(s.mcps) > 0 {
		mcps = append(mcps, s.mcps...)
	}
	// Skip MCP initialization if no configurations exist
	if len(mcps) == 0 {
		return
	}
	// Get tools from proxy once for all MCP configurations
	allTools := s.getToolsFromProxy(ctx)
	// Store tools for later use
	s.mcpTools = allTools
	logger.Info("MCP proxy initialization completed", "mcp_configs", len(mcps), "total_tools", len(allTools))
}

// getLLMCallTools returns properly configured tool definitions including MCP tools
func (s *Service) getLLMCallTools(ctx context.Context) []llms.Tool {
	var tools []llms.Tool
	proxyTools := s.getToolsFromProxy(ctx)
	for _, proxyTool := range proxyTools {
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
	}
	// Add agent-specific tools
	for _, toolConfig := range s.agent.Tools {
		tools = append(tools, toolConfig.GetLLMDefinition())
	}
	return tools
}

// buildCallOptions constructs the appropriate call options based on tools and schemas
func (s *Service) buildCallOptions(ctx context.Context) []llms.CallOption {
	var options []llms.CallOption
	// Add tools if available
	tools := s.getLLMCallTools(ctx)
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
			// Try to repair the JSON and retry
			repairedContent, repairErr := jsonrepair.JSONRepair(content)
			if repairErr != nil {
				return nil, fmt.Errorf(
					"expected structured output but received invalid JSON that could not be repaired: %w. Content: %s",
					err,
					content,
				)
			}
			if err := json.Unmarshal([]byte(repairedContent), &structuredOutput); err != nil {
				return nil, fmt.Errorf(
					"expected structured output but repaired JSON is still invalid: %w. Content: %s",
					err,
					repairedContent,
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
	if s.proxyClient != nil {
		return s.proxyClient.Close()
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

// executeMCPTool executes an MCP tool, handling both direct and proxy modes
func (s *Service) executeMCPTool(ctx context.Context, tool tools.Tool, arguments string) (string, error) {
	result, err := tool.Call(ctx, arguments)
	if err != nil {
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
func initProxyClient() (*mcp.Client, error) {
	timeout := 30 * time.Second
	proxyURL := os.Getenv("MCP_PROXY_URL")
	adminToken := os.Getenv("MCP_PROXY_ADMIN_TOKEN")
	if proxyURL == "" {
		return nil, fmt.Errorf("MCP_PROXY_URL is not set")
	}
	if adminToken == "" {
		return mcp.NewProxyClient(proxyURL, "", timeout), nil
	}
	return mcp.NewProxyClient(proxyURL, adminToken, timeout), nil
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
