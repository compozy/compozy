package llm

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestratorpkg "github.com/compozy/compozy/engine/llm/orchestrator"
	providermetrics "github.com/compozy/compozy/engine/llm/provider/metrics"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
)

const (
	directPromptActionID     = "direct-prompt"
	closeVectorStoresTimeout = 10 * time.Second
)

// Service provides LLM integration capabilities using clean architecture
type Service struct {
	orchestrator orchestratorpkg.Orchestrator
	config       *Config
	toolRegistry ToolRegistry
	knowledge    *knowledgeManager
	closeCtx     context.Context
}

type builtinRegistryAdapter struct {
	tool builtin.Tool
}

func (a builtinRegistryAdapter) Name() string {
	return a.tool.Name()
}

func (a builtinRegistryAdapter) Description() string {
	return a.tool.Description()
}

func (a builtinRegistryAdapter) Call(ctx context.Context, input string) (string, error) {
	return a.tool.Call(ctx, input)
}

func (a builtinRegistryAdapter) ParameterSchema() map[string]any {
	return a.tool.ParameterSchema()
}

func findReservedPrefix(configs []tool.Config) (string, bool) {
	for i := range configs {
		id := strings.TrimSpace(configs[i].ID)
		if strings.HasPrefix(id, "cp__") {
			return id, true
		}
	}
	return "", false
}

func registerNativeBuiltins(
	ctx context.Context,
	registry ToolRegistry,
	env toolenv.Environment,
) (*builtin.Result, error) {
	definitions := native.Definitions(env)
	registerFn := func(registerCtx context.Context, tool builtin.Tool) error {
		return registry.Register(registerCtx, builtinRegistryAdapter{tool: tool})
	}
	return builtin.RegisterBuiltins(ctx, registerFn, builtin.Options{Definitions: definitions})
}

func logNativeTools(
	ctx context.Context,
	cfg *appconfig.NativeToolsConfig,
	result *builtin.Result,
	userNative []string,
) {
	log := logger.FromContext(ctx)
	execAllowlistCount := 0
	builtinIDs := []string{}
	if result != nil {
		execAllowlistCount = len(result.ExecCommands)
		builtinIDs = append(builtinIDs, result.RegisteredIDs...)
	}
	enabled := cfg != nil && cfg.Enabled
	if enabled || len(builtinIDs) > 0 || len(userNative) > 0 {
		fields := []any{
			"enabled", enabled,
			"builtin_count", len(builtinIDs),
			"builtin_ids", builtinIDs,
			"user_native_count", len(userNative),
			"user_native_ids", userNative,
			"exec_allowlist_count", execAllowlistCount,
		}
		if cfg != nil {
			fields = append(fields,
				"root_dir", cfg.RootDir,
				"fetch_timeout_ms", cfg.Fetch.Timeout.Milliseconds(),
				"fetch_max_body_bytes", cfg.Fetch.MaxBodyBytes,
			)
		}
		log.Info("Native tools registered", fields...)
		return
	}
	log.Info("Native tools disabled", "enabled", enabled, "exec_allowlist_count", execAllowlistCount)
}

func configureToolRegistry(
	ctx context.Context,
	registry ToolRegistry,
	runtime runtime.Runtime,
	agent *agent.Config,
	cfg *Config,
) error {
	log := logger.FromContext(ctx)
	tools := selectTools(agent, cfg)
	if id, conflict := findReservedPrefix(tools); conflict {
		if closeErr := registry.Close(); closeErr != nil {
			log.Warn(
				"Failed to close tool registry after reserved prefix violation",
				"error",
				core.RedactError(closeErr),
			)
		}
		return fmt.Errorf("tool id %s uses reserved cp__ prefix", id)
	}
	result, err := registerNativeBuiltins(ctx, registry, cfg.ToolEnvironment)
	if err != nil {
		if closeErr := registry.Close(); closeErr != nil {
			log.Warn(
				"Failed to close tool registry after builtin registration error",
				"error",
				core.RedactError(closeErr),
			)
		}
		return fmt.Errorf("failed to register builtin tools: %w", err)
	}
	nativeCfg := appconfig.DefaultNativeToolsConfig()
	if appCfg := appconfig.FromContext(ctx); appCfg != nil {
		nativeCfg = appCfg.Runtime.NativeTools
	}
	userNative := registerRuntimeTools(ctx, registry, runtime, tools)
	logNativeTools(ctx, &nativeCfg, result, userNative)
	return nil
}

