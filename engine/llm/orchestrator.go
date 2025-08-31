package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/sethvargo/go-retry"
	"golang.org/x/sync/errgroup"
)

// Default configuration constants
const (
	defaultMaxConcurrentTools = 10
	defaultRetryAttempts      = 3
	defaultRetryBackoffBase   = 100 * time.Millisecond
	defaultRetryBackoffMax    = 10 * time.Second
)

// AsyncHook provides hooks for monitoring async operations
type AsyncHook interface {
	// OnMemoryStoreComplete is called when async memory storage completes
	OnMemoryStoreComplete(err error)
}

// Orchestrator coordinates LLM interactions, tool calls, and response processing
type Orchestrator interface {
	Execute(ctx context.Context, request Request) (*core.Output, error)
	Close() error
}

// Request represents a request to the orchestrator
type Request struct {
	Agent  *agent.Config
	Action *agent.ActionConfig
}

// OrchestratorConfig configures the LLM orchestrator
type OrchestratorConfig struct {
	ToolRegistry       ToolRegistry
	PromptBuilder      PromptBuilder
	RuntimeManager     runtime.Runtime
	LLMFactory         llmadapter.Factory
	MemoryProvider     MemoryProvider // Optional: provides memory instances for agents
	AsyncHook          AsyncHook      // Optional: hook for monitoring async operations
	Timeout            time.Duration  // Optional: timeout for LLM operations
	MaxConcurrentTools int            // Maximum concurrent tool executions
	// Retry configuration
	RetryAttempts    int           // Number of retry attempts for LLM operations
	RetryBackoffBase time.Duration // Base delay for exponential backoff retry strategy
	RetryBackoffMax  time.Duration // Maximum delay between retry attempts
	RetryJitter      bool          // Enable random jitter in retry delays
}

// Implementation of LLMOrchestrator
type llmOrchestrator struct {
	config     OrchestratorConfig
	memorySync *MemorySync
}

// NewOrchestrator creates a new LLM orchestrator
func NewOrchestrator(config *OrchestratorConfig) Orchestrator {
	if config == nil {
		config = &OrchestratorConfig{}
	}
	return &llmOrchestrator{config: *config, memorySync: NewMemorySync()}
}

// Execute processes an LLM request end-to-end
func (o *llmOrchestrator) Execute(ctx context.Context, request Request) (*core.Output, error) {
	log := logger.FromContext(ctx)
	if err := o.validateInput(ctx, request); err != nil {
		return nil, NewValidationError(err, "request", request)
	}
	return o.executeWithValidatedRequest(ctx, request, log)
}

func (o *llmOrchestrator) executeWithValidatedRequest(
	ctx context.Context,
	request Request,
	log logger.Logger,
) (*core.Output, error) {
	memories := o.prepareMemoryContext(ctx, request, log)
	llmClient, err := o.createLLMClient(request)
	if err != nil {
		return nil, err
	}
	defer o.closeLLMClient(llmClient, log)
	return o.executeWithClient(ctx, request, memories, llmClient, log)
}

func (o *llmOrchestrator) executeWithClient(
	ctx context.Context,
	request Request,
	memories map[string]Memory,
	llmClient llmadapter.LLMClient,
	log logger.Logger,
) (*core.Output, error) {
	llmReq, err := o.buildLLMRequest(ctx, request, memories)
	if err != nil {
		return nil, err
	}
	response, err := o.generateLLMResponse(ctx, llmClient, &llmReq, request)
	if err != nil {
		return nil, err
	}
	output, err := o.processResponse(ctx, response, request)
	if err != nil {
		return nil, fmt.Errorf("failed to process LLM response: %w", err)
	}
	o.storeResponseInMemoryAsync(ctx, memories, response, llmReq.Messages, request, log)
	return output, nil
}

func (o *llmOrchestrator) generateLLMResponse(
	ctx context.Context,
	llmClient llmadapter.LLMClient,
	llmReq *llmadapter.LLMRequest,
	request Request,
) (*llmadapter.LLMResponse, error) {
	// Apply timeout if configured
	if o.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.config.Timeout)
		defer cancel()
	}
	var response *llmadapter.LLMResponse
	// Use configurable retry with exponential backoff
	attempts := o.config.RetryAttempts
	if attempts <= 0 {
		attempts = defaultRetryAttempts
	}
	backoffBase := o.config.RetryBackoffBase
	if backoffBase <= 0 {
		backoffBase = defaultRetryBackoffBase
	}
	backoffMax := o.config.RetryBackoffMax
	if backoffMax <= 0 {
		backoffMax = defaultRetryBackoffMax
	}

	var backoff retry.Backoff
	exponential := retry.NewExponential(backoffBase)
	exponential = retry.WithMaxDuration(backoffMax, exponential)
	// Validate attempts is positive and within reasonable bounds to prevent overflow
	if attempts < 0 || attempts > 100 {
		attempts = defaultRetryAttempts
	}
	// Safe conversion: attempts is validated to be in range [0, 100]
	maxRetries := uint64(attempts) //nolint:gosec // G115: bounds checked above
	if o.config.RetryJitter {
		backoff = retry.WithMaxRetries(maxRetries, retry.WithJitter(time.Millisecond*50, exponential))
	} else {
		backoff = retry.WithMaxRetries(maxRetries, exponential)
	}

	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		var err error
		response, err = llmClient.GenerateContent(ctx, llmReq)
		if err != nil {
			// Check if error is retryable
			if isRetryableErrorWithContext(ctx, err) {
				return retry.RetryableError(err)
			}
			// Non-retryable error
			return err
		}
		return nil
	})
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMGeneration, map[string]any{
			"agent":  request.Agent.ID,
			"action": request.Action.ID,
		})
	}
	return response, nil
}

