package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/configutil"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/retriever"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	orchestratorpkg "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/builtin"
	"github.com/compozy/compozy/engine/tool/native"
	appconfig "github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const directPromptActionID = "direct-prompt"

// Service provides LLM integration capabilities using clean architecture
type Service struct {
	orchestrator             orchestratorpkg.Orchestrator
	config                   *Config
	toolRegistry             ToolRegistry
	knowledgeResolver        *knowledge.Resolver
	knowledgeWorkflowKBs     []knowledge.BaseConfig
	knowledgeProjectBinding  []core.KnowledgeBinding
	knowledgeWorkflowBinding []core.KnowledgeBinding
	knowledgeInlineBinding   []core.KnowledgeBinding
	knowledgeProjectID       string
	closeCtx                 context.Context
	cacheMu                  sync.RWMutex
	embedderCache            map[string]*embedder.Adapter
	vectorStoreCache         map[string]vectordb.Store
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

func findReservedPrefix(configs []tool.Config) (string, bool) {
	for i := range configs {
		id := strings.TrimSpace(configs[i].ID)
		if strings.HasPrefix(id, "cp__") {
			return id, true
		}
	}
	return "", false
}

func registerNativeBuiltins(ctx context.Context, registry ToolRegistry) (*builtin.Result, error) {
	definitions := native.Definitions()
	registerFn := func(registerCtx context.Context, tool builtin.Tool) error {
		return registry.Register(registerCtx, builtinRegistryAdapter{tool: tool})
	}
	return builtin.RegisterBuiltins(ctx, registerFn, builtin.Options{Definitions: definitions})
}

func logNativeTools(ctx context.Context, cfg *appconfig.NativeToolsConfig, result *builtin.Result) {
	log := logger.FromContext(ctx)
	execAllowlistCount := 0
	ids := []string{}
	if result != nil {
		execAllowlistCount = len(result.ExecCommands)
		ids = append(ids, result.RegisteredIDs...)
	}
	if cfg != nil && cfg.Enabled && len(ids) > 0 {
		log.Info(
			"Native builtin tools registered",
			"count",
			len(ids),
			"ids",
			ids,
			"exec_allowlist_count",
			execAllowlistCount,
			"root_dir",
			cfg.RootDir,
			"fetch_timeout_ms",
			cfg.Fetch.Timeout.Milliseconds(),
			"fetch_max_body_bytes",
			cfg.Fetch.MaxBodyBytes,
		)
		return
	}
	enabled := false
	if cfg != nil {
		enabled = cfg.Enabled
	}
	log.Info("Native builtin tools disabled", "enabled", enabled, "exec_allowlist_count", execAllowlistCount)
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
	result, err := registerNativeBuiltins(ctx, registry)
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
	logNativeTools(ctx, &nativeCfg, result)
	registerRuntimeTools(ctx, registry, runtime, tools)
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
) {
	log := logger.FromContext(ctx)
	for i := range configs {
		localTool := NewLocalToolAdapter(&configs[i], &runtimeAdapter{manager: runtime})
		if err := registry.Register(ctx, localTool); err != nil {
			log.Warn("Failed to register local tool", "tool", configs[i].ID, "error", err)
		}
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

// NewService creates a new LLM service with clean architecture
func NewService(ctx context.Context, runtime runtime.Runtime, agent *agent.Config, opts ...Option) (*Service, error) {
	log := logger.FromContext(ctx)
	// Build configuration
	config := DefaultConfig()
	// Context-first: merge application config when available
	if ac := appconfig.FromContext(ctx); ac != nil {
		WithAppConfig(ac)(config)
	}
	for _, opt := range opts {
		opt(config)
	}
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	knowledgeResolver,
		knowledgeWorkflowKBs,
		knowledgeProjectBinding,
		knowledgeWorkflowBinding,
		knowledgeInlineBinding,
		knowledgeProjectID,
		err := initKnowledgeRuntime(
		ctx,
		config.Knowledge,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize knowledge resolver: %w", err)
	}
	mcpClient, err := setupMCPClient(ctx, config, agent)
	if err != nil {
		return nil, err
	}
	// Create tool registry
	toolRegistry := NewToolRegistry(ToolRegistryConfig{
		ProxyClient:     mcpClient,
		CacheTTL:        config.CacheTTL,
		AllowedMCPNames: config.AllowedMCPNames,
	})
	if err := configureToolRegistry(ctx, toolRegistry, runtime, agent, config); err != nil {
		return nil, err
	}
	// Create components
	promptBuilder := NewPromptBuilder()
	// Create orchestrator
	orchestratorConfig := orchestratorpkg.Config{
		ToolRegistry:                  &orchestratorToolRegistryAdapter{registry: toolRegistry},
		PromptBuilder:                 promptBuilder,
		RuntimeManager:                runtime,
		LLMFactory:                    config.LLMFactory,
		MemoryProvider:                config.MemoryProvider,
		MemorySync:                    NewMemorySync(),
		Timeout:                       config.Timeout,
		MaxConcurrentTools:            config.MaxConcurrentTools,
		MaxToolIterations:             config.MaxToolIterations,
		MaxSequentialToolErrors:       config.MaxSequentialToolErrors,
		MaxConsecutiveSuccesses:       config.MaxConsecutiveSuccesses,
		EnableProgressTracking:        config.EnableProgressTracking,
		NoProgressThreshold:           config.NoProgressThreshold,
		StructuredOutputRetryAttempts: config.StructuredOutputRetryAttempts,
		RetryAttempts:                 config.RetryAttempts,
		RetryBackoffBase:              config.RetryBackoffBase,
		RetryBackoffMax:               config.RetryBackoffMax,
		RetryJitter:                   config.RetryJitter,
		ProjectRoot:                   config.ProjectRoot,
	}
	llmOrchestrator, err := orchestratorpkg.New(orchestratorConfig)
	if err != nil {
		if closeErr := toolRegistry.Close(); closeErr != nil {
			log.Warn("Failed to close tool registry after orchestrator init error", "error", core.RedactError(closeErr))
		}
		return nil, fmt.Errorf("failed to create orchestrator: %w", err)
	}
	return &Service{
		orchestrator:             llmOrchestrator,
		config:                   config,
		toolRegistry:             toolRegistry,
		knowledgeResolver:        knowledgeResolver,
		knowledgeWorkflowKBs:     knowledgeWorkflowKBs,
		knowledgeProjectBinding:  knowledgeProjectBinding,
		knowledgeWorkflowBinding: knowledgeWorkflowBinding,
		knowledgeInlineBinding:   knowledgeInlineBinding,
		knowledgeProjectID:       knowledgeProjectID,
		closeCtx:                 context.WithoutCancel(ctx),
		embedderCache:            make(map[string]*embedder.Adapter),
		vectorStoreCache:         make(map[string]vectordb.Store),
	}, nil
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
	if agentConfig == nil {
		return nil, fmt.Errorf("agent config cannot be nil")
	}
	dp := strings.TrimSpace(directPrompt)
	if actionID == "" && dp == "" {
		return nil, fmt.Errorf("either actionID or directPrompt must be provided")
	}

	actionConfig, err := s.buildActionConfig(agentConfig, actionID, dp)
	if err != nil {
		return nil, err
	}

	// Defensive copy to avoid shared mutation
	actionCopy, err := core.DeepCopy(actionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to clone action config: %w", err)
	}
	if taskWith != nil {
		inputCopy, err := core.DeepCopy(taskWith)
		if err != nil {
			return nil, fmt.Errorf("failed to clone task with: %w", err)
		}
		actionCopy.With = inputCopy
	}

	effectiveAgent, err := s.buildEffectiveAgent(agentConfig)
	if err != nil {
		return nil, err
	}

	knowledgeEntries, err := s.resolveKnowledge(ctx, agentConfig, actionCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve knowledge context: %w", err)
	}

	request := orchestratorpkg.Request{
		Agent:           effectiveAgent,
		Action:          actionCopy,
		AttachmentParts: attachmentParts,
		Knowledge:       knowledgeEntries,
	}
	return s.orchestrator.Execute(ctx, request)
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
		// Create a copy so we don't mutate the original action
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
	// Direct prompt only flow
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
	if s.knowledgeResolver == nil || action == nil {
		return nil, nil
	}
	query := strings.TrimSpace(action.Prompt)
	if query == "" {
		return nil, nil
	}
	inline := s.buildInlineKnowledgeBinding(agentConfig)
	input := knowledge.ResolveInput{
		WorkflowKnowledgeBases: s.knowledgeWorkflowKBs,
		ProjectBinding:         s.knowledgeProjectBinding,
		WorkflowBinding:        s.knowledgeWorkflowBinding,
		InlineBinding:          inline,
	}
	binding, err := s.knowledgeResolver.Resolve(&input)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, nil
	}
	entry, err := s.retrieveKnowledgeContexts(ctx, binding, query)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	if len(entry.Contexts) == 0 && strings.TrimSpace(entry.Retrieval.Fallback) == "" {
		return nil, nil
	}
	logger.FromContext(ctx).Debug(
		"Knowledge retrieval completed",
		"binding_id",
		binding.ID,
		"results",
		len(entry.Contexts),
	)
	return []orchestratorpkg.KnowledgeEntry{*entry}, nil
}

func (s *Service) retrieveKnowledgeContexts(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	query string,
) (*orchestratorpkg.KnowledgeEntry, error) {
	if binding == nil {
		return nil, nil
	}
	embedAdapter, err := s.getOrCreateEmbedder(ctx, &binding.Embedder)
	if err != nil {
		return nil, err
	}
	store, err := s.getOrCreateVectorStore(ctx, &binding.Vector)
	if err != nil {
		return nil, err
	}
	retrievalService, err := retriever.NewService(embedAdapter, store, nil)
	if err != nil {
		return nil, err
	}
	contexts, err := retrievalService.Retrieve(ctx, binding, query)
	if err != nil {
		return nil, err
	}
	return &orchestratorpkg.KnowledgeEntry{
		BindingID: binding.ID,
		Retrieval: binding.Retrieval,
		Contexts:  contexts,
	}, nil
}

func (s *Service) getOrCreateEmbedder(ctx context.Context, cfg *knowledge.EmbedderConfig) (*embedder.Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("knowledge: embedder config is required")
	}
	id := strings.TrimSpace(cfg.ID)
	if id == "" {
		return nil, fmt.Errorf("knowledge: embedder id is required")
	}
	s.cacheMu.RLock()
	adapter, ok := s.embedderCache[id]
	s.cacheMu.RUnlock()
	if ok {
		return adapter, nil
	}
	adapterCfg, err := configutil.ToEmbedderAdapterConfig(cfg)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if adapter, ok := s.embedderCache[id]; ok {
		return adapter, nil
	}
	created, err := embedder.New(ctx, adapterCfg)
	if err != nil {
		return nil, err
	}
	s.embedderCache[id] = created
	return created, nil
}

func (s *Service) getOrCreateVectorStore(ctx context.Context, cfg *knowledge.VectorDBConfig) (vectordb.Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("knowledge: vector store config is required")
	}
	id := strings.TrimSpace(cfg.ID)
	if id == "" {
		return nil, fmt.Errorf("knowledge: vector store id is required")
	}
	s.cacheMu.RLock()
	store, ok := s.vectorStoreCache[id]
	s.cacheMu.RUnlock()
	if ok {
		return store, nil
	}
	storeCfg, err := configutil.ToVectorStoreConfig(s.knowledgeProjectID, cfg)
	if err != nil {
		return nil, err
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if store, ok := s.vectorStoreCache[id]; ok {
		return store, nil
	}
	created, err := vectordb.New(ctx, storeCfg)
	if err != nil {
		return nil, err
	}
	s.vectorStoreCache[id] = created
	return created, nil
}

func (s *Service) buildInlineKnowledgeBinding(agentConfig *agent.Config) []core.KnowledgeBinding {
	var combined *core.KnowledgeBinding
	if len(s.knowledgeInlineBinding) > 0 {
		clone := s.knowledgeInlineBinding[0].Clone()
		combined = &clone
	}
	if agentConfig != nil && len(agentConfig.Knowledge) > 0 {
		agentClone := agentConfig.Knowledge[0].Clone()
		if combined == nil {
			combined = &agentClone
		} else {
			combined.Merge(&agentClone)
			if agentClone.ID != "" {
				combined.ID = agentClone.ID
			}
		}
	}
	if combined == nil {
		return nil
	}
	return []core.KnowledgeBinding{*combined}
}

func cloneBindingSlice(src []core.KnowledgeBinding) []core.KnowledgeBinding {
	if len(src) == 0 {
		return nil
	}
	out := make([]core.KnowledgeBinding, len(src))
	for i := range src {
		out[i] = src[i].Clone()
	}
	return out
}

func cloneWorkflowKnowledge(src []knowledge.BaseConfig) []knowledge.BaseConfig {
	if len(src) == 0 {
		return nil
	}
	if copied, err := core.DeepCopy(src); err == nil {
		return copied
	}
	return append([]knowledge.BaseConfig{}, src...)
}

func initKnowledgeRuntime(
	ctx context.Context,
	cfg *KnowledgeRuntimeConfig,
) (
	*knowledge.Resolver,
	[]knowledge.BaseConfig,
	[]core.KnowledgeBinding,
	[]core.KnowledgeBinding,
	[]core.KnowledgeBinding,
	string,
	error,
) {
	if cfg == nil {
		return nil, nil, nil, nil, nil, "", nil
	}
	defsCopy, err := core.DeepCopy(cfg.Definitions)
	if err != nil {
		defsCopy = cfg.Definitions
	}
	resolver, err := knowledge.NewResolver(defsCopy, knowledge.DefaultsFromContext(ctx))
	if err != nil {
		return nil, nil, nil, nil, nil, "", err
	}
	workflowKBs := cloneWorkflowKnowledge(cfg.WorkflowKnowledgeBases)
	projectBinding := cloneBindingSlice(cfg.ProjectBinding)
	workflowBinding := cloneBindingSlice(cfg.WorkflowBinding)
	inlineBinding := cloneBindingSlice(cfg.InlineBinding)
	projectID := strings.TrimSpace(cfg.ProjectID)
	return resolver, workflowKBs, projectBinding, workflowBinding, inlineBinding, projectID, nil
}

func setupMCPClient(ctx context.Context, cfg *Config, agent *agent.Config) (*mcp.Client, error) {
	if cfg == nil || cfg.ProxyURL == "" {
		return nil, nil
	}
	client, err := cfg.CreateMCPClient()
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
	stores := make([]vectordb.Store, 0, len(s.vectorStoreCache))
	s.cacheMu.Lock()
	for key, store := range s.vectorStoreCache {
		stores = append(stores, store)
		delete(s.vectorStoreCache, key)
	}
	s.cacheMu.Unlock()
	if len(stores) > 0 {
		timeoutCtx, cancel := context.WithTimeout(s.closeCtx, 10*time.Second)
		defer cancel()
		for i := range stores {
			if err := stores[i].Close(timeoutCtx); err != nil && result == nil {
				result = err
			}
		}
	}
	if s.toolRegistry != nil {
		if err := s.toolRegistry.Close(); err != nil && result == nil {
			result = err
		}
	}
	if s.orchestrator != nil {
		if err := s.orchestrator.Close(); err != nil && result == nil {
			result = err
		}
	}
	return result
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
	// Keeps the first occurrence of an ID (case-insensitive). Given the
	// collection order, agent-level entries take precedence over workflow ones.
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
	// Convert input to core.Input
	coreInput := core.NewInput(input)
	// Create tool execution ID
	toolExecID, err := core.NewID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate tool execution ID: %w", err)
	}
	// Get config from tool configuration
	config := toolConfig.GetConfig()
	// Execute the tool using the runtime manager (preserve tool env if provided)
	env := core.EnvMap{}
	if toolConfig.Env != nil {
		env = *toolConfig.Env
	}
	return r.manager.ExecuteTool(ctx, toolConfig.ID, toolExecID, &coreInput, config, env)
}