func selectTools(agent *agent.Config, cfg *Config) []tool.Config {
	if len(cfg.ResolvedTools) > 0 {
		return cfg.ResolvedTools
	}
	if agent != nil && len(agent.Tools) > 0 {
		return agent.Tools
	}
	return nil
}

func registerRuntimeTools(
	ctx context.Context,
	registry ToolRegistry,
	runtime runtime.Runtime,
	configs []tool.Config,
) []string {
	log := logger.FromContext(ctx)
	userNativeIDs := make([]string, 0)
	for i := range configs {
		cfg := &configs[i]
		if cfg.IsNative() {
			userNativeIDs = append(userNativeIDs, cfg.ID)
			nativeTool := NewNativeToolAdapter(cfg)
			if err := registry.Register(ctx, nativeTool); err != nil {
				log.Warn("Failed to register native tool", "tool", cfg.ID, "error", err)
			}
			continue
		}
		localTool := NewLocalToolAdapter(cfg, &runtimeAdapter{manager: runtime})
		if err := registry.Register(ctx, localTool); err != nil {
			log.Warn("Failed to register local tool", "tool", cfg.ID, "error", err)
		}
	}
	return userNativeIDs
}

func assembleOrchestratorConfig(
	config *Config,
	runtime runtime.Runtime,
	promptBuilder orchestratorpkg.PromptBuilder,
	systemPromptRenderer orchestratorpkg.SystemPromptRenderer,
	toolRegistry ToolRegistry,
) orchestratorpkg.Config {
	var telemetryOpts *telemetry.Options
	if len(config.TelemetryContextWarningThresholds) > 0 {
		thresholds := append([]float64(nil), config.TelemetryContextWarningThresholds...)
		telemetryOpts = &telemetry.Options{ContextWarningThresholds: thresholds}
	}
	return orchestratorpkg.Config{
		ToolRegistry:                   &orchestratorToolRegistryAdapter{registry: toolRegistry},
		PromptBuilder:                  promptBuilder,
		SystemPromptRenderer:           systemPromptRenderer,
		RuntimeManager:                 runtime,
		LLMFactory:                     config.LLMFactory,
		ProviderMetrics:                config.ProviderMetrics,
		RateLimiter:                    config.RateLimiter,
		MemoryProvider:                 config.MemoryProvider,
		MemorySync:                     NewMemorySync(),
		Timeout:                        config.Timeout,
		MaxConcurrentTools:             config.MaxConcurrentTools,
		MaxToolIterations:              config.MaxToolIterations,
		MaxSequentialToolErrors:        config.MaxSequentialToolErrors,
		MaxConsecutiveSuccesses:        config.MaxConsecutiveSuccesses,
		EnableProgressTracking:         config.EnableProgressTracking,
		NoProgressThreshold:            config.NoProgressThreshold,
		EnableLoopRestarts:             config.EnableLoopRestarts,
		RestartStallThreshold:          config.RestartStallThreshold,
		MaxLoopRestarts:                config.MaxLoopRestarts,
		EnableContextCompaction:        config.EnableContextCompaction,
		ContextCompactionThreshold:     config.ContextCompactionThreshold,
		ContextCompactionCooldown:      config.ContextCompactionCooldown,
		EnableDynamicPromptState:       config.EnableDynamicPromptState,
		ToolCallCaps:                   config.ToolCallCaps,
		Middlewares:                    config.OrchestratorMiddlewares,
		FinalizeOutputRetryAttempts:    config.FinalizeOutputRetryAttempts,
		StructuredOutputRetryAttempts:  config.StructuredOutputRetryAttempts,
		RetryAttempts:                  config.RetryAttempts,
		RetryBackoffBase:               config.RetryBackoffBase,
		RetryBackoffMax:                config.RetryBackoffMax,
		RetryJitter:                    config.RetryJitter,
		EnableAgentCallCompletionHints: true,
		ProjectRoot:                    config.ProjectRoot,
		TelemetryOptions:               telemetryOpts,
	}
}

