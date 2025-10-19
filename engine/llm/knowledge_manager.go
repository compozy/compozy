package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/configutil"
	"github.com/compozy/compozy/engine/knowledge/embedder"
	"github.com/compozy/compozy/engine/knowledge/retriever"
	"github.com/compozy/compozy/engine/knowledge/vectordb"
	orchestratorpkg "github.com/compozy/compozy/engine/llm/orchestrator"
	"github.com/compozy/compozy/pkg/logger"
)

type cachedVectorStore struct {
	store   vectordb.Store
	release func(context.Context) error
}

type knowledgeManager struct {
	resolver              *knowledge.Resolver
	workflowKBs           []knowledge.BaseConfig
	projectBinding        []core.KnowledgeBinding
	workflowBinding       []core.KnowledgeBinding
	inlineBinding         []core.KnowledgeBinding
	runtimeEmbedders      map[string]*knowledge.EmbedderConfig
	runtimeVectorDBs      map[string]*knowledge.VectorDBConfig
	runtimeKnowledgeBases map[string]*knowledge.BaseConfig
	runtimeWorkflowKBs    map[string]*knowledge.BaseConfig
	projectID             string

	embedderMu       sync.RWMutex
	embedderCache    map[string]*embedder.Adapter
	vectorStoreMu    sync.RWMutex
	vectorStoreCache map[string]*cachedVectorStore
}

func newKnowledgeManager(state *knowledgeRuntimeState) *knowledgeManager {
	if state == nil {
		return nil
	}
	return &knowledgeManager{
		resolver:              state.resolver,
		workflowKBs:           state.workflowKBs,
		projectBinding:        state.projectBinding,
		workflowBinding:       state.workflowBinding,
		inlineBinding:         state.inlineBinding,
		runtimeEmbedders:      state.runtimeEmbedders,
		runtimeVectorDBs:      state.runtimeVectorDBs,
		runtimeKnowledgeBases: state.runtimeKnowledgeBases,
		runtimeWorkflowKBs:    state.runtimeWorkflowKBs,
		projectID:             state.projectID,
		embedderCache:         make(map[string]*embedder.Adapter),
		vectorStoreCache:      make(map[string]*cachedVectorStore),
	}
}

func (m *knowledgeManager) resolveKnowledge(
	ctx context.Context,
	agentConfig *agent.Config,
	action *agent.ActionConfig,
) ([]orchestratorpkg.KnowledgeEntry, error) {
	if m == nil || m.resolver == nil || action == nil {
		return nil, nil
	}
	query := buildKnowledgeQuery(action)
	if query == "" {
		return nil, nil
	}
	binding, err := m.resolveKnowledgeBinding(ctx, agentConfig)
	if err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, nil
	}
	m.applyOverrides(binding)
	entry, err := m.retrieveContexts(ctx, binding, query)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	return []orchestratorpkg.KnowledgeEntry{*entry}, nil
}

func (m *knowledgeManager) resolveKnowledgeBinding(
	ctx context.Context,
	agentConfig *agent.Config,
) (*knowledge.ResolvedBinding, error) {
	if m == nil || m.resolver == nil {
		return nil, nil
	}
	inline := m.buildInlineBinding(agentConfig)
	input := knowledge.ResolveInput{
		WorkflowKnowledgeBases: m.workflowKBs,
		ProjectBinding:         m.projectBinding,
		WorkflowBinding:        m.workflowBinding,
		InlineBinding:          inline,
	}
	return m.resolver.Resolve(ctx, &input)
}

func (m *knowledgeManager) buildInlineBinding(agentConfig *agent.Config) []core.KnowledgeBinding {
	if m == nil {
		return nil
	}
	var combined *core.KnowledgeBinding
	merge := func(src []core.KnowledgeBinding) {
		for i := range src {
			clone := src[i].Clone()
			if combined == nil {
				combined = &clone
				continue
			}
			combined.Merge(&clone)
			if clone.ID != "" {
				combined.ID = clone.ID
			}
		}
	}
	merge(m.inlineBinding)
	if agentConfig != nil {
		merge(agentConfig.Knowledge)
	}
	if combined == nil {
		return nil
	}
	return []core.KnowledgeBinding{*combined}
}

