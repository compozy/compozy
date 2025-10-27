package agent

import (
	"context"
	"fmt"
	"testing"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/sdk/internal/testutil"
	"github.com/compozy/compozy/sdk/knowledge"
	"github.com/compozy/compozy/sdk/memory"
)

func BenchmarkAgentBuilderSimple(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	actions := buildAgentActions(b, setupCtx, 1)
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("agent-simple").WithInstructions("Answer succinctly").WithModel("openai", "gpt-4o-mini")
		for _, action := range actions {
			builder.AddAction(action)
		}
		return builder.Build(ctx)
	})
}

func BenchmarkAgentBuilderMedium(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	actions := buildAgentActions(b, setupCtx, 4)
	memoryRefs := buildMemoryReferences(b, setupCtx, 2)
	knowledgeBinding := buildKnowledgeBinding(b, setupCtx, testutil.BenchmarkID("kb", 1))
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("agent-medium").WithInstructions("Respond with detailed reasoning").WithModel("openai", "gpt-4o-mini")
		builder.AddTool("search")
		builder.AddTool("calendar")
		builder.AddMCP("slack")
		builder.AddMCP("jira")
		builder.WithKnowledge(knowledgeBinding)
		for _, ref := range memoryRefs {
			builder.WithMemory(ref)
		}
		for _, action := range actions {
			builder.AddAction(action)
		}
		return builder.Build(ctx)
	})
}

func BenchmarkAgentBuilderComplex(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	actions := buildAgentActions(b, setupCtx, 8)
	memoryRefs := buildMemoryReferences(b, setupCtx, 4)
	knowledgeBinding := buildKnowledgeBinding(b, setupCtx, testutil.BenchmarkID("kb", 9))
	testutil.RunBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("agent-complex").WithInstructions("Perform multi-step analysis with citations").WithModel("openai", "gpt-4o-mini")
		builder.AddTool("search")
		builder.AddTool("calendar")
		builder.AddTool("notifier")
		builder.AddMCP("slack")
		builder.AddMCP("jira")
		builder.AddMCP("github")
		builder.WithKnowledge(knowledgeBinding)
		for _, ref := range memoryRefs {
			builder.WithMemory(ref)
		}
		for idx, action := range actions {
			builder.AddAction(action)
			if idx%2 == 0 {
				builder.AddTool(fmt.Sprintf("integration-%d", idx))
			}
		}
		return builder.Build(ctx)
	})
}

func BenchmarkAgentBuilderParallel(b *testing.B) {
	setupCtx := testutil.NewBenchmarkContext(b)
	actions := buildAgentActions(b, setupCtx, 2)
	testutil.RunParallelBuilderBenchmark(b, func(ctx context.Context) (any, error) {
		builder := New("agent-parallel").WithInstructions("Handle concurrent requests").WithModel("openai", "gpt-4o-mini")
		builder.AddTool("search")
		for _, action := range actions {
			builder.AddAction(action)
		}
		return builder.Build(ctx)
	})
}

func buildAgentActions(b *testing.B, ctx context.Context, count int) []*engineagent.ActionConfig {
	b.Helper()
	actions := make([]*engineagent.ActionConfig, count)
	for i := 0; i < count; i++ {
		builder := NewAction(testutil.BenchmarkID("action", i)).WithPrompt(fmt.Sprintf("Handle scenario %d", i))
		if i%2 == 0 {
			builder = builder.WithTimeout(30*time.Second).WithRetry(3, time.Second)
		}
		cfg, err := builder.Build(ctx)
		if err != nil {
			b.Fatalf("failed to build action fixture %d: %v", i, err)
		}
		actions[i] = cfg
	}
	return actions
}

func buildMemoryReferences(b *testing.B, ctx context.Context, count int) []*memory.ReferenceConfig {
	b.Helper()
	refs := make([]*memory.ReferenceConfig, count)
	for i := 0; i < count; i++ {
		builder := memory.NewReference(testutil.BenchmarkID("memory", i)).WithKey(fmt.Sprintf("{{ .session[%d] }}", i))
		cfg, err := builder.Build(ctx)
		if err != nil {
			b.Fatalf("failed to build memory reference fixture %d: %v", i, err)
		}
		refs[i] = cfg
	}
	return refs
}

func buildKnowledgeBinding(b *testing.B, ctx context.Context, id string) *knowledge.BindingConfig {
	b.Helper()
	binding, err := knowledge.NewBinding(id).WithTopK(8).WithMinScore(0.4).Build(ctx)
	if err != nil {
		b.Fatalf("failed to build knowledge binding fixture: %v", err)
	}
	return binding
}
