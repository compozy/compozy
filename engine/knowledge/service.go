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
	var base *core.KnowledgeBinding
	if workflow != nil {
		clone := workflow.Clone()
		base = &clone
	} else if project != nil {
		clone := project.Clone()
		base = &clone
	}
	if inline == nil {
		return base
	}
	override := inline.Clone()
	if base == nil {
		return &override
	}
	if override.ID != "" && override.ID != base.ID {
		return &override
	}
	if override.ID == "" {
		override.ID = base.ID
	}
	if override.TopK == nil {
		override.TopK = base.TopK
	}
	if override.MinScore == nil {
		override.MinScore = base.MinScore
	}
	if override.MaxTokens == nil {
		override.MaxTokens = base.MaxTokens
	}
	if override.InjectAs == "" {
		override.InjectAs = base.InjectAs
	}
	if override.Fallback == "" {
		override.Fallback = base.Fallback
	}
	if len(override.Filters) == 0 && len(base.Filters) > 0 {
		override.Filters = copyFilters(base.Filters)
	}
	return &override
}

func applyOverrides(base RetrievalConfig, binding *core.KnowledgeBinding) RetrievalConfig {
	result := base
	if binding != nil {
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
		if len(binding.Filters) > 0 {
			result.Filters = copyFilters(binding.Filters)
		}
	}
	return result
}

func copyFilters(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
