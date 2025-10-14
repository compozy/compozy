package project

import (
	"strings"

	"github.com/compozy/compozy/engine/knowledge"
)

// KnowledgeBaseRef represents a resolved knowledge base and its origin.
type KnowledgeBaseRef struct {
	Base   knowledge.BaseConfig
	Origin string
}

// KnowledgeBaseProvider exposes workflow-scoped knowledge bases for aggregation.
type KnowledgeBaseProvider interface {
	KnowledgeBaseDefinitions() []knowledge.BaseConfig
	KnowledgeBaseProviderName() string
}

// AggregatedKnowledgeBases merges project-level knowledge bases with those provided by workflows.
// Ingest mode defaults to manual when unspecified to keep downstream logic consistent.
func (p *Config) AggregatedKnowledgeBases(providers ...KnowledgeBaseProvider) []KnowledgeBaseRef {
	if p == nil {
		return nil
	}
	result := make([]KnowledgeBaseRef, 0, len(p.KnowledgeBases))
	appendBase := func(base knowledge.BaseConfig, origin string) {
		kbCopy := base
		kbCopy.ID = strings.TrimSpace(kbCopy.ID)
		if kbCopy.Ingest == "" {
			kbCopy.Ingest = knowledge.IngestManual
		}
		if origin == "" {
			origin = "project"
		}
		result = append(result, KnowledgeBaseRef{Base: kbCopy, Origin: origin})
	}
	for i := range p.KnowledgeBases {
		appendBase(p.KnowledgeBases[i], "project")
	}
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		origin := strings.TrimSpace(provider.KnowledgeBaseProviderName())
		if origin == "" {
			origin = "workflow"
		}
		defs := provider.KnowledgeBaseDefinitions()
		if len(defs) == 0 {
			continue
		}
		for i := range defs {
			appendBase(defs[i], origin)
		}
	}
	return result
}
