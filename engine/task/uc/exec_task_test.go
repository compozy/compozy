package uc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowedMCPIDs(t *testing.T) {
	t.Run("Should return union of agent and workflow MCP IDs, lowercased and deduped", func(t *testing.T) {
		exec := &ExecuteTask{}
		ag := &agent.Config{LLMProperties: agent.LLMProperties{MCPs: []mcp.Config{
			{ID: "FileSystem"},
			{ID: "github"},
		}}}
		wf := &workflow.Config{MCPs: []mcp.Config{
			{ID: "filesystem"},
			{ID: "Search"},
		}}
		ids := exec.allowedMCPIDs(ag, &ExecuteTaskInput{WorkflowConfig: wf})
		require.NotNil(t, ids)
		// Expect: filesystem, github, search (lowercased, deduped)
		assert.ElementsMatch(t, []string{"filesystem", "github", "search"}, ids)
	})

	t.Run("Should return nil when neither agent nor workflow declares MCPs", func(t *testing.T) {
		exec := &ExecuteTask{}
		agentCfg := &agent.Config{
			Model: agent.Model{Config: core.ProviderConfig{Provider: core.ProviderMock, Model: "mock-model"}},
		}
		ids := exec.allowedMCPIDs(agentCfg, &ExecuteTaskInput{WorkflowConfig: &workflow.Config{}})
		assert.Nil(t, ids)
	})
	t.Run("Should trim spaces and normalize casing", func(t *testing.T) {
		exec := &ExecuteTask{}
		ag := &agent.Config{LLMProperties: agent.LLMProperties{MCPs: []mcp.Config{{ID: "  FileSystem  "}}}}
		wf := &workflow.Config{MCPs: []mcp.Config{{ID: "FILESYSTEM"}}}
		ids := exec.allowedMCPIDs(ag, &ExecuteTaskInput{WorkflowConfig: wf})
		require.NotNil(t, ids)
		assert.ElementsMatch(t, []string{"filesystem"}, ids)
	})
}

func TestNormalizeProviderConfigWithEnv(t *testing.T) {
	t.Run("ShouldResolveTemplatesWithProjectEnv", func(t *testing.T) {
		exec := &ExecuteTask{templateEngine: tplengine.NewEngine(tplengine.FormatJSON)}
		providerCfg := core.ProviderConfig{
			Provider: core.ProviderGroq,
			Model:    "llama-3",
			APIKey:   "{{ .env.GROQ_API_KEY }}",
		}
		taskCfg := &task.Config{BaseConfig: task.BaseConfig{ID: "direct-agent", Type: task.TaskTypeBasic}}
		projCfg := &project.Config{Name: "sync"}
		projCfg.SetEnv(core.EnvMap{"GROQ_API_KEY": "test-secret"})
		input := &ExecuteTaskInput{TaskConfig: taskCfg, ProjectConfig: projCfg}
		require.NoError(t, exec.normalizeProviderConfigWithEnv(context.Background(), &providerCfg, input))
		assert.Equal(t, "test-secret", providerCfg.APIKey)
		require.NotNil(t, taskCfg.Env)
		assert.Equal(t, "test-secret", (*taskCfg.Env)["GROQ_API_KEY"])
	})
}

func TestBuildKnowledgeRuntimeConfigRendersEmbedderTemplates(t *testing.T) {
	t.Run("Should build runtime config and resolve embedder templates", func(t *testing.T) {
		exec := &ExecuteTask{templateEngine: tplengine.NewEngine(tplengine.FormatJSON)}
		storePath := filepath.Join(t.TempDir(), "vector.store")
		projectCfg := &project.Config{
			Name: "demo",
			Embedders: []knowledge.EmbedderConfig{{
				ID:       "openai_default",
				Provider: "openai",
				Model:    "text-embedding-3-small",
				APIKey:   "{{ .env.OPENAI_API_KEY }}",
				Config: knowledge.EmbedderRuntimeConfig{
					Dimension: 1536,
					BatchSize: 64,
				},
			}},
			VectorDBs: []knowledge.VectorDBConfig{{
				ID:   "filesystem",
				Type: knowledge.VectorDBTypeFilesystem,
				Config: knowledge.VectorDBConnConfig{
					Path:      storePath,
					Dimension: 1536,
				},
			}},
			KnowledgeBases: []knowledge.BaseConfig{{
				ID:       "kb",
				Embedder: "openai_default",
				VectorDB: "filesystem",
				Sources: []knowledge.SourceConfig{{
					Type: knowledge.SourceTypePDFURL,
					URLs: []string{"https://example.com/example.pdf"},
				}},
			}},
		}
		projectCfg.SetEnv(core.EnvMap{"OPENAI_API_KEY": "resolved-secret"})
		input := &ExecuteTaskInput{ProjectConfig: projectCfg}

		cfg, err := exec.buildKnowledgeRuntimeConfig(context.Background(), input)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		require.Len(t, cfg.Definitions.Embedders, 1)
		assert.Equal(t, "{{ .env.OPENAI_API_KEY }}", cfg.Definitions.Embedders[0].APIKey)
		require.NotNil(t, cfg.RuntimeEmbedders)
		resolved, ok := cfg.RuntimeEmbedders["openai_default"]
		require.True(t, ok, "runtime embedder should be resolved")
		assert.Equal(t, "resolved-secret", resolved.APIKey)
	})
}

func TestNewKnowledgeRuntimeConfigAggregatesKnowledgeBases(t *testing.T) {
	t.Run("Should aggregate trimmed project and workflow knowledge bases", func(t *testing.T) {
		storePath := filepath.Join(t.TempDir(), "vector.store")
		projectCfg := &project.Config{
			Name: "demo",
			KnowledgeBases: []knowledge.BaseConfig{{
				ID:       "  project_kb  ",
				Embedder: "embedder",
				VectorDB: "vector",
				Sources:  []knowledge.SourceConfig{{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"}},
			}},
			Embedders: []knowledge.EmbedderConfig{
				{
					ID:       "embedder",
					Provider: "openai",
					Model:    "text",
					Config:   knowledge.EmbedderRuntimeConfig{Dimension: 1},
				},
			},
			VectorDBs: []knowledge.VectorDBConfig{
				{
					ID:   "vector",
					Type: knowledge.VectorDBTypeFilesystem,
					Config: knowledge.VectorDBConnConfig{
						Path:      storePath,
						Dimension: 1,
					},
				},
			},
		}
		workflowCfg := &workflow.Config{
			ID: "wf",
			KnowledgeBases: []knowledge.BaseConfig{{
				ID:       " wf_kb ",
				Embedder: "embedder",
				VectorDB: "vector",
				Sources:  []knowledge.SourceConfig{{Type: knowledge.SourceTypeMarkdownGlob, Path: "docs/**/*.md"}},
			}},
		}
		cfg := newKnowledgeRuntimeConfig(&ExecuteTaskInput{ProjectConfig: projectCfg, WorkflowConfig: workflowCfg})
		require.NotNil(t, cfg)
		require.Len(t, cfg.Definitions.KnowledgeBases, 1)
		assert.Equal(t, "project_kb", cfg.Definitions.KnowledgeBases[0].ID)
		assert.Equal(t, knowledge.IngestManual, cfg.Definitions.KnowledgeBases[0].Ingest)
		require.Len(t, cfg.WorkflowKnowledgeBases, 1)
		assert.Equal(t, "wf_kb", cfg.WorkflowKnowledgeBases[0].ID)
		assert.Equal(t, knowledge.IngestManual, cfg.WorkflowKnowledgeBases[0].Ingest)
	})
}
