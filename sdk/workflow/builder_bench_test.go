package workflow

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	enginetask "github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/sdk/internal/testutil"
	"github.com/compozy/compozy/sdk/task"
)

func BenchmarkWorkflowBuilderSimple(b *testing.B) {
	profile := testutil.BenchmarkSimple
	agents := buildBenchmarkAgents(b, profile.Agents)
	ctx := testutil.NewBenchmarkContext(b)
	tasks := buildBasicTasks(b, ctx, profile.Tasks, agents)
	testutil.RunBuilderBenchmark(b, func(runCtx context.Context) (any, error) {
		builder := New("workflow-simple")
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, taskCfg := range tasks {
			builder.AddTask(taskCfg)
		}
		return builder.Build(runCtx)
	})
}

func BenchmarkWorkflowBuilderMedium(b *testing.B) {
	profile := testutil.BenchmarkMedium
	agents := buildBenchmarkAgents(b, profile.Agents)
	ctx := testutil.NewBenchmarkContext(b)
	tasks := buildBasicTasks(b, ctx, profile.Tasks, agents)
	testutil.RunBuilderBenchmark(b, func(runCtx context.Context) (any, error) {
		builder := New("workflow-medium")
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, taskCfg := range tasks {
			builder.AddTask(taskCfg)
		}
		return builder.Build(runCtx)
	})
}

func BenchmarkWorkflowBuilderComplex(b *testing.B) {
	profile := testutil.BenchmarkComplex
	agents := buildBenchmarkAgents(b, profile.Agents)
	ctx := testutil.NewBenchmarkContext(b)
	tasks := buildBasicTasks(b, ctx, profile.Tasks, agents)
	testutil.RunBuilderBenchmark(b, func(runCtx context.Context) (any, error) {
		builder := New("workflow-complex")
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, taskCfg := range tasks {
			builder.AddTask(taskCfg)
		}
		return builder.Build(runCtx)
	})
}

func BenchmarkWorkflowBuilderParallel(b *testing.B) {
	profile := testutil.BenchmarkMedium
	agents := buildBenchmarkAgents(b, profile.Agents)
	ctx := testutil.NewBenchmarkContext(b)
	tasks := buildBasicTasks(b, ctx, profile.Tasks, agents)
	testutil.RunParallelBuilderBenchmark(b, func(runCtx context.Context) (any, error) {
		builder := New("workflow-parallel")
		for _, agentCfg := range agents {
			builder.AddAgent(agentCfg)
		}
		for _, taskCfg := range tasks {
			builder.AddTask(taskCfg)
		}
		return builder.Build(runCtx)
	})
}

func buildBenchmarkAgents(b *testing.B, count int) []*agent.Config {
	b.Helper()
	agents := make([]*agent.Config, count)
	for i := 0; i < count; i++ {
		agents[i] = testutil.NewTestAgent(testutil.BenchmarkID("agent", i))
	}
	return agents
}

func buildBasicTasks(b *testing.B, ctx context.Context, count int, agents []*agent.Config) []*enginetask.Config {
	b.Helper()
	tasks := make([]*enginetask.Config, count)
	for i := 0; i < count; i++ {
		agentID := agents[i%len(agents)].ID
		builder := task.NewBasic(testutil.BenchmarkID("task", i)).WithAgent(agentID).WithAction("run")
		cfg, err := builder.Build(ctx)
		if err != nil {
			b.Fatalf("failed to build basic task fixture %d: %v", i, err)
		}
		tasks[i] = cfg
	}
	return tasks
}