type orchestratorToolRegistryAdapter struct {
	registry ToolRegistry
}

func (a *orchestratorToolRegistryAdapter) Find(ctx context.Context, name string) (orchestratorpkg.RegistryTool, bool) {
	tool, ok := a.registry.Find(ctx, name)
	if !ok {
		return nil, false
	}
	return tool, true
}

func (a *orchestratorToolRegistryAdapter) ListAll(ctx context.Context) ([]orchestratorpkg.RegistryTool, error) {
	tools, err := a.registry.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]orchestratorpkg.RegistryTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, t)
	}
	return out, nil
}

func (a *orchestratorToolRegistryAdapter) Close() error {
	return a.registry.Close()
}

func newServiceInstance(
	ctx context.Context,
	orchestrator orchestratorpkg.Orchestrator,
	config *Config,
	registry ToolRegistry,
	state *knowledgeRuntimeState,
) *Service {
	service := &Service{
		orchestrator: orchestrator,
		config:       config,
		toolRegistry: registry,
		closeCtx:     deriveCloseContext(ctx),
	}
	if state != nil {
		service.knowledge = newKnowledgeManager(state)
	}
	return service
}

func deriveCloseContext(ctx context.Context) context.Context {
	base := context.WithoutCancel(ctx)
	if mgr := appconfig.ManagerFromContext(ctx); mgr != nil {
		base = appconfig.ContextWithManager(base, mgr)
	}
	if log := logger.FromContext(ctx); log != nil {
		base = logger.ContextWithLogger(base, log)
	}
	return base
}

// NewService creates a new LLM service with clean architecture
func NewService(ctx context.Context, runtime runtime.Runtime, agent *agent.Config, opts ...Option) (*Service, error) {
	config, knowledgeState, err := buildServiceConfig(ctx, opts)
	if err != nil {
		return nil, err
	}
	mcpClient, err := setupMCPClient(ctx, config, agent)
	if err != nil {
		return nil, err
	}
	toolRegistry, err := buildToolRegistry(ctx, runtime, agent, config, mcpClient)
	if err != nil {
		return nil, err
	}
	orchestrator, err := createOrchestrator(ctx, runtime, config, toolRegistry)
	if err != nil {
		return nil, err
	}
	return newServiceInstance(ctx, orchestrator, config, toolRegistry, knowledgeState), nil
}

func buildServiceConfig(ctx context.Context, opts []Option) (*Config, *knowledgeRuntimeState, error) {
	config := DefaultConfig()
	if ac := appconfig.FromContext(ctx); ac != nil {
		WithAppConfig(ctx, ac)(config)
	}
	for _, opt := range opts {
		opt(config)
	}
	if config.ProviderMetrics == nil {
		config.ProviderMetrics = providermetrics.Nop()
	}
	if err := config.Validate(ctx); err != nil {
		return nil, nil, fmt.Errorf("invalid configuration: %w", err)
	}
	knowledgeState, err := newKnowledgeRuntimeState(ctx, config.Knowledge)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize knowledge resolver: %w", err)
	}
	if config.LLMFactory != nil {
		return config, knowledgeState, nil
	}
	factory, err := llmadapter.NewDefaultFactory(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build default LLM factory: %w", err)
	}
	config.LLMFactory = factory
	return config, knowledgeState, nil
}

