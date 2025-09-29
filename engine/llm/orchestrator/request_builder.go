package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

type RequestBuilder interface {
	Build(ctx context.Context, request Request, memoryCtx *MemoryContext) (llmadapter.LLMRequest, error)
}

type requestBuilder struct {
	prompts PromptBuilder
	tools   ToolRegistry
	memory  MemoryManager
}

func NewRequestBuilder(prompts PromptBuilder, tools ToolRegistry, memory MemoryManager) RequestBuilder {
	return &requestBuilder{prompts: prompts, tools: tools, memory: memory}
}

func (b *requestBuilder) Build(
	ctx context.Context,
	request Request,
	memoryCtx *MemoryContext,
) (llmadapter.LLMRequest, error) {
	promptData, err := b.buildPromptData(ctx, request)
	if err != nil {
		return llmadapter.LLMRequest{}, err
	}

	toolDefs, err := b.buildToolDefinitions(ctx, request.Agent.Tools)
	if err != nil {
		return llmadapter.LLMRequest{}, NewLLMError(err, ErrCodeToolDefinitions, map[string]any{
			"agent": request.Agent.ID,
		})
	}
	toolDefs = b.appendGroqJSONToolIfNeeded(toolDefs, request)

	messages := b.buildMessages(ctx, promptData.enhancedPrompt, memoryCtx, request)
	if err := llmadapter.ValidateConversation(messages); err != nil {
		return llmadapter.LLMRequest{}, NewLLMError(err, ErrCodeInvalidConversation, map[string]any{
			"agent":          request.Agent.ID,
			"action":         request.Action.ID,
			"messages_count": len(messages),
		})
	}

	temperature := request.Agent.Model.Config.Params.Temperature
	toolChoice := "none"
	if len(toolDefs) > 0 {
		toolChoice = "auto"
	}

	logger.FromContext(ctx).Debug("LLM request prepared",
		"agent_id", request.Agent.ID,
		"action_id", request.Action.ID,
		"messages_count", len(messages),
		"tools_count", len(toolDefs),
		"tool_choice", toolChoice,
		"output_format", b.describeOutputFormat(promptData.format),
	)

	return llmadapter.LLMRequest{
		SystemPrompt: b.enhanceSystemPromptWithBuiltins(ctx, request.Agent.Instructions),
		Messages:     messages,
		Tools:        toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:  temperature,
			MaxTokens:    request.Agent.Model.Config.Params.MaxTokens,
			StopWords:    request.Agent.Model.Config.Params.StopWords,
			ToolChoice:   toolChoice,
			OutputFormat: promptData.format,
		},
	}, nil
}

type promptBuildData struct {
	enhancedPrompt string
	format         llmadapter.OutputFormat
}

func (b *requestBuilder) buildPromptData(ctx context.Context, request Request) (*promptBuildData, error) {
	basePrompt, err := b.prompts.Build(ctx, request.Action)
	if err != nil {
		return nil, NewLLMError(err, ErrCodePromptBuild, map[string]any{
			"action": request.Action.ID,
		})
	}
	enhancedPrompt := basePrompt
	format := llmadapter.DefaultOutputFormat()
	action := request.Action
	tools := request.Agent.Tools
	hasSchema := action != nil && action.OutputSchema != nil
	if hasSchema {
		nativeStructured := len(tools) == 0 && b.prompts.ShouldUseStructuredOutput(
			string(request.Agent.Model.Config.Provider),
			action,
			tools,
		)
		if nativeStructured {
			name := action.ID
			if name == "" {
				name = "action_output"
			}
			format = llmadapter.NewJSONSchemaOutputFormat(name, action.OutputSchema, true)
		} else {
			enhancedPrompt = b.prompts.EnhanceForStructuredOutput(
				ctx,
				basePrompt,
				action.OutputSchema,
				len(tools) > 0,
			)
		}
	}
	return &promptBuildData{
		enhancedPrompt: enhancedPrompt,
		format:         format,
	}, nil
}

func (b *requestBuilder) describeOutputFormat(format llmadapter.OutputFormat) string {
	if format.IsJSONSchema() {
		if format.Name != "" {
			return "json_schema:" + format.Name
		}
		return "json_schema"
	}
	return "default"
}

func (b *requestBuilder) buildMessages(
	ctx context.Context,
	enhancedPrompt string,
	memoryCtx *MemoryContext,
	request Request,
) []llmadapter.Message {
	parts := request.AttachmentParts
	if parts == nil {
		parts = []llmadapter.ContentPart{}
	}

	messages := []llmadapter.Message{{
		Role:    "user",
		Content: enhancedPrompt,
		Parts:   parts,
	}}

	if memoryCtx != nil {
		messages = b.memory.Inject(ctx, messages, memoryCtx)
	}

	return messages
}

func (b *requestBuilder) buildToolDefinitions(
	ctx context.Context,
	tools []tool.Config,
) ([]llmadapter.ToolDefinition, error) {
	defs, included, err := b.collectConfiguredToolDefs(ctx, tools)
	if err != nil {
		return nil, err
	}
	defs = b.appendRegistryToolDefs(ctx, defs, included)
	return defs, nil
}