func (o *llmOrchestrator) prepareMemoryContext(
	ctx context.Context,
	request Request,
	log logger.Logger,
) map[string]Memory {
	memoryRefs := request.Agent.Memory

	log.Debug("Preparing memory context for agent",
		"agent_id", request.Agent.ID,
		"memory_refs_count", len(memoryRefs),
	)

	if o.config.MemoryProvider == nil {
		log.Debug("No memory provider available")
		return nil
	}
	if len(memoryRefs) == 0 {
		log.Debug("No memory references configured for agent")
		return nil
	}

	memories := make(map[string]Memory)
	for _, ref := range memoryRefs {
		log.Debug("Retrieving memory for agent",
			"memory_id", ref.ID,
			"key", ref.Key,
		)

		memory, err := o.config.MemoryProvider.GetMemory(ctx, ref.ID, ref.Key)
		if err != nil {
			log.Error("Failed to get memory instance", "memory_id", ref.ID, "error", err)
			continue
		}
		if memory != nil {
			log.Debug("Memory instance retrieved successfully",
				"memory_id", ref.ID,
				"instance_id", memory.GetID())
			memories[ref.ID] = memory
		} else {
			log.Warn("Memory instance is nil", "memory_id", ref.ID)
		}
	}

	log.Debug("Memory context prepared", "count", len(memories))
	return memories
}

func (o *llmOrchestrator) createLLMClient(request Request) (llmadapter.LLMClient, error) {
	factory := o.config.LLMFactory
	if factory == nil {
		factory = llmadapter.NewDefaultFactory()
	}
	llmClient, err := factory.CreateClient(&request.Agent.Config)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeLLMCreation, map[string]any{
			"provider": request.Agent.Config.Provider,
			"model":    request.Agent.Config.Model,
		})
	}
	return llmClient, nil
}

func (o *llmOrchestrator) closeLLMClient(llmClient llmadapter.LLMClient, log logger.Logger) {
	if closeErr := llmClient.Close(); closeErr != nil {
		log.Error("Failed to close LLM client", "error", closeErr)
	}
}

func (o *llmOrchestrator) buildLLMRequest(
	ctx context.Context,
	request Request,
	memories map[string]Memory,
) (llmadapter.LLMRequest, error) {
	promptData, err := o.buildPromptData(ctx, request)
	if err != nil {
		return llmadapter.LLMRequest{}, err
	}
	toolDefs, err := o.buildToolDefinitions(ctx, request.Agent.Tools)
	if err != nil {
		return llmadapter.LLMRequest{}, NewLLMError(err, "TOOL_DEFINITIONS_ERROR", map[string]any{
			"agent": request.Agent.ID,
		})
	}
	messages := o.buildMessages(ctx, promptData.enhancedPrompt, memories)

	// Determine temperature: use agent's configured value (explicit zero allowed; upstream default applies)
	temperature := request.Agent.Config.Params.Temperature

	return llmadapter.LLMRequest{
		SystemPrompt: request.Agent.Instructions,
		Messages:     messages,
		Tools:        toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:      temperature,
			UseJSONMode:      request.Action.JSONMode || (promptData.shouldUseStructured && len(toolDefs) == 0),
			StructuredOutput: promptData.shouldUseStructured,
		},
	}, nil
}

type promptBuildData struct {
	enhancedPrompt      string
	shouldUseStructured bool
}

func (o *llmOrchestrator) buildPromptData(ctx context.Context, request Request) (*promptBuildData, error) {
	basePrompt, err := o.config.PromptBuilder.Build(ctx, request.Action)
	if err != nil {
		return nil, NewLLMError(err, "PROMPT_BUILD_ERROR", map[string]any{
			"action": request.Action.ID,
		})
	}
	shouldUseStructured := o.config.PromptBuilder.ShouldUseStructuredOutput(
		string(request.Agent.Config.Provider),
		request.Action,
		request.Agent.Tools,
	)
	enhancedPrompt := o.enhancePromptIfNeeded(ctx, basePrompt, shouldUseStructured, request)
	return &promptBuildData{
		enhancedPrompt:      enhancedPrompt,
		shouldUseStructured: shouldUseStructured,
	}, nil
}

