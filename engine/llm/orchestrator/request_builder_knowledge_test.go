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
	"github.com/compozy/compozy/engine/llm/telemetry"
)

type knowledgePromptBuilder struct{ prompt string }

//nolint:gocritic // Interface requires value parameter.
func (p knowledgePromptBuilder) Build(_ context.Context, input PromptBuildInput) (PromptBuildResult, error) {
	return PromptBuildResult{
		Prompt:   p.prompt,
		Format:   llmadapter.OutputFormat{},
		Template: staticPromptTemplate(p),
		Context:  input.Dynamic,
	}, nil
}

type staticPromptTemplate struct {
	prompt string
}

func (s staticPromptTemplate) Render(context.Context, PromptDynamicContext) (string, error) {
	return s.prompt, nil
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

func (stubKnowledgeMemory) StoreFailureEpisode(context.Context, *MemoryContext, Request, FailureEpisode) {
}

func (stubKnowledgeMemory) Compact(context.Context, *LoopContext, telemetry.ContextUsage) error {
	return nil
}

func TestRequestBuilder_KnowledgePrompts(t *testing.T) {
	t.Run("ShouldInjectKnowledgeBeforePrompt", func(t *testing.T) {
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
		output, err := rb.Build(context.Background(), req, &MemoryContext{})
		require.NoError(t, err)
		require.Len(t, output.Request.Messages, 2)
		assert.Equal(t, "memory", output.Request.Messages[0].Content)
		user := output.Request.Messages[1].Content
		require.Contains(t, user, "Support Docs:")
		require.Contains(t, user, "Reset the router and try again.")
		supportIdx := strings.Index(user, "Support Docs:")
		promptIdx := strings.LastIndex(user, "Answer the question.")
		require.True(t, supportIdx >= 0)
		require.True(t, promptIdx > supportIdx)
	})

	t.Run("ShouldUseFallbackWhenNoContexts", func(t *testing.T) {
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
		output, err := rb.Build(context.Background(), req, &MemoryContext{})
		require.NoError(t, err)
		require.Len(t, output.Request.Messages, 2)
		user := output.Request.Messages[1].Content
		require.Contains(t, user, "No indexed knowledge available.")
		require.Contains(t, user, "Summarize incident.")
	})
}