func buildToolRegistry(
	ctx context.Context,
	runtime runtime.Runtime,
	agent *agent.Config,
	config *Config,
	mcpClient *mcp.Client,
) (ToolRegistry, error) {
	log := logger.FromContext(ctx)
	registry, err := NewToolRegistry(ctx, ToolRegistryConfig{
		ProxyClient:     mcpClient,
		CacheTTL:        config.CacheTTL,
		AllowedMCPNames: config.AllowedMCPNames,
		DeniedMCPNames:  config.DeniedMCPNames,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}
	if err := configureToolRegistry(ctx, registry, runtime, agent, config); err != nil {
		if closeErr := registry.Close(); closeErr != nil {
			log.Warn("Failed to close tool registry after configuration error", "error", core.RedactError(closeErr))
		}
		return nil, err
	}
	return registry, nil
}

func createOrchestrator(
	ctx context.Context,
	runtime runtime.Runtime,
	config *Config,
	registry ToolRegistry,
) (orchestratorpkg.Orchestrator, error) {
	log := logger.FromContext(ctx)
	promptBuilder := NewPromptBuilder()
	systemPromptRenderer := NewSystemPromptRenderer()
	orchestratorConfig := assembleOrchestratorConfig(config, runtime, promptBuilder, systemPromptRenderer, registry)
	llmOrchestrator, err := orchestratorpkg.New(orchestratorConfig)
	if err != nil {
		if closeErr := registry.Close(); closeErr != nil {
			log.Warn("Failed to close tool registry after orchestrator init error", "error", core.RedactError(closeErr))
		}
		return nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}
	return llmOrchestrator, nil
}

// GenerateContent generates content using the orchestrator
func (s *Service) GenerateContent(
	ctx context.Context,
	agentConfig *agent.Config,
	taskWith *core.Input,
	actionID string,
	directPrompt string,
	attachmentParts []llmadapter.ContentPart,
) (*core.Output, error) {
	dp, err := validateGenerateInputs(agentConfig, actionID, directPrompt)
	if err != nil {
		return nil, err
	}
	actionCopy, err := s.prepareAction(agentConfig, actionID, dp, taskWith)
	if err != nil {
		return nil, err
	}
	effectiveAgent, err := s.buildEffectiveAgent(agentConfig)
	if err != nil {
		return nil, err
	}
	knowledgeEntries, err := s.resolveKnowledge(ctx, agentConfig, actionCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve knowledge context: %w", err)
	}
	providerCaps, err := s.resolveProviderCapabilities(agentConfig)
	if err != nil {
		return nil, err
	}
	request := s.buildExecutionRequest(effectiveAgent, actionCopy, attachmentParts, knowledgeEntries, providerCaps)
	session := newStreamSession(s.config.Stream)
	if session != nil {
		ctx = telemetry.ContextWithObserver(ctx, session.observer())
		ctx = withStreamSession(ctx, session)
	}
	output, err := s.orchestrator.Execute(ctx, request)
	if session != nil {
		if err != nil {
			session.emitError(ctx, err)
		} else {
			session.finalize(ctx, output)
		}
	}
	return output, err
}

func validateGenerateInputs(agentConfig *agent.Config, actionID string, directPrompt string) (string, error) {
	if agentConfig == nil {
		return "", fmt.Errorf("agent config cannot be nil")
	}
	dp := strings.TrimSpace(directPrompt)
	if actionID == "" && dp == "" {
		return "", fmt.Errorf("either actionID or directPrompt must be provided")
	}
	return dp, nil
}

func (s *Service) prepareAction(
	agentConfig *agent.Config,
	actionID string,
	directPrompt string,
	taskWith *core.Input,
) (*agent.ActionConfig, error) {
	actionConfig, err := s.buildActionConfig(agentConfig, actionID, directPrompt)
	if err != nil {
		return nil, err
	}
	actionCopy, err := core.DeepCopy(actionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to clone action config: %w", err)
	}
	if taskWith == nil {
		return actionCopy, nil
	}
	inputCopy, err := core.DeepCopy(taskWith)
	if err != nil {
		return nil, fmt.Errorf("failed to clone task with: %w", err)
	}
	actionCopy.With = inputCopy
	return actionCopy, nil
}

func (s *Service) resolveProviderCapabilities(agentConfig *agent.Config) (llmadapter.ProviderCapabilities, error) {
	if s.config.LLMFactory == nil {
		return llmadapter.ProviderCapabilities{}, nil
	}
	providerConfig := agentConfig.GetProviderConfig()
	if providerConfig == nil {
		return llmadapter.ProviderCapabilities{}, nil
	}
	caps, err := s.config.LLMFactory.Capabilities(providerConfig.Provider)
	if err != nil {
		return llmadapter.ProviderCapabilities{}, fmt.Errorf("failed to get provider capabilities: %w", err)
	}
	return caps, nil
}

func (s *Service) buildExecutionRequest(
	agentConfig *agent.Config,
	action *agent.ActionConfig,
	attachmentParts []llmadapter.ContentPart,
	knowledgeEntries []orchestratorpkg.KnowledgeEntry,
	providerCaps llmadapter.ProviderCapabilities,
) orchestratorpkg.Request {
	return orchestratorpkg.Request{
		Agent:  agentConfig,
		Action: action,
		Prompt: orchestratorpkg.PromptPayload{
			Attachments: attachmentParts,
		},
		Knowledge: orchestratorpkg.KnowledgePayload{
			Entries: knowledgeEntries,
		},
		Execution: orchestratorpkg.ExecutionOptions{
			ProviderCaps: providerCaps,
		},
	}
}

// buildActionConfig resolves the action configuration from either an action ID
// or a direct prompt, augmenting the prompt when both are provided.
func (s *Service) buildActionConfig(
	agentConfig *agent.Config,
	actionID string,
	directPrompt string,
) (*agent.ActionConfig, error) {
	if actionID != "" {
		ac, err := agent.FindActionConfig(agentConfig.Actions, actionID)
		if err != nil {
			return nil, fmt.Errorf("failed to find action config: %w", err)
		}
		if directPrompt == "" {
			return ac, nil
		}
		acCopy := *ac
		if acCopy.Prompt != "" {
			basePrompt := strings.TrimRight(acCopy.Prompt, "\n")
			acCopy.Prompt = fmt.Sprintf(
				"%s\n\nAdditional context:\n\"\"\"\n%s\n\"\"\"",
				basePrompt,
				directPrompt,
			)
		} else {
			acCopy.Prompt = directPrompt
		}
		return &acCopy, nil
	}
	return &agent.ActionConfig{ID: directPromptActionID, Prompt: directPrompt}, nil
}

// buildEffectiveAgent ensures the LLM is informed about available tools. If the
// agent doesn't declare tools but resolved tools exist (from project/workflow),
// clone the agent and attach those tool definitions for LLM advertisement.
func (s *Service) buildEffectiveAgent(agentConfig *agent.Config) (*agent.Config, error) {
	if agentConfig == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}
	if len(agentConfig.Tools) > 0 || len(s.config.ResolvedTools) == 0 {
		return agentConfig, nil
	}
	if cloned, err := agentConfig.Clone(); err == nil && cloned != nil {
		cloned.Tools = s.config.ResolvedTools
		return cloned, nil
	}
	tmp := *agentConfig
	tmp.Tools = s.config.ResolvedTools
	return &tmp, nil
}