func (b *requestBuilder) collectConfiguredToolDefs(
	ctx context.Context,
	tools []tool.Config,
) ([]llmadapter.ToolDefinition, map[string]struct{}, error) {
	if len(tools) > 0 && b.tools == nil {
		return nil, nil, NewLLMError(
			fmt.Errorf("tool registry unavailable"),
			ErrCodeToolNotFound,
			map[string]any{"configured_tools": len(tools)},
		)
	}
	defs := make([]llmadapter.ToolDefinition, 0, len(tools))
	included := make(map[string]struct{}, len(tools))
	for i := range tools {
		toolConfig := &tools[i]
		t, found := b.tools.Find(ctx, toolConfig.ID)
		if !found {
			return nil, nil, NewToolError(
				fmt.Errorf("tool not found"),
				ErrCodeToolNotFound,
				toolConfig.ID,
				map[string]any{"configured_tools": len(tools)},
			)
		}

		def := llmadapter.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
		}
		if toolConfig.InputSchema != nil {
			def.Parameters = normalizeToolParameters(*toolConfig.InputSchema)
		} else {
			def.Parameters = map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}
		}

		defs = append(defs, def)
		included[canonicalToolName(def.Name)] = struct{}{}
	}

	return defs, included, nil
}

func (b *requestBuilder) appendGroqJSONToolIfNeeded(
	defs []llmadapter.ToolDefinition,
	request Request,
) []llmadapter.ToolDefinition {
	if request.Agent == nil {
		return defs
	}
	if !strings.EqualFold(string(request.Agent.Model.Config.Provider), string(core.ProviderGroq)) {
		return defs
	}
	for i := range defs {
		if strings.EqualFold(defs[i].Name, "json") {
			return defs
		}
	}
	jsonParams := map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
	return append(defs, llmadapter.ToolDefinition{
		Name:        "json",
		Description: "Internal JSON structured output tool for Groq compatibility.",
		Parameters:  jsonParams,
	})
}

func (b *requestBuilder) appendRegistryToolDefs(
	ctx context.Context,
	defs []llmadapter.ToolDefinition,
	included map[string]struct{},
) []llmadapter.ToolDefinition {
	if b.tools == nil {
		return defs
	}

	log := logger.FromContext(ctx)
	allTools, err := b.tools.ListAll(ctx)
	if err != nil {
		log.Warn("Failed to list tools from registry", "error", core.RedactError(err))
		return defs
	}

	for _, rt := range allTools {
		name := rt.Name()
		lower := canonicalToolName(name)
		if _, ok := included[lower]; ok {
			continue
		}

		params := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}

		schemaApplied := false
		type inputSchemaProvider interface{ InputSchema() *schema.Schema }
		if sp, ok := any(rt).(inputSchemaProvider); ok {
			if s := sp.InputSchema(); s != nil {
				params = normalizeToolParameters(map[string]any(*s))
				schemaApplied = true
			}
		}
		if !schemaApplied {
			type argsTyper interface{ ArgsType() any }
			if at, ok := any(rt).(argsTyper); ok {
				if v := at.ArgsType(); v != nil {
					if m, isMap := v.(map[string]any); isMap && len(m) > 0 {
						params = normalizeToolParameters(m)
					}
				}
			}
		}

		defs = append(defs, llmadapter.ToolDefinition{Name: name, Description: rt.Description(), Parameters: params})
		included[lower] = struct{}{}
	}

	return defs
}

func canonicalToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeToolParameters(input map[string]any) map[string]any {
	params := cloneMap(input)
	if !isObjectType(params["type"]) {
		params["type"] = "object"
	}
	if _, ok := params["properties"]; !ok {
		params["properties"] = map[string]any{}
	}
	return params
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	clone := make(map[string]any, len(src))
	for k, v := range src {
		clone[k] = v
	}
	return clone
}

func isObjectType(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "object")
	case []string:
		for _, t := range v {
			if strings.EqualFold(strings.TrimSpace(t), "object") {
				return true
			}
		}
		return false
	case []any:
		for _, t := range v {
			if s, ok := t.(string); ok && strings.EqualFold(strings.TrimSpace(s), "object") {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// enhanceSystemPromptWithBuiltins enhances the agent instructions with information
// about available built-in tools that agents can use for common tasks.
func (b *requestBuilder) enhanceSystemPromptWithBuiltins(
	_ context.Context,
	originalInstructions string,
) string {
	builtinToolsInfo := `
<built-in-tools>
## Built-in Tools Available

You have access to several built-in tools for common operations.
When appropriate, use these tools instead of asking users to perform manual tasks:

### File Operations
- **cp__read_file**: Read text content from files in the sandboxed filesystem
- **cp__write_file**: Write or append text content to files
- **cp__delete_file**: Delete files or directories (with recursive option)
- **cp__list_files**: List files in directories with pattern filtering
- **cp__list_dir**: List directory contents with pagination and filtering
- **cp__grep**: Search for text patterns within files using regex

### System Operations
- **cp__exec**: Execute pre-approved system commands from an allowlist (e.g., ls, pwd, cat, etc.)

### Network Operations
- **cp__fetch**: Make HTTP requests (GET, POST, PUT, etc.) with configurable timeouts and headers

### Usage Guidelines
- Use these tools proactively when they would help accomplish the user's request
- Tools have appropriate security restrictions and input validation
- Always prefer using tools over asking users to perform manual file or system operations
- Check tool responses for errors and handle them appropriately
</built-in-tools>

`

	// If original instructions are empty, use just the builtin info
	if strings.TrimSpace(originalInstructions) == "" {
		return builtinToolsInfo
	}

	// Otherwise, append the builtin info to existing instructions
	return originalInstructions + "\n\n" + builtinToolsInfo
}
