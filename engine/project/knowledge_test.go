package project

import (
	"testing"

	"github.com/compozy/compozy/engine/knowledge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubKnowledgeProvider struct {
	defs   []knowledge.BaseConfig
	origin string
}

func (s stubKnowledgeProvider) KnowledgeBaseDefinitions() []knowledge.BaseConfig {
	return s.defs
}

func (s stubKnowledgeProvider) KnowledgeBaseProviderName() string {
	return s.origin
}

func TestAggregatedKnowledgeBases(t *testing.T) {
	t.Run("Should aggregate project and provider bases", func(t *testing.T) {
		projectConfig := &Config{
			KnowledgeBases: []knowledge.BaseConfig{
				{ID: "project_manual"},
				{ID: "project_on_start", Ingest: knowledge.IngestOnStart},
			},
		}
		providers := []KnowledgeBaseProvider{
			stubKnowledgeProvider{
				defs: []knowledge.BaseConfig{
					{ID: "workflow_default"},
					{ID: "workflow_on_start", Ingest: knowledge.IngestOnStart},
				},
				origin: "workflow \"tickets\"",
			},
		}
		refs := projectConfig.AggregatedKnowledgeBases(providers...)
		require.Len(t, refs, 4)
		assert.Equal(t, "project", refs[0].Origin)
		assert.Equal(t, knowledge.IngestManual, refs[0].Base.Ingest)
		assert.Equal(t, knowledge.IngestOnStart, refs[1].Base.Ingest)
		assert.Equal(t, "project_on_start", refs[1].Base.ID)
		assert.Equal(t, "workflow \"tickets\"", refs[2].Origin)
		assert.Equal(t, knowledge.IngestManual, refs[2].Base.Ingest)
		assert.Equal(t, knowledge.IngestOnStart, refs[3].Base.Ingest)
		assert.Equal(t, "workflow_on_start", refs[3].Base.ID)
		// ensure original config remains unchanged
		assert.Equal(t, knowledge.IngestMode(""), projectConfig.KnowledgeBases[0].Ingest)
	})
}