func (s *Service) resolveKnowledge(
	ctx context.Context,
	agentConfig *agent.Config,
	action *agent.ActionConfig,
) ([]orchestratorpkg.KnowledgeEntry, error) {
	if s.knowledge == nil {
		return nil, nil
	}
	return s.knowledge.resolveKnowledge(ctx, agentConfig, action)
}

func buildKnowledgeQuery(action *agent.ActionConfig) string {
	if action == nil {
		return ""
	}
	seen := make(map[string]struct{})
	parts := make([]string, 0)
	if action.With != nil {
		inputMap := action.With.AsMap()
		appendKnowledgeQueryValue(inputMap, &parts, seen)
	}
	prompt := strings.TrimSpace(action.Prompt)
	cleanPrompt := stripTemplateTokens(prompt)
	if cleanPrompt != "" {
		addKnowledgeQueryPart(cleanPrompt, &parts, seen)
	}
	if len(parts) == 0 {
		return prompt
	}
	return strings.Join(parts, "\n\n")
}

func appendKnowledgeQueryValue(
	value any,
	dest *[]string,
	seen map[string]struct{},
) {
	switch v := value.(type) {
	case nil:
		return
	case string:
		addKnowledgeQueryPart(v, dest, seen)
	case fmt.Stringer:
		addKnowledgeQueryPart(v.String(), dest, seen)
	case *core.Input:
		if v != nil {
			appendKnowledgeFromMap(v.AsMap(), dest, seen)
		}
	case map[string]any:
		appendKnowledgeFromMap(v, dest, seen)
	case map[string]string:
		appendKnowledgeFromStringMap(v, dest, seen)
	case []string:
		appendKnowledgeFromStringSlice(v, dest, seen)
	case []any:
		appendKnowledgeFromSlice(v, dest, seen)
	default:
		addKnowledgeQueryPart(fmt.Sprintf("%v", v), dest, seen)
	}
}

