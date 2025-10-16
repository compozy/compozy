package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"
	"text/template"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/orchestrator/prompts"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
)

var (
	fallbackTemplateOnce sync.Once
	fallbackTemplate     *template.Template
	fallbackTemplateErr  error
)

type RequestBuilder interface {
	Build(ctx context.Context, request Request, memoryCtx *MemoryContext) (RequestBuildOutput, error)
}

type requestBuilder struct {
	prompts       PromptBuilder
	systemPrompts SystemPromptRenderer
	tools         ToolRegistry
	memory        MemoryManager
}

func NewRequestBuilder(
	prompts PromptBuilder,
	systemPrompts SystemPromptRenderer,
	tools ToolRegistry,
	memory MemoryManager,
) RequestBuilder {
	return &requestBuilder{prompts: prompts, systemPrompts: systemPrompts, tools: tools, memory: memory}
}

//nolint:gocritic // Request copied to detach builder-side mutations from caller state.
func (b *requestBuilder) Build(
	ctx context.Context,
	request Request,
	memoryCtx *MemoryContext,
) (RequestBuildOutput, error) {
	promptResult, err := b.buildPromptData(ctx, request)
	if err != nil {
		return RequestBuildOutput{}, err
	}

	toolDefs, err := b.buildToolDefinitions(ctx, request.Agent.Tools)
	if err != nil {
		return RequestBuildOutput{}, NewLLMError(err, ErrCodeToolDefinitions, map[string]any{
			"agent": request.Agent.ID,
		})
	}

	messages, err := b.buildMessages(ctx, promptResult.Prompt, memoryCtx, request)
	if err != nil {
		return RequestBuildOutput{}, err
	}
	if err := llmadapter.ValidateConversation(messages); err != nil {
		return RequestBuildOutput{}, NewLLMError(err, ErrCodeInvalidConversation, map[string]any{
			"agent":          request.Agent.ID,
			"action":         request.Action.ID,
			"messages_count": len(messages),
		})
	}

	temperature := request.Agent.Model.Config.Params.Temperature
	toolChoice := ""
	if len(toolDefs) > 0 {
		toolChoice, toolDefs = b.decideToolStrategy(&request, toolDefs)
		if len(toolDefs) == 0 {
			// Preserve empty choice when no tools are advertised so downstream adapters skip the field.
			toolChoice = ""
		}
	}
	forceJSON := b.requiresJSONMode(request, promptResult.Format)
	logger.FromContext(ctx).Debug("LLM request prepared",
		"agent_id", request.Agent.ID,
		"action_id", request.Action.ID,
		"messages_count", len(messages),
		"tools_count", len(toolDefs),
		"tool_choice", toolChoice,
		"output_format", b.describeOutputFormat(promptResult.Format),
		"force_json", forceJSON,
	)
	llmReq := llmadapter.LLMRequest{
		SystemPrompt: b.enhanceSystemPromptWithBuiltins(ctx, request.Agent.Instructions),
		Messages:     messages,
		Tools:        toolDefs,
		Options: llmadapter.CallOptions{
			Temperature:       temperature,
			MaxTokens:         request.Agent.Model.Config.Params.MaxTokens,
			StopWords:         request.Agent.Model.Config.Params.StopWords,
			ToolChoice:        toolChoice,
			OutputFormat:      promptResult.Format,
			ForceJSON:         forceJSON,
			Provider:          request.Agent.Model.Config.Provider,
			Model:             request.Agent.Model.Config.Model,
			TopP:              request.Agent.Model.Config.Params.TopP,
			TopK:              request.Agent.Model.Config.Params.TopK,
			FrequencyPenalty:  request.Agent.Model.Config.Params.FrequencyPenalty,
			PresencePenalty:   request.Agent.Model.Config.Params.PresencePenalty,
			Seed:              request.Agent.Model.Config.Params.Seed,
			N:                 request.Agent.Model.Config.Params.N,
			CandidateCount:    request.Agent.Model.Config.Params.CandidateCount,
			RepetitionPenalty: request.Agent.Model.Config.Params.RepetitionPenalty,
			MaxLength:         request.Agent.Model.Config.Params.MaxLength,
			MinLength:         request.Agent.Model.Config.Params.MinLength,
			Metadata:          request.Agent.Model.Config.Params.Metadata,
		},
	}
	return RequestBuildOutput{
		Request:        llmReq,
		PromptTemplate: promptResult.Template,
		PromptContext:  promptResult.Context,
	}, nil
}

//nolint:gocritic // Request copied for clarity while inspecting schema and tooling state.
func (b *requestBuilder) requiresJSONMode(request Request, format llmadapter.OutputFormat) bool {
	action := request.Action
	if action == nil || action.OutputSchema == nil {
		return false
	}
	// If we already requested native JSON schema support, only skip forcing JSON when the
	// provider supports structured output natively.
	if format.IsJSONSchema() {
		return !request.Execution.ProviderCaps.StructuredOutput
	}
	return true
}

