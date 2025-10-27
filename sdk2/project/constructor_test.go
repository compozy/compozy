package project

import (
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/memory"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	"github.com/compozy/compozy/engine/tool"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_MinimalConfig(t *testing.T) {
	cfg, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "test-project", cfg.Name)
	assert.Len(t, cfg.Workflows, 1)
	assert.Equal(t, "./workflow.yaml", cfg.Workflows[0].Source)
}

func TestNew_FullConfig(t *testing.T) {
	cfg, err := New(t.Context(), "test-project",
		WithVersion("1.0.0"),
		WithDescription("Test project"),
		WithAuthor(core.Author{
			Name:         "Test Author",
			Email:        "test@example.com",
			Organization: "Test Org",
		}),
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithModels([]*core.ProviderConfig{
			{
				Provider: core.ProviderOpenAI,
				Model:    "gpt-4",
				Default:  true,
			},
		}),
		WithTools([]tool.Config{
			{ID: "tool1"},
		}),
		WithMemories([]*memory.Config{
			{ID: "memory1", Resource: string(core.ConfigMemory)},
		}),
		WithEmbedders([]knowledge.EmbedderConfig{
			{ID: "embedder1", Provider: "openai"},
		}),
		WithVectorDBs([]knowledge.VectorDBConfig{
			{ID: "vectordb1", Type: "pinecone"},
		}),
		WithKnowledgeBases([]knowledge.BaseConfig{
			{ID: "kb1"},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "test-project", cfg.Name)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, "Test project", cfg.Description)
	assert.Equal(t, "Test Author", cfg.Author.Name)
	assert.Equal(t, "test@example.com", cfg.Author.Email)
	assert.Equal(t, "Test Org", cfg.Author.Organization)
	assert.Len(t, cfg.Workflows, 1)
	assert.Len(t, cfg.Models, 1)
	assert.True(t, cfg.Models[0].Default)
	assert.Len(t, cfg.Tools, 1)
	assert.Len(t, cfg.Memories, 1)
	assert.Len(t, cfg.Embedders, 1)
	assert.Len(t, cfg.VectorDBs, 1)
	assert.Len(t, cfg.KnowledgeBases, 1)
}

func TestNew_NilContext(t *testing.T) {
	_, err := New(nil, "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context is required")
}

func TestNew_EmptyName(t *testing.T) {
	_, err := New(t.Context(), "",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project name")
}

func TestNew_InvalidName(t *testing.T) {
	_, err := New(t.Context(), "test project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alphanumeric or hyphenated")
}

func TestNew_InvalidVersion(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithVersion("invalid-version"),
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version must be valid semver")
}

func TestNew_InvalidAuthorEmail(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithAuthor(core.Author{
			Name:  "Test",
			Email: "invalid-email",
		}),
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "author email must be valid")
}

func TestNew_NoWorkflows(t *testing.T) {
	_, err := New(t.Context(), "test-project")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one workflow must be registered")
}

func TestNew_EmptyWorkflowSource(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: ""},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "workflow[0] source cannot be empty")
}