func addKnowledgeQueryPart(
	text string,
	dest *[]string,
	seen map[string]struct{},
) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	if tplengine.HasTemplate(trimmed) {
		return
	}
	runes := []rune(trimmed)
	if len(runes) > knowledgeQueryMaxPartRunes {
		trimmed = string(runes[:knowledgeQueryMaxPartRunes])
	}
	if _, exists := seen[trimmed]; exists {
		return
	}
	seen[trimmed] = struct{}{}
	*dest = append(*dest, trimmed)
}

func appendKnowledgeFromMap(
	m map[string]any,
	dest *[]string,
	seen map[string]struct{},
) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		appendKnowledgeQueryValue(m[k], dest, seen)
	}
}

func appendKnowledgeFromStringMap(
	m map[string]string,
	dest *[]string,
	seen map[string]struct{},
) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		addKnowledgeQueryPart(m[k], dest, seen)
	}
}

func appendKnowledgeFromStringSlice(
	values []string,
	dest *[]string,
	seen map[string]struct{},
) {
	for _, v := range values {
		addKnowledgeQueryPart(v, dest, seen)
	}
}

func appendKnowledgeFromSlice(
	values []any,
	dest *[]string,
	seen map[string]struct{},
) {
	for _, v := range values {
		appendKnowledgeQueryValue(v, dest, seen)
	}
}

func stripTemplateTokens(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if !tplengine.HasTemplate(text) {
		return strings.TrimSpace(text)
	}
	var b strings.Builder
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "{{") {
			end := strings.Index(text[i:], "}}")
			if end == -1 {
				break
			}
			i += end + 2
			continue
		}
		b.WriteByte(text[i])
		i++
	}
	clean := strings.TrimSpace(b.String())
	if clean == "" {
		return ""
	}
	return strings.Join(strings.Fields(clean), " ")
}

type knowledgeRuntimeResult struct {
	Resolver                   *knowledge.Resolver
	WorkflowKnowledgeBases     []knowledge.BaseConfig
	ProjectBinding             []core.KnowledgeBinding
	WorkflowBinding            []core.KnowledgeBinding
	InlineBinding              []core.KnowledgeBinding
	EmbedderOverrides          map[string]*knowledge.EmbedderConfig
	VectorOverrides            map[string]*knowledge.VectorDBConfig
	KnowledgeOverrides         map[string]*knowledge.BaseConfig
	WorkflowKnowledgeOverrides map[string]*knowledge.BaseConfig
	ProjectID                  string
}

func initKnowledgeRuntime(
	ctx context.Context,
	cfg *KnowledgeRuntimeConfig,
) (*knowledgeRuntimeResult, error) {
	if cfg == nil {
		return nil, nil
	}
	defsCopy, err := core.DeepCopy(cfg.Definitions)
	if err != nil {
		defsCopy = cfg.Definitions
	}
	resolver, err := knowledge.NewResolver(ctx, defsCopy, knowledge.DefaultsFromContext(ctx))
	if err != nil {
		return nil, err
	}
	result := &knowledgeRuntimeResult{
		Resolver:                   resolver,
		WorkflowKnowledgeBases:     cloneWorkflowKnowledge(cfg.WorkflowKnowledgeBases),
		ProjectBinding:             cloneBindingSlice(cfg.ProjectBinding),
		WorkflowBinding:            cloneBindingSlice(cfg.WorkflowBinding),
		InlineBinding:              cloneBindingSlice(cfg.InlineBinding),
		EmbedderOverrides:          cloneEmbedderOverrides(cfg.RuntimeEmbedders),
		VectorOverrides:            cloneVectorOverrides(cfg.RuntimeVectorDBs),
		KnowledgeOverrides:         cloneKnowledgeOverrides(cfg.RuntimeKnowledgeBases),
		WorkflowKnowledgeOverrides: cloneKnowledgeOverrides(cfg.RuntimeWorkflowKBs),
		ProjectID:                  strings.TrimSpace(cfg.ProjectID),
	}
	return result, nil
}