func (o *llmOrchestrator) enhancePromptIfNeeded(
	ctx context.Context,
	basePrompt string,
	shouldUseStructured bool,
	request Request,
) string {
	if !shouldUseStructured {
		return basePrompt
	}
	return o.config.PromptBuilder.EnhanceForStructuredOutput(
		ctx,
		basePrompt,
		request.Action.OutputSchema,
		len(request.Agent.Tools) > 0,
	)
}

func (o *llmOrchestrator) buildMessages(
	ctx context.Context,
	enhancedPrompt string,
	memories map[string]Memory,
) []llmadapter.Message {
	messages := []llmadapter.Message{{
		Role:    "user",
		Content: enhancedPrompt,
	}}
	if len(memories) > 0 {
		messages = PrepareMemoryContext(ctx, memories, messages)
	}
	return messages
}

func (o *llmOrchestrator) storeResponseInMemoryAsync(
	ctx context.Context,
	memories map[string]Memory,
	response *llmadapter.LLMResponse,
	messages []llmadapter.Message,
	request Request,
	log logger.Logger,
) {
	if len(memories) == 0 || response.Content == "" {
		return
	}
	go func() {
		// Create a detached context with timeout to prevent goroutine leaks
		// context.WithoutCancel preserves values from the parent context
		// while allowing the goroutine to continue even if the parent is canceled
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()

		// Collect memory IDs for multi-lock synchronization
		var memoryIDs []string
		for _, memory := range memories {
			if memory != nil {
				memoryIDs = append(memoryIDs, memory.GetID())
			}
		}

		// Use WithMultipleLocks for safe concurrent memory access
		var err error
		o.memorySync.WithMultipleLocks(memoryIDs, func() {
			assistantMsg := llmadapter.Message{
				Role:    "assistant",
				Content: response.Content,
			}
			err = StoreResponseInMemory(
				bgCtx,
				memories,
				request.Agent.Memory,
				assistantMsg,
				messages[len(messages)-1],
			)
		})
		if err != nil {
			log.Error("Failed to store response in memory",
				"error", err,
				"agent_id", request.Agent.ID,
				"action_id", request.Action.ID)
			// Consider sending to a metrics/alerting system
			// - **Example**: metrics.RecordMemoryStorageFailure(request.Agent.ID, err)
		}
		// Call async hook if configured
		if o.config.AsyncHook != nil {
			o.config.AsyncHook.OnMemoryStoreComplete(err)
		}
	}()
}

// validateInput validates the input request
func (o *llmOrchestrator) validateInput(ctx context.Context, request Request) error {
	if request.Agent == nil {
		return fmt.Errorf("agent config is required")
	}

	if request.Action == nil {
		return fmt.Errorf("action config is required")
	}

	if request.Agent.Instructions == "" {
		return fmt.Errorf("agent instructions are required")
	}

	if request.Action.Prompt == "" {
		return fmt.Errorf("action prompt is required")
	}

	// Validate input schema if defined
	if request.Action.InputSchema != nil {
		if err := request.Action.ValidateInput(ctx, request.Action.GetInput()); err != nil {
			return fmt.Errorf("input validation failed: %w", err)
		}
	}

	return nil
}

// buildToolDefinitions converts agent tools to LLM adapter format
func (o *llmOrchestrator) buildToolDefinitions(
	ctx context.Context,
	tools []tool.Config,
) ([]llmadapter.ToolDefinition, error) {
	var definitions []llmadapter.ToolDefinition

	for i := range tools {
		toolConfig := &tools[i]
		// Find the tool in registry
		tool, found := o.config.ToolRegistry.Find(ctx, toolConfig.ID)
		if !found {
			return nil, NewToolError(
				fmt.Errorf("tool not found"),
				ErrCodeToolNotFound,
				toolConfig.ID,
				map[string]any{"configured_tools": len(tools)},
			)
		}

		// Build tool definition
		def := llmadapter.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
		}

		// Add parameters schema if available
		if toolConfig.InputSchema != nil {
			def.Parameters = *toolConfig.InputSchema
		}

		definitions = append(definitions, def)
	}

	return definitions, nil
}

// processResponse processes the LLM response and executes any tool calls
func (o *llmOrchestrator) processResponse(
	ctx context.Context,
	response *llmadapter.LLMResponse,
	request Request,
) (*core.Output, error) {
	if len(response.ToolCalls) > 0 {
		return o.executeToolCalls(ctx, response.ToolCalls, request)
	}
	return o.parseContentResponse(ctx, response.Content, request.Action)
}