func TestNew_MultipleDefaultModels(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithModels([]*core.ProviderConfig{
			{Provider: core.ProviderOpenAI, Model: "gpt-4", Default: true},
			{Provider: core.ProviderAnthropic, Model: "claude-3", Default: true},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one model can be marked as default")
}

func TestNew_DuplicateScheduleIDs(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow1.yaml"},
		}),
		WithSchedules([]*projectschedule.Config{
			{ID: "schedule1", WorkflowID: "./workflow1.yaml"},
			{ID: "schedule1", WorkflowID: "./workflow1.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate schedule id 'schedule1'")
}

func TestNew_ScheduleReferencesUnknownWorkflow(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow1.yaml"},
		}),
		WithSchedules([]*projectschedule.Config{
			{ID: "schedule1", WorkflowID: "./unknown.yaml"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown workflow")
}

func TestNew_DuplicateToolIDs(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithTools([]tool.Config{
			{ID: "tool1"},
			{ID: "tool1"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate tool id 'tool1'")
}

func TestNew_DuplicateMemoryIDs(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithMemories([]*memory.Config{
			{ID: "mem1"},
			{ID: "mem1"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate memory id 'mem1'")
}

func TestNew_MemoryResourceDefaulting(t *testing.T) {
	cfg, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithMemories([]*memory.Config{
			{ID: "mem1"},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, string(core.ConfigMemory), cfg.Memories[0].Resource)
}

func TestNew_MultipleKnowledgeBindings(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithKnowledge([]core.KnowledgeBinding{
			{ID: "kb1"},
			{ID: "kb2"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only one knowledge binding is supported")
}

func TestNew_EmptyKnowledgeBindingID(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithKnowledge([]core.KnowledgeBinding{
			{ID: ""},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "knowledge binding requires an id reference")
}

func TestNew_DuplicateEmbedderIDs(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithEmbedders([]knowledge.EmbedderConfig{
			{ID: "emb1", Provider: "openai"},
			{ID: "emb1", Provider: "cohere"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate embedder id 'emb1'")
}

func TestNew_DuplicateVectorDBIDs(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithVectorDBs([]knowledge.VectorDBConfig{
			{ID: "vdb1", Type: "pinecone"},
			{ID: "vdb1", Type: "qdrant"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate vector_db id 'vdb1'")
}

func TestNew_DuplicateKnowledgeBaseIDs(t *testing.T) {
	_, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithKnowledgeBases([]knowledge.BaseConfig{
			{ID: "kb1"},
			{ID: "kb1"},
		}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate knowledge_base id 'kb1'")
}

func TestNew_WhitespaceTrimming(t *testing.T) {
	cfg, err := New(t.Context(), "  test-project  ",
		WithVersion("  1.0.0  "),
		WithDescription("  Test description  "),
		WithAuthor(core.Author{
			Name:         "  Test Author  ",
			Email:        "  test@example.com  ",
			Organization: "  Test Org  ",
		}),
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "  ./workflow.yaml  "},
		}),
	)
	require.NoError(t, err)
	assert.Equal(t, "test-project", cfg.Name)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, "Test description", cfg.Description)
	assert.Equal(t, "Test Author", cfg.Author.Name)
	assert.Equal(t, "test@example.com", cfg.Author.Email)
	assert.Equal(t, "Test Org", cfg.Author.Organization)
	assert.Equal(t, "./workflow.yaml", cfg.Workflows[0].Source)
}

func TestNew_DeepCopy(t *testing.T) {
	tools := []tool.Config{{ID: "tool1"}}
	cfg1, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithTools(tools),
	)
	require.NoError(t, err)
	tools[0].ID = "modified"
	cfg2, err := New(t.Context(), "test-project",
		WithWorkflows([]*engineproject.WorkflowSourceConfig{
			{Source: "./workflow.yaml"},
		}),
		WithTools([]tool.Config{{ID: "tool1"}}),
	)
	require.NoError(t, err)
	assert.Equal(t, "tool1", cfg1.Tools[0].ID)
	assert.Equal(t, "tool1", cfg2.Tools[0].ID)
}

func TestNew_MultipleValidationErrors(t *testing.T) {
	_, err := New(t.Context(), "",
		WithVersion("invalid"),
		WithAuthor(core.Author{Email: "invalid"}),
	)
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.GreaterOrEqual(t, len(buildErr.Errors), 3)
}

func TestValidateCrossReferences_AgentToolReference(t *testing.T) {
	cfg := &engineproject.Config{
		Tools: []tool.Config{
			{ID: "tool1"},
		},
	}
	agents := []agent.Config{
		{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Tools: []tool.Config{{ID: "tool1"}},
			},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.NoError(t, err)
}

func TestValidateCrossReferences_AgentToolReferenceUnknown(t *testing.T) {
	cfg := &engineproject.Config{
		Tools: []tool.Config{
			{ID: "tool1"},
		},
	}
	agents := []agent.Config{
		{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Tools: []tool.Config{{ID: "unknown-tool"}},
			},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown tool 'unknown-tool'")
}

func TestValidateCrossReferences_AgentMemoryReference(t *testing.T) {
	cfg := &engineproject.Config{
		Memories: []*memory.Config{
			{ID: "mem1"},
		},
	}
	agents := []agent.Config{
		{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{{ID: "mem1"}},
			},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.NoError(t, err)
}

func TestValidateCrossReferences_AgentMemoryReferenceUnknown(t *testing.T) {
	cfg := &engineproject.Config{
		Memories: []*memory.Config{
			{ID: "mem1"},
		},
	}
	agents := []agent.Config{
		{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{{ID: "unknown-mem"}},
			},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown memory 'unknown-mem'")
}

func TestValidateCrossReferences_AgentKnowledgeReference(t *testing.T) {
	cfg := &engineproject.Config{
		KnowledgeBases: []knowledge.BaseConfig{
			{ID: "kb1"},
		},
	}
	agents := []agent.Config{
		{
			ID:        "agent1",
			Knowledge: []core.KnowledgeBinding{{ID: "kb1"}},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.NoError(t, err)
}

func TestValidateCrossReferences_AgentKnowledgeReferenceUnknown(t *testing.T) {
	cfg := &engineproject.Config{
		KnowledgeBases: []knowledge.BaseConfig{
			{ID: "kb1"},
		},
	}
	agents := []agent.Config{
		{
			ID:        "agent1",
			Knowledge: []core.KnowledgeBinding{{ID: "unknown-kb"}},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown knowledge base 'unknown-kb'")
}

func TestValidateCrossReferences_MultipleErrors(t *testing.T) {
	cfg := &engineproject.Config{
		Tools:          []tool.Config{{ID: "tool1"}},
		Memories:       []*memory.Config{{ID: "mem1"}},
		KnowledgeBases: []knowledge.BaseConfig{{ID: "kb1"}},
	}
	agents := []agent.Config{
		{
			ID: "agent1",
			LLMProperties: agent.LLMProperties{
				Tools:  []tool.Config{{ID: "unknown-tool"}},
				Memory: []core.MemoryReference{{ID: "unknown-mem"}},
			},
			Knowledge: []core.KnowledgeBinding{{ID: "unknown-kb"}},
		},
	}
	err := ValidateCrossReferences(cfg, agents, nil)
	require.Error(t, err)
	var buildErr *sdkerrors.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Len(t, buildErr.Errors, 3)
}