func setupMCPClient(ctx context.Context, cfg *Config, agent *agent.Config) (*mcp.Client, error) {
	if cfg == nil || cfg.ProxyURL == "" {
		return nil, nil
	}
	client, err := cfg.CreateMCPClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}
	toRegister := collectMCPsToRegister(agent, cfg)
	uniq := dedupeMCPsByID(toRegister)
	if err := registerMCPsWithProxy(ctx, client, uniq, cfg.FailOnMCPRegistrationError); err != nil {
		return nil, err
	}
	return client, nil
}

// InvalidateToolsCache invalidates the tools cache
func (s *Service) InvalidateToolsCache(ctx context.Context) {
	if s.toolRegistry != nil {
		s.toolRegistry.InvalidateCache(ctx)
	}
}

// Close cleans up resources
func (s *Service) Close() error {
	var result error
	entries := s.drainVectorStoreCache()
	if len(entries) > 0 {
		timeoutCtx, cancel := context.WithTimeout(s.closeCtx, closeVectorStoresTimeout)
		defer cancel()
		if err := closeVectorStores(timeoutCtx, entries); err != nil {
			result = err
		}
	}
	if err := s.closeToolRegistry(); err != nil && result == nil {
		result = err
	}
	if err := s.closeOrchestrator(); err != nil && result == nil {
		result = err
	}
	return result
}

func (s *Service) drainVectorStoreCache() []*cachedVectorStore {
	if s.knowledge == nil {
		return nil
	}
	return s.knowledge.drainVectorStores()
}

func closeVectorStores(ctx context.Context, entries []*cachedVectorStore) error {
	var firstErr error
	for i := range entries {
		entry := entries[i]
		if entry == nil {
			continue
		}
		if entry.release != nil {
			if err := entry.release(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
			continue
		}
		if entry.store != nil {
			if err := entry.store.Close(ctx); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (s *Service) closeToolRegistry() error {
	if s.toolRegistry == nil {
		return nil
	}
	return s.toolRegistry.Close()
}

func (s *Service) closeOrchestrator() error {
	if s.orchestrator == nil {
		return nil
	}
	return s.orchestrator.Close()
}

// collectMCPsToRegister combines agent-declared and workflow-level MCPs for
// proxy registration. Precedence: agent-level definitions override workflow
// duplicates (dedupe keeps the first occurrence).
func collectMCPsToRegister(agentCfg *agent.Config, cfg *Config) []mcp.Config {
	var out []mcp.Config
	if agentCfg != nil && len(agentCfg.MCPs) > 0 {
		out = append(out, agentCfg.MCPs...)
	}
	if cfg != nil && len(cfg.RegisterMCPs) > 0 {
		out = append(out, cfg.RegisterMCPs...)
	}
	return out
}

// dedupeMCPsByID removes duplicates using case-insensitive ID comparison.
func dedupeMCPsByID(in []mcp.Config) []mcp.Config {
	seen := make(map[string]struct{})
	out := make([]mcp.Config, 0, len(in))
	for i := range in {
		id := strings.ToLower(strings.TrimSpace(in[i].ID))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, in[i])
	}
	return out
}

// registerMCPsWithProxy registers MCPs via proxy, honoring strict mode.
func registerMCPsWithProxy(ctx context.Context, client *mcp.Client, mcps []mcp.Config, strict bool) error {
	if client == nil || len(mcps) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	reg := mcp.NewRegisterService(client)
	if err := reg.EnsureMultiple(ctx, mcps); err != nil {
		if strict {
			return fmt.Errorf("failed to register MCPs: %w", err)
		}
		logger.FromContext(ctx).
			Warn("Failed to register MCPs; tools may be unavailable", "mcp_count", len(mcps), "error", err)
	}
	return nil
}

// runtimeAdapter adapts runtime.Runtime to the registry.ToolRuntime interface
type runtimeAdapter struct {
	manager runtime.Runtime
}

func (r *runtimeAdapter) ExecuteTool(
	ctx context.Context,
	toolConfig *tool.Config,
	input map[string]any,
) (*core.Output, error) {
	coreInput := core.NewInput(input)
	toolExecID, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate tool execution ID: %w", err)
	}
	config := toolConfig.GetConfig()
	env := core.EnvMap{}
	if toolConfig.Env != nil {
		env = *toolConfig.Env
	}
	return r.manager.ExecuteTool(ctx, toolConfig.ID, toolExecID, &coreInput, config, env)
}
