package knowledge

import "github.com/compozy/compozy/engine/core"

// BindingOption is a functional option for configuring KnowledgeBinding
type BindingOption func(*core.KnowledgeBinding)

// WithBindingTopK sets the TopK field for knowledge binding
func WithBindingTopK(topK *int) BindingOption {
	return func(cfg *core.KnowledgeBinding) {
		cfg.TopK = topK
	}
}

// WithBindingMinScore sets the MinScore field for knowledge binding
func WithBindingMinScore(minScore *float64) BindingOption {
	return func(cfg *core.KnowledgeBinding) {
		cfg.MinScore = minScore
	}
}

// WithBindingMaxTokens sets the MaxTokens field for knowledge binding
func WithBindingMaxTokens(maxTokens *int) BindingOption {
	return func(cfg *core.KnowledgeBinding) {
		cfg.MaxTokens = maxTokens
	}
}

// WithBindingInjectAs sets the InjectAs field for knowledge binding
func WithBindingInjectAs(injectAs string) BindingOption {
	return func(cfg *core.KnowledgeBinding) {
		cfg.InjectAs = injectAs
	}
}

// WithBindingFallback sets the Fallback field for knowledge binding
func WithBindingFallback(fallback string) BindingOption {
	return func(cfg *core.KnowledgeBinding) {
		cfg.Fallback = fallback
	}
}

// WithBindingFilters sets the Filters field for knowledge binding
func WithBindingFilters(filters map[string]string) BindingOption {
	return func(cfg *core.KnowledgeBinding) {
		cfg.Filters = filters
	}
}
