package knowledge

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

type Resolver struct {
	defaults      Defaults
	defs          Definitions
	embedderIndex map[string]EmbedderConfig
	vectorIndex   map[string]VectorDBConfig
	projectKBs    map[string]BaseConfig
}

type ResolveInput struct {
	WorkflowKnowledgeBases []BaseConfig
	ProjectBinding         []core.KnowledgeBinding
	WorkflowBinding        []core.KnowledgeBinding
	InlineBinding          []core.KnowledgeBinding
}

type ResolvedBinding struct {
	ID            string
	KnowledgeBase BaseConfig
	Embedder      EmbedderConfig
	Vector        VectorDBConfig
	Retrieval     RetrievalConfig
}

func NewResolver(defs Definitions, defaults Defaults) (*Resolver, error) {
	defaults = sanitizeDefaults(defaults)
	defs.NormalizeWithDefaults(defaults)
	if err := defs.Validate(); err != nil {
		return nil, fmt.Errorf("knowledge: invalid project definitions: %w", err)
	}
	r := &Resolver{
		defaults:      defaults,
		defs:          defs,
		embedderIndex: make(map[string]EmbedderConfig, len(defs.Embedders)),
		vectorIndex:   make(map[string]VectorDBConfig, len(defs.VectorDBs)),
		projectKBs:    make(map[string]BaseConfig, len(defs.KnowledgeBases)),
	}
	for i := range defs.Embedders {
		embedder := defs.Embedders[i]
		r.embedderIndex[embedder.ID] = embedder
	}
	for i := range defs.VectorDBs {
		vector := defs.VectorDBs[i]
		r.vectorIndex[vector.ID] = vector
	}
	for i := range defs.KnowledgeBases {
		kb := defs.KnowledgeBases[i]
		r.projectKBs[kb.ID] = kb
	}
	return r, nil
}

func (r *Resolver) Resolve(input *ResolveInput) (*ResolvedBinding, error) {
	if input == nil {
		return nil, nil
	}
	if err := r.validateWorkflowDefinitions(input.WorkflowKnowledgeBases); err != nil {
		return nil, err
	}
	projectBinding, err := singleBinding("project", input.ProjectBinding)
	if err != nil {
		return nil, err
	}
	workflowBinding, err := singleBinding("workflow", input.WorkflowBinding)
	if err != nil {
		return nil, err
	}
	inlineBinding, err := singleBinding("inline", input.InlineBinding)
	if err != nil {
		return nil, err
	}
	finalBinding := mergeBindings(projectBinding, workflowBinding, inlineBinding)
	if finalBinding == nil {
		return nil, nil
	}
	if finalBinding.ID == "" {
		return nil, fmt.Errorf("knowledge: binding is missing required id reference")
	}
	kb, err := r.resolveKnowledgeBase(finalBinding.ID, input.WorkflowKnowledgeBases)
	if err != nil {
		return nil, err
	}
	embedder, err := r.resolveEmbedder(kb.Embedder)
	if err != nil {
		return nil, err
	}
	vector, err := r.resolveVector(kb.VectorDB)
	if err != nil {
		return nil, err
	}
	retrieval := applyOverrides(kb.Retrieval, finalBinding)
	result := &ResolvedBinding{
		ID:            finalBinding.ID,
		KnowledgeBase: kb,
		Embedder:      embedder,
		Vector:        vector,
		Retrieval:     retrieval,
	}
	return result, nil
}

func (r *Resolver) validateWorkflowDefinitions(kbs []BaseConfig) error {
	if len(kbs) == 0 {
		return nil
	}
	defs := Definitions{
		Embedders:      r.defs.Embedders,
		VectorDBs:      r.defs.VectorDBs,
		KnowledgeBases: append([]BaseConfig(nil), kbs...),
	}
	defs.NormalizeWithDefaults(r.defaults)
	if err := defs.Validate(); err != nil {
		return fmt.Errorf("knowledge: workflow knowledge base validation failed: %w", err)
	}
	return nil
}

func (r *Resolver) resolveKnowledgeBase(id string, workflowKBs []BaseConfig) (BaseConfig, error) {
	for i := range workflowKBs {
		if workflowKBs[i].ID == id {
			return workflowKBs[i], nil
		}
	}
	kb, ok := r.projectKBs[id]
	if !ok {
		return BaseConfig{}, fmt.Errorf("knowledge: knowledge_base %q not found", id)
	}
	return kb, nil
}

func (r *Resolver) resolveEmbedder(id string) (EmbedderConfig, error) {
	embedder, ok := r.embedderIndex[id]
	if !ok {
		return EmbedderConfig{}, fmt.Errorf("knowledge: embedder %q not defined", id)
	}
	return embedder, nil
}

func (r *Resolver) resolveVector(id string) (VectorDBConfig, error) {
	vector, ok := r.vectorIndex[id]
	if !ok {
		return VectorDBConfig{}, fmt.Errorf("knowledge: vector_db %q not defined", id)
	}
	return vector, nil
}

func singleBinding(scope string, bindings []core.KnowledgeBinding) (*core.KnowledgeBinding, error) {
	if len(bindings) == 0 {
		return nil, nil
	}
	if len(bindings) > 1 {
		return nil, fmt.Errorf("knowledge: %s scope supports only one binding in MVP", scope)
	}
	clone := bindings[0].Clone()
	return &clone, nil
}

func mergeBindings(project, workflow, inline *core.KnowledgeBinding) *core.KnowledgeBinding {
	chain := []*core.KnowledgeBinding{project, workflow, inline}
	var resolved *core.KnowledgeBinding
	for _, candidate := range chain {
		if candidate == nil {
			continue
		}
		clone := candidate.Clone()
		if resolved == nil {
			resolved = &clone
			continue
		}
		if clone.ID != "" && resolved.ID != "" && clone.ID != resolved.ID {
			resolved = &clone
			continue
		}
		if clone.ID != "" {
			resolved.ID = clone.ID
		}
		resolved.Merge(&clone)
	}
	return resolved
}

func applyOverrides(base RetrievalConfig, binding *core.KnowledgeBinding) RetrievalConfig {
	if binding == nil {
		return base
	}
	baseBinding := bindingFromRetrieval(base)
	baseBinding.Merge(binding)
	return retrievalFromBinding(base, &baseBinding)
}

func bindingFromRetrieval(cfg RetrievalConfig) core.KnowledgeBinding {
	result := core.KnowledgeBinding{}
	topK := cfg.TopK
	result.TopK = &topK
	minScore := cfg.MinScore
	result.MinScore = &minScore
	maxTokens := cfg.MaxTokens
	result.MaxTokens = &maxTokens
	result.InjectAs = cfg.InjectAs
	result.Fallback = cfg.Fallback
	result.Filters = core.CopyStringMap(cfg.Filters)
	return result
}

func retrievalFromBinding(base RetrievalConfig, binding *core.KnowledgeBinding) RetrievalConfig {
	result := base
	if binding.TopK != nil {
		result.TopK = *binding.TopK
	}
	if binding.MinScore != nil {
		result.MinScore = *binding.MinScore
	}
	if binding.MaxTokens != nil {
		result.MaxTokens = *binding.MaxTokens
	}
	if binding.InjectAs != "" {
		result.InjectAs = binding.InjectAs
	}
	if binding.Fallback != "" {
		result.Fallback = binding.Fallback
	}
	switch {
	case binding.Filters != nil:
		result.Filters = core.CopyStringMap(binding.Filters)
	case len(base.Filters) > 0:
		result.Filters = core.CopyStringMap(base.Filters)
	default:
		result.Filters = nil
	}
	return result
}