func (o *llmOrchestrator) parseContentResponse(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	output, err := o.parseContent(ctx, content, action)
	if err != nil {
		return nil, NewLLMError(err, ErrCodeInvalidResponse, map[string]any{
			"content": content,
		})
	}
	return output, nil
}

// executeToolCalls executes tool calls and returns the result
func (o *llmOrchestrator) executeToolCalls(
	ctx context.Context,
	toolCalls []llmadapter.ToolCall,
	request Request,
) (*core.Output, error) {
	if len(toolCalls) == 0 {
		return nil, fmt.Errorf("no tool calls to execute")
	}
	// Use parallel execution with semaphore for concurrency control
	maxConcurrent := o.config.MaxConcurrentTools
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentTools
	}
	// Create error group for parallel execution
	g, ctx := errgroup.WithContext(ctx)
	// Create semaphore to limit concurrent executions
	sem := make(chan struct{}, maxConcurrent)
	// Results need to be collected in a thread-safe way
	results := make([]map[string]any, len(toolCalls))
	var resultsMu sync.Mutex
	// Execute tool calls in parallel with concurrency limit
	for i, tc := range toolCalls {
		index := i
		toolCall := tc
		g.Go(func() error {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}
			// Execute the tool call
			result, err := o.executeSingleToolCall(ctx, toolCall, request)
			if err != nil {
				return err
			}
			// Store result thread-safely
			resultsMu.Lock()
			results[index] = map[string]any{
				"tool_call_id": toolCall.ID,
				"tool_name":    toolCall.Name,
				"result":       result,
			}
			resultsMu.Unlock()
			return nil
		})
	}
	// Wait for all tool calls to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}
	// If only one tool call, return its result directly
	if len(results) == 1 {
		if result, ok := results[0]["result"].(*core.Output); ok {
			return result, nil
		}
	}
	// Multiple tool calls - return combined results
	output := core.Output(map[string]any{
		"results": results,
	})
	return &output, nil
}

// executeSingleToolCall executes a single tool call
func (o *llmOrchestrator) executeSingleToolCall(
	ctx context.Context,
	toolCall llmadapter.ToolCall,
	request Request,
) (*core.Output, error) {
	// Find the tool
	tool, found := o.config.ToolRegistry.Find(ctx, toolCall.Name)
	if !found {
		return nil, NewToolError(
			fmt.Errorf("tool not found for execution"),
			ErrCodeToolNotFound,
			toolCall.Name,
			map[string]any{"call_id": toolCall.ID},
		)
	}
	// Execute the tool
	result, err := tool.Call(ctx, string(toolCall.Arguments))
	if err != nil {
		return nil, NewToolError(err, ErrCodeToolExecution, toolCall.Name, map[string]any{
			"call_id":   toolCall.ID,
			"arguments": toolCall.Arguments,
		})
	}
	// Check for tool execution errors using improved error detection
	if toolErr, isError := IsToolExecutionError(result); isError {
		return nil, NewToolError(
			fmt.Errorf("tool execution failed: %s", toolErr.Message),
			ErrCodeToolExecution,
			toolCall.Name,
			map[string]any{
				"error_code":    toolErr.Code,
				"error_details": toolErr.Details,
			},
		)
	}
	// Parse the tool result with appropriate schema
	// Note: Tool output schema should come from the tool configuration, not action
	// For now, use action schema as fallback until tool schemas are properly wired
	return o.parseContent(ctx, result, request.Action)
}

// parseContent parses content and validates against schema if provided
func (o *llmOrchestrator) parseContent(
	ctx context.Context,
	content string,
	action *agent.ActionConfig,
) (*core.Output, error) {
	// Try to parse as JSON first
	var data any
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		// Successfully parsed as JSON - check if it's an object
		if obj, ok := data.(map[string]any); ok {
			output := core.Output(obj)

			// Validate against schema if provided
			if err := o.validateOutput(ctx, &output, action); err != nil {
				return nil, NewValidationError(err, "output", obj)
			}

			return &output, nil
		}

		// Valid JSON but not an object - return error since core.Output expects map
		return nil, NewLLMError(
			fmt.Errorf("expected JSON object, got %T", data),
			ErrCodeInvalidResponse,
			map[string]any{"content": data},
		)
	}

	// Not valid JSON, return as text response
	output := core.Output(map[string]any{
		"response": content,
	})
	return &output, nil
}

// validateOutput validates output against schema
func (o *llmOrchestrator) validateOutput(ctx context.Context, output *core.Output, action *agent.ActionConfig) error {
	if action.OutputSchema == nil {
		return nil
	}
	return action.ValidateOutput(ctx, output)
}

// Close cleans up resources
func (o *llmOrchestrator) Close() error {
	if o.config.ToolRegistry != nil {
		return o.config.ToolRegistry.Close()
	}
	return nil
}