func (m *knowledgeManager) applyOverrides(binding *knowledge.ResolvedBinding) {
	if m == nil || binding == nil {
		return
	}
	if id := strings.TrimSpace(binding.Embedder.ID); id != "" {
		if override, ok := m.runtimeEmbedders[id]; ok && override != nil {
			binding.Embedder = *override
		}
	}
	if id := strings.TrimSpace(binding.Vector.ID); id != "" {
		if override, ok := m.runtimeVectorDBs[id]; ok && override != nil {
			binding.Vector = *override
		}
	}
	if id := strings.TrimSpace(binding.KnowledgeBase.ID); id != "" {
		if override, ok := m.runtimeKnowledgeBases[id]; ok && override != nil {
			binding.KnowledgeBase = *override
		} else if override, ok := m.runtimeWorkflowKBs[id]; ok && override != nil {
			binding.KnowledgeBase = *override
		}
	}
}

func (m *knowledgeManager) retrieveContexts(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	query string,
) (*orchestratorpkg.KnowledgeEntry, error) {
	if binding == nil {
		return nil, nil
	}
	embedAdapter, err := m.getOrCreateEmbedder(ctx, &binding.Embedder)
	if err != nil {
		return nil, err
	}
	store, err := m.getOrCreateVectorStore(ctx, &binding.Vector)
	if err != nil {
		return nil, err
	}
	retrievalService, err := retriever.NewService(embedAdapter, store, nil)
	if err != nil {
		return nil, err
	}
	contexts, stage, err := m.runRetrievalStages(ctx, retrievalService, binding, query)
	if err != nil {
		return nil, err
	}
	entry := summarizeRetrieval(ctx, binding, contexts, stage)
	return entry, nil
}

func (m *knowledgeManager) runRetrievalStages(
	ctx context.Context,
	svc *retriever.Service,
	binding *knowledge.ResolvedBinding,
	query string,
) ([]knowledge.RetrievedContext, string, error) {
	stages := buildRetrievalStages(query)
	minResults := binding.Retrieval.MinResultsValue()
	for i := range stages {
		stage := stages[i]
		knowledge.RecordRetrievalAttempt(ctx, binding.KnowledgeBase.ID, stage.name)
		stageCtx := retriever.ContextWithStrategy(ctx, strategyForStage(stage.name))
		contexts, err := svc.Retrieve(stageCtx, binding, stage.query)
		if err != nil {
			return nil, "", err
		}
		if len(contexts) >= minResults {
			return contexts, stage.name, nil
		}
		knowledge.RecordRetrievalEmpty(ctx, binding.KnowledgeBase.ID, stage.name)
	}
	return nil, "", nil
}

func summarizeRetrieval(
	ctx context.Context,
	binding *knowledge.ResolvedBinding,
	contexts []knowledge.RetrievedContext,
	stage string,
) *orchestratorpkg.KnowledgeEntry {
	retrieval := binding.Retrieval
	minResults := retrieval.MinResultsValue()
	status := knowledge.RetrievalStatusHit
	notice := ""
	if len(contexts) < minResults {
		notice = strings.TrimSpace(retrieval.Fallback)
		if notice == "" {
			notice = fmt.Sprintf("No indexed knowledge available for %s.", binding.ID)
			retrieval.Fallback = notice
		}
		switch retrieval.ToolFallback {
		case knowledge.ToolFallbackEscalate, knowledge.ToolFallbackAuto:
			status = knowledge.RetrievalStatusEscalated
			knowledge.RecordToolEscalation(ctx, binding.KnowledgeBase.ID)
		default:
			status = knowledge.RetrievalStatusFallback
		}
		contexts = nil
	}
	knowledge.RecordRouterDecision(ctx, binding.KnowledgeBase.ID, string(status))
	logger.FromContext(ctx).Debug(
		"Knowledge retrieval completed",
		"binding_id", binding.ID,
		"status", status,
		"stage", stage,
		"results", len(contexts),
	)
	return &orchestratorpkg.KnowledgeEntry{
		BindingID: binding.ID,
		Retrieval: retrieval,
		Contexts:  contexts,
		Status:    status,
		Notice:    notice,
	}
}

