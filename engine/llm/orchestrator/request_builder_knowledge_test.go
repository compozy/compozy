package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/knowledge"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
)

type knowledgePromptBuilder struct{ prompt string }

func (p knowledgePromptBuilder) Build(context.Context, *agent.ActionConfig) (string, error) {
	return p.prompt, nil
}

func (p knowledgePromptBuilder) EnhanceForStructuredOutput(
	_ context.Context,
	prompt string,
	_ *schema.Schema,
	_ bool,
) string {
	return prompt
}

func (knowledgePromptBuilder) ShouldUseStructuredOutput(string, *agent.ActionConfig, []tool.Config) bool {
	return false
}

type stubKnowledgeMemory struct{}

func (stubKnowledgeMemory) Prepare(context.Context, Request) *MemoryContext {
	return &MemoryContext{}
}

func (stubKnowledgeMemory) Inject(
	_ context.Context,
	base []llmadapter.Message,
	_ *MemoryContext,
) []llmadapter.Message {
	mem := llmadapter.Message{Role: "system", Content: "memory"}
	return append([]llmadapter.Message{mem}, base...)
}

func (stubKnowledgeMemory) StoreAsync(
	context.Context,
	*MemoryContext,
	*llmadapter.LLMResponse,
	[]llmadapter.Message,
	Request,
) {
}

func TestRequestBuilder_ShouldInjectKnowledgeBeforePrompt(t *testing.T) {
	rb := &requestBuilder{
		prompts: knowledgePromptBuilder{prompt: "Answer the question."},
		memory:  stubKnowledgeMemory{},
	}
	entry := KnowledgeEntry{
		BindingID: "support_docs",
		Retrieval: knowledge.RetrievalConfig{InjectAs: "Support Docs"},
		Contexts: []knowledge.RetrievedContext{{
			BindingID:     "support_docs",
			Content:       "Reset the router and try again.",
			Score:         0.9123,
			TokenEstimate: 42,
		}},
	}
	req := Request{
		Agent:     &agent.Config{ID: "agent"},
		Action:    &agent.ActionConfig{ID: "action"},
		Knowledge: []KnowledgeEntry{entry},
	}
	llmReq, err := rb.Build(context.Background(), req, &MemoryContext{})
	require.NoError(t, err)
	require.Len(t, llmReq.Messages, 2)
	assert.Equal(t, "memory", llmReq.Messages[0].Content)
	user := llmReq.Messages[1].Content
	require.Contains(t, user, "Support Docs:")
	require.Contains(t, user, "Reset the router and try again.")
	supportIdx := strings.Index(user, "Support Docs:")
	promptIdx := strings.LastIndex(user, "Answer the question.")
	require.True(t, supportIdx >= 0)
	require.True(t, promptIdx > supportIdx)
}

func TestRequestBuilder_ShouldUseFallbackWhenNoContexts(t *testing.T) {
	rb := &requestBuilder{
		prompts: knowledgePromptBuilder{prompt: "Summarize incident."},
		memory:  stubKnowledgeMemory{},
	}
	req := Request{
		Agent:  &agent.Config{ID: "agent"},
		Action: &agent.ActionConfig{ID: "action"},
		Knowledge: []KnowledgeEntry{{
			BindingID: "support_docs",
			Retrieval: knowledge.RetrievalConfig{
				InjectAs: "Support Docs",
				Fallback: "No indexed knowledge available.",
			},
		}},
	}
	llmReq, err := rb.Build(context.Background(), req, &MemoryContext{})
	require.NoError(t, err)
	require.Len(t, llmReq.Messages, 2)
	user := llmReq.Messages[1].Content
	require.Contains(t, user, "No indexed knowledge available.")
	require.Contains(t, user, "Summarize incident.")
}