//nolint:gocritic // Request copied to isolate prompt rendering from caller mutations.
func (b *requestBuilder) buildPromptData(ctx context.Context, request Request) (PromptBuildResult, error) {
	result, err := b.prompts.Build(ctx, PromptBuildInput{
		Action:       request.Action,
		ProviderCaps: request.Execution.ProviderCaps,
		Tools:        request.Agent.Tools,
		Dynamic:      request.Prompt.DynamicContext,
	})
	if err != nil {
		return PromptBuildResult{}, NewLLMError(err, ErrCodePromptBuild, map[string]any{
			"action": request.Action.ID,
		})
	}
	return result, nil
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

//nolint:gocritic // Request copied to construct immutable message slices for the LLM conversation.
func (b *requestBuilder) buildMessages(
	ctx context.Context,
	enhancedPrompt string,
	memoryCtx *MemoryContext,
	request Request,
) ([]llmadapter.Message, error) {
	parts := request.Prompt.Attachments
	if len(parts) > 0 {
		clonedParts, err := llmadapter.CloneContentParts(parts)
		if err != nil {
			return nil, NewLLMError(
				fmt.Errorf("failed to clone attachment parts: %w", err),
				ErrCodeInvalidConversation,
				map[string]any{"attachments": len(parts)},
			)
		}
		parts = clonedParts
	} else {
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
	messages = b.injectKnowledge(ctx, messages, request.Knowledge.Entries)

	return messages, nil
}

func (b *requestBuilder) injectKnowledge(
	ctx context.Context,
	messages []llmadapter.Message,
	entries []KnowledgeEntry,
) []llmadapter.Message {
	if len(entries) == 0 || len(messages) == 0 {
		return messages
	}
	idx := len(messages) - 1
	existing := messages[idx].Content
	combined, injected := combineKnowledgeWithPrompt(existing, entries)
	if !injected {
		return messages
	}
	messages[idx].Content = combined
	logger.FromContext(ctx).Debug(
		"Knowledge context injected into prompt",
		"entries",
		len(entries),
	)
	return messages
}

// decideToolStrategy determines final tool advertisement based on knowledge routing outcomes.
func (b *requestBuilder) decideToolStrategy(
	request *Request,
	defs []llmadapter.ToolDefinition,
) (string, []llmadapter.ToolDefinition) {
	if len(defs) == 0 {
		return "", defs
	}
	allowTools := true
	for i := range request.Knowledge.Entries {
		entry := request.Knowledge.Entries[i]
		switch entry.Status {
		case knowledge.RetrievalStatusEscalated:
			return "auto", defs
		case knowledge.RetrievalStatusFallback:
			allowTools = false
		}
	}
	if allowTools {
		return "auto", defs
	}
	return "none", nil
}

func buildKnowledgeBlock(entries []KnowledgeEntry) string {
	if len(entries) == 0 {
		return ""
	}
	var builder strings.Builder
	for i := range entries {
		e := entries[i]
		label := strings.TrimSpace(e.Retrieval.InjectAs)
		if label == "" {
			label = "Retrieved Knowledge"
			if e.BindingID != "" {
				label = label + " (" + e.BindingID + ")"
			}
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(label)
		builder.WriteString(":\n")
		if len(e.Contexts) == 0 {
			fallback := strings.TrimSpace(e.Retrieval.Fallback)
			if fallback != "" {
				builder.WriteString(fallback)
			}
			continue
		}
		for j := range e.Contexts {
			ctx := e.Contexts[j]
			builder.WriteString(fmt.Sprintf("[%d] score=%.3f", j+1, ctx.Score))
			if ctx.TokenEstimate > 0 {
				builder.WriteString(fmt.Sprintf(" tokens=%d", ctx.TokenEstimate))
			}
			builder.WriteString("\n")
			builder.WriteString(strings.TrimSpace(ctx.Content))
			if j+1 < len(e.Contexts) {
				builder.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func combineKnowledgeWithPrompt(prompt string, entries []KnowledgeEntry) (string, bool) {
	block := buildKnowledgeBlock(entries)
	if block == "" {
		return prompt, false
	}
	if strings.TrimSpace(prompt) == "" {
		return block, true
	}
	return block + "\n\n" + prompt, true
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
			details := map[string]any{"configured_tools": len(tools)}
			if suggestions := b.suggestToolNames(ctx, toolConfig.ID); len(suggestions) > 0 {
				details["suggestions"] = suggestions
			}
			return nil, nil, NewToolError(
				fmt.Errorf("tool not found"),
				ErrCodeToolNotFound,
				toolConfig.ID,
				details,
			)
		}

		def := llmadapter.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  b.toolParametersFor(t, toolConfig),
		}

		defs = append(defs, def)
		included[canonicalToolName(def.Name)] = struct{}{}
	}

	return defs, included, nil
}

func (b *requestBuilder) toolParametersFor(t RegistryTool, cfg *tool.Config) map[string]any {
	if cfg != nil && cfg.InputSchema != nil {
		return normalizeToolParameters(map[string]any(*cfg.InputSchema))
	}
	return normalizeToolParameters(t.ParameterSchema())
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

		params := normalizeToolParameters(rt.ParameterSchema())

		defs = append(defs, llmadapter.ToolDefinition{Name: name, Description: rt.Description(), Parameters: params})
		included[lower] = struct{}{}
	}

	return defs
}

func canonicalToolName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (b *requestBuilder) suggestToolNames(ctx context.Context, missing string) []string {
	if b.tools == nil {
		return nil
	}
	allTools, err := b.tools.ListAll(ctx)
	if err != nil {
		logger.FromContext(ctx).Debug(
			"Failed to list tools for suggestions",
			"error",
			core.RedactError(err),
		)
		return nil
	}
	names := make([]string, 0, len(allTools))
	seen := make(map[string]struct{}, len(allTools))
	for _, tool := range allTools {
		if tool == nil {
			continue
		}
		raw := tool.Name()
		key := canonicalToolName(raw)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, raw)
	}
	return nearestToolNames(missing, names, 3)
}

func nearestToolNames(target string, names []string, limit int) []string {
	target = canonicalToolName(target)
	if target == "" || len(names) == 0 || limit <= 0 {
		return nil
	}
	jw := metrics.NewJaroWinkler()
	lev := metrics.NewLevenshtein()
	type candidate struct {
		name   string
		prefix bool
		jwSim  float64
		lev    int
	}
	candidates := make([]candidate, 0, len(names))
	for _, name := range names {
		clean := canonicalToolName(name)
		if clean == "" {
			continue
		}
		cand := candidate{
			name:   name,
			prefix: strings.HasPrefix(clean, target),
			jwSim:  strutil.Similarity(target, clean, jw),
			lev:    lev.Distance(target, clean),
		}
		candidates = append(candidates, cand)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].prefix != candidates[j].prefix {
			return candidates[i].prefix && !candidates[j].prefix
		}
		if candidates[i].jwSim != candidates[j].jwSim {
			return candidates[i].jwSim > candidates[j].jwSim
		}
		if candidates[i].lev != candidates[j].lev {
			return candidates[i].lev < candidates[j].lev
		}
		return canonicalToolName(candidates[i].name) < canonicalToolName(candidates[j].name)
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	out := make([]string, 0, len(candidates))
	for _, cand := range candidates {
		out = append(out, cand.name)
	}
	return out
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
	maps.Copy(clone, src)
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
	ctx context.Context,
	originalInstructions string,
) string {
	if b.systemPrompts == nil {
		logger.FromContext(ctx).Error("System prompt renderer is not configured")
		return composeSystemPromptFallback(ctx, originalInstructions)
	}
	rendered, err := b.systemPrompts.Render(ctx, originalInstructions)
	if err != nil {
		logger.FromContext(ctx).Error(
			"Failed to render system prompt template",
			"error", core.RedactError(err),
		)
		return composeSystemPromptFallback(ctx, originalInstructions)
	}
	return rendered
}

func loadFallbackTemplate() (*template.Template, error) {
	fallbackTemplateOnce.Do(func() {
		fallbackTemplate, fallbackTemplateErr = template.ParseFS(
			prompts.TemplateFS,
			"templates/system_prompt_with_builtins.tmpl",
		)
	})
	return fallbackTemplate, fallbackTemplateErr
}

func composeSystemPromptFallback(ctx context.Context, instructions string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	tpl, err := loadFallbackTemplate()
	if err != nil {
		logger.FromContext(ctx).Error(
			"Failed to load fallback system prompt template",
			"error", core.RedactError(err),
		)
		return strings.TrimSpace(instructions)
	}
	var buf bytes.Buffer
	data := struct {
		HasInstructions bool
		Instructions    string
	}{
		HasInstructions: strings.TrimSpace(instructions) != "",
		Instructions:    instructions,
	}
	if err := tpl.Execute(&buf, data); err != nil {
		logger.FromContext(ctx).Error(
			"Failed to execute fallback system prompt template",
			"error", core.RedactError(err),
		)
		trimmed := strings.TrimSpace(instructions)
		if trimmed == "" {
			return ""
		}
		return trimmed + "\n"
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		buf.WriteString("\n")
	}
	return buf.String()
}