type retrievalStage struct {
	name  string
	query string
}

func buildRetrievalStages(query string) []retrievalStage {
	stages := make([]retrievalStage, 0, 3)
	seen := make(map[string]struct{})
	addStage := func(name string, value string) {
		trimmed := trimToRunes(strings.TrimSpace(value), knowledgeQueryMaxPartRunes)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		stages = append(stages, retrievalStage{name: name, query: trimmed})
	}
	addStage(stageInitial, query)
	addStage(stageKeywords, keywordQuery(query))
	addStage(stageFocus, focusQuery(query))
	if len(stages) == 0 {
		addStage(stageFallback, query)
	}
	return stages
}

func strategyForStage(stage string) string {
	switch stage {
	case stageKeywords:
		return retriever.StrategyKeyword
	case stageFocus:
		return retriever.StrategyHybrid
	default:
		return retriever.StrategySimilarity
	}
}

func sanitizeIdentifier(raw string, field string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("knowledge: %s is required", field)
	}
	if trimmed != raw {
		return "", fmt.Errorf("knowledge: %s cannot contain leading or trailing whitespace", field)
	}
	for _, r := range trimmed {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' {
			continue
		}
		return "", fmt.Errorf("knowledge: %s %q contains invalid character %q", field, trimmed, r)
	}
	return trimmed, nil
}

func (m *knowledgeManager) getOrCreateEmbedder(
	ctx context.Context,
	cfg *knowledge.EmbedderConfig,
) (*embedder.Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("knowledge: embedder config is required")
	}
	id, err := sanitizeIdentifier(cfg.ID, "embedder id")
	if err != nil {
		return nil, err
	}
	m.embedderMu.RLock()
	adapter, ok := m.embedderCache[id]
	m.embedderMu.RUnlock()
	if ok {
		return adapter, nil
	}
	cfgCopy := *cfg
	cfgCopy.ID = id
	adapterCfg, err := configutil.ToEmbedderAdapterConfig(&cfgCopy)
	if err != nil {
		return nil, err
	}
	m.embedderMu.Lock()
	defer m.embedderMu.Unlock()
	if adapter, ok := m.embedderCache[id]; ok {
		return adapter, nil
	}
	created, err := embedder.New(ctx, adapterCfg)
	if err != nil {
		return nil, err
	}
	if cacheCfg := cfg.Config.Cache; cacheCfg != nil && cacheCfg.Enabled {
		if err := created.EnableCache(cacheCfg.Size); err != nil {
			return nil, err
		}
	}
	m.embedderCache[id] = created
	return created, nil
}

func (m *knowledgeManager) getOrCreateVectorStore(
	ctx context.Context,
	cfg *knowledge.VectorDBConfig,
) (vectordb.Store, error) {
	if cfg == nil {
		return nil, fmt.Errorf("knowledge: vector store config is required")
	}
	id, err := sanitizeIdentifier(cfg.ID, "vector store id")
	if err != nil {
		return nil, err
	}
	m.vectorStoreMu.RLock()
	cached, ok := m.vectorStoreCache[id]
	m.vectorStoreMu.RUnlock()
	if ok {
		return cached.store, nil
	}
	cfgCopy := *cfg
	cfgCopy.ID = id
	storeCfg, err := configutil.ToVectorStoreConfig(ctx, m.projectID, &cfgCopy)
	if err != nil {
		return nil, err
	}
	store, release, err := vectordb.AcquireShared(ctx, storeCfg)
	if err != nil {
		return nil, err
	}
	m.vectorStoreMu.Lock()
	if existing, ok := m.vectorStoreCache[id]; ok {
		m.vectorStoreMu.Unlock()
		if release != nil {
			if err := release(ctx); err != nil {
				logger.FromContext(ctx).Warn(
					"failed to release vector store handle",
					"vector_id", id,
					"error", err,
				)
			}
		}
		return existing.store, nil
	}
	m.vectorStoreCache[id] = &cachedVectorStore{store: store, release: release}
	m.vectorStoreMu.Unlock()
	return store, nil
}

