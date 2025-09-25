package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
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
		return llmadapter.LLMRequest{}, NewLLMError(err, "TOOL_DEFINITIONS_ERROR", map[string]any{
			"agent": request.Agent.ID,
		})
	}

	messages := b.buildMessages(ctx, promptData.enhancedPrompt, memoryCtx, request)
	if err := llmadapter.ValidateConversation(messages); err != nil {
		return llmadapter.LLMRequest{}, NewLLMError(err, "INVALID_CONVERSATION", map[string]any{
			"agent":          request.Agent.ID,
			"action":         request.Action.ID,
			"messages_count": len(messages),
		})
	}

	temperature := request.Agent.Model.Config.Params.Temperature
	toolChoice := ""
	if len(toolDefs) > 0 {
		toolChoice = "auto"
	}

	logger.FromContext(ctx).Debug("LLM request prepared",
		"agent_id", request.Agent.ID,
		"action_id", request.Action.ID,
		"messages_count", len(messages),
		"tools_count", len(toolDefs),
		"tool_choice", toolChoice,
		"json_mode", request.Action.JSONMode || (promptData.shouldUseStructured && len(toolDefs) == 0),
		"structured_output", promptData.shouldUseStructured,
	)

	return llmadapter.LLMRequest{
		SystemPrompt: request.Agent.Instructions,
		Messages:     messages,
		Tools:        toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:      temperature,
			UseJSONMode:      request.Action.JSONMode || (promptData.shouldUseStructured && len(toolDefs) == 0),
			ToolChoice:       toolChoice,
			StructuredOutput: promptData.shouldUseStructured,
		},
	}, nil
}

type promptBuildData struct {
	enhancedPrompt      string
	shouldUseStructured bool
}

func (b *requestBuilder) buildPromptData(ctx context.Context, request Request) (*promptBuildData, error) {
	basePrompt, err := b.prompts.Build(ctx, request.Action)
	if err != nil {
		return nil, NewLLMError(err, "PROMPT_BUILD_ERROR", map[string]any{
			"action": request.Action.ID,
		})
	}
	shouldUseStructured := b.prompts.ShouldUseStructuredOutput(
		string(request.Agent.Model.Config.Provider),
		request.Action,
		request.Agent.Tools,
	)
	enhancedPrompt := basePrompt
	if shouldUseStructured {
		enhancedPrompt = b.prompts.EnhanceForStructuredOutput(
			ctx,
			basePrompt,
			request.Action.OutputSchema,
			len(request.Agent.Tools) > 0,
		)
	}
	return &promptBuildData{
		enhancedPrompt:      enhancedPrompt,
		shouldUseStructured: shouldUseStructured,
	}, nil
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
	canonical := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}

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
		included[canonical(def.Name)] = struct{}{}
	}

	return defs, included, nil
}

func (b *requestBuilder) appendRegistryToolDefs(
	ctx context.Context,
	defs []llmadapter.ToolDefinition,
	included map[string]struct{},
) []llmadapter.ToolDefinition {
	canonical := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}

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
		if _, ok := included[canonical(name)]; ok {
			continue
		}

		params := map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}

		type argsTyper interface{ ArgsType() any }
		if at, ok := any(rt).(argsTyper); ok {
			if v := at.ArgsType(); v != nil {
				if m, isMap := v.(map[string]any); isMap && len(m) > 0 {
					params = normalizeToolParameters(m)
				}
			}
		}

		defs = append(defs, llmadapter.ToolDefinition{
			Name:        name,
			Description: rt.Description(),
			Parameters:  params,
		})
		included[canonical(name)] = struct{}{}
	}

	return defs
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