func (m *knowledgeManager) drainVectorStores() []*cachedVectorStore {
	if m == nil {
		return nil
	}
	m.vectorStoreMu.Lock()
	defer m.vectorStoreMu.Unlock()
	entries := make([]*cachedVectorStore, 0, len(m.vectorStoreCache))
	for key, entry := range m.vectorStoreCache {
		entries = append(entries, entry)
		delete(m.vectorStoreCache, key)
	}
	return entries
}

var knowledgeStopwords = map[string]struct{}{
	"the": {}, "and": {}, "for": {}, "with": {}, "that": {},
	"from": {}, "this": {}, "have": {}, "your": {}, "about": {},
	"into": {}, "only": {}, "which": {}, "would": {}, "their": {},
	"there": {}, "should": {}, "could": {}, "while": {}, "where": {},
	"when": {}, "what": {}, "question": {}, "answer": {}, "please": {},
}

const (
	knowledgeQueryMaxPartRunes = 2048
	retrievalKeywordLimit      = 32
	retrievalFocusMaxRunes     = 512
)

const (
	stageInitial  = "initial"
	stageKeywords = "keywords"
	stageFocus    = "focus"
	stageFallback = "fallback"
)

func keywordQuery(query string) string {
	lowered := strings.ToLower(query)
	tokens := strings.FieldsFunc(lowered, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	if len(tokens) == 0 {
		return ""
	}
	keywords := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if len(tok) <= 2 {
			continue
		}
		if _, stop := knowledgeStopwords[tok]; stop {
			continue
		}
		keywords = append(keywords, tok)
		if len(keywords) >= retrievalKeywordLimit {
			break
		}
	}
	if len(keywords) == 0 {
		return ""
	}
	return strings.Join(keywords, " ")
}

func focusQuery(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}
	idx := strings.LastIndexAny(trimmed, "?!")
	if idx >= 0 && idx+1 < len(trimmed) {
		candidate := strings.TrimSpace(trimmed[idx+1:])
		if candidate != "" {
			return trimToRunes(candidate, retrievalFocusMaxRunes)
		}
	}
	lines := strings.Split(trimmed, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return trimToRunes(line, retrievalFocusMaxRunes)
		}
	}
	return trimToRunes(trimmed, retrievalFocusMaxRunes)
}

func trimToRunes(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

type knowledgeRuntimeState struct {
	resolver              *knowledge.Resolver
	workflowKBs           []knowledge.BaseConfig
	projectBinding        []core.KnowledgeBinding
	workflowBinding       []core.KnowledgeBinding
	inlineBinding         []core.KnowledgeBinding
	runtimeEmbedders      map[string]*knowledge.EmbedderConfig
	runtimeVectorDBs      map[string]*knowledge.VectorDBConfig
	runtimeKnowledgeBases map[string]*knowledge.BaseConfig
	runtimeWorkflowKBs    map[string]*knowledge.BaseConfig
	projectID             string
}

func newKnowledgeRuntimeState(ctx context.Context, cfg *KnowledgeRuntimeConfig) (*knowledgeRuntimeState, error) {
	result, err := initKnowledgeRuntime(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &knowledgeRuntimeState{}, nil
	}
	return &knowledgeRuntimeState{
		resolver:              result.Resolver,
		workflowKBs:           result.WorkflowKnowledgeBases,
		projectBinding:        result.ProjectBinding,
		workflowBinding:       result.WorkflowBinding,
		inlineBinding:         result.InlineBinding,
		runtimeEmbedders:      result.EmbedderOverrides,
		runtimeVectorDBs:      result.VectorOverrides,
		runtimeKnowledgeBases: result.KnowledgeOverrides,
		runtimeWorkflowKBs:    result.WorkflowKnowledgeOverrides,
		projectID:             result.ProjectID,
	}, nil
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
